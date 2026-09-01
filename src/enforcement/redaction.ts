/**
 * Author: Deepankar Das
 */

/**
 * Enforcer — Secrets/PII Redaction Engine
 *
 * Scans text for secrets, credentials, PII, and sensitive data.
 * Replaces detected values with safe placeholder tokens.
 * Supports de-tokenization for approved access.
 *
 * This completes depth area 4c from the venture prompt:
 * "secrets/PII redaction in agent context"
 *
 * Redaction modes:
 *   - mask:       Replace with [REDACTED:TYPE:hash]
 *   - tokenize:   Replace with reversible token (can de-tokenize)
 *   - summarize:  Replace with description ("an AWS access key")
 *   - deny:       Block the entire action (handled by policy, not here)
 */

import * as crypto from "node:crypto";

// ── Detection Patterns ──────────────────────────────────────────────────────

interface DetectionPattern {
  type: string;
  category: "secret" | "pii" | "credential" | "sensitive";
  pattern: RegExp;
  description: string;
  severity: "critical" | "high" | "medium";
}

const DETECTION_PATTERNS: DetectionPattern[] = [
  // API Keys and Tokens
  { type: "aws_access_key", category: "credential", pattern: /AKIA[0-9A-Z]{16}/g, description: "AWS Access Key ID", severity: "critical" },
  { type: "aws_secret_key", category: "credential", pattern: /(?:aws_secret_access_key|AWS_SECRET_ACCESS_KEY)\s*[=:]\s*["']?([A-Za-z0-9/+=]{40})["']?/g, description: "AWS Secret Access Key", severity: "critical" },
  { type: "github_token", category: "credential", pattern: /gh[ps]_[A-Za-z0-9_]{36,255}/g, description: "GitHub Token", severity: "critical" },
  { type: "github_pat", category: "credential", pattern: /github_pat_[A-Za-z0-9_]{22,255}/g, description: "GitHub Personal Access Token", severity: "critical" },
  { type: "slack_token", category: "credential", pattern: /xox[baprs]-[0-9]{10,13}-[0-9]{10,13}-[a-zA-Z0-9]{24,34}/g, description: "Slack Token", severity: "critical" },
  { type: "stripe_key", category: "credential", pattern: /sk_live_[0-9a-zA-Z]{24,99}/g, description: "Stripe Secret Key", severity: "critical" },
  { type: "generic_api_key", category: "credential", pattern: /(?:api[_-]?key|apikey|api_secret)\s*[=:]\s*["']?([A-Za-z0-9_\-]{20,64})["']?/gi, description: "API Key", severity: "high" },
  { type: "bearer_token", category: "credential", pattern: /Bearer\s+[A-Za-z0-9\-._~+/]+=*/g, description: "Bearer Token", severity: "high" },
  { type: "jwt_token", category: "credential", pattern: /eyJ[A-Za-z0-9_-]{10,}\.[A-Za-z0-9_-]{10,}\.[A-Za-z0-9_-]{10,}/g, description: "JWT Token", severity: "high" },

  // Private Keys
  { type: "private_key", category: "secret", pattern: /-----BEGIN (?:RSA |EC |DSA |OPENSSH )?PRIVATE KEY-----[\s\S]*?-----END (?:RSA |EC |DSA |OPENSSH )?PRIVATE KEY-----/g, description: "Private Key", severity: "critical" },
  { type: "pgp_private", category: "secret", pattern: /-----BEGIN PGP PRIVATE KEY BLOCK-----[\s\S]*?-----END PGP PRIVATE KEY BLOCK-----/g, description: "PGP Private Key", severity: "critical" },

  // Connection Strings
  { type: "database_url", category: "credential", pattern: /(?:postgres(?:ql)?|mysql|mongodb(?:\+srv)?|redis):\/\/[^\s"']+/g, description: "Database Connection String", severity: "critical" },
  { type: "connection_string", category: "credential", pattern: /(?:Server|Data Source)=[^;]+;.*(?:Password|Pwd)=[^;]+/gi, description: "Connection String with Password", severity: "critical" },

  // Passwords
  { type: "password_assignment", category: "credential", pattern: /(?:password|passwd|pwd|secret)\s*[=:]\s*["']([^"']{8,})["']/gi, description: "Password Assignment", severity: "critical" },

  // PII - Email
  { type: "email", category: "pii", pattern: /[a-zA-Z0-9._%+-]+@[a-zA-Z0-9.-]+\.[a-zA-Z]{2,}/g, description: "Email Address", severity: "medium" },

  // PII - Phone
  { type: "phone", category: "pii", pattern: /(?:\+1[-.\s]?)?\(?\d{3}\)?[-.\s]?\d{3}[-.\s]?\d{4}/g, description: "Phone Number", severity: "medium" },

  // PII - SSN
  { type: "ssn", category: "pii", pattern: /\b\d{3}-\d{2}-\d{4}\b/g, description: "Social Security Number", severity: "critical" },

  // PII - Credit Card
  { type: "credit_card", category: "pii", pattern: /\b(?:4[0-9]{12}(?:[0-9]{3})?|5[1-5][0-9]{14}|3[47][0-9]{13}|6(?:011|5[0-9]{2})[0-9]{12})\b/g, description: "Credit Card Number", severity: "critical" },

  // IP Addresses (private ranges — might be infrastructure)
  { type: "private_ip", category: "sensitive", pattern: /\b(?:10\.\d{1,3}\.\d{1,3}\.\d{1,3}|172\.(?:1[6-9]|2\d|3[01])\.\d{1,3}\.\d{1,3}|192\.168\.\d{1,3}\.\d{1,3})\b/g, description: "Private IP Address", severity: "medium" },

  // Environment variable values
  { type: "env_value", category: "secret", pattern: /(?:SECRET|TOKEN|KEY|PASSWORD|CREDENTIAL|AUTH)_?[A-Z_]*\s*=\s*["']?([^\s"']{8,})["']?/g, description: "Environment Variable Secret", severity: "high" },
];

// ── Token Store (for reversible tokenization) ───────────────────────────────

interface TokenEntry {
  token: string;
  original: string;
  type: string;
  created_at: string;
  expires_at: string;
}

const tokenStore = new Map<string, TokenEntry>();

function generateToken(type: string): string {
  // Use letters-only token IDs so generated placeholders never look like
  // phone numbers or other numeric-sensitive patterns that could be re-redacted.
  const alphabet = "ABCDEFGHIJKLMNOPQRSTUVWXYZ";
  const bytes = crypto.randomBytes(12);
  let tokenId = "";
  for (const byte of bytes) {
    tokenId += alphabet[byte % alphabet.length];
  }
  return `[REDACTED:${type.toUpperCase()}:${tokenId}]`;
}

// ── Scanner ─────────────────────────────────────────────────────────────────

export interface ScanResult {
  found: boolean;
  detections: Array<{
    type: string;
    category: string;
    description: string;
    severity: string;
    match: string;
    position: number;
    length: number;
  }>;
  total: number;
  categories: Record<string, number>;
}

/**
 * Scan text for secrets, credentials, and PII.
 */
export function scanForSecrets(text: string): ScanResult {
  const detections: ScanResult["detections"] = [];
  const categories: Record<string, number> = {};

  for (const pattern of DETECTION_PATTERNS) {
    // Reset regex lastIndex for global patterns
    pattern.pattern.lastIndex = 0;
    let match: RegExpExecArray | null;

    while ((match = pattern.pattern.exec(text)) !== null) {
      detections.push({
        type: pattern.type,
        category: pattern.category,
        description: pattern.description,
        severity: pattern.severity,
        match: match[0].slice(0, 20) + (match[0].length > 20 ? "..." : ""),  // Truncate for safety
        position: match.index,
        length: match[0].length,
      });
      categories[pattern.category] = (categories[pattern.category] || 0) + 1;
    }
  }

  return {
    found: detections.length > 0,
    detections,
    total: detections.length,
    categories,
  };
}

// ── Tokenizer (replace secrets with safe tokens) ────────────────────────────

export type RedactionMode = "mask" | "tokenize" | "summarize";

export interface RedactionResult {
  redacted_text: string;
  redaction_count: number;
  tokens: string[];              // List of tokens created (for tokenize mode)
  detections: ScanResult["detections"];
}

/**
 * Redact secrets and PII from text.
 *
 * Modes:
 *   - mask: Replace with [REDACTED:TYPE:hash] (irreversible)
 *   - tokenize: Replace with reversible token (can de-tokenize with token)
 *   - summarize: Replace with description ("contains an AWS key")
 */
export function redactSecrets(
  text: string,
  mode: RedactionMode = "mask",
  ttlMinutes: number = 30,
): RedactionResult {
  let result = text;
  const tokens: string[] = [];
  const allDetections: ScanResult["detections"] = [];
  let count = 0;

  for (const pattern of DETECTION_PATTERNS) {
    pattern.pattern.lastIndex = 0;

    result = result.replace(pattern.pattern, (match) => {
      count++;

      allDetections.push({
        type: pattern.type,
        category: pattern.category,
        description: pattern.description,
        severity: pattern.severity,
        match: match.slice(0, 10) + "***",
        position: 0,
        length: match.length,
      });

      switch (mode) {
        case "mask": {
          const hash = crypto.createHash("sha256").update(match).digest("hex").slice(0, 8);
          return `[REDACTED:${pattern.type.toUpperCase()}:${hash}]`;
        }
        case "tokenize": {
          const token = generateToken(pattern.type);
          const expiresAt = new Date(Date.now() + ttlMinutes * 60 * 1000).toISOString();
          tokenStore.set(token, {
            token,
            original: match,
            type: pattern.type,
            created_at: new Date().toISOString(),
            expires_at: expiresAt,
          });
          tokens.push(token);
          return token;
        }
        case "summarize": {
          return `[contains ${pattern.description}]`;
        }
      }
    });
  }

  return {
    redacted_text: result,
    redaction_count: count,
    tokens,
    detections: allDetections,
  };
}

// ── De-tokenizer (restore originals for approved access) ────────────────────

export interface DeTokenResult {
  text: string;
  restored_count: number;
  expired_tokens: string[];
}

/**
 * Restore redacted tokens to their original values.
 * Only works for tokens created with "tokenize" mode.
 * Tokens expire after TTL.
 */
export function deTokenize(text: string): DeTokenResult {
  let result = text;
  let restored = 0;
  const expired: string[] = [];
  const now = Date.now();

  for (const [token, entry] of tokenStore.entries()) {
    if (result.includes(token)) {
      if (new Date(entry.expires_at).getTime() < now) {
        expired.push(token);
        // Don't restore expired tokens — leave redacted
      } else {
        result = result.replace(token, entry.original);
        restored++;
      }
    }
  }

  // Clean up expired tokens
  for (const token of expired) {
    tokenStore.delete(token);
  }

  return {
    text: result,
    restored_count: restored,
    expired_tokens: expired,
  };
}

/**
 * Check if text contains any unredacted secrets.
 * Used as a verification step before sending to LLM or external service.
 */
export function verifyNoPlaintextSecrets(text: string): {
  clean: boolean;
  violations: Array<{ type: string; description: string; severity: string }>;
} {
  const scan = scanForSecrets(text);
  return {
    clean: !scan.found,
    violations: scan.detections.map(d => ({
      type: d.type,
      description: d.description,
      severity: d.severity,
    })),
  };
}

/**
 * Get redaction metrics.
 */
export function getRedactionMetrics(): {
  active_tokens: number;
  patterns: number;
  categories: string[];
} {
  // Clean expired tokens
  const now = Date.now();
  for (const [token, entry] of tokenStore.entries()) {
    if (new Date(entry.expires_at).getTime() < now) {
      tokenStore.delete(token);
    }
  }

  return {
    active_tokens: tokenStore.size,
    patterns: DETECTION_PATTERNS.length,
    categories: [...new Set(DETECTION_PATTERNS.map(p => p.category))],
  };
}
