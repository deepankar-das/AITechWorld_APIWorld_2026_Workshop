/**
 * Author: Deepankar Das
 */

package daemon

import (
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"os/exec"
	"os/user"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/anthropics/enforcer/internal/approval"
	"github.com/anthropics/enforcer/internal/audit"
	"github.com/anthropics/enforcer/internal/console"
	"github.com/anthropics/enforcer/internal/daemon/routes"
	"github.com/anthropics/enforcer/internal/policy"
	"github.com/anthropics/enforcer/internal/types"
	"gopkg.in/yaml.v3"
)

const allowLocalPolicyEditsEnv = "AA_ALLOW_LOCAL_POLICY_EDITS"

// writeJSON writes a JSON response with the given status code and CORS
// headers.
func writeJSON(w http.ResponseWriter, status int, body interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
	w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization, X-Admin-Token, X-AA-Token")
	w.WriteHeader(status)
	if body != nil {
		_ = json.NewEncoder(w).Encode(body)
	}
}

// readBody reads the entire request body.
func readBody(r *http.Request) ([]byte, error) {
	defer r.Body.Close()
	return io.ReadAll(r.Body)
}

// extractPathParam extracts the trailing path segment after the given prefix.
// For example, extractPathParam("/v1/approvals/abc123/resolve", "/v1/approvals/")
// returns "abc123".
func extractPathParam(path, prefix string) string {
	rest := strings.TrimPrefix(path, prefix)
	// Remove trailing slashes and any further path segments.
	parts := strings.SplitN(rest, "/", 2)
	if len(parts) > 0 {
		return parts[0]
	}
	return ""
}

// extractPathRemainder returns the full path tail after a prefix.
func extractPathRemainder(path, prefix string) string {
	return strings.TrimPrefix(path, prefix)
}

func localPolicyEditsEnabled() bool {
	v := strings.TrimSpace(strings.ToLower(os.Getenv(allowLocalPolicyEditsEnv)))
	switch v {
	case "1", "true", "yes", "on":
		return true
	default:
		return false
	}
}

func sendLocalPolicyEditDisabled(w http.ResponseWriter) {
	writeJSON(w, http.StatusForbidden, map[string]interface{}{
		"error":               "local_policy_edit_disabled",
		"message":             "Local Sentinel policy editing is disabled in managed mode. Policy changes must be made in the Management Hub.",
		"phase2_note":         "Standalone Sentinel policy authoring is planned for Phase 2.",
		"managed_mode":        true,
		"allow_local_edits":   false,
		"override_env_var":    allowLocalPolicyEditsEnv,
		"override_env_status": "unset_or_false",
	})
}

