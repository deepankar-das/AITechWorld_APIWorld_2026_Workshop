/**
 * Author: Deepankar Das
 */

package enforcement

import (
	"strings"
	"testing"
)

// --- Detection tests ---

func TestScanForSecrets_AWSAccessKey(t *testing.T) {
	text := "key = AKIAIOSFODNN7EXAMPLE"
	scan := ScanForSecrets(text)
	found := false
	for _, f := range scan.Findings {
		if f.Type == "aws_access_key" {
			found = true
			if f.Value != "AKIAIOSFODNN7EXAMPLE" {
				t.Fatalf("expected AKIAIOSFODNN7EXAMPLE, got %s", f.Value)
			}
		}
	}
	if !found {
		t.Fatal("expected to detect aws_access_key")
	}
}

func TestScanForSecrets_AWSSecretKey(t *testing.T) {
	text := "aws_secret_access_key=wJalrXUtnFEMI/K7MDENG/bPxRfiCYEXAMPLEKEYaa"
	scan := ScanForSecrets(text)
	found := false
	for _, f := range scan.Findings {
		if f.Type == "aws_secret_key" {
			found = true
		}
	}
	if !found {
		t.Fatal("expected to detect aws_secret_key")
	}
}

func TestScanForSecrets_GitHubToken(t *testing.T) {
	text := "token=ghp_ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghij"
	scan := ScanForSecrets(text)
	found := false
	for _, f := range scan.Findings {
		if f.Type == "github_token" {
			found = true
		}
	}
	if !found {
		t.Fatal("expected to detect github_token")
	}
}

func TestScanForSecrets_GitHubPAT(t *testing.T) {
	text := "pat=github_pat_ABCDEFGHIJKLMNOPQRSTUV0123456789ab"
	scan := ScanForSecrets(text)
	found := false
	for _, f := range scan.Findings {
		if f.Type == "github_pat" {
			found = true
		}
	}
	if !found {
		t.Fatal("expected to detect github_pat")
	}
}

func TestScanForSecrets_JWT(t *testing.T) {
	text := "auth: eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJzdWIiOiIxMjM0NTY3ODkwIn0.dozjgNryP4J3jVmNHl0w5N_XgL0n3I9PlFUP0THsR8U"
	scan := ScanForSecrets(text)
	found := false
	for _, f := range scan.Findings {
		if f.Type == "jwt" {
			found = true
		}
	}
	if !found {
		t.Fatal("expected to detect jwt")
	}
}

func TestScanForSecrets_PrivateKey(t *testing.T) {
	text := "-----BEGIN RSA PRIVATE KEY-----\nMIIEow..."
	scan := ScanForSecrets(text)
	found := false
	for _, f := range scan.Findings {
		if f.Type == "private_key" {
			found = true
		}
	}
	if !found {
		t.Fatal("expected to detect private_key")
	}
}

func TestScanForSecrets_PrivateKeyGeneric(t *testing.T) {
	text := "-----BEGIN PRIVATE KEY-----\nMIIEow..."
	scan := ScanForSecrets(text)
	found := false
	for _, f := range scan.Findings {
		if f.Type == "private_key" {
			found = true
		}
	}
	if !found {
		t.Fatal("expected to detect private_key (generic)")
	}
}

func TestScanForSecrets_DatabaseURL(t *testing.T) {
	text := "DATABASE_URL=postgres://user:pass@db.example.com:5432/mydb"
	scan := ScanForSecrets(text)
	found := false
	for _, f := range scan.Findings {
		if f.Type == "database_url" {
			found = true
		}
	}
	if !found {
		t.Fatal("expected to detect database_url")
	}
}

func TestScanForSecrets_Email(t *testing.T) {
	text := "contact: admin@example.com"
	scan := ScanForSecrets(text)
	found := false
	for _, f := range scan.Findings {
		if f.Type == "email" {
			found = true
			if f.Value != "admin@example.com" {
				t.Fatalf("expected admin@example.com, got %s", f.Value)
			}
		}
	}
	if !found {
		t.Fatal("expected to detect email")
	}
}

