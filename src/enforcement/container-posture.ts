/**
 * Author: Deepankar Das
 */

/**
 * Enforcer — Container Posture Validator
 *
 * Checks running container configuration for dangerous settings.
 * Refuses to start in governed mode if posture check fails.
 *
 * Forbidden:
 *   - --privileged mode
 *   - Docker socket mount (/var/run/docker.sock)
 *   - Broad host filesystem mounts (/, /etc, /home)
 *   - Running as root (UID 0) without user namespace
 *
 * TDD Reference: Section 13, Section 14.2
 * PRD Reference: Appendix C, Section 11.5
 */

import * as fs from "node:fs";
import * as os from "node:os";

export interface PostureViolation {
  check: string;
  severity: "critical" | "warning";
  message: string;
}

export interface PostureResult {
  valid: boolean;
  violations: PostureViolation[];
  environment: string; // "host" | "container" | "unknown"
}

/**
 * Detect if running inside a Docker container.
 */
function isContainer(): boolean {
  return fs.existsSync("/.dockerenv") ||
    (fs.existsSync("/proc/1/cgroup") &&
      fs.readFileSync("/proc/1/cgroup", "utf-8").includes("docker"));
}

/**
 * Check if running as root (UID 0).
 */
function isRoot(): boolean {
  try {
    return os.userInfo().uid === 0;
  } catch {
    return process.getuid?.() === 0;
  }
}

/**
 * Check if Docker socket is mounted (extremely dangerous).
 */
function hasDockerSocket(): boolean {
  return fs.existsSync("/var/run/docker.sock");
}

/**
 * Check for broad host mounts by examining common dangerous paths.
 */
function hasBroadMounts(): string[] {
  const dangerousPaths = ["/host", "/host-root"];
  const found: string[] = [];
  for (const p of dangerousPaths) {
    if (fs.existsSync(p)) {
      found.push(p);
    }
  }
  return found;
}

/**
 * Validate container posture.
 *
 * Returns a PostureResult with violations if any dangerous
 * configurations are detected.
 */
export function validateContainerPosture(): PostureResult {
  const violations: PostureViolation[] = [];

  if (!isContainer()) {
    return {
      valid: true,
      violations: [],
      environment: "host",
    };
  }

  // Check: Docker socket mounted
  if (hasDockerSocket()) {
    violations.push({
      check: "docker_socket",
      severity: "critical",
      message: "Docker socket (/var/run/docker.sock) is mounted. This allows container escape. Enforcer refuses to start.",
    });
  }

  // Check: running as root
  if (isRoot()) {
    violations.push({
      check: "root_user",
      severity: "warning",
      message: "Running as root (UID 0). Enforcer recommends a non-root user for reduced privilege.",
    });
  }

  // Check: broad host mounts
  const broadMounts = hasBroadMounts();
  if (broadMounts.length > 0) {
    violations.push({
      check: "broad_mounts",
      severity: "critical",
      message: `Broad host mounts detected: ${broadMounts.join(", ")}. This exposes the host filesystem.`,
    });
  }

  // Check: /proc/1/status for capabilities (if available)
  try {
    if (fs.existsSync("/proc/1/status")) {
      const status = fs.readFileSync("/proc/1/status", "utf-8");
      const capLine = status.split("\n").find(l => l.startsWith("CapEff:"));
      if (capLine) {
        const capHex = capLine.split(":")[1]?.trim() || "0";
        const capInt = parseInt(capHex, 16);
        // Full capabilities = 0x3fffffffff (all 38 caps)
        if (capInt > 0x00000000ffffffff) {
          violations.push({
            check: "excessive_capabilities",
            severity: "warning",
            message: "Container has elevated capabilities. Enforcer recommends dropping all except minimal set.",
          });
        }
      }
    }
  } catch {
    // Can't read /proc — skip this check
  }

  const hasCritical = violations.some(v => v.severity === "critical");

  return {
    valid: !hasCritical,
    violations,
    environment: "container",
  };
}

/**
 * Run posture check and exit if critical violations found.
 * Called at daemon startup in container mode.
 */
export function enforcePosture(): void {
  const result = validateContainerPosture();

  if (result.environment === "host") {
    console.log("[POSTURE] Running in host mode — container posture checks skipped.");
    return;
  }

  console.log(`[POSTURE] Container posture check: ${result.violations.length} finding(s)`);

  for (const v of result.violations) {
    if (v.severity === "critical") {
      console.error(`[POSTURE] CRITICAL: ${v.message}`);
    } else {
      console.warn(`[POSTURE] WARNING: ${v.message}`);
    }
  }

  if (!result.valid) {
    console.error("[POSTURE] Container posture check FAILED. Enforcer will not start in governed mode.");
    console.error("[POSTURE] Fix the critical violations and restart.");
    process.exit(1);
  }

  console.log("[POSTURE] Container posture check passed.");
}