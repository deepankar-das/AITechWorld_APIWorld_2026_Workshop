/**
 * Author: Deepankar Das
 */

package central

import (
	"context"
	"crypto/sha256"
	"crypto/tls"
	"crypto/x509"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/anthropics/enforcer/internal/audit"
	"github.com/anthropics/enforcer/internal/console"
	"github.com/anthropics/enforcer/internal/policy"
	"github.com/anthropics/enforcer/internal/types"
)

// RegisteredClient tracks a connected Sentinel agent.
type RegisteredClient struct {
	ClientID      string    `json:"client_id"`
	Hostname      string    `json:"hostname"`
	GovernedUsers []string  `json:"governed_users"`
	RegisteredAt  time.Time `json:"registered_at"`
	LastHeartbeat time.Time `json:"last_heartbeat"`
	PolicyVersion string    `json:"policy_version"`
	Status        string    `json:"status"` // "online", "stale", "offline"
}

// HubApprovalRequest represents an approval request forwarded by a Sentinel.
type HubApprovalRequest struct {
	ApprovalID string          `json:"approval_id"`
	ClientID   string          `json:"client_id"`
	Request    json.RawMessage `json:"request"` // Full approval request from Sentinel
	ReceivedAt time.Time       `json:"received_at"`
	ResolvedAt *time.Time      `json:"resolved_at,omitempty"`
	Decision   string          `json:"decision,omitempty"` // "", "approve", "deny"
	ApproverID string          `json:"approver_id,omitempty"`
	Rationale  string          `json:"rationale,omitempty"`
}

// CentralServer is the Enforcer Management Hub.
// It distributes policies, aggregates audit events from all Sentinels,
// and serves the Hub Console for security admins.
// Runtime state is persisted in PostgreSQL — no in-memory policy/client registry.
type CentralServer struct {
	auditStore  audit.AuditStore
	stateStore  stateStore
	authConfig  authConfig
	certDir     string
	clientPort  int
	adminPort   int
	approvalsMu sync.Mutex
	approvals   map[string]*HubApprovalRequest // keyed by approval_id
}

