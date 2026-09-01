/**
 * Author: Deepankar Das
 */

package daemon

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/anthropics/enforcer/internal/authsecrets"
)

// Role defines an authorization tier for API access.
type Role string

const (
	RoleNone     Role = "none"
	RoleOperator Role = "operator"
	RoleReviewer Role = "reviewer"
	RoleAdmin    Role = "admin"
)

// AuthConfig holds per-role bearer tokens.
type AuthConfig struct {
	AdminToken    string
	ReviewerToken string
	OperatorToken string
}

// RouteAuthRule maps an HTTP method + path pattern to a required role.
// PathPattern supports a trailing wildcard ("/v1/foo/*") for prefix matches.
type RouteAuthRule struct {
	Method      string
	PathPattern string
	MinRole     Role
}

// RouteAuthMatrix is the canonical authorization policy for daemon routes.
// This matrix is covered by security regression tests.
var RouteAuthMatrix = []RouteAuthRule{
	{Method: http.MethodPost, PathPattern: "/v1/evaluate", MinRole: RoleOperator},
	{Method: http.MethodPost, PathPattern: "/v1/audit/enrich", MinRole: RoleOperator},
	{Method: http.MethodPost, PathPattern: "/v1/enforcement/toggle", MinRole: RoleAdmin},
	{Method: http.MethodPost, PathPattern: "/v1/policy/rules", MinRole: RoleAdmin},
	{Method: http.MethodPut, PathPattern: "/v1/policy/rules/*", MinRole: RoleAdmin},
	{Method: http.MethodDelete, PathPattern: "/v1/policy/rules/*", MinRole: RoleAdmin},
	{Method: http.MethodPost, PathPattern: "/v1/policy/rules/*", MinRole: RoleAdmin}, // includes /toggle
	{Method: http.MethodPost, PathPattern: "/v1/policy/packs", MinRole: RoleAdmin},
	{Method: http.MethodPost, PathPattern: "/v1/policy/packs/*", MinRole: RoleAdmin}, // includes /apply
	{Method: http.MethodPost, PathPattern: "/v1/approvals/*", MinRole: RoleReviewer}, // includes /resolve
	{Method: http.MethodGet, PathPattern: "/v1/auth/me", MinRole: RoleOperator},
	{Method: http.MethodGet, PathPattern: "/v1/metrics", MinRole: RoleOperator},
	{Method: http.MethodGet, PathPattern: "/v1/audit/events", MinRole: RoleOperator},
	{Method: http.MethodGet, PathPattern: "/v1/audit/sessions", MinRole: RoleOperator},
	{Method: http.MethodGet, PathPattern: "/v1/audit/sessions/*", MinRole: RoleOperator},
	{Method: http.MethodGet, PathPattern: "/v1/audit/export", MinRole: RoleOperator},
	{Method: http.MethodGet, PathPattern: "/v1/audit/metrics", MinRole: RoleOperator},
	{Method: http.MethodGet, PathPattern: "/v1/approvals/pending", MinRole: RoleOperator},
	{Method: http.MethodGet, PathPattern: "/v1/approvals/*/status", MinRole: RoleOperator},
	{Method: http.MethodGet, PathPattern: "/v1/approvals/metrics", MinRole: RoleOperator},
	{Method: http.MethodGet, PathPattern: "/v1/policy/rules", MinRole: RoleOperator},
	{Method: http.MethodGet, PathPattern: "/v1/policy/bundle", MinRole: RoleOperator},
	{Method: http.MethodGet, PathPattern: "/v1/policy/packs", MinRole: RoleOperator},
	{Method: http.MethodGet, PathPattern: "/v1/policy/packs/*", MinRole: RoleOperator},
}

func roleRank(role Role) int {
	switch role {
	case RoleAdmin:
		return 3
	case RoleReviewer:
		return 2
	case RoleOperator:
		return 1
	default:
		return 0
	}
}

// RoleAtLeast returns true when actual role can satisfy required role.
func RoleAtLeast(actual Role, required Role) bool {
	return roleRank(actual) >= roleRank(required)
}

func loadTokenFromEnvOrFile(envKey, filePath string) string {
	if token := strings.TrimSpace(os.Getenv(envKey)); token != "" {
		return token
	}
	data, err := os.ReadFile(filePath)
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(data))
}

func generateDevToken(prefix string) string {
	randomBytes := make([]byte, 16)
	_, _ = rand.Read(randomBytes)
	return fmt.Sprintf("%s_%d_%s", prefix, time.Now().Unix(), hex.EncodeToString(randomBytes))
}