// StartDaemon initialises all subsystems and starts the HTTP server.
func StartDaemon() error {
	strictMode := IsStrictMode()
	if strictMode {
		fmt.Fprintf(os.Stderr, "[Enforcer] Strict mode: ENABLED\n")
	} else {
		fmt.Fprintf(os.Stderr, "[Enforcer] Strict mode: DISABLED\n")
	}

	// --- Load role-based auth configuration ---
	authConfig := LoadAuthConfig()

	// --- Load policy bundle (with hierarchy merge if multiple sources exist) ---
	bundle := loadPolicyBundle()
	// Apply hierarchy merge — currently single-bundle, but ready for org/team/repo layers
	bundle = policy.MergeHierarchy(&bundle)
	policyPersistPath := resolvePolicyPersistPath()
	customPacksPath := filepath.Join(filepath.Dir(policyPersistPath), "custom-packs.json")
	if err := loadCustomPacks(customPacksPath); err != nil {
		fmt.Fprintf(os.Stderr, "[Enforcer] Warning: failed to load custom policy packs from %s: %v\n", customPacksPath, err)
	}

	if strictMode && len(bundle.Rules) == 0 {
		return fmt.Errorf("[Enforcer] STRICT MODE: Cannot start without policy bundle")
	}

	// --- Load network allowlist ---
	loadNetworkAllowlist()

	// --- Create audit subsystem (PostgreSQL is the sole persistence layer) ---
	buffer := audit.NewAuditBuffer()

	pgStore, pgErr := audit.NewPostgresStore()
	if pgErr != nil {
		if strictMode {
			return fmt.Errorf("[Enforcer] STRICT MODE: Cannot start without audit persistence — PostgreSQL unavailable: %v", pgErr)
		}
		fmt.Fprintf(os.Stderr, "[Enforcer] WARNING: PostgreSQL unavailable (%v).\n", pgErr)
		fmt.Fprintf(os.Stderr, "[Enforcer] Daemon will start but audit queries will be limited.\n")
		fmt.Fprintf(os.Stderr, "[Enforcer] Set DATABASE_URL to enable persistent audit storage.\n")
	}

	// PostgreSQL is the SOLE persistence layer. No in-memory fallback.
	// If PostgreSQL is unavailable, the daemon starts but audit queries return errors.
	// In strict mode, the daemon refuses to start without PostgreSQL (handled above).
	var store audit.AuditStore
	if pgStore != nil {
		store = pgStore
	} else {
		// No InMemoryStore fallback — audit data must persist.
		// Return an error store that rejects all operations with a clear message.
		fmt.Fprintf(os.Stderr, "[Enforcer] CRITICAL: No audit persistence. Audit queries will fail.\n")
		fmt.Fprintf(os.Stderr, "[Enforcer] Set DATABASE_URL or start PostgreSQL to enable audit storage.\n")
		store = audit.NewNoOpStore()
	}
	flushSvc := audit.NewFlushService(buffer, store)
	flushSvc.Start()

	// --- Create approval service ---
	approvalSvc := approval.NewApprovalService(300, types.TimeoutDeny) // 5 min (300 seconds)

	// --- Policy routes (holds bundle + mutex) ---
	policyRoutes := routes.NewPolicyRoutes(
		&bundle,
		func(b types.PolicyBundle) error {
			return persistPolicyBundle(policyPersistPath, b)
		},
		func(packs []policy.PolicyPack) error {
			return persistCustomPacks(customPacksPath, packs)
		},
	)

	// --- Determine port ---
	port := os.Getenv("DAEMON_PORT")
	if port == "" {
		port = "9100"
	}

	// --- Embedded console handler ---
	consoleHandler := console.SentinelHandler()

	// --- Build handler ---
	mux := http.NewServeMux()

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// CORS preflight.
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization, X-Admin-Token, X-AA-Token")
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}

		path := r.URL.Path

		switch {
		// --- Auth introspection ---
		case path == "/v1/auth/me" && r.Method == http.MethodGet:
			if !RequireRole(w, r, authConfig, RoleOperator) {
				return
			}
			role := AuthenticateRole(r, authConfig)
			writeJSON(w, http.StatusOK, map[string]interface{}{
				"role":         role,
				"capabilities": RoleCapabilities(role),
			})

		// --- Health ---
		case path == "/v1/health" && r.Method == http.MethodGet:
			handleHealth(w, policyRoutes)

		// --- Enforcement ---
		case path == "/v1/enforcement" && r.Method == http.MethodGet:
			handleGetEnforcement(w)

		case path == "/v1/enforcement/toggle" && r.Method == http.MethodPost:
			if !RequireRole(w, r, authConfig, RoleAdmin) {
				return
			}
			handleToggleEnforcement(w, r)

		// --- Evaluate ---
		case path == "/v1/evaluate" && r.Method == http.MethodPost:
			if !RequireRole(w, r, authConfig, RoleOperator) {
				return
			}
			body, err := readBody(r)
			if err != nil {
				writeJSON(w, http.StatusBadRequest, map[string]string{"error": "failed to read body"})
				return
			}
			policyRoutes.RLock()
			currentBundle := policyRoutes.GetBundle()
			policyRoutes.RUnlock()
			status, resp := routes.HandleEvaluate(body, &currentBundle, buffer, approvalSvc, IsEnforcementEnabled())
			writeJSON(w, status, resp)

		// --- Audit: enrich ---
		case path == "/v1/audit/enrich" && r.Method == http.MethodPost:
			if !RequireRole(w, r, authConfig, RoleOperator) {
				return
			}
			body, err := readBody(r)
			if err != nil {
				writeJSON(w, http.StatusBadRequest, map[string]string{"error": "failed to read body"})
				return
			}
			status, resp := routes.HandleEnrich(body, store)
			writeJSON(w, status, resp)

		// --- Audit: events ---
		case path == "/v1/audit/events" && r.Method == http.MethodGet:
			if !RequireRole(w, r, authConfig, RoleOperator) {
				return
			}
			status, resp := routes.HandleQueryEvents(r.URL.Query(), store)
			writeJSON(w, status, resp)

		// --- Audit: sessions list ---
		case path == "/v1/audit/sessions" && r.Method == http.MethodGet:
			if !RequireRole(w, r, authConfig, RoleOperator) {
				return
			}
			status, resp := routes.HandleGetSessions(store)
			writeJSON(w, status, resp)

		// --- Audit: session by ID ---
		case strings.HasPrefix(path, "/v1/audit/sessions/") && r.Method == http.MethodGet:
			if !RequireRole(w, r, authConfig, RoleOperator) {
				return
			}
			sessionID := extractPathParam(path, "/v1/audit/sessions/")
			status, resp := routes.HandleGetSession(sessionID, store)
			writeJSON(w, status, resp)

		// --- Audit: export ---
		case path == "/v1/audit/export" && r.Method == http.MethodGet:
			if !RequireRole(w, r, authConfig, RoleOperator) {
				return
			}
			status, resp := routes.HandleExportEvents(r.URL.Query(), store)
			writeJSON(w, status, resp)

		// --- Audit: metrics ---
		case path == "/v1/audit/metrics" && r.Method == http.MethodGet:
			if !RequireRole(w, r, authConfig, RoleOperator) {
				return
			}
			status, resp := routes.HandleAuditMetrics(store)
			writeJSON(w, status, resp)

		// --- Approvals: pending ---
		case path == "/v1/approvals/pending" && r.Method == http.MethodGet:
			if !RequireRole(w, r, authConfig, RoleOperator) {
				return
			}
			status, resp := routes.HandleGetPending(approvalSvc)
			writeJSON(w, status, resp)

		// --- Approvals: status (operator+) — hook handler polls this ---
		case strings.HasPrefix(path, "/v1/approvals/") && strings.HasSuffix(path, "/status") && r.Method == http.MethodGet:
			if !RequireRole(w, r, authConfig, RoleOperator) {
				return
			}
			approvalID := extractPathParam(path, "/v1/approvals/")
			approvalID = strings.TrimSuffix(approvalID, "/status")
			status, resp := routes.HandleGetApprovalStatus(approvalID, approvalSvc)
			writeJSON(w, status, resp)

		// --- Approvals: resolve (reviewer+) ---
		case strings.HasPrefix(path, "/v1/approvals/") && strings.HasSuffix(path, "/resolve") && r.Method == http.MethodPost:
			if !RequireRole(w, r, authConfig, RoleReviewer) {
				return
			}
			approvalID := extractPathParam(path, "/v1/approvals/")
			// Remove /resolve suffix if it leaked into the ID.
			approvalID = strings.TrimSuffix(approvalID, "/resolve")
			body, err := readBody(r)
			if err != nil {
				writeJSON(w, http.StatusBadRequest, map[string]string{"error": "failed to read body"})
				return
			}
			status, resp := routes.HandleResolveApproval(approvalID, body, approvalSvc)
			writeJSON(w, status, resp)

		// --- Approvals: metrics ---
		case path == "/v1/approvals/metrics" && r.Method == http.MethodGet:
			if !RequireRole(w, r, authConfig, RoleOperator) {
				return
			}
			status, resp := routes.HandleGetApprovalMetrics(approvalSvc)
			writeJSON(w, status, resp)

		// --- Metrics ---
		case path == "/v1/metrics" && r.Method == http.MethodGet:
			if !RequireRole(w, r, authConfig, RoleOperator) {
				return
			}
			policyRoutes.RLock()
			currentBundle := policyRoutes.GetBundle()
			policyRoutes.RUnlock()
			enfState := GetEnforcementState()
			enfSnapshot := routes.EnforcementSnapshot{
				Enabled:   enfState.Enabled,
				Since:     enfState.Since,
				ChangedBy: enfState.ChangedBy,
			}
			status, resp := routes.HandleMetrics(&currentBundle, buffer, store, approvalSvc, enfSnapshot)
			writeJSON(w, status, resp)

		// --- Policy: rules ---
		case path == "/v1/policy/rules" && r.Method == http.MethodGet:
			if !RequireRole(w, r, authConfig, RoleOperator) {
				return
			}
			status, resp := policyRoutes.HandleListRules()
			writeJSON(w, status, resp)

		case path == "/v1/policy/rules" && r.Method == http.MethodPost:
			if !localPolicyEditsEnabled() {
				sendLocalPolicyEditDisabled(w)
				return
			}
			if !RequireRole(w, r, authConfig, RoleAdmin) {
				return
			}
			body, err := readBody(r)
			if err != nil {
				writeJSON(w, http.StatusBadRequest, map[string]string{"error": "failed to read body"})
				return
			}
			status, resp := policyRoutes.HandleAddRule(body)
			writeJSON(w, status, resp)

		case strings.HasPrefix(path, "/v1/policy/rules/") && strings.HasSuffix(path, "/toggle") && r.Method == http.MethodPost:
			if !localPolicyEditsEnabled() {
				sendLocalPolicyEditDisabled(w)
				return
			}
			if !RequireRole(w, r, authConfig, RoleAdmin) {
				return
			}
			ruleID := extractPathParam(path, "/v1/policy/rules/")
			ruleID = strings.TrimSuffix(ruleID, "/toggle")
			status, resp := policyRoutes.HandleToggleRule(ruleID)
			writeJSON(w, status, resp)

		case strings.HasPrefix(path, "/v1/policy/rules/") && r.Method == http.MethodPut:
			if !localPolicyEditsEnabled() {
				sendLocalPolicyEditDisabled(w)
				return
			}
			if !RequireRole(w, r, authConfig, RoleAdmin) {
				return
			}
			ruleID := extractPathParam(path, "/v1/policy/rules/")
			body, err := readBody(r)
			if err != nil {
				writeJSON(w, http.StatusBadRequest, map[string]string{"error": "failed to read body"})
				return
			}
			status, resp := policyRoutes.HandleUpdateRule(ruleID, body)
			writeJSON(w, status, resp)

		case strings.HasPrefix(path, "/v1/policy/rules/") && r.Method == http.MethodDelete:
			if !localPolicyEditsEnabled() {
				sendLocalPolicyEditDisabled(w)
				return
			}
			if !RequireRole(w, r, authConfig, RoleAdmin) {
				return
			}
			ruleID := extractPathParam(path, "/v1/policy/rules/")
			status, resp := policyRoutes.HandleDeleteRule(ruleID)
			writeJSON(w, status, resp)

		// --- Policy: bundle ---
		case path == "/v1/policy/bundle" && r.Method == http.MethodGet:
			if !RequireRole(w, r, authConfig, RoleOperator) {
				return
			}
			status, resp := policyRoutes.HandleGetBundle()
			writeJSON(w, status, resp)

		// --- Policy: packs ---
		case path == "/v1/policy/packs" && r.Method == http.MethodGet:
			if !RequireRole(w, r, authConfig, RoleOperator) {
				return
			}
			status, resp := policyRoutes.HandleListPacks()
			writeJSON(w, status, resp)

		case path == "/v1/policy/packs" && r.Method == http.MethodPost:
			if !localPolicyEditsEnabled() {
				sendLocalPolicyEditDisabled(w)
				return
			}
			if !RequireRole(w, r, authConfig, RoleAdmin) {
				return
			}
			body, err := readBody(r)
			if err != nil {
				writeJSON(w, http.StatusBadRequest, map[string]string{"error": "failed to read body"})
				return
			}
			status, resp := policyRoutes.HandleCreatePack(body)
			writeJSON(w, status, resp)

		case strings.HasPrefix(path, "/v1/policy/packs/") && strings.HasSuffix(path, "/apply") && r.Method == http.MethodPost:
			if !localPolicyEditsEnabled() {
				sendLocalPolicyEditDisabled(w)
				return
			}
			if !RequireRole(w, r, authConfig, RoleAdmin) {
				return
			}
			packID := extractPathParam(path, "/v1/policy/packs/")
			packID = strings.TrimSuffix(packID, "/apply")
			status, resp := policyRoutes.HandleApplyPack(packID)
			writeJSON(w, status, resp)

		case strings.HasPrefix(path, "/v1/policy/packs/") && r.Method == http.MethodGet:
			if !RequireRole(w, r, authConfig, RoleOperator) {
				return
			}
			packID := extractPathParam(path, "/v1/policy/packs/")
			status, resp := policyRoutes.HandleGetPack(packID)
			writeJSON(w, status, resp)

		// --- Analytics (operator+ can read, admin can modify) ---
		case path == "/v1/analytics/blocked-operations" && r.Method == http.MethodGet:
			if !RequireRole(w, r, authConfig, RoleOperator) {
				return
			}
			status, resp := routes.HandleBlockedOperations(r.URL.Query(), store)
			writeJSON(w, status, resp)

		case path == "/v1/analytics/approval-bottlenecks" && r.Method == http.MethodGet:
			if !RequireRole(w, r, authConfig, RoleOperator) {
				return
			}
			status, resp := routes.HandleApprovalBottlenecks(store)
			writeJSON(w, status, resp)

		case path == "/v1/analytics/developer-impact" && r.Method == http.MethodGet:
			if !RequireRole(w, r, authConfig, RoleOperator) {
				return
			}
			status, resp := routes.HandleDeveloperImpact(r.URL.Query(), store)
			writeJSON(w, status, resp)

		case path == "/v1/analytics/groups" && r.Method == http.MethodGet:
			if !RequireRole(w, r, authConfig, RoleOperator) {
				return
			}
			status, resp := routes.HandleGetGroups(store)
			writeJSON(w, status, resp)

		case strings.HasPrefix(path, "/v1/analytics/groups/") && strings.HasSuffix(path, "/members") && r.Method == http.MethodGet:
			if !RequireRole(w, r, authConfig, RoleOperator) {
				return
			}
			groupID := extractPathParam(path, "/v1/analytics/groups/")
			groupID = strings.TrimSuffix(groupID, "/members")
			status, resp := routes.HandleGetGroupMembers(groupID, store)
			writeJSON(w, status, resp)

		case path == "/v1/analytics/recommendations" && r.Method == http.MethodGet:
			if !RequireRole(w, r, authConfig, RoleOperator) {
				return
			}
			status, resp := routes.HandleGetRecommendations(store)
			writeJSON(w, status, resp)

		case strings.HasPrefix(path, "/v1/analytics/recommendations/") && strings.HasSuffix(path, "/apply") && r.Method == http.MethodPost:
			if !RequireRole(w, r, authConfig, RoleAdmin) {
				return
			}
			recID := extractPathParam(path, "/v1/analytics/recommendations/")
			recID = strings.TrimSuffix(recID, "/apply")
			body, err := readBody(r)
			if err != nil {
				writeJSON(w, http.StatusBadRequest, map[string]string{"error": "failed to read body"})
				return
			}
			policyRoutes.Lock()
			currentBundle := policyRoutes.GetBundle()
			status, resp := routes.HandleApplyRecommendation(recID, body, &currentBundle)
			policyRoutes.Unlock()
			writeJSON(w, status, resp)

		case strings.HasPrefix(path, "/v1/analytics/developer/") && strings.HasSuffix(path, "/trends") && r.Method == http.MethodGet:
			if !RequireRole(w, r, authConfig, RoleOperator) {
				return
			}
			userID := extractPathRemainder(path, "/v1/analytics/developer/")
			userID = strings.TrimSuffix(userID, "/trends")
			userID = strings.TrimSuffix(userID, "/")
			status, resp := routes.HandleDeveloperTrends(userID, store)
			writeJSON(w, status, resp)

		case strings.HasPrefix(path, "/v1/analytics/developer/") && r.Method == http.MethodGet:
			if !RequireRole(w, r, authConfig, RoleOperator) {
				return
			}
			userID := extractPathParam(path, "/v1/analytics/developer/")
			status, resp := routes.HandleDeveloperScorecard(userID, store)
			writeJSON(w, status, resp)

		// --- Console (embedded static assets) or 404 ---
		default:
			if !strings.HasPrefix(path, "/v1/") {
				consoleHandler.ServeHTTP(w, r)
				return
			}
			writeJSON(w, http.StatusNotFound, map[string]string{
				"error": "not_found",
				"path":  path,
			})
		}
	})

	mux.Handle("/", handler)

	fmt.Fprintf(os.Stderr, "[Enforcer] Daemon starting on port %s\n", port)
	fmt.Fprintf(os.Stderr, "[Enforcer] Policy bundle loaded: %d rules\n", len(bundle.Rules))

	return http.ListenAndServe(":"+port, mux)
}

