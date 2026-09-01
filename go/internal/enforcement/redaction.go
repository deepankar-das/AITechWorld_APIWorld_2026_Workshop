/**
 * Author: Deepankar Das
 */

package enforcement

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"regexp"
	"strings"
	"sync"
)

// SecretFinding describes a single secret or PII occurrence in scanned text.
type SecretFinding struct {
	Type     string `json:"type"`
	Value    string `json:"value"`
	Start    int    `json:"start"`
	End      int    `json:"end"`
	Severity string `json:"severity"` // "critical", "high", "medium", "low"
	Category string `json:"category"` // "credential", "token", "pii", "key"
}

// ScanResult holds the findings from a secret scan.
type ScanResult struct {
	Findings []SecretFinding `json:"findings"`
}

// secretPattern defines a compiled regexp pattern and its metadata.
type secretPattern struct {
	name     string
	re       *regexp.Regexp
	severity string
	category string
}

// secretPatterns is the set of compiled patterns used by the scanner.
// Patterns are compiled once at package init time.
var secretPatterns []secretPattern

func init() {
	defs := []struct {
		name     string
		pattern  string
		severity string
		category string
	}{
		{"aws_access_key", `AKIA[0-9A-Z]{16}`, "critical", "credential"},
		{"aws_secret_key", `(?i)aws_secret_access_key[^A-Za-z0-9]*[A-Za-z0-9/+=]{40}`, "critical", "credential"},
		{"github_token", `gh[ps]_[A-Za-z0-9_]{36,255}`, "critical", "token"},
		{"github_pat", `github_pat_[A-Za-z0-9_]{22,255}`, "critical", "token"},
		{"slack_token", `xox[baprs]-[0-9]{10,13}-[0-9]{10,13}-[a-zA-Z0-9]{24,34}`, "critical", "token"},
		{"stripe_key", `sk_live_[0-9a-zA-Z]{24,99}`, "critical", "credential"},
		{"jwt", `eyJ[A-Za-z0-9_-]{10,}\.[A-Za-z0-9_-]{10,}\.[A-Za-z0-9_-]{10,}`, "high", "token"},
		{"private_key", `-----BEGIN (?:RSA |EC |DSA |OPENSSH )?PRIVATE KEY-----`, "critical", "key"},
		{"database_url", `(?i)(?:postgres|mysql|mongodb|redis)://[^\s]+`, "critical", "credential"},
		{"email", `[a-zA-Z0-9._%+\-]+@[a-zA-Z0-9.\-]+\.[a-zA-Z]{2,}`, "low", "pii"},
		{"ssn", `\b\d{3}-\d{2}-\d{4}\b`, "critical", "pii"},
		{"credit_card", `\b(?:4[0-9]{12}(?:[0-9]{3})?|5[1-5][0-9]{14}|3[47][0-9]{13})\b`, "critical", "pii"},
		{"generic_api_key", `(?i)(?:api[_\-]?key|apikey)[^A-Za-z0-9]*[A-Za-z0-9]{20,}`, "high", "credential"},
		{"bearer_token", `(?i)bearer\s+[A-Za-z0-9._~+/=\-]{20,}`, "high", "token"},
		{"basic_auth", `(?i)basic\s+[A-Za-z0-9+/=]{20,}`, "high", "credential"},
		{"password_in_url", `://[^:]+:[^@]+@`, "critical", "credential"},
		{"generic_secret", `(?i)(?:secret|password|passwd|pwd)[^A-Za-z0-9]*[=:]\s*[^\s]{8,}`, "high", "credential"},
		{"generic_token", `(?i)(?:token|auth_token|access_token)[^A-Za-z0-9]*[=:]\s*[^\s]{8,}`, "high", "token"},
	}

	secretPatterns = make([]secretPattern, 0, len(defs))
	for _, d := range defs {
		compiled := regexp.MustCompile(d.pattern)
		secretPatterns = append(secretPatterns, secretPattern{
			name:     d.name,
			re:       compiled,
			severity: d.severity,
			category: d.category,
		})
	}
}

