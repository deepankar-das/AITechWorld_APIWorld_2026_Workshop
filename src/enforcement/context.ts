/**
 * Author: Deepankar Das
 */

/**
 * Enforcer — Enforcement Context Builder
 *
 * Reads workspace path, repo name, branch, environment tier, and
 * deployment mode from environment variables and git metadata.
 * Attached to every ActionRequest.
 *
 * TDD Reference: Implementation Plan A8
 */

import * as fs from "node:fs";
import * as path from "node:path";
import { execSync } from "node:child_process";
import type { Environment } from "../../types/action.js";

/**
 * Detect git repo name from the .git directory or remote URL.
 */
function detectRepo(workspace: string): string {
  try {
    const remote = execSync("git remote get-url origin", {
      cwd: workspace,
      encoding: "utf-8",
      timeout: 3000,
    }).trim();
    // Extract org/repo from git URL
    const match = remote.match(/[:/]([^/]+\/[^/]+?)(?:\.git)?$/);
    return match ? match[1] : "";
  } catch {
    return "";
  }
}

/**
 * Detect current git branch.
 */
function detectBranch(workspace: string): string {
  try {
    return execSync("git branch --show-current", {
      cwd: workspace,
      encoding: "utf-8",
      timeout: 3000,
    }).trim();
  } catch {
    return "";
  }
}

/**
 * Detect deployment mode from environment.
 */
function detectDeploymentMode(): string {
  // Check if running inside Docker
  if (fs.existsSync("/.dockerenv")) {
    return "container";
  }
  // Check for remote workspace markers
  if (process.env.REMOTE_WORKSPACE || process.env.CODESPACE_NAME) {
    return "remote";
  }
  return "host";
}

/**
 * Build enforcement context from environment and git metadata.
 */
export function buildEnforcementContext(workspaceOverride?: string): Environment {
  const workspace = workspaceOverride || process.env.AA_WORKSPACE || process.cwd();
  const absoluteWorkspace = path.resolve(workspace);

  return {
    workspace: absoluteWorkspace,
    repo: process.env.AA_REPO || detectRepo(absoluteWorkspace),
    branch: process.env.AA_BRANCH || detectBranch(absoluteWorkspace),
    tier: process.env.AA_TIER || process.env.NODE_ENV || "development",
    deployment_mode: process.env.AA_DEPLOYMENT_MODE || detectDeploymentMode(),
  };
}