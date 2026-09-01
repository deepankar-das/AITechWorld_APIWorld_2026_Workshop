/**
 * Author: Deepankar Das
 */

"use client";

import { useAuth } from "@/lib/auth-context";
import { usePathname, useRouter } from "next/navigation";
import { useEffect } from "react";

/**
 * Auth gate — redirects to /login if not authenticated.
 * The /login page itself is excluded from the gate.
 */
export function AuthGate({ children }: { children: React.ReactNode }) {
  const { role, ready } = useAuth();
  const pathname = usePathname();
  const router = useRouter();
  const normalizedPath = pathname.replace(/\/+$/, "") || "/";
  const isLoginPath = normalizedPath === "/login";

  useEffect(() => {
    if (!ready) return;
    if (role === "none" && !isLoginPath) {
      router.replace("/login");
    }
  }, [ready, role, isLoginPath, router]);

  if (!ready) {
    return null;
  }

  if (role === "none" && !isLoginPath) {
    return null; // Don't flash content before redirect
  }

  return <>{children}</>;
}
