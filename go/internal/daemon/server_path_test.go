/**
 * Author: Deepankar Das
 */

package daemon

import (
	"strings"
	"testing"
)

func TestExtractPathParam(t *testing.T) {
	got := extractPathParam("/v1/analytics/developer/dev_001/trends", "/v1/analytics/developer/")
	if got != "dev_001" {
		t.Fatalf("extractPathParam() = %q, want %q", got, "dev_001")
	}
}

func TestExtractPathRemainder(t *testing.T) {
	got := extractPathRemainder("/v1/analytics/developer/dev_001/trends", "/v1/analytics/developer/")
	if got != "dev_001/trends" {
		t.Fatalf("extractPathRemainder() = %q, want %q", got, "dev_001/trends")
	}
}

func TestTrendsUserIDParsing(t *testing.T) {
	userID := extractPathRemainder("/v1/analytics/developer/dev_001/trends", "/v1/analytics/developer/")
	userID = strings.TrimSuffix(userID, "/trends")
	userID = strings.TrimSuffix(userID, "/")
	if userID != "dev_001" {
		t.Fatalf("trends userID parsing = %q, want %q", userID, "dev_001")
	}
}
