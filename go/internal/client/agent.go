/**
 * Author: Deepankar Das
 */

package client

import (
	"bytes"
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"os/exec"
	"os/user"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/anthropics/enforcer/internal/policy"
)

// ClientAgent handles registration, policy sync, and audit forwarding to a central server.
type ClientAgent struct {
	mu            sync.Mutex
	centralURL    string
	clientID      string
	hostname      string
	governedUsers []string
	certDir       string
	configDir     string
	httpClient    *http.Client
	daemonURL     string
	daemonToken   string
	syncInterval  time.Duration
	stopCh        chan struct{}
	policyHash    string
	auditQueue    []interface{}
	auditQueueMu  sync.Mutex
}

// NewClientAgent creates a new client agent.
func NewClientAgent() *ClientAgent {
	certDir := envOr("CERT_DIR", "/etc/enforcer/certs")
	hostname, _ := os.Hostname()

	agent := &ClientAgent{
		centralURL: envOr("AA_CENTRAL_URL", "https://localhost:9200"),
		clientID:   envOr("AA_CLIENT_ID", fmt.Sprintf("client_%s_%d", hostname, time.Now().Unix())),
		hostname:   hostname,
		governedUsers: []string{func() string {
			if user := os.Getenv("SUDO_USER"); user != "" {
				return user
			}
			if user := os.Getenv("USER"); user != "" {
				return user
			}
			return "unknown"
		}()},
		certDir:      certDir,
		configDir:    envOr("AA_CONFIG_DIR", "/etc/enforcer"),
		daemonURL:    fmt.Sprintf("http://127.0.0.1:%s", envOr("DAEMON_PORT", "9100")),
		daemonToken:  "",
		syncInterval: 5 * time.Second,
		stopCh:       make(chan struct{}),
	}
	agent.daemonToken = agent.resolveDaemonToken()

	if interval := os.Getenv("AA_SYNC_INTERVAL"); interval != "" {
		if d, err := time.ParseDuration(interval); err == nil {
			agent.syncInterval = d
		}
	}

	agent.httpClient = agent.buildHTTPClient()
	return agent
}

func (a *ClientAgent) buildHTTPClient() *http.Client {
	tlsConfig := a.loadClientTLS()
	transport := &http.Transport{}
	if tlsConfig != nil {
		transport.TLSClientConfig = tlsConfig
	} else {
		transport.TLSClientConfig = &tls.Config{InsecureSkipVerify: true}
	}
	return &http.Client{
		Transport: transport,
		Timeout:   10 * time.Second,
	}
}

func (a *ClientAgent) loadClientTLS() *tls.Config {
	certFile := a.certDir + "/client.crt"
	keyFile := a.certDir + "/client.key"
	caFile := a.certDir + "/ca.crt"

	cert, err := tls.LoadX509KeyPair(certFile, keyFile)
	if err != nil {
		slog.Warn("No client TLS cert found", "error", err)
		return nil
	}

	caCert, err := os.ReadFile(caFile)
	if err != nil {
		slog.Warn("No CA cert found", "error", err)
		return nil
	}

	caCertPool := x509.NewCertPool()
	caCertPool.AppendCertsFromPEM(caCert)

	return &tls.Config{
		Certificates: []tls.Certificate{cert},
		RootCAs:      caCertPool,
		MinVersion:   tls.VersionTLS12,
	}
}

