package central

import (
	"net/http/httptest"
	"testing"
)

func TestAuthenticateRole(t *testing.T) {
	cfg := authConfig{
		AdminToken:    "admin-token",
		ReviewerToken: "review-token",
		OperatorToken: "op-token",
	}

	tests := []struct {
		name  string
		token string
		want  role
	}{
		{name: "admin", token: "admin-token", want: roleAdmin},
		{name: "reviewer", token: "review-token", want: roleReviewer},
		{name: "operator", token: "op-token", want: roleOperator},
		{name: "invalid", token: "nope", want: roleNone},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest("GET", "/api/v1/auth/me", nil)
			req.Header.Set("Authorization", "Bearer "+tc.token)
			if got := authenticateRole(req, cfg); got != tc.want {
				t.Fatalf("authenticateRole = %q, want %q", got, tc.want)
			}
		})
	}
}

func TestExtractAccessTokenFallbacks(t *testing.T) {
	req := httptest.NewRequest("GET", "/api/v1/auth/me?access_token=query-token", nil)
	if got := extractAccessToken(req); got != "query-token" {
		t.Fatalf("expected query token, got %q", got)
	}

	req2 := httptest.NewRequest("GET", "/api/v1/auth/me", nil)
	req2.Header.Set("X-AA-Token", "xaa-token")
	if got := extractAccessToken(req2); got != "xaa-token" {
		t.Fatalf("expected X-AA-Token fallback, got %q", got)
	}

	req3 := httptest.NewRequest("GET", "/api/v1/auth/me", nil)
	req3.Header.Set("X-Admin-Token", "xadmin-token")
	if got := extractAccessToken(req3); got != "xadmin-token" {
		t.Fatalf("expected X-Admin-Token fallback, got %q", got)
	}

	req4 := httptest.NewRequest("GET", "/api/v1/auth/me?admin_token=legacy-token", nil)
	if got := extractAccessToken(req4); got != "legacy-token" {
		t.Fatalf("expected admin_token fallback, got %q", got)
	}
}
