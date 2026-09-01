/**
 * Author: Deepankar Das
 */

"use client";

import { createContext, useContext, useState, useEffect, type ReactNode } from "react";
import { getAuthMe } from "@/lib/api";
import { getConsoleMode, storageKey } from "./console-mode";

export type UserRole = "admin" | "reviewer" | "operator" | "none";

interface AuthState {
  ready: boolean;
  role: UserRole;
  token: string | null;
  username: string | null;
  login: (token: string, username: string) => Promise<UserRole>;
  logout: () => void;
  isAdmin: boolean;
  canApprove: boolean;
  canView: boolean;
}

const AuthContext = createContext<AuthState>({
  ready: false,
  role: "none",
  token: null,
  username: null,
  login: async () => "none",
  logout: () => {},
  isAdmin: false,
  canApprove: false,
  canView: false,
});

function isSeedEnabled(): boolean {
  return process.env.NEXT_PUBLIC_AA_AUTH_SEED === "true";
}

function seededRole(): UserRole {
  const role = process.env.NEXT_PUBLIC_AA_AUTH_SEED_ROLE;
  if (role === "admin" || role === "reviewer" || role === "operator") {
    return role;
  }
  return getConsoleMode() === "hub" ? "admin" : "operator";
}

function seededToken(): string | null {
  const token = process.env.NEXT_PUBLIC_AA_AUTH_SEED_TOKEN;
  return token && token.trim() ? token.trim() : "admin";
}

function seededUsername(): string | null {
  const username = process.env.NEXT_PUBLIC_AA_AUTH_SEED_USERNAME;
  return username && username.trim() ? username.trim() : "seeded-admin";
}

function isRoleAllowedForMode(role: UserRole): boolean {
  const mode = getConsoleMode();
  if (mode === "hub") {
    return role === "admin" || role === "reviewer";
  }
  return role === "operator";
}

export function AuthProvider({ children }: { children: ReactNode }) {
  const [ready, setReady] = useState(false);
  const [role, setRole] = useState<UserRole>("none");
  const [token, setToken] = useState<string | null>(null);
  const [username, setUsername] = useState<string | null>(null);

  // Hydration-safe auth bootstrap:
  // server and first client render both start as unauthenticated, then
  // session storage (or seeded fallback) is applied after mount.
  useEffect(() => {
    if (typeof window === "undefined") return;
    const roleKey = storageKey("role");
    const tokenKey = storageKey("token");
    const usernameKey = storageKey("username");

    const storedRole = (sessionStorage.getItem(roleKey) as UserRole) || "none";
    const storedToken = sessionStorage.getItem(tokenKey);
    const storedUsername = sessionStorage.getItem(usernameKey);

    const nextRole = storedRole !== "none" ? storedRole : (isSeedEnabled() ? seededRole() : "none");
    const nextToken = storedToken || (isSeedEnabled() ? seededToken() : null);
    const nextUsername = storedUsername || (isSeedEnabled() ? seededUsername() : null);

    if (isRoleAllowedForMode(nextRole)) {
      setRole(nextRole);
      setToken(nextToken);
      setUsername(nextUsername);
    } else {
      sessionStorage.removeItem(roleKey);
      sessionStorage.removeItem(tokenKey);
      sessionStorage.removeItem(usernameKey);
      setRole("none");
      setToken(null);
      setUsername(null);
    }
    setReady(true);
  }, []);

  // Persist seeded auth so the AuthGate does not redirect before first interaction.
  useEffect(() => {
    if (typeof window === "undefined") return;
    if (!ready) return;
    if (!isSeedEnabled()) return;
    if (role === "none") return;
    sessionStorage.setItem(storageKey("role"), role);
    if (token) sessionStorage.setItem(storageKey("token"), token);
    if (username) sessionStorage.setItem(storageKey("username"), username);
  }, [ready, role, token, username]);

  async function login(accessToken: string, user: string): Promise<UserRole> {
    try {
      if (accessToken) {
        const me = await getAuthMe(accessToken);
        if (!isRoleAllowedForMode(me.role)) {
          return "none";
        }
        setRole(me.role);
        setToken(accessToken);
        setUsername(user);
        sessionStorage.setItem(storageKey("role"), me.role);
        sessionStorage.setItem(storageKey("token"), accessToken);
        sessionStorage.setItem(storageKey("username"), user);
        return me.role;
      }
    } catch { /* daemon unreachable */ }

    // No/invalid token -> unauthenticated; user must retry with a valid role token.
    setRole("none");
    setToken(null);
    setUsername(null);
    sessionStorage.setItem(storageKey("role"), "none");
    sessionStorage.removeItem(storageKey("token"));
    sessionStorage.removeItem(storageKey("username"));
    return "none";
  }

  function logout() {
    setRole("none");
    setToken(null);
    setUsername(null);
    sessionStorage.removeItem(storageKey("role"));
    sessionStorage.removeItem(storageKey("token"));
    sessionStorage.removeItem(storageKey("username"));
  }

  return (
    <AuthContext.Provider
      value={{
        ready,
        role,
        token,
        username,
        login,
        logout,
        isAdmin: role === "admin",
        canApprove: role === "admin" || role === "reviewer",
        canView: role === "admin" || role === "reviewer" || role === "operator",
      }}
    >
      {children}
    </AuthContext.Provider>
  );
}

export function useAuth() {
  return useContext(AuthContext);
}

/**
 * Build headers with admin token for API calls.
 */
export function authHeaders(token: string | null): Record<string, string> {
  const headers: Record<string, string> = { "Content-Type": "application/json" };
  if (token) {
    headers.Authorization = `Bearer ${token}`;
    headers["X-AA-Token"] = token;
    headers["X-Admin-Token"] = token;
  }
  return headers;
}