func (a *ClientAgent) callCentral(method, path string, body interface{}) (int, json.RawMessage, error) {
	var bodyReader io.Reader
	if body != nil {
		data, err := json.Marshal(body)
		if err != nil {
			return 0, nil, err
		}
		bodyReader = bytes.NewReader(data)
	}

	req, err := http.NewRequest(method, a.centralURL+path, bodyReader)
	if err != nil {
		return 0, nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Client-Id", a.clientID)

	resp, err := a.httpClient.Do(req)
	if err != nil {
		return 0, nil, err
	}
	defer resp.Body.Close()

	respData, _ := io.ReadAll(resp.Body)
	return resp.StatusCode, json.RawMessage(respData), nil
}

// Register registers this client with the central server.
func (a *ClientAgent) Register() error {
	status, _, err := a.callCentral("POST", "/api/v1/register", map[string]interface{}{
		"client_id":      a.clientID,
		"hostname":       a.hostname,
		"governed_users": a.governedUsers,
	})
	if err != nil {
		return fmt.Errorf("registration failed: %w", err)
	}
	if status != 200 {
		return fmt.Errorf("registration returned status %d", status)
	}
	slog.Info("Registered with central server", "client_id", a.clientID)
	return nil
}

// SyncPolicy fetches the latest policy from central and writes to local config.
func (a *ClientAgent) SyncPolicy() error {
	status, data, err := a.callCentral("GET", "/api/v1/policy/pull", nil)
	if err != nil {
		return err
	}
	if status != 200 {
		return fmt.Errorf("policy pull returned status %d", status)
	}

	var resp struct {
		Version            string `json:"version"`
		Hash               string `json:"hash"`
		Bundle             string `json:"bundle"`
		EnforcementEnabled *bool  `json:"enforcement_enabled,omitempty"`
		FilePermissions    *struct {
			Mode  string `json:"mode"`
			Owner string `json:"owner"`
			Group string `json:"group"`
		} `json:"file_permissions,omitempty"`
	}
	if err := json.Unmarshal(data, &resp); err != nil {
		return err
	}

	if resp.Hash == a.policyHash {
		return nil // No changes
	}

	// Never overwrite local baseline policy with an empty/invalid bundle from Hub.
	bundleText := strings.TrimSpace(resp.Bundle)
	if bundleText == "" {
		return fmt.Errorf("policy pull returned empty bundle")
	}
	parsed, err := policy.LoadPolicyFromBytes([]byte(resp.Bundle))
	if err != nil {
		return fmt.Errorf("policy pull returned invalid YAML: %w", err)
	}
	if len(parsed.Rules) == 0 {
		return fmt.Errorf("policy pull returned 0 rules; refusing to overwrite local baseline")
	}

	// Determine file mode from Hub-specified permissions (default: 0644 root-owned, world-readable).
	fileMode := os.FileMode(0644)
	if resp.FilePermissions != nil && resp.FilePermissions.Mode != "" {
		if parsed, err := parseFileMode(resp.FilePermissions.Mode); err == nil {
			fileMode = parsed
		} else {
			slog.Warn("Hub sent invalid file_permissions.mode, using default 0644", "mode", resp.FilePermissions.Mode, "err", err)
		}
	}

	policyPath := a.configDir + "/default.yaml"
	if err := os.WriteFile(policyPath, []byte(resp.Bundle), fileMode); err != nil {
		return fmt.Errorf("writing policy: %w", err)
	}

	// Apply ownership if Hub specified it and we're running as root.
	applyFileOwnership(policyPath, resp.FilePermissions)

	a.policyHash = resp.Hash
	slog.Info("Policy synced", "version", resp.Version, "mode", fmt.Sprintf("%04o", fileMode))
	if resp.EnforcementEnabled != nil {
		_ = a.ApplyLocalEnforcement(*resp.EnforcementEnabled)
	}
	return nil
}

// QueueAuditEvent adds an event to the forwarding queue.
func (a *ClientAgent) QueueAuditEvent(event interface{}) {
	a.auditQueueMu.Lock()
	defer a.auditQueueMu.Unlock()
	a.auditQueue = append(a.auditQueue, event)
}

// FlushAuditEvents forwards queued audit events to the central server.
func (a *ClientAgent) FlushAuditEvents() error {
	a.auditQueueMu.Lock()
	if len(a.auditQueue) == 0 {
		a.auditQueueMu.Unlock()
		return nil
	}
	batch := a.auditQueue
	if len(batch) > 100 {
		batch = batch[:100]
		a.auditQueue = a.auditQueue[100:]
	} else {
		a.auditQueue = nil
	}
	a.auditQueueMu.Unlock()

	_, _, err := a.callCentral("POST", "/api/v1/audit/push", map[string]interface{}{
		"events": batch,
	})
	return err
}

// SendHeartbeat sends a heartbeat to the central server.
func (a *ClientAgent) SendHeartbeat() error {
	status, data, err := a.callCentral("POST", "/api/v1/heartbeat", map[string]interface{}{
		"client_id":      a.clientID,
		"hostname":       a.hostname,
		"governed_users": a.governedUsers,
		"status":         "online",
	})
	if err != nil {
		return err
	}
	if status != 200 {
		return fmt.Errorf("heartbeat returned status %d", status)
	}

	var resp struct {
		PolicyHash         string `json:"policy_hash"`
		EnforcementEnabled *bool  `json:"enforcement_enabled,omitempty"`
	}
	json.Unmarshal(data, &resp)
	if resp.PolicyHash != "" && resp.PolicyHash != a.policyHash {
		slog.Info("Policy drift detected, syncing")
		a.SyncPolicy()
	}
	if resp.EnforcementEnabled != nil {
		_ = a.ApplyLocalEnforcement(*resp.EnforcementEnabled)
	}
	return nil
}

// SyncApprovals pushes pending local approvals to the Hub and pulls back any
// decisions the Hub admin has made.  The Hub is the single source of truth for
// approval decisions — the Sentinel daemon holds the pending queue and the Hub
// resolves it.
func (a *ClientAgent) SyncApprovals() error {
	// Step 1: Fetch pending approvals from the local daemon.
	pending, err := a.fetchLocalPendingApprovals()
	if err != nil {
		return fmt.Errorf("fetching local pending approvals: %w", err)
	}
	if len(pending) == 0 {
		return nil
	}

	// Step 2: Push pending approvals to Hub.
	status, data, err := a.callCentral("POST", "/api/v1/approvals/push", map[string]interface{}{
		"client_id": a.clientID,
		"approvals": pending,
	})
	if err != nil {
		return fmt.Errorf("pushing approvals to hub: %w", err)
	}
	if status != 200 {
		return fmt.Errorf("hub approval push returned status %d", status)
	}

	// Step 3: Parse Hub response — it returns decisions for any approvals
	// that have already been resolved by the admin.
	var resp struct {
		Resolved []struct {
			ApprovalID string `json:"approval_id"`
			Decision   string `json:"decision"` // "approve" or "deny"
			ApproverID string `json:"approver_id"`
			Rationale  string `json:"rationale"`
		} `json:"resolved"`
	}
	if err := json.Unmarshal(data, &resp); err != nil {
		return fmt.Errorf("parsing hub approval response: %w", err)
	}

	// Step 4: Resolve any decisions in the local daemon.
	for _, r := range resp.Resolved {
		if err := a.resolveLocalApproval(r.ApprovalID, r.Decision, r.ApproverID, r.Rationale); err != nil {
			slog.Warn("Failed to resolve local approval", "approval_id", r.ApprovalID, "err", err)
		} else {
			slog.Info("Approval resolved from Hub", "approval_id", r.ApprovalID, "decision", r.Decision)
		}
	}

	return nil
}

// fetchLocalPendingApprovals calls the local daemon to get pending approvals.
func (a *ClientAgent) fetchLocalPendingApprovals() ([]json.RawMessage, error) {
	if strings.TrimSpace(a.daemonToken) == "" {
		return nil, nil
	}
	req, err := http.NewRequest("GET", a.daemonURL+"/v1/approvals/pending", nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+a.daemonToken)
	resp, err := a.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("daemon returned status %d", resp.StatusCode)
	}

	var result struct {
		Approvals []json.RawMessage `json:"approvals"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}
	return result.Approvals, nil
}

// resolveLocalApproval calls the local daemon to resolve a pending approval
// with the Hub admin's decision.  When approved, a single-use scope is
// attached so the developer can retry the same action exactly once.
func (a *ClientAgent) resolveLocalApproval(approvalID, decision, approverID, rationale string) error {
	if strings.TrimSpace(a.daemonToken) == "" {
		return fmt.Errorf("no daemon token available")
	}
	payload := map[string]interface{}{
		"decision":    decision,
		"approver_id": approverID,
		"rationale":   rationale,
	}
	// Attach a single-use scope on approve so the developer's retry is
	// automatically allowed by CheckScope().
	if decision == "approve" {
		payload["scope"] = map[string]string{
			"type": "single",
		}
	}
	body, _ := json.Marshal(payload)
	req, err := http.NewRequest("POST", a.daemonURL+"/v1/approvals/"+approvalID+"/resolve", bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+a.daemonToken)
	resp, err := a.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		raw, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("resolve returned %d: %s", resp.StatusCode, string(raw))
	}
	return nil
}

// Start begins the sync loop.
func (a *ClientAgent) Start() error {
	if err := a.Register(); err != nil {
		slog.Error("Registration failed, will retry", "error", err)
	}
	if err := a.SyncPolicy(); err != nil {
		slog.Error("Initial policy sync failed", "error", err)
	}

	ticker := time.NewTicker(a.syncInterval)
	// Approval sync runs on a faster cadence (every 3 seconds) because the
	// hook handler is blocking while waiting for admin approval.
	approvalTicker := time.NewTicker(3 * time.Second)
	go func() {
		for {
			select {
			case <-approvalTicker.C:
				if err := a.SyncApprovals(); err != nil {
					slog.Debug("Approval sync error", "error", err)
				}
			case <-ticker.C:
				a.SyncPolicy()
				a.SendHeartbeat()
				a.FlushAuditEvents()
			case <-a.stopCh:
				ticker.Stop()
				approvalTicker.Stop()
				return
			}
		}
	}()

	slog.Info("Client agent started", "client_id", a.clientID, "interval", a.syncInterval)
	return nil
}

// Stop halts the sync loop.
func (a *ClientAgent) Stop() {
	close(a.stopCh)
}

func envOr(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func (a *ClientAgent) resolveDaemonToken() string {
	if t := strings.TrimSpace(os.Getenv("AA_ADMIN_TOKEN")); t != "" {
		return t
	}
	if t := strings.TrimSpace(os.Getenv("AA_OPERATOR_TOKEN")); t != "" {
		return t
	}
	candidates := []string{
		filepath.Join(a.configDir, ".admin_token"),
		filepath.Join(a.configDir, ".operator_token"),
		"/etc/enforcer/.admin_token",
		"/etc/enforcer/.operator_token",
	}
	for _, path := range candidates {
		data, err := os.ReadFile(path)
		if err != nil {
			continue
		}
		token := strings.TrimSpace(string(data))
		if token != "" {
			return token
		}
	}
	return ""
}

// ApplyLocalEnforcement aligns local Sentinel daemon enforcement with Hub state.
func (a *ClientAgent) ApplyLocalEnforcement(enabled bool) error {
	if strings.TrimSpace(a.daemonToken) == "" {
		return nil
	}
	req, err := http.NewRequest("GET", a.daemonURL+"/v1/enforcement", nil)
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+a.daemonToken)
	req.Header.Set("X-AA-Token", a.daemonToken)
	resp, err := a.httpClient.Do(req)
	if err != nil {
		return err
	}
	var current struct {
		Enabled bool `json:"enabled"`
	}
	if resp.Body != nil {
		body, _ := io.ReadAll(resp.Body)
		_ = resp.Body.Close()
		_ = json.Unmarshal(body, &current)
	}
	if current.Enabled == enabled {
		return nil
	}

	payload := map[string]interface{}{
		"enabled":    enabled,
		"changed_by": "hub_policy_sync",
	}
	data, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	req, err = http.NewRequest("POST", a.daemonURL+"/v1/enforcement/toggle", bytes.NewReader(data))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+a.daemonToken)
	req.Header.Set("X-AA-Token", a.daemonToken)
	resp, err = a.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		return fmt.Errorf("local enforcement toggle failed with status %d", resp.StatusCode)
	}
	slog.Info("Applied Hub enforcement state to Sentinel", "enabled", enabled)
	return nil
}

// parseFileMode converts a string like "0644" to os.FileMode.
func parseFileMode(s string) (os.FileMode, error) {
	v, err := strconv.ParseUint(s, 8, 32)
	if err != nil {
		return 0, fmt.Errorf("invalid file mode %q: %w", s, err)
	}
	return os.FileMode(v), nil
}

// applyFileOwnership sets file owner/group using chown.  Only effective when
// running as root (Sentinel daemon runs as root via LaunchDaemon).  If perms
// is nil or owner is empty, defaults to root:wheel.
func applyFileOwnership(path string, perms *struct {
	Mode  string `json:"mode"`
	Owner string `json:"owner"`
	Group string `json:"group"`
}) {
	// Only attempt ownership change when running as root.
	cur, err := user.Current()
	if err != nil || cur.Uid != "0" {
		return
	}

	owner := "root"
	group := "wheel"
	if perms != nil {
		if perms.Owner != "" {
			owner = perms.Owner
		}
		if perms.Group != "" {
			group = perms.Group
		}
	}

	if err := exec.Command("chown", owner+":"+group, path).Run(); err != nil {
		slog.Warn("Failed to set file ownership", "path", path, "owner", owner, "group", group, "err", err)
	}
}
