/**
 * Author: Deepankar Das
 */

/**
 * Enforcer — Secrets/PII Redaction Tests
 *
 * Tests the full redaction pipeline: scan, tokenize, de-tokenize, verify.
 * Depth area 4c from the venture prompt.
 */

import { describe, it, expect } from "vitest";
import {
  scanForSecrets,
  redactSecrets,
  deTokenize,
  verifyNoPlaintextSecrets,
} from "../../src/enforcement/redaction.js";

// ── Scanner Tests ───────────────────────────────────────────────────────────

describe("Secret Scanner", () => {
  it("detects AWS access keys", () => {
    const result = scanForSecrets("My key is AKIAIOSFODNN7EXAMPLE");
    expect(result.found).toBe(true);
    expect(result.detections[0].type).toBe("aws_access_key");
    expect(result.detections[0].severity).toBe("critical");
  });

  it("detects GitHub tokens", () => {
    const result = scanForSecrets("token: ghp_ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghij");
    expect(result.found).toBe(true);
    expect(result.detections[0].type).toBe("github_token");
  });

  it("detects JWT tokens", () => {
    const result = scanForSecrets("Authorization: Bearer eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJzdWIiOiIxMjM0NTY3ODkwIn0.dozjgNryP4J3jVmNHl0w5N_XgL0n3I9PlFUP0THsR8U");
    expect(result.found).toBe(true);
    expect(result.detections.some(d => d.type === "jwt_token" || d.type === "bearer_token")).toBe(true);
  });

  it("detects private keys", () => {
    const result = scanForSecrets("-----BEGIN RSA PRIVATE KEY-----\nMIIEowIBAAKCAQ...\n-----END RSA PRIVATE KEY-----");
    expect(result.found).toBe(true);
    expect(result.detections[0].type).toBe("private_key");
    expect(result.detections[0].severity).toBe("critical");
  });

  it("detects database connection strings", () => {
    const result = scanForSecrets("DATABASE_URL=postgresql://admin:secretpass@db.corp.com:5432/production");
    expect(result.found).toBe(true);
    expect(result.detections.some(d => d.type === "database_url")).toBe(true);
  });

  it("detects email addresses", () => {
    const result = scanForSecrets("Contact: john.doe@company.com for support");
    expect(result.found).toBe(true);
    expect(result.detections[0].type).toBe("email");
    expect(result.detections[0].category).toBe("pii");
  });

  it("detects SSN", () => {
    const result = scanForSecrets("SSN: 123-45-6789");
    expect(result.found).toBe(true);
    expect(result.detections[0].type).toBe("ssn");
    expect(result.detections[0].severity).toBe("critical");
  });

  it("detects credit card numbers", () => {
    const result = scanForSecrets("Payment card number is 4111111111111111 on file");
    expect(result.found).toBe(true);
    expect(result.detections.some(d => d.type === "credit_card")).toBe(true);
  });

  it("detects password assignments", () => {
    const result = scanForSecrets('password = "my_super_secret_password"');
    expect(result.found).toBe(true);
    expect(result.detections.some(d => d.type === "password_assignment")).toBe(true);
  });

  it("detects Stripe keys", () => {
    // Built at runtime so no live-key-shaped literal sits in the source tree.
    const fakeStripeKey = "sk_live_" + "0".repeat(30);
    const result = scanForSecrets(fakeStripeKey);
    expect(result.found).toBe(true);
    expect(result.detections[0].type).toBe("stripe_key");
  });

  it("returns clean for normal code", () => {
    const result = scanForSecrets("function hello() { return 'world'; }");
    expect(result.found).toBe(false);
    expect(result.total).toBe(0);
  });

  it("counts categories correctly", () => {
    const text = "key: AKIAIOSFODNN7EXAMPLE, email: test@test.com, ssn: 123-45-6789";
    const result = scanForSecrets(text);
    expect(result.categories.credential).toBeGreaterThanOrEqual(1);
    expect(result.categories.pii).toBeGreaterThanOrEqual(1);
  });

  it("detects multiple secrets in one text", () => {
    const text = `
      AWS_ACCESS_KEY=AKIAIOSFODNN7EXAMPLE
      DATABASE_URL=postgresql://admin:pass@db:5432/prod
      API_KEY="sk_live_${"0".repeat(30)}"
    `;
    const result = scanForSecrets(text);
    expect(result.total).toBeGreaterThanOrEqual(3);
  });
});

