/**
 * Author: Deepankar Das
 */

export type ConsoleMode = "hub" | "sentinel";

export function getConsoleMode(): ConsoleMode {
  const mode = process.env.NEXT_PUBLIC_AA_CONSOLE_MODE;
  return mode === "hub" ? "hub" : "sentinel";
}

export function isHubMode(): boolean {
  return getConsoleMode() === "hub";
}

export function consoleLabel(): string {
  return isHubMode() ? "Hub Console" : "Sentinel Console";
}

export function storageKey(name: "role" | "token" | "username"): string {
  const prefix = isHubMode() ? "aa_hub_" : "aa_sentinel_";
  return `${prefix}${name}`;
}

