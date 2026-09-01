package authsecrets

import "testing"

func TestEncryptDecryptRoundTrip(t *testing.T) {
	key := []byte("0123456789abcdef0123456789abcdef")
	role := "admin"
	plaintext := "adm1-secret-token"

	enc, err := encrypt(key, role, plaintext)
	if err != nil {
		t.Fatalf("encrypt failed: %v", err)
	}
	got, err := decrypt(key, role, enc.nonceB64, enc.ciphertextB64)
	if err != nil {
		t.Fatalf("decrypt failed: %v", err)
	}
	if got != plaintext {
		t.Fatalf("round-trip mismatch: got %q want %q", got, plaintext)
	}
}

func TestDecryptRoleAADMismatch(t *testing.T) {
	key := []byte("0123456789abcdef0123456789abcdef")
	enc, err := encrypt(key, "admin", "value")
	if err != nil {
		t.Fatalf("encrypt failed: %v", err)
	}
	if _, err := decrypt(key, "operator", enc.nonceB64, enc.ciphertextB64); err == nil {
		t.Fatal("expected decryption failure on AAD role mismatch")
	}
}

func TestParseKeyVariants(t *testing.T) {
	raw := "0123456789abcdef0123456789abcdef"
	if _, err := parseKey(raw); err != nil {
		t.Fatalf("raw parse failed: %v", err)
	}

	hexKey := "3031323334353637383961626364656630313233343536373839616263646566"
	if _, err := parseKey(hexKey); err != nil {
		t.Fatalf("hex parse failed: %v", err)
	}

	b64Key := "MDEyMzQ1Njc4OWFiY2RlZjAxMjM0NTY3ODlhYmNkZWY="
	if _, err := parseKey(b64Key); err != nil {
		t.Fatalf("base64 parse failed: %v", err)
	}
}