// LoadAuthConfig loads role tokens from env/file sources.
// Admin token is mandatory; if absent, a dev token is generated.
// Operator token is also mandatory — the hook handler (running as the
// developer user) needs it to authenticate.  If absent, a token is generated
// and persisted.  The file is always set to 0644 so the hook handler can
// read it.
func LoadAuthConfig() AuthConfig {
	operatorTokenPath := tokenFilePath("AA_CONFIG_DIR", "/etc/enforcer/.operator_token")
	ensureOperatorToken(operatorTokenPath)
	adminTokenPath := tokenFilePath("AA_CONFIG_DIR", "/etc/enforcer/.admin_token")
	reviewerTokenPath := tokenFilePath("AA_CONFIG_DIR", "/etc/enforcer/.reviewer_token")

	cfg := AuthConfig{
		AdminToken:    loadTokenFromEnvOrFile("AA_ADMIN_TOKEN", adminTokenPath),
		ReviewerToken: loadTokenFromEnvOrFile("AA_REVIEWER_TOKEN", reviewerTokenPath),
		OperatorToken: loadTokenFromEnvOrFile("AA_OPERATOR_TOKEN", operatorTokenPath),
	}

	// Optional convenience for local testing: allow colocated token files.
	if cfg.ReviewerToken == "" {
		if t := loadTokenFromEnvOrFile("", filepath.Join(".", ".reviewer_token")); t != "" {
			cfg.ReviewerToken = t
		}
	}
	if cfg.OperatorToken == "" {
		if t := loadTokenFromEnvOrFile("", filepath.Join(".", ".operator_token")); t != "" {
			cfg.OperatorToken = t
		}
	}

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	dbTokens, err := authsecrets.LoadTokens(ctx)
	if err == nil {
		if strings.TrimSpace(os.Getenv("AA_ADMIN_TOKEN")) == "" && dbTokens.AdminToken != "" {
			cfg.AdminToken = dbTokens.AdminToken
		}
		if strings.TrimSpace(os.Getenv("AA_REVIEWER_TOKEN")) == "" && dbTokens.ReviewerToken != "" {
			cfg.ReviewerToken = dbTokens.ReviewerToken
		}
		if strings.TrimSpace(os.Getenv("AA_OPERATOR_TOKEN")) == "" && dbTokens.OperatorToken != "" {
			cfg.OperatorToken = dbTokens.OperatorToken
		}
	} else if !errors.Is(err, authsecrets.ErrDatabaseURLMissing) && !errors.Is(err, authsecrets.ErrEncryptionKeyUnset) {
		fmt.Fprintf(os.Stderr, "[Enforcer] Warning: failed to load encrypted auth tokens from DB: %v\n", err)
	}

	if cfg.AdminToken == "" {
		cfg.AdminToken = generateDevToken("dev_admin")
		fmt.Fprintf(os.Stderr, "[Enforcer] Generated dev admin token: %s\n", cfg.AdminToken)
	}
	if cfg.OperatorToken == "" {
		cfg.OperatorToken = generateDevToken("op")
	}

	// Keep local token files synchronized for compatibility (hook handler reads operator token from file).
	writeTokenFile(operatorTokenPath, cfg.OperatorToken, 0644)
	if cfg.AdminToken != "" {
		writeTokenFile(adminTokenPath, cfg.AdminToken, 0600)
	}
	if cfg.ReviewerToken != "" {
		writeTokenFile(reviewerTokenPath, cfg.ReviewerToken, 0600)
	}

	saveCtx, saveCancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer saveCancel()
	if err := authsecrets.SaveTokens(saveCtx, authsecrets.RoleTokens{
		AdminToken:    cfg.AdminToken,
		ReviewerToken: cfg.ReviewerToken,
		OperatorToken: cfg.OperatorToken,
	}); err != nil && !errors.Is(err, authsecrets.ErrDatabaseURLMissing) && !errors.Is(err, authsecrets.ErrEncryptionKeyUnset) {
		fmt.Fprintf(os.Stderr, "[Enforcer] Warning: failed to persist encrypted auth tokens to DB: %v\n", err)
	}

	return cfg
}

// tokenFilePath returns the path for a token file, respecting AA_CONFIG_DIR.
func tokenFilePath(envKey, defaultPath string) string {
	if dir := os.Getenv(envKey); dir != "" {
		return filepath.Join(dir, filepath.Base(defaultPath))
	}
	return defaultPath
}

// ensureOperatorToken guarantees the operator token file exists and is
// world-readable (0644).  The hook handler runs as the developer user (not
// root) and must be able to read this file to authenticate with the daemon.
func ensureOperatorToken(path string) {
	dir := filepath.Dir(path)

	// Generate the token if the file doesn't exist.
	if _, err := os.Stat(path); os.IsNotExist(err) {
		_ = os.MkdirAll(dir, 0755)
		token := generateDevToken("op")
		if err := os.WriteFile(path, []byte(token), 0644); err != nil {
			fmt.Fprintf(os.Stderr, "[Enforcer] Warning: could not write operator token to %s: %v\n", path, err)
			return
		}
		fmt.Fprintf(os.Stderr, "[Enforcer] Generated operator token: %s\n", path)
	}

	// Always fix permissions — previous deploys may have set 0600 on the file
	// or 0700 on the directory.  The hook handler runs as the developer user
	// and must be able to traverse the directory and read the token.
	_ = os.Chmod(dir, 0755)
	_ = os.Chmod(path, 0644)
}

