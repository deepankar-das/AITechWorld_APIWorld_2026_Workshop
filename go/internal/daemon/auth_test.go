/**
 * Author: Deepankar Das
 */

package daemon

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func makeReq(method, path, token string) *http.Request {
	req := httptest.NewRequest(method, path, nil)
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	return req
}

func TestAuthenticateRole(t *testing.T) {
	cfg := AuthConfig{
		AdminToken:    "admin-token",
		ReviewerToken: "review-token",
		OperatorToken: "op-token",
	}

	tests := []struct {
		name  string
		token string
		want  Role
	}{
		{"admin", "admin-token", RoleAdmin},
		{"reviewer", "review-token", RoleReviewer},
		{"operator", "op-token", RoleOperator},
		{"unknown", "bogus", RoleNone},
		{"none", "", RoleNone},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := makeReq(http.MethodGet, "/v1/metrics", tt.token)
			got := AuthenticateRole(req, cfg)
			if got != tt.want {
				t.Fatalf("AuthenticateRole() = %s, want %s", got, tt.want)
			}
		})
	}
}

func TestRoleAtLeast(t *testing.T) {
	if !RoleAtLeast(RoleAdmin, RoleReviewer) {
		t.Fatal("admin should satisfy reviewer")
	}
	if !RoleAtLeast(RoleReviewer, RoleOperator) {
		t.Fatal("reviewer should satisfy operator")
	}
	if RoleAtLeast(RoleOperator, RoleReviewer) {
		t.Fatal("operator must not satisfy reviewer")
	}
}

func TestRequiredRoleForRoute(t *testing.T) {
	r, ok := RequiredRoleForRoute(http.MethodPost, "/v1/evaluate")
	if !ok || r != RoleOperator {
		t.Fatalf("expected operator rule for evaluate, got role=%s ok=%v", r, ok)
	}

	r, ok = RequiredRoleForRoute(http.MethodPost, "/v1/audit/enrich")
	if !ok || r != RoleOperator {
		t.Fatalf("expected operator rule for enrich, got role=%s ok=%v", r, ok)
	}

	r, ok = RequiredRoleForRoute(http.MethodPost, "/v1/approvals/abc/resolve")
	if !ok || r != RoleReviewer {
		t.Fatalf("expected reviewer rule for approvals resolve, got role=%s ok=%v", r, ok)
	}

	r, ok = RequiredRoleForRoute(http.MethodPost, "/v1/policy/rules/org.block/toggle")
	if !ok || r != RoleAdmin {
		t.Fatalf("expected admin rule for policy toggle, got role=%s ok=%v", r, ok)
	}

	r, ok = RequiredRoleForRoute(http.MethodGet, "/v1/metrics")
	if !ok || r != RoleOperator {
		t.Fatalf("expected operator rule for metrics, got role=%s ok=%v", r, ok)
	}
}

func TestExtractAccessTokenVariants(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/v1/metrics?access_token=query-token&admin_token=legacy-query", nil)
	req.Header.Set("Authorization", "Bearer bearer-token")
	req.Header.Set("X-AA-Token", "xaa-token")
	req.Header.Set("X-Admin-Token", "xadmin-token")

	if got := ExtractAccessToken(req); got != "bearer-token" {
		t.Fatalf("priority mismatch: got %q, want %q", got, "bearer-token")
	}

	req2 := httptest.NewRequest(http.MethodGet, "/v1/metrics?access_token=query-token", nil)
	req2.Header.Set("X-AA-Token", "xaa-token")
	if got := ExtractAccessToken(req2); got != "xaa-token" {
		t.Fatalf("expected X-AA-Token fallback, got %q", got)
	}

	req3 := httptest.NewRequest(http.MethodGet, "/v1/metrics?access_token=query-token", nil)
	req3.Header.Set("X-Admin-Token", "xadmin-token")
	if got := ExtractAccessToken(req3); got != "xadmin-token" {
		t.Fatalf("expected X-Admin-Token fallback, got %q", got)
	}

	req4 := httptest.NewRequest(http.MethodGet, "/v1/metrics?access_token=query-token", nil)
	if got := ExtractAccessToken(req4); got != "query-token" {
		t.Fatalf("expected access_token query fallback, got %q", got)
	}

	req5 := httptest.NewRequest(http.MethodGet, "/v1/metrics?admin_token=legacy-query", nil)
	if got := ExtractAccessToken(req5); got != "legacy-query" {
		t.Fatalf("expected admin_token query fallback, got %q", got)
	}
}

func TestRequireRole(t *testing.T) {
	cfg := AuthConfig{
		AdminToken:    "admin-token",
		ReviewerToken: "review-token",
		OperatorToken: "op-token",
	}

	// Missing token => 401
	{
		req := makeReq(http.MethodPost, "/v1/policy/rules", "")
		rr := httptest.NewRecorder()
		if RequireRole(rr, req, cfg, RoleAdmin) {
			t.Fatal("RequireRole should reject missing token")
		}
		if rr.Code != http.StatusUnauthorized {
			t.Fatalf("status = %d, want 401", rr.Code)
		}
	}

	// Insufficient role => 403
	{
		req := makeReq(http.MethodPost, "/v1/approvals/id/resolve", "op-token")
		rr := httptest.NewRecorder()
		if RequireRole(rr, req, cfg, RoleReviewer) {
			t.Fatal("RequireRole should reject operator for reviewer action")
		}
		if rr.Code != http.StatusForbidden {
			t.Fatalf("status = %d, want 403", rr.Code)
		}
	}

	// Sufficient role => pass
	{
		req := makeReq(http.MethodPost, "/v1/approvals/id/resolve", "review-token")
		rr := httptest.NewRecorder()
		if !RequireRole(rr, req, cfg, RoleReviewer) {
			t.Fatal("RequireRole should allow reviewer for reviewer action")
		}
	}
}

func TestRouteAuthMatrixCoverage(t *testing.T) {
	required := []struct {
		method string
		path   string
		role   Role
	}{
		{http.MethodPost, "/v1/evaluate", RoleOperator},
		{http.MethodPost, "/v1/audit/enrich", RoleOperator},
		{http.MethodPost, "/v1/enforcement/toggle", RoleAdmin},
		{http.MethodPost, "/v1/approvals/abc/resolve", RoleReviewer},
		{http.MethodGet, "/v1/audit/events", RoleOperator},
		{http.MethodGet, "/v1/metrics", RoleOperator},
		{http.MethodPost, "/v1/policy/rules", RoleAdmin},
		{http.MethodPost, "/v1/policy/packs/pack/apply", RoleAdmin},
	}

	for _, tc := range required {
		got, ok := RequiredRoleForRoute(tc.method, tc.path)
		if !ok {
			t.Fatalf("route missing from matrix: %s %s", tc.method, tc.path)
		}
		if got != tc.role {
			t.Fatalf("role mismatch for %s %s: got %s want %s", tc.method, tc.path, got, tc.role)
		}
	}
}
