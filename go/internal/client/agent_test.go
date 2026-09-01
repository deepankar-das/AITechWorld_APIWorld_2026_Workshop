/**
 * Author: Deepankar Das
 */

package client

import (
	"os"
	"testing"
)

func TestParseFileMode(t *testing.T) {
	tests := []struct {
		input   string
		want    os.FileMode
		wantErr bool
	}{
		{"0644", 0644, false},
		{"0600", 0600, false},
		{"0755", 0755, false},
		{"0444", 0444, false},
		{"", 0, true},
		{"invalid", 0, true},
		{"9999", 0, true}, // 9 is not valid octal
	}
	for _, tt := range tests {
		got, err := parseFileMode(tt.input)
		if tt.wantErr {
			if err == nil {
				t.Errorf("parseFileMode(%q) expected error, got %04o", tt.input, got)
			}
			continue
		}
		if err != nil {
			t.Errorf("parseFileMode(%q) unexpected error: %v", tt.input, err)
			continue
		}
		if got != tt.want {
			t.Errorf("parseFileMode(%q) = %04o, want %04o", tt.input, got, tt.want)
		}
	}
}

func TestApplyFileOwnership_NonRoot(t *testing.T) {
	// When not running as root, applyFileOwnership should be a no-op (no panic, no error).
	tmp := t.TempDir() + "/test-policy.yaml"
	if err := os.WriteFile(tmp, []byte("test"), 0644); err != nil {
		t.Fatal(err)
	}
	// Should not panic or fail — just silently skip when not root.
	applyFileOwnership(tmp, nil)

	// Verify file still exists and is readable.
	if _, err := os.ReadFile(tmp); err != nil {
		t.Fatalf("file unreadable after applyFileOwnership: %v", err)
	}
}