// loadPolicyBundle attempts to load the policy from the default path or
// returns an empty bundle as fallback.
func loadPolicyBundle() types.PolicyBundle {
	// Prefer explicit policy directory from environment (used by managed deploy).
	if policyDir := os.Getenv("AA_POLICY_DIR"); policyDir != "" {
		policyPath := filepath.Join(policyDir, "default.yaml")
		loaded, err := policy.LoadPolicyBundle(policyPath)
		if err == nil {
			return loaded.Bundle
		}
	}

	// Try relative to binary location.
	exe, err := os.Executable()
	if err == nil {
		policyPath := filepath.Join(filepath.Dir(exe), "..", "..", "policies", "default.yaml")
		loaded, err := policy.LoadPolicyBundle(policyPath)
		if err == nil {
			return loaded.Bundle
		}
	}

	// Try relative to working directory.
	candidates := []string{
		"/etc/enforcer/default.yaml",
		"policies/default.yaml",
		"../../policies/default.yaml",
		"../policies/default.yaml",
	}
	for _, candidate := range candidates {
		loaded, err := policy.LoadPolicyBundle(candidate)
		if err == nil {
			return loaded.Bundle
		}
	}

	// Fallback: return a minimal bundle.
	fmt.Fprintf(os.Stderr, "[Enforcer] Warning: no policy file found, using empty bundle\n")
	return types.PolicyBundle{
		BundleVersion: "v0.0.0-empty",
		ScopeLevel:    types.ScopeLocal,
		Rules:         []types.PolicyRule{},
	}
}

