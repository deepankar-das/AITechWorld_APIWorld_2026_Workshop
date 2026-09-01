package central

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"
)

type memoryStateStore struct {
	latestPolicy      *policyState
	latestEnforcement *enforcementState
}

func (m *memoryStateStore) PutPolicyRevision(_ context.Context, state policyState) error {
	copyState := state
	m.latestPolicy = &copyState
	return nil
}

func (m *memoryStateStore) GetLatestPolicy(_ context.Context) (*policyState, error) {
	if m.latestPolicy == nil {
		return nil, errPolicyNotFound
	}
	copyState := *m.latestPolicy
	return &copyState, nil
}

func (m *memoryStateStore) PutEnforcementState(_ context.Context, state enforcementState) error {
	copyState := state
	m.latestEnforcement = &copyState
	return nil
}

func (m *memoryStateStore) GetLatestEnforcementState(_ context.Context) (*enforcementState, error) {
	if m.latestEnforcement == nil {
		return nil, errEnforcementStateNotFound
	}
	copyState := *m.latestEnforcement
	return &copyState, nil
}

func (m *memoryStateStore) RecordClientSnapshot(_ context.Context, _ clientSnapshot) error {
	return nil
}

func (m *memoryStateStore) ListLatestClients(_ context.Context) ([]*RegisteredClient, error) {
	return nil, nil
}

func (m *memoryStateStore) CountClients(_ context.Context) (int, error) {
	return 0, nil
}

func (m *memoryStateStore) Close() {}

func writeTempFile(t *testing.T, content string) string {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "policy.yaml")
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write temp policy: %v", err)
	}
	return path
}

func TestLoadPolicyValidation(t *testing.T) {
	s := &CentralServer{stateStore: &memoryStateStore{}}

	t.Run("rejects invalid yaml", func(t *testing.T) {
		path := writeTempFile(t, "rules: [")
		if err := s.LoadPolicy(path); err == nil {
			t.Fatal("expected invalid YAML error")
		}
	})

	t.Run("rejects empty rules", func(t *testing.T) {
		path := writeTempFile(t, "bundle_version: v1\nscope_level: organization\nrules: []\n")
		if err := s.LoadPolicy(path); err == nil {
			t.Fatal("expected empty policy rules error")
		}
	})

	t.Run("accepts non-empty bundle", func(t *testing.T) {
		path := writeTempFile(t, `
bundle_version: v1
scope_level: organization
rules:
  - policy_id: org.allow_demo
    version: v1
    effect:
      decision: allow
`)
		if err := s.LoadPolicy(path); err != nil {
			t.Fatalf("expected valid policy load, got: %v", err)
		}
	})
}

func TestHandleAdminSetPolicyValidation(t *testing.T) {
	s := &CentralServer{stateStore: &memoryStateStore{}}

	validBundle := `
bundle_version: v1
scope_level: organization
rules:
  - policy_id: org.allow_demo
    version: v1
    effect:
      decision: allow
`

	tests := []struct {
		name     string
		body     map[string]string
		wantCode int
	}{
		{
			name: "reject invalid yaml",
			body: map[string]string{
				"bundle":  "rules: [",
				"version": "v1",
			},
			wantCode: http.StatusBadRequest,
		},
		{
			name: "reject empty bundle",
			body: map[string]string{
				"bundle":  "bundle_version: v1\nscope_level: organization\nrules: []\n",
				"version": "v1",
			},
			wantCode: http.StatusBadRequest,
		},
		{
			name: "accept valid bundle",
			body: map[string]string{
				"bundle":  validBundle,
				"version": "v1",
			},
			wantCode: http.StatusOK,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			payload, err := json.Marshal(tc.body)
			if err != nil {
				t.Fatalf("marshal request: %v", err)
			}
			req := httptest.NewRequest(http.MethodPut, "/api/v1/policy", bytes.NewReader(payload))
			rec := httptest.NewRecorder()
			s.handleAdminSetPolicy(rec, req)
			if rec.Code != tc.wantCode {
				t.Fatalf("expected status %d, got %d body=%s", tc.wantCode, rec.Code, rec.Body.String())
			}
		})
	}
}

