/**
 * Author: Deepankar Das
 */

package daemon

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func authReq(method, path, token string) *http.Request {
	req := httptest.NewRequest(method, path, nil)
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	return req
}

func TestSecurityRegression_RoleEscalationBlocked(t *testing.T) {
	cfg := AuthConfig{
		AdminToken:    "adm",
		ReviewerToken: "rev",
		OperatorToken: "op",
	}

	cases := []struct {
		name         string
		method       string
		path         string
		token        string
		requiredRole Role
		wantStatus   int
	}{
		{
			name:         "operator cannot resolve approvals",
			method:       http.MethodPost,
			path:         "/v1/approvals/apr_1/resolve",
			token:        "op",
			requiredRole: RoleReviewer,
			wantStatus:   http.StatusForbidden,
		},
		{
			name:         "reviewer cannot mutate policy",
			method:       http.MethodPost,
			path:         "/v1/policy/rules",
			token:        "rev",
			requiredRole: RoleAdmin,
			wantStatus:   http.StatusForbidden,
		},
		{
			name:         "missing token denied",
			method:       http.MethodGet,
			path:         "/v1/audit/events",
			token:        "",
			requiredRole: RoleOperator,
			wantStatus:   http.StatusUnauthorized,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			req := authReq(tc.method, tc.path, tc.token)
			rr := httptest.NewRecorder()
			ok := RequireRole(rr, req, cfg, tc.requiredRole)
			if ok {
				t.Fatalf("expected RequireRole to reject request")
			}
			if rr.Code != tc.wantStatus {
				t.Fatalf("status=%d want=%d", rr.Code, tc.wantStatus)
			}
		})
	}
}

func TestSecurityRegression_RouteAuthorizationMatrix(t *testing.T) {
	// Regression guard: critical management routes must stay protected.
	tests := []struct {
		method string
		path   string
		want   Role
	}{
		{http.MethodPost, "/v1/evaluate", RoleOperator},
		{http.MethodPost, "/v1/audit/enrich", RoleOperator},
		{http.MethodPost, "/v1/enforcement/toggle", RoleAdmin},
		{http.MethodPost, "/v1/policy/rules", RoleAdmin},
		{http.MethodPost, "/v1/policy/packs/pack-1/apply", RoleAdmin},
		{http.MethodPost, "/v1/approvals/a/resolve", RoleReviewer},
		{http.MethodGet, "/v1/metrics", RoleOperator},
		{http.MethodGet, "/v1/audit/export", RoleOperator},
	}

	for _, tc := range tests {
		got, ok := RequiredRoleForRoute(tc.method, tc.path)
		if !ok {
			t.Fatalf("missing matrix rule for %s %s", tc.method, tc.path)
		}
		if got != tc.want {
			t.Fatalf("matrix role mismatch for %s %s: got=%s want=%s", tc.method, tc.path, got, tc.want)
		}
	}
}