func resolvePolicyPersistPath() string {
	var candidates []string
	if policyDir := os.Getenv("AA_POLICY_DIR"); policyDir != "" {
		candidates = append(candidates, filepath.Join(policyDir, "default.yaml"))
	}
	candidates = append(candidates,
		"policies/default.yaml",
		"/etc/enforcer/default.yaml",
		"../../policies/default.yaml",
		"../policies/default.yaml",
	)
	for _, candidate := range candidates {
		if _, err := os.Stat(candidate); err == nil {
			return candidate
		}
	}
	return candidates[0]
}

func persistPolicyBundle(path string, bundle types.PolicyBundle) error {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("creating policy directory %s: %w", dir, err)
	}
	data, err := yaml.Marshal(bundle)
	if err != nil {
		return fmt.Errorf("serializing policy bundle: %w", err)
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, data, 0o644); err != nil {
		return fmt.Errorf("writing temp policy file: %w", err)
	}
	if err := os.Rename(tmp, path); err != nil {
		return fmt.Errorf("replacing policy file: %w", err)
	}
	enforcePolicyFileOwnership(path)
	return nil
}

func loadCustomPacks(path string) error {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	var packs []policy.PolicyPack
	if err := json.Unmarshal(data, &packs); err != nil {
		return err
	}
	policy.SetCustomPacks(packs)
	return nil
}