func writeTokenFile(path string, token string, mode os.FileMode) {
	if strings.TrimSpace(path) == "" || strings.TrimSpace(token) == "" {
		return
	}
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		fmt.Fprintf(os.Stderr, "[Enforcer] Warning: could not create token directory %s: %v\n", dir, err)
		return
	}
	if err := os.WriteFile(path, []byte(token), mode); err != nil {
		fmt.Fprintf(os.Stderr, "[Enforcer] Warning: could not write token file %s: %v\n", path, err)
		return
	}
	_ = os.Chmod(path, mode)
}

// LoadAdminToken is retained for backward compatibility.
func LoadAdminToken() string {
	return LoadAuthConfig().AdminToken
}

// ExtractAccessToken returns the bearer token from supported request locations.
func ExtractAccessToken(r *http.Request) string {
	auth := strings.TrimSpace(r.Header.Get("Authorization"))
	if strings.HasPrefix(auth, "Bearer ") {
		return strings.TrimSpace(strings.TrimPrefix(auth, "Bearer "))
	}
	if token := strings.TrimSpace(r.Header.Get("X-AA-Token")); token != "" {
		return token
	}
	if token := strings.TrimSpace(r.Header.Get("X-Admin-Token")); token != "" {
		return token
	}
	if token := strings.TrimSpace(r.URL.Query().Get("access_token")); token != "" {
		return token
	}
	return strings.TrimSpace(r.URL.Query().Get("admin_token"))
}

// AuthenticateRole resolves the caller role based on request token.
func AuthenticateRole(r *http.Request, cfg AuthConfig) Role {
	token := ExtractAccessToken(r)
	if token == "" {
		return RoleNone
	}
	if cfg.AdminToken != "" && token == cfg.AdminToken {
		return RoleAdmin
	}
	if cfg.ReviewerToken != "" && token == cfg.ReviewerToken {
		return RoleReviewer
	}
	if cfg.OperatorToken != "" && token == cfg.OperatorToken {
		return RoleOperator
	}
	return RoleNone
}

// IsAuthenticated is retained for backward compatibility with admin-only checks.
func IsAuthenticated(r *http.Request, token string) bool {
	cfg := AuthConfig{AdminToken: token}
	return AuthenticateRole(r, cfg) == RoleAdmin
}

// RequiredRoleForRoute returns the minimum role required for a route.
// The second return value indicates whether the route is covered by the matrix.
func RequiredRoleForRoute(method, path string) (Role, bool) {
	for _, rule := range RouteAuthMatrix {
		if rule.Method != method {
			continue
		}
		if pathMatches(rule.PathPattern, path) {
			return rule.MinRole, true
		}
	}
	return RoleNone, false
}

func pathMatches(pattern, path string) bool {
	if strings.HasSuffix(pattern, "*") {
		prefix := strings.TrimSuffix(pattern, "*")
		return strings.HasPrefix(path, prefix)
	}
	return path == pattern
}

// RequireRole enforces role-based authorization and writes 401/403 when denied.
func RequireRole(w http.ResponseWriter, r *http.Request, cfg AuthConfig, required Role) bool {
	role := AuthenticateRole(r, cfg)
	if role == RoleNone {
		slog.Warn("Auth failure: no valid token",
			"method", r.Method,
			"path", r.URL.Path,
			"remote", r.RemoteAddr,
			"required_role", string(required))
		SendUnauthorized(w)
		return false
	}
	if !RoleAtLeast(role, required) {
		slog.Warn("Auth failure: insufficient role",
			"method", r.Method,
			"path", r.URL.Path,
			"actual_role", string(role),
			"required_role", string(required))
		SendForbidden(w, role, required)
		return false
	}
	return true
}

// RoleCapabilities reports console-relevant permissions for a role.
func RoleCapabilities(role Role) map[string]bool {
	return map[string]bool{
		"view":               RoleAtLeast(role, RoleOperator),
		"approve":            RoleAtLeast(role, RoleReviewer),
		"toggle_enforcement": RoleAtLeast(role, RoleAdmin),
		"manage_policy":      RoleAtLeast(role, RoleAdmin),
	}
}

// SendUnauthorized writes a 401 Unauthorized JSON response.
func SendUnauthorized(w http.ResponseWriter) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusUnauthorized)
	_ = json.NewEncoder(w).Encode(map[string]interface{}{
		"error":   "unauthorized",
		"message": "Valid access token required. Provide via Authorization: Bearer, X-AA-Token/X-Admin-Token header, or access_token query parameter.",
	})
}

// SendForbidden writes a 403 Forbidden response for insufficient role.
func SendForbidden(w http.ResponseWriter, actual Role, required Role) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusForbidden)
	_ = json.NewEncoder(w).Encode(map[string]interface{}{
		"error":         "forbidden",
		"message":       "Token role is insufficient for this operation.",
		"required_role": required,
		"actual_role":   actual,
	})
}