// ScanForSecrets scans text for known secret and PII patterns and returns
// all findings with their positions.
func ScanForSecrets(text string) ScanResult {
	var findings []SecretFinding

	for _, sp := range secretPatterns {
		matches := sp.re.FindAllStringIndex(text, -1)
		for _, loc := range matches {
			findings = append(findings, SecretFinding{
				Type:     sp.name,
				Value:    text[loc[0]:loc[1]],
				Start:    loc[0],
				End:      loc[1],
				Severity: sp.severity,
				Category: sp.category,
			})
		}
	}

	return ScanResult{Findings: findings}
}

// tokenStore maps placeholder tokens to their original values for reversal.
var tokenStore = struct {
	sync.Mutex
	m map[string]string
}{m: make(map[string]string)}

// generateToken creates a random hex token for the tokenize redaction mode.
func generateToken() string {
	b := make([]byte, 8)
	_, err := rand.Read(b)
	if err != nil {
		// Fallback: should not happen in practice.
		return "TOK_fallback"
	}
	return "TOK_" + hex.EncodeToString(b)
}

// RedactSecrets replaces all detected secrets in text according to the
// specified mode:
//   - "mask":      replaces with [REDACTED:<TYPE>]
//   - "tokenize":  replaces with a reversible token (TOK_xxx), stored for DeTokenize
//   - "summarize": replaces with [DETECTED: <type> at pos <start>-<end>]
//
// If mode is empty or unrecognised, "mask" is used as the default.
func RedactSecrets(text string, mode string) string {
	if mode == "" {
		mode = "mask"
	}

	scan := ScanForSecrets(text)
	if len(scan.Findings) == 0 {
		return text
	}

	// Sort findings by start position descending so that replacements
	// don't shift indices for earlier matches. We process from the end
	// of the string backwards.
	sorted := sortFindingsDesc(scan.Findings)

	result := text
	for _, f := range sorted {
		var replacement string
		switch mode {
		case "mask":
			replacement = "[REDACTED:" + strings.ToUpper(f.Type) + "]"
		case "tokenize":
			tok := generateToken()
			tokenStore.Lock()
			tokenStore.m[tok] = f.Value
			tokenStore.Unlock()
			replacement = tok
		case "summarize":
			replacement = fmt.Sprintf("[DETECTED: %s at pos %d-%d]", f.Type, f.Start, f.End)
		default:
			replacement = "[REDACTED:" + strings.ToUpper(f.Type) + "]"
		}
		result = result[:f.Start] + replacement + result[f.End:]
	}

	return result
}

// DeTokenize restores all TOK_xxx placeholders in text back to their
// original secret values. This is the reverse of tokenize mode.
func DeTokenize(text string) string {
	tokenStore.Lock()
	defer tokenStore.Unlock()

	result := text
	for tok, original := range tokenStore.m {
		result = strings.ReplaceAll(result, tok, original)
	}
	return result
}

// ClearTokenStore removes all stored tokens. Useful for testing isolation.
func ClearTokenStore() {
	tokenStore.Lock()
	defer tokenStore.Unlock()
	tokenStore.m = make(map[string]string)
}

// VerifyNoPlaintextSecrets scans text and returns whether it is clean
// (contains no detected secrets). If secrets are found, the findings
// are returned.
func VerifyNoPlaintextSecrets(text string) (bool, []SecretFinding) {
	scan := ScanForSecrets(text)
	if len(scan.Findings) == 0 {
		return true, nil
	}
	return false, scan.Findings
}

// sortFindingsDesc returns findings sorted by Start position in descending order.
// This is a simple insertion sort since the number of findings is typically small.
func sortFindingsDesc(findings []SecretFinding) []SecretFinding {
	sorted := make([]SecretFinding, len(findings))
	copy(sorted, findings)
	for i := 1; i < len(sorted); i++ {
		key := sorted[i]
		j := i - 1
		for j >= 0 && sorted[j].Start < key.Start {
			sorted[j+1] = sorted[j]
			j--
		}
		sorted[j+1] = key
	}
	return sorted
}