func TestMemoryStateStorePolicyNotFound(t *testing.T) {
	store := &memoryStateStore{}
	_, err := store.GetLatestPolicy(context.Background())
	if !errors.Is(err, errPolicyNotFound) {
		t.Fatalf("expected errPolicyNotFound, got %v", err)
	}
}

func TestMemoryStateStoreStoresPolicy(t *testing.T) {
	store := &memoryStateStore{}
	err := store.PutPolicyRevision(context.Background(), policyState{Version: "v1", Hash: "abc", Bundle: "rules: []"})
	if err != nil {
		t.Fatalf("unexpected store error: %v", err)
	}
	state, err := store.GetLatestPolicy(context.Background())
	if err != nil {
		t.Fatalf("unexpected get error: %v", err)
	}
	if state.Version != "v1" || state.Hash != "abc" {
		t.Fatalf("unexpected state: %+v", state)
	}
}

func TestHandlePolicyPullWhenPolicyMissing(t *testing.T) {
	s := &CentralServer{stateStore: &memoryStateStore{}}
	req := httptest.NewRequest(http.MethodGet, "/api/v1/policy/pull", nil)
	rec := httptest.NewRecorder()
	s.handlePolicyPull(rec, req)
	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected 503 when no policy, got %d", rec.Code)
	}
}

func TestHandlePolicyPullReturnsLatestPolicy(t *testing.T) {
	store := &memoryStateStore{}
	_ = store.PutPolicyRevision(context.Background(), policyState{Version: "v1", Hash: "h", Bundle: "bundle"})
	s := &CentralServer{stateStore: store}
	req := httptest.NewRequest(http.MethodGet, "/api/v1/policy/pull", nil)
	rec := httptest.NewRecorder()
	s.handlePolicyPull(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
}

func TestHandleRegisterRequiresClientID(t *testing.T) {
	s := &CentralServer{stateStore: &memoryStateStore{}}
	req := httptest.NewRequest(http.MethodPost, "/api/v1/register", bytes.NewReader([]byte(`{"hostname":"h"}`)))
	rec := httptest.NewRecorder()
	s.handleRegister(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", rec.Code)
	}
}

func TestHandleHeartbeatRequiresClientID(t *testing.T) {
	s := &CentralServer{stateStore: &memoryStateStore{}}
	req := httptest.NewRequest(http.MethodPost, "/api/v1/heartbeat", bytes.NewReader([]byte(`{"status":"online"}`)))
	rec := httptest.NewRecorder()
	s.handleHeartbeat(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", rec.Code)
	}
}

func TestHashPolicyBundleStable(t *testing.T) {
	if hashPolicyBundle("a") != hashPolicyBundle("a") {
		t.Fatal("expected deterministic hash")
	}
	if hashPolicyBundle("a") == hashPolicyBundle("b") {
		t.Fatal("expected distinct hashes")
	}
}

func TestHandleAdminSetAndGetEnforcement(t *testing.T) {
	s := &CentralServer{stateStore: &memoryStateStore{}}

	payload := []byte(`{"enabled":false,"changed_by":"test_admin"}`)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/enforcement/toggle", bytes.NewReader(payload))
	rec := httptest.NewRecorder()
	s.handleAdminSetEnforcement(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d body=%s", rec.Code, rec.Body.String())
	}

	getReq := httptest.NewRequest(http.MethodGet, "/api/v1/enforcement", nil)
	getRec := httptest.NewRecorder()
	s.handleAdminGetEnforcement(getRec, getReq)
	if getRec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d body=%s", getRec.Code, getRec.Body.String())
	}
	var body map[string]interface{}
	if err := json.Unmarshal(getRec.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode body: %v", err)
	}
	enabled, _ := body["enabled"].(bool)
	if enabled {
		t.Fatalf("expected enabled=false, got true")
	}
}

func TestMemoryStoreCloseNoop(t *testing.T) {
	store := &memoryStateStore{}
	store.Close()
	time.Sleep(1 * time.Millisecond)
}