// enforcePolicyFileOwnership sets root:wheel ownership on policy files.
// Only effective when the daemon is running as root (LaunchDaemon).
// Files are 0644: root-owned, world-readable — the hook handler (running as
// the developer user) can read the policy but cannot modify it.
func enforcePolicyFileOwnership(path string) {
	cur, err := user.Current()
	if err != nil || cur.Uid != "0" {
		return
	}
	if err := exec.Command("chown", "root:wheel", path).Run(); err != nil {
		slog.Warn("Failed to set policy file ownership", "path", path, "err", err)
	}
}

func persistCustomPacks(path string, packs []policy.PolicyPack) error {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("creating pack directory %s: %w", dir, err)
	}
	data, err := json.MarshalIndent(packs, "", "  ")
	if err != nil {
		return fmt.Errorf("serializing custom packs: %w", err)
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, append(data, '\n'), 0o644); err != nil {
		return fmt.Errorf("writing temp custom pack file: %w", err)
	}
	if err := os.Rename(tmp, path); err != nil {
		return fmt.Errorf("replacing custom pack file: %w", err)
	}
	enforcePolicyFileOwnership(path)
	return nil
}

func loadNetworkAllowlist() {
	if policyDir := os.Getenv("AA_POLICY_DIR"); policyDir != "" {
		allowlistPath := filepath.Join(policyDir, "network-allowlist.yaml")
		al, err := policy.LoadNetworkAllowlist(allowlistPath)
		if err == nil {
			policy.SetGlobalAllowlist(al)
			fmt.Fprintf(os.Stderr, "[Enforcer] Loaded network allowlist: %d allowed, %d warning\n", len(al.Allowlist), len(al.WarningList))
			return
		}
	}

	candidates := []string{
		"/etc/enforcer/network-allowlist.yaml",
		"policies/network-allowlist.yaml",
		"../../policies/network-allowlist.yaml",
		"../policies/network-allowlist.yaml",
	}
	exe, err := os.Executable()
	if err == nil {
		candidates = append([]string{
			filepath.Join(filepath.Dir(exe), "..", "..", "policies", "network-allowlist.yaml"),
		}, candidates...)
	}
	for _, candidate := range candidates {
		al, err := policy.LoadNetworkAllowlist(candidate)
		if err == nil {
			policy.SetGlobalAllowlist(al)
			fmt.Fprintf(os.Stderr, "[Enforcer] Loaded network allowlist: %d allowed, %d warning\n", len(al.Allowlist), len(al.WarningList))
			return
		}
	}
	fmt.Fprintf(os.Stderr, "[Enforcer] Warning: no network allowlist found\n")
}

