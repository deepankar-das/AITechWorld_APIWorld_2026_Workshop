/**
 * Author: Deepankar Das
 */

/**
 * Enforcer — Package Install Guard
 *
 * Intercepts package manager operations (npm install, pip install, brew install, etc.)
 * and routes them through the daemon for policy evaluation.
 *
 * The command classifier (F15) already tags these as "package_manager".
 * This guard adds specific policy evaluation for package install actions
 * and integrates with the approval workflow.
 *
 * Venture Prompt: "require approval before installing packages"
 */

import { v4 as uuidv4 } from "uuid";
import { classifyCommand } from "./command-classifier.js";
import type { ActionRequest } from "../../types/action.js";
import type { PolicyDecision } from "../../types/policy.js";
import type { Environment } from "../../types/action.js";

export interface PackageGuardOptions {
  daemonUrl: string;
  actor: {
    user_id: string;
    agent_type: string;
    agent_instance: string;
    session_id: string;
  };
  environment: Environment;
}

/**
 * Known package manager commands and their registries.
 */
const PACKAGE_MANAGERS: Record<string, { registry: string; extractPackage: (cmd: string) => string }> = {
  "npm install": { registry: "registry.npmjs.org", extractPackage: (cmd) => cmd.replace(/^npm\s+i(nstall)?\s+/, "").split(/\s/)[0] || "unknown" },
  "npm i ": { registry: "registry.npmjs.org", extractPackage: (cmd) => cmd.replace(/^npm\s+i\s+/, "").split(/\s/)[0] || "unknown" },
  "yarn add": { registry: "registry.yarnpkg.com", extractPackage: (cmd) => cmd.replace(/^yarn\s+add\s+/, "").split(/\s/)[0] || "unknown" },
  "pnpm add": { registry: "registry.npmjs.org", extractPackage: (cmd) => cmd.replace(/^pnpm\s+add\s+/, "").split(/\s/)[0] || "unknown" },
  "pip install": { registry: "pypi.org", extractPackage: (cmd) => cmd.replace(/^pip3?\s+install\s+/, "").split(/\s/)[0] || "unknown" },
  "pip3 install": { registry: "pypi.org", extractPackage: (cmd) => cmd.replace(/^pip3\s+install\s+/, "").split(/\s/)[0] || "unknown" },
  "brew install": { registry: "formulae.brew.sh", extractPackage: (cmd) => cmd.replace(/^brew\s+install\s+/, "").split(/\s/)[0] || "unknown" },
  "gem install": { registry: "rubygems.org", extractPackage: (cmd) => cmd.replace(/^gem\s+install\s+/, "").split(/\s/)[0] || "unknown" },
  "cargo install": { registry: "crates.io", extractPackage: (cmd) => cmd.replace(/^cargo\s+install\s+/, "").split(/\s/)[0] || "unknown" },
};

/**
 * Detect if a command is a package install and extract details.
 */
export function detectPackageInstall(command: string): {
  isPackageInstall: boolean;
  packageManager?: string;
  packageName?: string;
  registry?: string;
} {
  const normalized = command.trim().toLowerCase();

  for (const [prefix, info] of Object.entries(PACKAGE_MANAGERS)) {
    if (normalized.startsWith(prefix)) {
      return {
        isPackageInstall: true,
        packageManager: prefix.split(" ")[0],
        packageName: info.extractPackage(command.trim()),
        registry: info.registry,
      };
    }
  }

  return { isPackageInstall: false };
}

/**
 * Intercept a package install command and evaluate against policy.
 *
 * Returns PolicyDecision. Package installs are typically routed to
 * require_approval unless from a trusted registry.
 */
export async function interceptPackageInstall(
  command: string,
  cwd: string,
  options: PackageGuardOptions,
): Promise<PolicyDecision> {
  const pkgInfo = detectPackageInstall(command);
  const classifications = classifyCommand(command);

  const request: ActionRequest = {
    request_id: uuidv4(),
    timestamp: new Date().toISOString(),
    actor: options.actor,
    environment: options.environment,
    action: {
      type: "package.install",
      attempted_action: command,
    },
    resource: {
      kind: "command",
      value: command,
      path: cwd,
      host: pkgInfo.registry,
      classification: classifications,
    },
  };

  try {
    const response = await fetch(`${options.daemonUrl}/v1/evaluate`, {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify(request),
    });

    if (!response.ok) {
      return {
        request_id: request.request_id,
        decision: "deny",
        reason_code: "DAEMON_ERROR",
        reason_human: `Daemon returned ${response.status}. Fail-closed: package install denied.`,
        policy_id: "system.fail_closed",
        policy_version: "unknown",
        approval_required: false,
      };
    }

    return response.json() as Promise<PolicyDecision>;
  } catch {
    return {
      request_id: uuidv4(),
      decision: "deny",
      reason_code: "DAEMON_UNREACHABLE",
      reason_human: "Daemon unreachable. Fail-closed: package install denied.",
      policy_id: "system.fail_closed",
      policy_version: "unknown",
      approval_required: false,
    };
  }
}