func TestScanForSecrets_SSN(t *testing.T) {
	text := "SSN: 123-45-6789"
	scan := ScanForSecrets(text)
	found := false
	for _, f := range scan.Findings {
		if f.Type == "ssn" {
			found = true
			if f.Value != "123-45-6789" {
				t.Fatalf("expected 123-45-6789, got %s", f.Value)
			}
		}
	}
	if !found {
		t.Fatal("expected to detect ssn")
	}
}

func TestScanForSecrets_CreditCardVisa(t *testing.T) {
	text := "card: 4111111111111111"
	scan := ScanForSecrets(text)
	found := false
	for _, f := range scan.Findings {
		if f.Type == "credit_card" {
			found = true
		}
	}
	if !found {
		t.Fatal("expected to detect credit_card (Visa)")
	}
}

func TestScanForSecrets_CreditCardMastercard(t *testing.T) {
	text := "card: 5500000000000004"
	scan := ScanForSecrets(text)
	found := false
	for _, f := range scan.Findings {
		if f.Type == "credit_card" {
			found = true
		}
	}
	if !found {
		t.Fatal("expected to detect credit_card (Mastercard)")
	}
}

func TestScanForSecrets_CreditCardAmex(t *testing.T) {
	text := "card: 378282246310005"
	scan := ScanForSecrets(text)
	found := false
	for _, f := range scan.Findings {
		if f.Type == "credit_card" {
			found = true
		}
	}
	if !found {
		t.Fatal("expected to detect credit_card (Amex)")
	}
}

func TestScanForSecrets_StripeKey(t *testing.T) {
	// Assembled at runtime so no live-key-shaped literal sits in the source tree.
	text := "stripe_key=sk_live_" + strings.Repeat("0", 30)
	scan := ScanForSecrets(text)
	found := false
	for _, f := range scan.Findings {
		if f.Type == "stripe_key" {
			found = true
		}
	}
	if !found {
		t.Fatal("expected to detect stripe_key")
	}
}

func TestScanForSecrets_SlackToken(t *testing.T) {
	// Assembled at runtime so no token-shaped literal sits in the source tree.
	text := "slack=xoxb-" + strings.Repeat("1", 11) + "-" + strings.Repeat("2", 11) + "-" + strings.Repeat("A", 28)
	scan := ScanForSecrets(text)
	found := false
	for _, f := range scan.Findings {
		if f.Type == "slack_token" {
			found = true
		}
	}
	if !found {
		t.Fatal("expected to detect slack_token")
	}
}

func TestScanForSecrets_BearerToken(t *testing.T) {
	text := "Authorization: Bearer eyJhbGciOiJSUzI1NiIsInR5cCI6IkpXVCJ9abcdef"
	scan := ScanForSecrets(text)
	found := false
	for _, f := range scan.Findings {
		if f.Type == "bearer_token" {
			found = true
		}
	}
	if !found {
		t.Fatal("expected to detect bearer_token")
	}
}

func TestScanForSecrets_BasicAuth(t *testing.T) {
	text := "Authorization: Basic dXNlcm5hbWU6cGFzc3dvcmQxMjM0NTY="
	scan := ScanForSecrets(text)
	found := false
	for _, f := range scan.Findings {
		if f.Type == "basic_auth" {
			found = true
		}
	}
	if !found {
		t.Fatal("expected to detect basic_auth")
	}
}

func TestScanForSecrets_PasswordInURL(t *testing.T) {
	text := "url=https://admin:s3cret@api.example.com/v1"
	scan := ScanForSecrets(text)
	found := false
	for _, f := range scan.Findings {
		if f.Type == "password_in_url" {
			found = true
		}
	}
	if !found {
		t.Fatal("expected to detect password_in_url")
	}
}