// ── Redaction Tests ─────────────────────────────────────────────────────────

describe("Secret Redaction", () => {
  describe("mask mode", () => {
    it("replaces secrets with [REDACTED:TYPE:hash]", () => {
      const result = redactSecrets("My key: AKIAIOSFODNN7EXAMPLE", "mask");
      expect(result.redacted_text).not.toContain("AKIAIOSFODNN7EXAMPLE");
      expect(result.redacted_text).toContain("[REDACTED:AWS_ACCESS_KEY:");
      expect(result.redaction_count).toBe(1);
    });

    it("redacts multiple secrets", () => {
      const text = "key=AKIAIOSFODNN7EXAMPLE email=test@company.com";
      const result = redactSecrets(text, "mask");
      expect(result.redacted_text).not.toContain("AKIAIOSFODNN7EXAMPLE");
      expect(result.redacted_text).not.toContain("test@company.com");
      expect(result.redaction_count).toBeGreaterThanOrEqual(2);
    });

    it("preserves non-secret text", () => {
      const result = redactSecrets("Hello AKIAIOSFODNN7EXAMPLE world", "mask");
      expect(result.redacted_text).toContain("Hello");
      expect(result.redacted_text).toContain("world");
    });
  });

  describe("tokenize mode", () => {
    it("replaces secrets with reversible tokens", () => {
      const result = redactSecrets("My key: AKIAIOSFODNN7EXAMPLE", "tokenize");
      expect(result.redacted_text).not.toContain("AKIAIOSFODNN7EXAMPLE");
      expect(result.tokens.length).toBe(1);
      expect(result.tokens[0]).toMatch(/\[REDACTED:AWS_ACCESS_KEY:[A-Z]+\]/);
    });

    it("tokens can be de-tokenized", () => {
      const original = "Connect with AKIAIOSFODNN7EXAMPLE to AWS";
      const redacted = redactSecrets(original, "tokenize");
      expect(redacted.redacted_text).not.toContain("AKIAIOSFODNN7EXAMPLE");

      const restored = deTokenize(redacted.redacted_text);
      expect(restored.text).toContain("AKIAIOSFODNN7EXAMPLE");
      expect(restored.restored_count).toBe(1);
    });
  });

  describe("summarize mode", () => {
    it("replaces secrets with descriptions", () => {
      const result = redactSecrets("My key: AKIAIOSFODNN7EXAMPLE", "summarize");
      expect(result.redacted_text).toContain("[contains AWS Access Key ID]");
      expect(result.redacted_text).not.toContain("AKIAIOSFODNN7EXAMPLE");
    });
  });
});

// ── Verification Tests ──────────────────────────────────────────────────────

describe("Plaintext Secret Verification", () => {
  it("flags text containing secrets", () => {
    const result = verifyNoPlaintextSecrets("key: AKIAIOSFODNN7EXAMPLE");
    expect(result.clean).toBe(false);
    expect(result.violations.length).toBeGreaterThan(0);
  });

  it("passes clean text", () => {
    const result = verifyNoPlaintextSecrets("function add(a, b) { return a + b; }");
    expect(result.clean).toBe(true);
    expect(result.violations.length).toBe(0);
  });

  it("passes already-redacted text", () => {
    const result = verifyNoPlaintextSecrets("key: [REDACTED:AWS_ACCESS_KEY:abc123]");
    expect(result.clean).toBe(true);
  });
});
