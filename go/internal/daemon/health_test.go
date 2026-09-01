package daemon

import (
	"encoding/json"
	"net/http/httptest"
	"testing"

	"github.com/anthropics/enforcer/internal/daemon/routes"
	"github.com/anthropics/enforcer/internal/types"
)

func TestHandleHealth_IncludesGovernedUserFromEnv(t *testing.T) {
	t.Setenv("AA_GOVERNED_USER", "dev-user")

	bundle := &types.PolicyBundle{
		BundleVersion: "v-test",
		Rules: []types.PolicyRule{
			{PolicyID: "org.test"},
		},
	}
	policyRoutes := routes.NewPolicyRoutes(bundle, nil, nil)

	rec := httptest.NewRecorder()
	handleHealth(rec, policyRoutes)

	if rec.Code != 200 {
		t.Fatalf("unexpected status: got %d want 200", rec.Code)
	}

	var body map[string]interface{}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if got, _ := body["governed_user"].(string); got != "dev-user" {
		t.Fatalf("governed_user mismatch: got %q want %q", got, "dev-user")
	}
	if got, _ := body["policy_version"].(string); got != "v-test" {
		t.Fatalf("policy_version mismatch: got %q want %q", got, "v-test")
	}
}

func TestHandleHealth_AlwaysReturnsGovernedUserField(t *testing.T) {
	t.Setenv("AA_GOVERNED_USER", "")

	bundle := &types.PolicyBundle{BundleVersion: "v-empty"}
	policyRoutes := routes.NewPolicyRoutes(bundle, nil, nil)

	rec := httptest.NewRecorder()
	handleHealth(rec, policyRoutes)

	var body map[string]interface{}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	v, ok := body["governed_user"]
	if !ok {
		t.Fatal("expected governed_user key in health response")
	}
	if got, _ := v.(string); got != "" {
		t.Fatalf("governed_user should be empty string when env is unset, got %q", got)
	}
}