func TestScanForSecrets_GenericAPIKey(t *testing.T) {
	text := "api_key=abcdef1234567890abcdef1234567890"
	scan := ScanForSecrets(text)
	found := false
	for _, f := range scan.Findings {
		if f.Type == "generic_api_key" {
			found = true
		}
	}
	if !found {
		t.Fatal("expected to detect generic_api_key")
	}
}

// --- Mask mode ---

func TestRedactSecrets_MaskMode_AWSKey(t *testing.T) {
	text := "key=AKIAIOSFODNN7EXAMPLE"
	result := RedactSecrets(text, "mask")
	if strings.Contains(result, "AKIAIOSFODNN7EXAMPLE") {
		t.Fatal("expected AWS key to be redacted in mask mode")
	}
	if !strings.Contains(result, "[REDACTED:AWS_ACCESS_KEY]") {
		t.Fatalf("expected [REDACTED:AWS_ACCESS_KEY] in result, got: %s", result)
	}
}

func TestRedactSecrets_MaskMode_Email(t *testing.T) {
	text := "contact: admin@example.com"
	result := RedactSecrets(text, "mask")
	if strings.Contains(result, "admin@example.com") {
		t.Fatal("expected email to be redacted in mask mode")
	}
	if !strings.Contains(result, "[REDACTED:EMAIL]") {
		t.Fatalf("expected [REDACTED:EMAIL] in result, got: %s", result)
	}
}

func TestRedactSecrets_MaskMode_SSN(t *testing.T) {
	text := "SSN: 123-45-6789"
	result := RedactSecrets(text, "mask")
	if strings.Contains(result, "123-45-6789") {
		t.Fatal("expected SSN to be redacted in mask mode")
	}
	if !strings.Contains(result, "[REDACTED:SSN]") {
		t.Fatalf("expected [REDACTED:SSN] in result, got: %s", result)
	}
}

// --- Tokenize mode ---

func TestRedactSecrets_TokenizeMode(t *testing.T) {
	ClearTokenStore()
	text := "key=AKIAIOSFODNN7EXAMPLE"
	result := RedactSecrets(text, "tokenize")
	if strings.Contains(result, "AKIAIOSFODNN7EXAMPLE") {
		t.Fatal("expected AWS key to be replaced with token")
	}
	if !strings.Contains(result, "TOK_") {
		t.Fatalf("expected TOK_ placeholder in result, got: %s", result)
	}
}

func TestDeTokenize_RestoresValue(t *testing.T) {
	ClearTokenStore()
	original := "key=AKIAIOSFODNN7EXAMPLE"
	tokenized := RedactSecrets(original, "tokenize")
	restored := DeTokenize(tokenized)
	if !strings.Contains(restored, "AKIAIOSFODNN7EXAMPLE") {
		t.Fatalf("expected DeTokenize to restore original value, got: %s", restored)
	}
}

func TestDeTokenize_RoundTrip(t *testing.T) {
	ClearTokenStore()
	original := "admin@example.com"
	text := "email: " + original
	tokenized := RedactSecrets(text, "tokenize")
	if strings.Contains(tokenized, original) {
		t.Fatal("expected email to be replaced with token")
	}
	restored := DeTokenize(tokenized)
	if !strings.Contains(restored, original) {
		t.Fatalf("expected DeTokenize to restore email, got: %s", restored)
	}
}

// --- Summarize mode ---

func TestRedactSecrets_SummarizeMode(t *testing.T) {
	text := "key=AKIAIOSFODNN7EXAMPLE"
	result := RedactSecrets(text, "summarize")
	if strings.Contains(result, "AKIAIOSFODNN7EXAMPLE") {
		t.Fatal("expected AWS key to be removed in summarize mode")
	}
	if !strings.Contains(result, "[DETECTED: aws_access_key at pos") {
		t.Fatalf("expected [DETECTED: aws_access_key at pos ...] in result, got: %s", result)
	}
}

// --- Verify function ---

