/**
 * Author: Deepankar Das
 */

/**
 * Enforcer — Command Classifier Tests
 *
 * Verifies classification of 20+ commands across all categories.
 * TDD Reference: Appendix B.7 VP-1, Implementation Plan A13
 */

import { describe, it, expect } from "vitest";
import { classifyCommand, commandHasClassification } from "../../src/enforcement/command-classifier.js";

describe("Command Classifier", () => {
  describe("destructive commands", () => {
    it("classifies rm -rf as destructive", () => {
      expect(classifyCommand("rm -rf node_modules")).toContain("destructive");
    });

    it("classifies git push --force as destructive", () => {
      expect(classifyCommand("git push --force origin main")).toContain("destructive");
    });

    it("classifies git push -f as destructive", () => {
      expect(classifyCommand("git push -f")).toContain("destructive");
    });

    it("classifies git reset --hard as destructive", () => {
      expect(classifyCommand("git reset --hard HEAD~1")).toContain("destructive");
    });

    it("classifies chmod as destructive", () => {
      expect(classifyCommand("chmod 777 /tmp/secret")).toContain("destructive");
    });

    it("classifies kill -9 as destructive", () => {
      expect(classifyCommand("kill -9 12345")).toContain("destructive");
    });

    it("classifies pkill as destructive", () => {
      expect(classifyCommand("pkill node")).toContain("destructive");
    });
  });

  describe("network tool commands", () => {
    it("classifies curl as network_tool", () => {
      expect(classifyCommand("curl https://api.example.com")).toContain("network_tool");
    });

    it("classifies wget as network_tool", () => {
      expect(classifyCommand("wget https://file.example.com/data.tar")).toContain("network_tool");
    });

    it("classifies ssh as network_tool", () => {
      expect(classifyCommand("ssh user@server.com")).toContain("network_tool");
    });

    it("classifies nmap as network_tool", () => {
      expect(classifyCommand("nmap -sS 192.168.1.0/24")).toContain("network_tool");
    });
  });

  describe("package manager commands", () => {
    it("classifies npm install as package_manager", () => {
      expect(classifyCommand("npm install express")).toContain("package_manager");
    });

    it("classifies npm i as package_manager", () => {
      expect(classifyCommand("npm i lodash")).toContain("package_manager");
    });

    it("classifies pip install as package_manager", () => {
      expect(classifyCommand("pip install requests")).toContain("package_manager");
    });

    it("classifies brew install as package_manager", () => {
      expect(classifyCommand("brew install postgresql")).toContain("package_manager");
    });

    it("classifies yarn add as package_manager", () => {
      expect(classifyCommand("yarn add react")).toContain("package_manager");
    });
  });

  describe("safe commands", () => {
    it("classifies ls as safe", () => {
      expect(classifyCommand("ls -la")).toEqual(["safe"]);
    });

    it("classifies echo as safe", () => {
      expect(classifyCommand("echo hello")).toEqual(["safe"]);
    });

    it("classifies npm test as safe", () => {
      expect(classifyCommand("npm test")).toEqual(["safe"]);
    });

    it("classifies npm run build as safe", () => {
      expect(classifyCommand("npm run build")).toEqual(["safe"]);
    });

    it("classifies git status as safe", () => {
      expect(classifyCommand("git status")).toEqual(["safe"]);
    });

    it("classifies cat as safe", () => {
      expect(classifyCommand("cat README.md")).toEqual(["safe"]);
    });
  });

  describe("compound commands", () => {
    it("classifies rm -rf && npm install as both destructive and package_manager", () => {
      const tags = classifyCommand("rm -rf node_modules && npm install");
      expect(tags).toContain("destructive");
      expect(tags).toContain("package_manager");
    });

    it("classifies piped safe commands as safe", () => {
      expect(classifyCommand("ls -la | grep test")).toEqual(["safe"]);
    });

    it("classifies curl | bash as both network_tool", () => {
      const tags = classifyCommand("curl https://install.sh | bash");
      expect(tags).toContain("network_tool");
    });
  });

  describe("commandHasClassification helper", () => {
    it("returns true for matching classification", () => {
      expect(commandHasClassification("rm -rf /tmp", "destructive")).toBe(true);
    });

    it("returns false for non-matching classification", () => {
      expect(commandHasClassification("npm test", "destructive")).toBe(false);
    });
  });
});