// NewCentralServer creates a Management Hub instance.
// Requires PostgreSQL for audit aggregation and hub state persistence.
func NewCentralServer(certDir string, clientPort, adminPort int) (*CentralServer, error) {
	store, err := audit.NewPostgresStore()
	if err != nil {
		return nil, fmt.Errorf("management Hub requires PostgreSQL for audit aggregation: %w", err)
	}

	hubStateStore, err := newPostgresStateStore()
	if err != nil {
		return nil, fmt.Errorf("management Hub requires PostgreSQL for policy/client state: %w", err)
	}

	slog.Info("Management Hub stores connected (PostgreSQL)")

	s := &CentralServer{
		auditStore: store,
		stateStore: hubStateStore,
		authConfig: loadAuthConfig(),
		certDir:    certDir,
		clientPort: clientPort,
		adminPort:  adminPort,
		approvals:  make(map[string]*HubApprovalRequest),
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if _, err := s.stateStore.GetLatestEnforcementState(ctx); errors.Is(err, errEnforcementStateNotFound) {
		_ = s.stateStore.PutEnforcementState(ctx, enforcementState{
			Enabled:   true,
			ChangedBy: "system:init",
			ChangedAt: time.Now().UTC(),
		})
	}

	return s, nil
}

// LoadPolicy validates and persists the policy file as the current hub policy revision.
func (s *CentralServer) LoadPolicy(path string) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	parsed, err := policy.LoadPolicyFromBytes(data)
	if err != nil {
		return fmt.Errorf("invalid policy YAML: %w", err)
	}
	if len(parsed.Rules) == 0 {
		return fmt.Errorf("policy bundle has 0 rules")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	return s.persistPolicy(ctx, string(data), "")
}

func (s *CentralServer) persistPolicy(ctx context.Context, bundle string, version string) error {
	if strings.TrimSpace(version) == "" {
		version = time.Now().UTC().Format("v2006.01.02.1")
	}
	hash := hashPolicyBundle(bundle)
	return s.stateStore.PutPolicyRevision(ctx, policyState{
		Version: version,
		Hash:    hash,
		Bundle:  bundle,
	})
}

func hashPolicyBundle(bundle string) string {
	sum := sha256.Sum256([]byte(bundle))
	return hex.EncodeToString(sum[:])
}

// Start launches both the mTLS Sentinel API and the admin Hub Console API.
func (s *CentralServer) Start() error {
	clientMux := http.NewServeMux()
	clientMux.HandleFunc("POST /api/v1/register", s.handleRegister)
	clientMux.HandleFunc("GET /api/v1/policy/pull", s.handlePolicyPull)
	clientMux.HandleFunc("POST /api/v1/audit/push", s.handleAuditPush)
	clientMux.HandleFunc("POST /api/v1/heartbeat", s.handleHeartbeat)
	clientMux.HandleFunc("POST /api/v1/approvals/push", s.handleApprovalPush)

	consoleHandler := console.HubHandler()

	adminHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization, X-Admin-Token, X-AA-Token")
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}

		path := r.URL.Path

		switch {
		case path == "/api/v1/auth/me" && r.Method == http.MethodGet:
			if !requireRole(w, r, s.authConfig, roleReviewer) {
				return
			}
			role := authenticateRole(r, s.authConfig)
			writeJSON(w, 200, map[string]interface{}{
				"role":         role,
				"capabilities": roleCapabilities(role),
			})
		// ── Health (no auth — needed for load balancer probes) ──
		case path == "/api/v1/health" && r.Method == http.MethodGet:
			s.handleAdminHealth(w, r)
		// ── Enforcement (read: operator+, write: admin) ──
		case path == "/api/v1/enforcement" && r.Method == http.MethodGet:
			if !requireRole(w, r, s.authConfig, roleReviewer) { return }
			s.handleAdminGetEnforcement(w, r)
		case path == "/api/v1/enforcement/toggle" && r.Method == http.MethodPost:
			if !requireRole(w, r, s.authConfig, roleAdmin) { return }
			s.handleAdminSetEnforcement(w, r)
		// ── Clients (operator+) ──
		case path == "/api/v1/clients" && r.Method == http.MethodGet:
			if !requireRole(w, r, s.authConfig, roleReviewer) { return }
			s.handleAdminClients(w, r)
		// ── Audit (operator+) ──
		case path == "/api/v1/audit" && r.Method == http.MethodGet:
			if !requireRole(w, r, s.authConfig, roleReviewer) { return }
			s.handleAdminAudit(w, r)
		case path == "/api/v1/audit/events" && r.Method == http.MethodGet:
			if !requireRole(w, r, s.authConfig, roleReviewer) { return }
			s.handleAdminAuditEvents(w, r)
		case path == "/api/v1/audit/sessions" && r.Method == http.MethodGet:
			if !requireRole(w, r, s.authConfig, roleReviewer) { return }
			s.handleAdminSessions(w, r)
		case strings.HasPrefix(path, "/api/v1/audit/sessions/") && r.Method == http.MethodGet:
			if !requireRole(w, r, s.authConfig, roleReviewer) { return }
			s.handleAdminSessionDetail(w, r)
		case path == "/api/v1/audit/metrics" && r.Method == http.MethodGet:
			if !requireRole(w, r, s.authConfig, roleReviewer) { return }
			s.handleAdminAuditMetrics(w, r)
		case path == "/api/v1/audit/export" && r.Method == http.MethodGet:
			if !requireRole(w, r, s.authConfig, roleReviewer) { return }
			s.handleAdminExport(w, r)
		// ── Approvals (read: operator+, resolve: reviewer+) ──
		case (path == "/api/v1/approvals" || path == "/api/v1/approvals/pending") && r.Method == http.MethodGet:
			if !requireRole(w, r, s.authConfig, roleReviewer) { return }
			s.handleAdminGetApprovals(w, r)
		case path == "/api/v1/approvals/metrics" && r.Method == http.MethodGet:
			if !requireRole(w, r, s.authConfig, roleReviewer) { return }
			s.handleAdminApprovalMetrics(w, r)
		case strings.HasPrefix(path, "/api/v1/approvals/") && strings.HasSuffix(path, "/resolve") && r.Method == http.MethodPost:
			if !requireRole(w, r, s.authConfig, roleReviewer) { return }
			s.handleAdminResolveApproval(w, r)
		// ── Policy (read: operator+, write: admin) ──
		case path == "/api/v1/policy" && r.Method == http.MethodGet:
			if !requireRole(w, r, s.authConfig, roleReviewer) { return }
			s.handleAdminGetPolicy(w, r)
		case path == "/api/v1/policy" && r.Method == http.MethodPut:
			if !requireRole(w, r, s.authConfig, roleAdmin) { return }
			s.handleAdminSetPolicy(w, r)
		case path == "/api/v1/policy/rules" && r.Method == http.MethodGet:
			if !requireRole(w, r, s.authConfig, roleReviewer) { return }
			s.handleAdminPolicyRules(w, r)
		case path == "/api/v1/policy/rules" && r.Method == http.MethodPost:
			if !requireRole(w, r, s.authConfig, roleAdmin) { return }
			s.handleAdminAddRule(w, r)
		case strings.HasPrefix(path, "/api/v1/policy/rules/") && strings.HasSuffix(path, "/toggle") && r.Method == http.MethodPost:
			if !requireRole(w, r, s.authConfig, roleAdmin) { return }
			s.handleAdminToggleRule(w, r)
		case strings.HasPrefix(path, "/api/v1/policy/rules/") && r.Method == http.MethodDelete:
			if !requireRole(w, r, s.authConfig, roleAdmin) { return }
			s.handleAdminDeleteRule(w, r)
		case path == "/api/v1/policy/packs" && r.Method == http.MethodGet:
			if !requireRole(w, r, s.authConfig, roleReviewer) { return }
			s.handleAdminPolicyPacks(w, r)
		case strings.HasPrefix(path, "/api/v1/policy/packs/") && strings.HasSuffix(path, "/apply") && r.Method == http.MethodPost:
			if !requireRole(w, r, s.authConfig, roleAdmin) { return }
			s.handleAdminApplyPack(w, r)
		case strings.HasPrefix(path, "/api/v1/policy/packs/") && r.Method == http.MethodGet:
			if !requireRole(w, r, s.authConfig, roleReviewer) { return }
			s.handleAdminPolicyPackDetail(w, r)
		// ── Analytics (read: operator+, apply recommendation: admin) ──
		case path == "/api/v1/analytics/blocked-operations" && r.Method == http.MethodGet:
			if !requireRole(w, r, s.authConfig, roleReviewer) { return }
			s.handleAdminBlockedOps(w, r)
		case path == "/api/v1/analytics/approval-bottlenecks" && r.Method == http.MethodGet:
			if !requireRole(w, r, s.authConfig, roleReviewer) { return }
			s.handleAdminApprovalBottlenecks(w, r)
		case path == "/api/v1/analytics/developer-impact" && r.Method == http.MethodGet:
			if !requireRole(w, r, s.authConfig, roleReviewer) { return }
			s.handleAdminDeveloperImpact(w, r)
		case path == "/api/v1/analytics/groups" && r.Method == http.MethodGet:
			if !requireRole(w, r, s.authConfig, roleReviewer) { return }
			s.handleAdminGroups(w, r)
		case strings.HasPrefix(path, "/api/v1/analytics/groups/") && strings.HasSuffix(path, "/members") && r.Method == http.MethodGet:
			if !requireRole(w, r, s.authConfig, roleReviewer) { return }
			s.handleAdminGroupMembers(w, r)
		case path == "/api/v1/analytics/recommendations" && r.Method == http.MethodGet:
			if !requireRole(w, r, s.authConfig, roleReviewer) { return }
			s.handleAdminRecommendations(w, r)
		case strings.HasPrefix(path, "/api/v1/analytics/recommendations/") && strings.HasSuffix(path, "/apply") && r.Method == http.MethodPost:
			if !requireRole(w, r, s.authConfig, roleAdmin) { return }
			s.handleAdminApplyRecommendation(w, r)
		case strings.HasPrefix(path, "/api/v1/analytics/developer/") && r.Method == http.MethodGet:
			if !requireRole(w, r, s.authConfig, roleReviewer) { return }
			s.handleAdminDeveloperAnalytics(w, r)
		// ── Metrics (operator+) ──
		case path == "/api/v1/metrics" && r.Method == http.MethodGet:
			if !requireRole(w, r, s.authConfig, roleReviewer) { return }
			s.handleAdminMetrics(w, r)

		default:
			if !strings.HasPrefix(path, "/api/") {
				consoleHandler.ServeHTTP(w, r)
				return
			}
			writeJSON(w, 404, map[string]string{"error": "not_found", "path": path})
		}
	})

	adminAddr := fmt.Sprintf(":%d", s.adminPort)
	go func() {
		slog.Info("Hub Console starting", "port", s.adminPort, "ui", "embedded")
		if err := http.ListenAndServe(adminAddr, adminHandler); err != nil {
			slog.Error("Hub Console error", "error", err)
		}
	}()

	clientAddr := fmt.Sprintf(":%d", s.clientPort)
	tlsConfig := s.loadTLSConfig()
	clientServer := &http.Server{
		Addr:      clientAddr,
		Handler:   clientMux,
		TLSConfig: tlsConfig,
	}

	slog.Info("Management Hub Sentinel API starting", "port", s.clientPort, "tls", tlsConfig != nil)
	if tlsConfig != nil {
		certFile := s.certDir + "/server.crt"
		keyFile := s.certDir + "/server.key"
		return clientServer.ListenAndServeTLS(certFile, keyFile)
	}
	return clientServer.ListenAndServe()
}