func TestVerifyNoPlaintextSecrets_Dirty(t *testing.T) {
	text := "key=AKIAIOSFODNN7EXAMPLE and ssn=123-45-6789"
	clean, findings := VerifyNoPlaintextSecrets(text)
	if clean {
		t.Fatal("expected clean=false for text with secrets")
	}
	if len(findings) == 0 {
		t.Fatal("expected non-empty findings")
	}
}

func TestVerifyNoPlaintextSecrets_Clean(t *testing.T) {
	text := "This is a safe string with no secrets at all."
	clean, findings := VerifyNoPlaintextSecrets(text)
	if !clean {
		t.Fatalf("expected clean=true for safe text, got findings: %v", findings)
	}
	if findings != nil {
		t.Fatalf("expected nil findings, got %v", findings)
	}
}

// --- Multiple secrets in one text ---

func TestScanForSecrets_MultipleSecrets(t *testing.T) {
	text := "key=AKIAIOSFODNN7EXAMPLE email=admin@example.com ssn=123-45-6789"
	scan := ScanForSecrets(text)

	types := make(map[string]bool)
	for _, f := range scan.Findings {
		types[f.Type] = true
	}

	if !types["aws_access_key"] {
		t.Fatal("expected aws_access_key in findings")
	}
	if !types["email"] {
		t.Fatal("expected email in findings")
	}
	if !types["ssn"] {
		t.Fatal("expected ssn in findings")
	}
}

func TestRedactSecrets_MultipleSecrets_MaskAll(t *testing.T) {
	text := "key=AKIAIOSFODNN7EXAMPLE contact=admin@example.com"
	result := RedactSecrets(text, "mask")
	if strings.Contains(result, "AKIAIOSFODNN7EXAMPLE") {
		t.Fatal("expected AWS key to be redacted")
	}
	if strings.Contains(result, "admin@example.com") {
		t.Fatal("expected email to be redacted")
	}
	if !strings.Contains(result, "[REDACTED:AWS_ACCESS_KEY]") {
		t.Fatalf("expected [REDACTED:AWS_ACCESS_KEY], got: %s", result)
	}
	if !strings.Contains(result, "[REDACTED:EMAIL]") {
		t.Fatalf("expected [REDACTED:EMAIL], got: %s", result)
	}
}

// --- Default mode ---

func TestRedactSecrets_DefaultMode(t *testing.T) {
	text := "key=AKIAIOSFODNN7EXAMPLE"
	result := RedactSecrets(text, "")
	if !strings.Contains(result, "[REDACTED:AWS_ACCESS_KEY]") {
		t.Fatalf("expected mask mode as default, got: %s", result)
	}
}

// --- No secrets returns original ---

func TestRedactSecrets_NoSecrets(t *testing.T) {
	text := "This is a completely safe string."
	result := RedactSecrets(text, "mask")
	if result != text {
		t.Fatalf("expected original text back, got: %s", result)
	}
}

func TestScanForSecrets_DatabaseURL_MySQL(t *testing.T) {
	text := "db=mysql://root:password@localhost:3306/mydb"
	scan := ScanForSecrets(text)
	found := false
	for _, f := range scan.Findings {
		if f.Type == "database_url" {
			found = true
		}
	}
	if !found {
		t.Fatal("expected to detect database_url for MySQL")
	}
}

func TestScanForSecrets_DatabaseURL_MongoDB(t *testing.T) {
	text := "mongo=mongodb://user:pass@mongo.example.com:27017/db"
	scan := ScanForSecrets(text)
	found := false
	for _, f := range scan.Findings {
		if f.Type == "database_url" {
			found = true
		}
	}
	if !found {
		t.Fatal("expected to detect database_url for MongoDB")
	}
}

func TestScanForSecrets_DatabaseURL_Redis(t *testing.T) {
	text := "cache=redis://default:secret@redis.example.com:6379/0"
	scan := ScanForSecrets(text)
	found := false
	for _, f := range scan.Findings {
		if f.Type == "database_url" {
			found = true
		}
	}
	if !found {
		t.Fatal("expected to detect database_url for Redis")
	}
}