// --- Built-in route handlers ---

func handleHealth(w http.ResponseWriter, policyRoutes *routes.PolicyRoutes) {
	state := GetEnforcementState()
	policyRoutes.RLock()
	bundle := policyRoutes.GetBundle()
	policyRoutes.RUnlock()

	// AA_GOVERNED_USER is set by the deploy script to the developer's OS username.
	// The Sentinel Console uses this to filter data to the local developer.
	governedUser := os.Getenv("AA_GOVERNED_USER")

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"status":         "healthy",
		"service":        "enforcer-daemon",
		"version":        "0.1.0",
		"timestamp":      time.Now().UTC().Format(time.RFC3339),
		"enforcement":    state.Enabled,
		"policy_version": bundle.BundleVersion,
		"policy_rules":   len(bundle.Rules),
		"governed_user":  governedUser,
	})
}

func handleGetEnforcement(w http.ResponseWriter) {
	state := GetEnforcementState()
	writeJSON(w, http.StatusOK, state)
}

func handleToggleEnforcement(w http.ResponseWriter, r *http.Request) {
	body, err := readBody(r)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "failed to read body"})
		return
	}

	var req struct {
		Enabled   bool   `json:"enabled"`
		ChangedBy string `json:"changed_by"`
	}
	if err := json.Unmarshal(body, &req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid JSON"})
		return
	}
	if req.ChangedBy == "" {
		req.ChangedBy = "admin"
	}

	SetEnforcementEnabled(req.Enabled, req.ChangedBy)
	state := GetEnforcementState()
	writeJSON(w, http.StatusOK, state)
}

// Ensure sync is imported since PolicyRoutes embeds sync.RWMutex.
var _ sync.RWMutex
