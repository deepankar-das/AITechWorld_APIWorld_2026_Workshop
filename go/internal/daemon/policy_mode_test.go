package daemon

import "testing"

func TestLocalPolicyEditsEnabled_DefaultFalse(t *testing.T) {
	t.Setenv(allowLocalPolicyEditsEnv, "")
	if localPolicyEditsEnabled() {
		t.Fatal("expected local policy edits disabled by default")
	}
}

func TestLocalPolicyEditsEnabled_TrueValues(t *testing.T) {
	values := []string{"1", "true", "TRUE", "yes", "on"}
	for _, v := range values {
		t.Run(v, func(t *testing.T) {
			t.Setenv(allowLocalPolicyEditsEnv, v)
			if !localPolicyEditsEnabled() {
				t.Fatalf("expected %q to enable local policy edits", v)
			}
		})
	}
}

func TestLocalPolicyEditsEnabled_FalseValues(t *testing.T) {
	values := []string{"0", "false", "no", "off", "random"}
	for _, v := range values {
		t.Run(v, func(t *testing.T) {
			t.Setenv(allowLocalPolicyEditsEnv, v)
			if localPolicyEditsEnabled() {
				t.Fatalf("expected %q to keep local policy edits disabled", v)
			}
		})
	}
}
