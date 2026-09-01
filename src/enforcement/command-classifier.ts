/**
 * Author: Deepankar Das
 */

/**
 * Enforcer — Command Classifier
 *
 * Classifies shell commands into risk categories by prefix matching.
 * Used by the shell proxy to tag ActionRequests with resource classification.
 *
 * TDD Reference: Section 6, Implementation Plan A4
 * PRD Reference: Appendix C, R-1
 */

import type { ResourceClassification } from "../../types/action.js";

interface ClassificationRule {
  patterns: string[];
  classification: ResourceClassification;
}

const CLASSIFICATION_RULES: ClassificationRule[] = [
  {
    classification: "destructive",
    patterns: [
      "rm -rf",
      "rm -r",
      "rmdir",
      "git push --force",
      "git push -f",
      "git reset --hard",
      "git clean -fd",
      "git checkout -- .",
      "chmod",
      "chown",
      "kill -9",
      "kill -KILL",
      "pkill",
      "killall",
      "mkfs",
      "dd if=",
      "truncate",
      "> /dev/",
    ],
  },
  {
    classification: "network_tool",
    patterns: [
      "curl ",
      "curl\t",
      "wget ",
      "wget\t",
      "ssh ",
      "scp ",
      "sftp ",
      "nc ",
      "ncat ",
      "nmap ",
      "telnet ",
      "ftp ",
      "rsync ",
    ],
  },
  {
    classification: "package_manager",
    patterns: [
      "npm install",
      "npm i ",
      "npm add",
      "npm uninstall",
      "npm remove",
      "yarn add",
      "yarn remove",
      "pnpm add",
      "pnpm remove",
      "pip install",
      "pip3 install",
      "pip uninstall",
      "brew install",
      "brew uninstall",
      "apt install",
      "apt-get install",
      "apt remove",
      "apt-get remove",
      "gem install",
      "cargo install",
      "go install",
    ],
  },
];

/**
 * Classify a shell command string into risk categories.
 * Returns an array of matching classifications.
 * If no patterns match, returns ["safe"].
 *
 * For compound commands (e.g., "rm -rf node_modules && npm install"),
 * all components are classified and the most dangerous tags are included.
 */
export function classifyCommand(command: string): ResourceClassification[] {
  const normalizedCommand = command.trim().toLowerCase();
  const classifications = new Set<ResourceClassification>();

  // Split compound commands on && || ; | and classify each segment
  const segments = normalizedCommand.split(/\s*(?:&&|\|\||[;|])\s*/);

  for (const segment of segments) {
    const trimmed = segment.trim();
    if (trimmed.length === 0) continue;

    for (const rule of CLASSIFICATION_RULES) {
      for (const pattern of rule.patterns) {
        if (trimmed.startsWith(pattern) || trimmed.includes(pattern)) {
          classifications.add(rule.classification);
        }
      }
    }
  }

  if (classifications.size === 0) {
    return ["safe"];
  }

  return Array.from(classifications);
}

/**
 * Check if a command has any classification matching the given tag.
 */
export function commandHasClassification(
  command: string,
  tag: ResourceClassification,
): boolean {
  return classifyCommand(command).includes(tag);
}