func (s *CentralServer) loadTLSConfig() *tls.Config {
	caFile := s.certDir + "/ca.crt"
	caCert, err := os.ReadFile(caFile)
	if err != nil {
		slog.Warn("No CA cert found, running without mTLS", "path", caFile)
		return nil
	}

	caCertPool := x509.NewCertPool()
	caCertPool.AppendCertsFromPEM(caCert)

	return &tls.Config{
		ClientCAs:  caCertPool,
		ClientAuth: tls.RequireAndVerifyClientCert,
		MinVersion: tls.VersionTLS12,
	}
}

// ── Sentinel API handlers ──────────────────────────────────────────────────

func (s *CentralServer) handleRegister(w http.ResponseWriter, r *http.Request) {
	var body struct {
		ClientID      string   `json:"client_id"`
		Hostname      string   `json:"hostname"`
		GovernedUsers []string `json:"governed_users"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeJSON(w, 400, map[string]string{"error": "Invalid JSON"})
		return
	}
	if strings.TrimSpace(body.ClientID) == "" {
		writeJSON(w, 400, map[string]string{"error": "client_id is required"})
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	policyState, err := s.stateStore.GetLatestPolicy(ctx)
	if err != nil && !errors.Is(err, errPolicyNotFound) {
		writeJSON(w, 500, map[string]string{"error": "policy_lookup_failed", "message": err.Error()})
		return
	}

	now := time.Now().UTC()
	snap := clientSnapshot{
		ClientID:      body.ClientID,
		Hostname:      body.Hostname,
		GovernedUsers: body.GovernedUsers,
		RegisteredAt:  now,
		LastHeartbeat: now,
		Status:        "online",
	}
	if policyState != nil {
		snap.PolicyVersion = policyState.Version
	}

	if err := s.stateStore.RecordClientSnapshot(ctx, snap); err != nil {
		writeJSON(w, 500, map[string]string{"error": "client_register_failed", "message": err.Error()})
		return
	}

	slog.Info("Sentinel registered", "client_id", body.ClientID, "hostname", body.Hostname)
	writeJSON(w, 200, map[string]string{"status": "registered"})
}

func (s *CentralServer) handlePolicyPull(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	state, err := s.stateStore.GetLatestPolicy(ctx)
	if err != nil {
		if errors.Is(err, errPolicyNotFound) {
			writeJSON(w, 503, map[string]string{"error": "policy_unavailable", "message": "No policy loaded in Management Hub"})
			return
		}
		writeJSON(w, 500, map[string]string{"error": "policy_lookup_failed", "message": err.Error()})
		return
	}

	enf, err := s.getEffectiveEnforcementState(ctx)
	if err != nil {
		writeJSON(w, 500, map[string]string{"error": "enforcement_lookup_failed", "message": err.Error()})
		return
	}

	writeJSON(w, 200, map[string]interface{}{
		"version":             state.Version,
		"hash":                state.Hash,
		"bundle":              state.Bundle,
		"enforcement_enabled": enf.Enabled,
		"file_permissions": map[string]interface{}{
			"mode":  "0644",
			"owner": "root",
			"group": "wheel",
		},
	})
}

func (s *CentralServer) handleAuditPush(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Events []json.RawMessage `json:"events"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeJSON(w, 400, map[string]string{"error": "Invalid JSON"})
		return
	}

	stored := 0
	for _, raw := range body.Events {
		var event types.AuditEvent
		if err := json.Unmarshal(raw, &event); err != nil {
			slog.Warn("Skipping malformed audit event from Sentinel", "error", err)
			continue
		}
		if err := s.auditStore.StoreEvent(event); err != nil {
			slog.Warn("Failed to store forwarded audit event", "error", err)
			continue
		}
		stored++
	}

	writeJSON(w, 200, map[string]interface{}{"accepted": stored, "total": len(body.Events)})
}

