/**
 * Author: Deepankar Das
 */

/**
 * Enforcer — Package Guard + Secret Detector Tests
 */

import { describe, it, expect } from "vitest";
import { detectPackageInstall } from "../../src/enforcement/package-guard.js";
import {
  detectSensitiveFilePath,
  detectSecretCommandAccess,
  detectSecretAccess,
} from "../../src/enforcement/secret-detector.js";

// ── Package Install Detection ───────────────────────────────────────────────

describe("Package Install Detection", () => {
  it("detects npm install", () => {
    const result = detectPackageInstall("npm install express");
    expect(result.isPackageInstall).toBe(true);
    expect(result.packageManager).toBe("npm");
    expect(result.packageName).toBe("express");
    expect(result.registry).toBe("registry.npmjs.org");
  });

  it("detects npm i shorthand", () => {
    const result = detectPackageInstall("npm i lodash");
    expect(result.isPackageInstall).toBe(true);
    expect(result.packageName).toBe("lodash");
  });

  it("detects pip install", () => {
    const result = detectPackageInstall("pip install requests");
    expect(result.isPackageInstall).toBe(true);
    expect(result.packageManager).toBe("pip");
    expect(result.registry).toBe("pypi.org");
  });

  it("detects brew install", () => {
    const result = detectPackageInstall("brew install postgresql");
    expect(result.isPackageInstall).toBe(true);
    expect(result.packageManager).toBe("brew");
  });

  it("detects yarn add", () => {
    const result = detectPackageInstall("yarn add react");
    expect(result.isPackageInstall).toBe(true);
    expect(result.packageManager).toBe("yarn");
  });

  it("does not detect npm test as package install", () => {
    const result = detectPackageInstall("npm test");
    expect(result.isPackageInstall).toBe(false);
  });

  it("does not detect npm run build as package install", () => {
    const result = detectPackageInstall("npm run build");
    expect(result.isPackageInstall).toBe(false);
  });

  it("does not detect git push as package install", () => {
    const result = detectPackageInstall("git push origin main");
    expect(result.isPackageInstall).toBe(false);
  });
});

// ── Sensitive File Path Detection ───────────────────────────────────────────

describe("Sensitive File Path Detection", () => {
  it("detects SSH key access", () => {
    const result = detectSensitiveFilePath("~/.ssh/id_rsa");
    expect(result.isSensitive).toBe(true);
    expect(result.type).toBe("file");
    expect(result.severity).toBe("high");
    expect(result.description).toContain("SSH");
  });

  it("detects AWS credentials access", () => {
    const result = detectSensitiveFilePath("~/.aws/credentials");
    expect(result.isSensitive).toBe(true);
    expect(result.description).toContain("AWS");
  });

  it("detects .env file access", () => {
    const result = detectSensitiveFilePath("/Users/dev/project/.env");
    expect(result.isSensitive).toBe(true);
    expect(result.matchedPattern).toBe(".env");
  });

  it("detects .pem key file access", () => {
    const result = detectSensitiveFilePath("/tmp/server.pem");
    expect(result.isSensitive).toBe(true);
    expect(result.description).toContain("key/certificate");
  });

  it("detects credentials.json access", () => {
    const result = detectSensitiveFilePath("/home/user/credentials.json");
    expect(result.isSensitive).toBe(true);
  });

  it("does not flag normal source files", () => {
    const result = detectSensitiveFilePath("/Users/dev/project/src/index.ts");
    expect(result.isSensitive).toBe(false);
  });

  it("does not flag package.json", () => {
    const result = detectSensitiveFilePath("/Users/dev/project/package.json");
    expect(result.isSensitive).toBe(false);
  });
});

// ── Secret Command Detection ────────────────────────────────────────────────

describe("Secret Command Detection", () => {
  it("detects cat .env", () => {
    const result = detectSecretCommandAccess("cat .env");
    expect(result.isSensitive).toBe(true);
    expect(result.type).toBe("command");
  });

  it("detects printenv", () => {
    const result = detectSecretCommandAccess("printenv");
    expect(result.isSensitive).toBe(true);
  });

  it("detects aws configure", () => {
    const result = detectSecretCommandAccess("aws configure");
    expect(result.isSensitive).toBe(true);
  });

  it("detects echo $API_KEY", () => {
    const result = detectSecretCommandAccess("echo $API_KEY");
    expect(result.isSensitive).toBe(true);
    expect(result.type).toBe("env_var");
    expect(result.matchedPattern).toBe("API_KEY");
  });

  it("detects echo ${SECRET_KEY}", () => {
    const result = detectSecretCommandAccess("echo ${SECRET_KEY}");
    expect(result.isSensitive).toBe(true);
    expect(result.type).toBe("env_var");
  });

  it("does not flag npm test", () => {
    const result = detectSecretCommandAccess("npm test");
    expect(result.isSensitive).toBe(false);
  });

  it("does not flag ls -la", () => {
    const result = detectSecretCommandAccess("ls -la");
    expect(result.isSensitive).toBe(false);
  });
});

// ── Combined Detection ──────────────────────────────────────────────────────

describe("Combined Secret Access Detection", () => {
  it("detects via file path", () => {
    const result = detectSecretAccess("~/.ssh/id_rsa", undefined);
    expect(result.isSensitive).toBe(true);
  });

  it("detects via command", () => {
    const result = detectSecretAccess(undefined, "cat .env");
    expect(result.isSensitive).toBe(true);
  });

  it("file path takes priority over command", () => {
    const result = detectSecretAccess("~/.aws/credentials", "ls");
    expect(result.isSensitive).toBe(true);
    expect(result.type).toBe("file");
  });

  it("returns not sensitive when neither matches", () => {
    const result = detectSecretAccess("/Users/dev/project/src/index.ts", "npm test");
    expect(result.isSensitive).toBe(false);
  });
});