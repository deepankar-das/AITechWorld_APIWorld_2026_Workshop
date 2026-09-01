/**
 * Author: Deepankar Das
 */

package central

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/anthropics/enforcer/internal/authsecrets"
)

type role string

const (
	roleNone     role = "none"
	roleOperator role = "operator"
	roleReviewer role = "reviewer"
	roleAdmin    role = "admin"
)

type authConfig struct {
	AdminToken    string
	ReviewerToken string
	OperatorToken string
}

func tokenFilePath(defaultPath string) string {
	if dir := strings.TrimSpace(os.Getenv("AA_CONFIG_DIR")); dir != "" {
		return filepath.Join(dir, filepath.Base(defaultPath))
	}
	return defaultPath
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

func loadAuthConfig() authConfig {
	adminTokenPath := tokenFilePath("/etc/enforcer/.admin_token")
	reviewerTokenPath := tokenFilePath("/etc/enforcer/.reviewer_token")
	operatorTokenPath := tokenFilePath("/etc/enforcer/.operator_token")

	cfg := authConfig{
		AdminToken:    loadTokenFromEnvOrFile("AA_ADMIN_TOKEN", adminTokenPath),
		ReviewerToken: loadTokenFromEnvOrFile("AA_REVIEWER_TOKEN", reviewerTokenPath),
		OperatorToken: loadTokenFromEnvOrFile("AA_OPERATOR_TOKEN", operatorTokenPath),
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
		_, _ = os.Stderr.WriteString("[Enforcer] Warning: failed to load encrypted Hub auth tokens from DB: " + err.Error() + "\n")
	}

	saveCtx, saveCancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer saveCancel()
	if err := authsecrets.SaveTokens(saveCtx, authsecrets.RoleTokens{
		AdminToken:    cfg.AdminToken,
		ReviewerToken: cfg.ReviewerToken,
		OperatorToken: cfg.OperatorToken,
	}); err != nil && !errors.Is(err, authsecrets.ErrDatabaseURLMissing) && !errors.Is(err, authsecrets.ErrEncryptionKeyUnset) {
		_, _ = os.Stderr.WriteString("[Enforcer] Warning: failed to persist encrypted Hub auth tokens to DB: " + err.Error() + "\n")
	}

	return cfg
}

func roleRank(r role) int {
	switch r {
	case roleAdmin:
		return 3
	case roleReviewer:
		return 2
	case roleOperator:
		return 1
	default:
		return 0
	}
}

func roleAtLeast(actual role, required role) bool {
	return roleRank(actual) >= roleRank(required)
}

func roleCapabilities(r role) map[string]bool {
	return map[string]bool{
		"view":               roleAtLeast(r, roleOperator),
		"approve":            roleAtLeast(r, roleReviewer),
		"toggle_enforcement": roleAtLeast(r, roleAdmin),
		"manage_policy":      roleAtLeast(r, roleAdmin),
	}
}

func extractAccessToken(r *http.Request) string {
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

func authenticateRole(r *http.Request, cfg authConfig) role {
	token := extractAccessToken(r)
	if token == "" {
		return roleNone
	}
	if cfg.AdminToken != "" && token == cfg.AdminToken {
		return roleAdmin
	}
	if cfg.ReviewerToken != "" && token == cfg.ReviewerToken {
		return roleReviewer
	}
	if cfg.OperatorToken != "" && token == cfg.OperatorToken {
		return roleOperator
	}
	return roleNone
}

func sendUnauthorized(w http.ResponseWriter) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusUnauthorized)
	_ = json.NewEncoder(w).Encode(map[string]interface{}{
		"error":   "unauthorized",
		"message": "Valid access token required. Provide via Authorization: Bearer, X-AA-Token/X-Admin-Token header, or access_token query parameter.",
	})
}

func sendForbidden(w http.ResponseWriter, actual role, required role) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusForbidden)
	_ = json.NewEncoder(w).Encode(map[string]interface{}{
		"error":         "forbidden",
		"message":       "Token role is insufficient for this operation.",
		"required_role": required,
		"actual_role":   actual,
	})
}

func requireRole(w http.ResponseWriter, r *http.Request, cfg authConfig, required role) bool {
	actual := authenticateRole(r, cfg)
	if actual == roleNone {
		sendUnauthorized(w)
		return false
	}
	if !roleAtLeast(actual, required) {
		sendForbidden(w, actual, required)
		return false
	}
	return true
}
