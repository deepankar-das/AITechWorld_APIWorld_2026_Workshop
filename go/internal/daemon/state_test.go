/**
 * Author: Deepankar Das
 */

package daemon

import (
	"os"
	"testing"
)

func TestIsStrictMode_DefaultTrue(t *testing.T) {
	os.Unsetenv("AA_STRICT_MODE")
	if !IsStrictMode() {
		t.Error("IsStrictMode() should return true by default (env unset)")
	}
}

func TestIsStrictMode_TrueWhenSetToTrue(t *testing.T) {
	os.Setenv("AA_STRICT_MODE", "true")
	defer os.Unsetenv("AA_STRICT_MODE")
	if !IsStrictMode() {
		t.Error("IsStrictMode() should return true when AA_STRICT_MODE=true")
	}
}

func TestIsStrictMode_TrueWhenSetTo1(t *testing.T) {
	os.Setenv("AA_STRICT_MODE", "1")
	defer os.Unsetenv("AA_STRICT_MODE")
	if !IsStrictMode() {
		t.Error("IsStrictMode() should return true when AA_STRICT_MODE=1")
	}
}

func TestIsStrictMode_TrueWhenSetToYes(t *testing.T) {
	os.Setenv("AA_STRICT_MODE", "yes")
	defer os.Unsetenv("AA_STRICT_MODE")
	if !IsStrictMode() {
		t.Error("IsStrictMode() should return true when AA_STRICT_MODE=yes")
	}
}

func TestIsStrictMode_FalseWhenSetToFalse(t *testing.T) {
	os.Setenv("AA_STRICT_MODE", "false")
	defer os.Unsetenv("AA_STRICT_MODE")
	if IsStrictMode() {
		t.Error("IsStrictMode() should return false when AA_STRICT_MODE=false")
	}
}

func TestIsStrictMode_TrueWhenSetToEmpty(t *testing.T) {
	os.Setenv("AA_STRICT_MODE", "")
	defer os.Unsetenv("AA_STRICT_MODE")
	if !IsStrictMode() {
		t.Error("IsStrictMode() should return true when AA_STRICT_MODE is empty")
	}
}

func TestIsStrictMode_FalseWhenSetToZero(t *testing.T) {
	os.Setenv("AA_STRICT_MODE", "0")
	defer os.Unsetenv("AA_STRICT_MODE")
	if IsStrictMode() {
		t.Error("IsStrictMode() should return false when AA_STRICT_MODE=0")
	}
}

func TestIsStrictMode_FalseWhenSetToNo(t *testing.T) {
	os.Setenv("AA_STRICT_MODE", "no")
	defer os.Unsetenv("AA_STRICT_MODE")
	if IsStrictMode() {
		t.Error("IsStrictMode() should return false when AA_STRICT_MODE=no")
	}
}

func TestIsStrictMode_CaseInsensitive(t *testing.T) {
	os.Setenv("AA_STRICT_MODE", "TRUE")
	defer os.Unsetenv("AA_STRICT_MODE")
	if !IsStrictMode() {
		t.Error("IsStrictMode() should be case-insensitive (TRUE should work)")
	}
}

func TestIsStrictMode_TrimsWhitespace(t *testing.T) {
	os.Setenv("AA_STRICT_MODE", "  true  ")
	defer os.Unsetenv("AA_STRICT_MODE")
	if !IsStrictMode() {
		t.Error("IsStrictMode() should trim whitespace")
	}
}