func (s *CentralServer) handleHeartbeat(w http.ResponseWriter, r *http.Request) {
	var body struct {
		ClientID      string   `json:"client_id"`
		Hostname      string   `json:"hostname"`
		GovernedUsers []string `json:"governed_users"`
		Status        string   `json:"status"`
		PolicyVersion string   `json:"policy_version"`
	}
	data, _ := io.ReadAll(r.Body)
	_ = json.Unmarshal(data, &body)

	if strings.TrimSpace(body.ClientID) == "" {
		body.ClientID = strings.TrimSpace(r.Header.Get("X-Client-Id"))
	}
	if body.ClientID == "" {
		writeJSON(w, 400, map[string]string{"error": "client_id is required"})
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	latestPolicy, err := s.stateStore.GetLatestPolicy(ctx)
	if err != nil && !errors.Is(err, errPolicyNotFound) {
		writeJSON(w, 500, map[string]string{"error": "policy_lookup_failed", "message": err.Error()})
		return
	}
	enf, err := s.getEffectiveEnforcementState(ctx)
	if err != nil {
		writeJSON(w, 500, map[string]string{"error": "enforcement_lookup_failed", "message": err.Error()})
		return
	}

	now := time.Now().UTC()
	snap := clientSnapshot{
		ClientID:      body.ClientID,
		Hostname:      body.Hostname,
		GovernedUsers: body.GovernedUsers,
		LastHeartbeat: now,
		Status:        "online",
		PolicyVersion: body.PolicyVersion,
	}
	if body.Status != "" {
		snap.Status = body.Status
	}
	if snap.PolicyVersion == "" && latestPolicy != nil {
		snap.PolicyVersion = latestPolicy.Version
	}
	if err := s.stateStore.RecordClientSnapshot(ctx, snap); err != nil {
		writeJSON(w, 500, map[string]string{"error": "heartbeat_persist_failed", "message": err.Error()})
		return
	}

	resp := map[string]interface{}{"enforcement_enabled": enf.Enabled}
	if latestPolicy != nil {
		resp["policy_version"] = latestPolicy.Version
		resp["policy_hash"] = latestPolicy.Hash
	} else {
		resp["policy_version"] = ""
		resp["policy_hash"] = ""
	}
	writeJSON(w, 200, resp)
}

// ── Hub Console API handlers ───────────────────────────────────────────────

func (s *CentralServer) handleAdminHealth(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	policyVersion := ""
	if p, err := s.stateStore.GetLatestPolicy(ctx); err == nil {
		policyVersion = p.Version
	}
	enf, err := s.getEffectiveEnforcementState(ctx)
	if err != nil {
		writeJSON(w, 500, map[string]string{"error": "enforcement_lookup_failed", "message": err.Error()})
		return
	}
	clientCount, err := s.stateStore.CountClients(ctx)
	if err != nil {
		writeJSON(w, 500, map[string]string{"error": "client_count_failed", "message": err.Error()})
		return
	}
	count := s.auditStore.GetCount()
	writeJSON(w, 200, map[string]interface{}{
		"status":         "ok",
		"clients":        clientCount,
		"policy_version": policyVersion,
		"audit_events":   count,
		"enforcement": map[string]interface{}{
			"enabled":    enf.Enabled,
			"since":      enf.ChangedAt.Format(time.RFC3339),
			"changed_by": enf.ChangedBy,
		},
	})
}

func (s *CentralServer) handleAdminGetEnforcement(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	enf, err := s.getEffectiveEnforcementState(ctx)
	if err != nil {
		writeJSON(w, 500, map[string]string{"error": "enforcement_lookup_failed", "message": err.Error()})
		return
	}
	writeJSON(w, 200, map[string]interface{}{
		"enabled":    enf.Enabled,
		"since":      enf.ChangedAt.Format(time.RFC3339),
		"changed_by": enf.ChangedBy,
	})
}

func (s *CentralServer) handleAdminSetEnforcement(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Enabled   bool   `json:"enabled"`
		ChangedBy string `json:"changed_by"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeJSON(w, 400, map[string]string{"error": "invalid_json", "message": err.Error()})
		return
	}
	if strings.TrimSpace(body.ChangedBy) == "" {
		body.ChangedBy = "hub_console"
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	err := s.stateStore.PutEnforcementState(ctx, enforcementState{
		Enabled:   body.Enabled,
		ChangedBy: body.ChangedBy,
		ChangedAt: time.Now().UTC(),
	})
	if err != nil {
		writeJSON(w, 500, map[string]string{"error": "enforcement_persist_failed", "message": err.Error()})
		return
	}
	enf, err := s.getEffectiveEnforcementState(ctx)
	if err != nil {
		writeJSON(w, 500, map[string]string{"error": "enforcement_lookup_failed", "message": err.Error()})
		return
	}
	writeJSON(w, 200, map[string]interface{}{
		"enabled":    enf.Enabled,
		"since":      enf.ChangedAt.Format(time.RFC3339),
		"changed_by": enf.ChangedBy,
	})
}

func (s *CentralServer) getEffectiveEnforcementState(ctx context.Context) (*enforcementState, error) {
	enf, err := s.stateStore.GetLatestEnforcementState(ctx)
	if errors.Is(err, errEnforcementStateNotFound) {
		return &enforcementState{
			Enabled:   true,
			ChangedBy: "system:default",
			ChangedAt: time.Now().UTC(),
		}, nil
	}
	return enf, err
}

func (s *CentralServer) handleAdminClients(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	clients, err := s.stateStore.ListLatestClients(ctx)
	if err != nil {
		writeJSON(w, 500, map[string]string{"error": "client_list_failed", "message": err.Error()})
		return
	}

	now := time.Now().UTC()
	for _, c := range clients {
		c.Status = "online"
		if now.Sub(c.LastHeartbeat) > 120*time.Second {
			c.Status = "stale"
		}
		if now.Sub(c.LastHeartbeat) > 300*time.Second {
			c.Status = "offline"
		}
	}
	writeJSON(w, 200, map[string]interface{}{"clients": clients, "count": len(clients)})
}

func (s *CentralServer) handleAdminGetPolicy(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	state, err := s.stateStore.GetLatestPolicy(ctx)
	if err != nil {
		if errors.Is(err, errPolicyNotFound) {
			writeJSON(w, 200, map[string]interface{}{"version": "", "bundle": ""})
			return
		}
		writeJSON(w, 500, map[string]string{"error": "policy_lookup_failed", "message": err.Error()})
		return
	}

	writeJSON(w, 200, map[string]interface{}{
		"version": state.Version,
		"bundle":  state.Bundle,
	})
}

func (s *CentralServer) handleAdminSetPolicy(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Bundle  string `json:"bundle"`
		Version string `json:"version"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeJSON(w, 400, map[string]string{"error": "Invalid JSON"})
		return
	}
	parsed, err := policy.LoadPolicyFromBytes([]byte(body.Bundle))
	if err != nil {
		writeJSON(w, 400, map[string]string{"error": "invalid_policy_yaml", "message": err.Error()})
		return
	}
	if len(parsed.Rules) == 0 {
		writeJSON(w, 400, map[string]string{"error": "empty_policy_bundle", "message": "Policy bundle must contain at least one rule"})
		return
	}
	if strings.TrimSpace(body.Version) == "" {
		body.Version = time.Now().UTC().Format("v2006.01.02.1")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := s.persistPolicy(ctx, body.Bundle, body.Version); err != nil {
		writeJSON(w, 500, map[string]string{"error": "policy_persist_failed", "message": err.Error()})
		return
	}
	writeJSON(w, 200, map[string]string{"status": "updated"})
}

func (s *CentralServer) handleAdminAudit(w http.ResponseWriter, r *http.Request) {
	query := audit.AuditQuery{Limit: 100}
	if sid := r.URL.Query().Get("session_id"); sid != "" {
		query.SessionID = sid
	}
	if uid := r.URL.Query().Get("user_id"); uid != "" {
		query.ActorUserID = uid
	}
	if dec := r.URL.Query().Get("decision"); dec != "" {
		query.Decision = dec
	}

	events, err := s.auditStore.QueryEvents(query)
	if err != nil {
		writeJSON(w, 500, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, 200, map[string]interface{}{
		"events": events,
		"count":  len(events),
		"total":  s.auditStore.GetCount(),
	})
}

func (s *CentralServer) handleAdminSessions(w http.ResponseWriter, r *http.Request) {
	sessions, err := s.auditStore.GetSessions()
	if err != nil {
		writeJSON(w, 500, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, 200, map[string]interface{}{
		"sessions": sessions,
		"count":    len(sessions),
	})
}

func (s *CentralServer) handleAdminExport(w http.ResponseWriter, r *http.Request) {
	query := audit.AuditQuery{Limit: 10000}
	if sid := r.URL.Query().Get("session_id"); sid != "" {
		query.SessionID = sid
	}
	result, err := s.auditStore.ExportEvents(query)
	if err != nil {
		writeJSON(w, 500, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, 200, result)
}

// ── Approval handlers ──────────────────────────────────────────────────

// handleApprovalPush receives pending approvals from a Sentinel and returns
// any decisions the Hub admin has already made.  This is called by the
// Sentinel client agent on its fast-cadence approval sync loop.
func (s *CentralServer) handleApprovalPush(w http.ResponseWriter, r *http.Request) {
	var body struct {
		ClientID  string            `json:"client_id"`
		Approvals []json.RawMessage `json:"approvals"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeJSON(w, 400, map[string]string{"error": "invalid_json"})
		return
	}

	s.approvalsMu.Lock()

	// Ingest any new approvals from the Sentinel.
	for _, raw := range body.Approvals {
		var stub struct {
			ApprovalID string `json:"approval_id"`
		}
		if err := json.Unmarshal(raw, &stub); err != nil || stub.ApprovalID == "" {
			continue
		}
		if _, exists := s.approvals[stub.ApprovalID]; !exists {
			s.approvals[stub.ApprovalID] = &HubApprovalRequest{
				ApprovalID: stub.ApprovalID,
				ClientID:   body.ClientID,
				Request:    raw,
				ReceivedAt: time.Now().UTC(),
			}
			slog.Info("Approval received from Sentinel", "approval_id", stub.ApprovalID, "client_id", body.ClientID)
		}
	}

	// Collect resolved approvals to send back to this Sentinel.
	var resolved []map[string]string
	for _, apr := range s.approvals {
		if apr.ClientID == body.ClientID && apr.Decision != "" {
			resolved = append(resolved, map[string]string{
				"approval_id": apr.ApprovalID,
				"decision":    apr.Decision,
				"approver_id": apr.ApproverID,
				"rationale":   apr.Rationale,
			})
		}
	}

	s.approvalsMu.Unlock()

	writeJSON(w, 200, map[string]interface{}{
		"resolved": resolved,
	})
}

// handleAdminGetApprovals returns all approval requests for the Hub Console.
// The response includes the unwrapped approval request so the console can
// display actor, resource, risk rationale, and policy rule.
func (s *CentralServer) handleAdminGetApprovals(w http.ResponseWriter, r *http.Request) {
	s.approvalsMu.Lock()
	defer s.approvalsMu.Unlock()

	// Unwrap raw requests into the console-compatible shape.
	pendingList := make([]json.RawMessage, 0)
	resolvedList := make([]json.RawMessage, 0)
	for _, apr := range s.approvals {
		if apr.Decision == "" {
			pendingList = append(pendingList, apr.Request)
		} else {
			resolvedList = append(resolvedList, apr.Request)
		}
	}

	// Return both canonical keys and console-compatible keys (approvals/count).
	writeJSON(w, 200, map[string]interface{}{
		"approvals":      pendingList,
		"count":          len(pendingList),
		"pending":        pendingList,
		"resolved":       resolvedList,
		"pending_count":  len(pendingList),
		"resolved_count": len(resolvedList),
	})
}

// handleAdminResolveApproval lets the Hub admin approve or deny an approval.
func (s *CentralServer) handleAdminResolveApproval(w http.ResponseWriter, r *http.Request) {
	path := r.URL.Path
	// Extract approval ID from /api/v1/approvals/{id}/resolve
	trimmed := strings.TrimPrefix(path, "/api/v1/approvals/")
	approvalID := strings.TrimSuffix(trimmed, "/resolve")
	if approvalID == "" {
		writeJSON(w, 400, map[string]string{"error": "missing_approval_id"})
		return
	}

	var body struct {
		Decision   string `json:"decision"` // "approve" or "deny"
		ApproverID string `json:"approver_id"`
		Rationale  string `json:"rationale"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeJSON(w, 400, map[string]string{"error": "invalid_json"})
		return
	}
	if body.Decision != "approve" && body.Decision != "deny" {
		writeJSON(w, 400, map[string]string{"error": "decision must be 'approve' or 'deny'"})
		return
	}

	s.approvalsMu.Lock()
	apr, exists := s.approvals[approvalID]
	if !exists {
		s.approvalsMu.Unlock()
		writeJSON(w, 404, map[string]string{"error": "approval_not_found"})
		return
	}
	if apr.Decision != "" {
		s.approvalsMu.Unlock()
		writeJSON(w, 409, map[string]string{"error": "already_resolved", "decision": apr.Decision})
		return
	}

	now := time.Now().UTC()
	apr.Decision = body.Decision
	apr.ApproverID = body.ApproverID
	apr.Rationale = body.Rationale
	apr.ResolvedAt = &now
	s.approvalsMu.Unlock()

	slog.Info("Approval resolved by admin", "approval_id", approvalID, "decision", body.Decision, "approver", body.ApproverID)
	writeJSON(w, 200, map[string]interface{}{
		"approval_id": approvalID,
		"decision":    body.Decision,
		"resolved":    true,
	})
}

func writeJSON(w http.ResponseWriter, status int, body interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(body)
}
