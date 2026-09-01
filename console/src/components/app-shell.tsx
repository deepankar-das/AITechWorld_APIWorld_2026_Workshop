/**
 * Author: Deepankar Das
 */

"use client";

import { useEffect, useState, type ReactNode } from "react";
import Link from "next/link";
import { usePathname } from "next/navigation";
import { getApprovalMetrics } from "@/lib/api";
import { consoleLabel, getConsoleMode } from "../lib/console-mode";
import {
  LayoutDashboard,
  Monitor,
  ShieldCheck,
  Search,
  Download,
  FileText,
  LogOut,
  User,
  Shield,
  BarChart3,
} from "lucide-react";
import { AALogo } from "@/components/aa-logo";
import { AuthProvider, useAuth } from "@/lib/auth-context";
import { AuthGate } from "@/components/auth-gate";

const HUB_NAV_ITEMS = [
  { href: "/", label: "Dashboard", icon: LayoutDashboard },
  { href: "/sessions", label: "Sessions", icon: Monitor },
  { href: "/approvals", label: "Approvals", icon: ShieldCheck },
  { href: "/search", label: "Search", icon: Search },
  { href: "/export", label: "Export", icon: Download },
  { href: "/policies", label: "Policies", icon: FileText },
  { href: "/analytics", label: "Analytics", icon: BarChart3 },
];

const SENTINEL_NAV_ITEMS = [
  { href: "/", label: "Dashboard", icon: LayoutDashboard },
  { href: "/sessions", label: "Sessions", icon: Monitor },
  { href: "/search", label: "Search", icon: Search },
  { href: "/export", label: "Export", icon: Download },
  { href: "/developer/me", label: "My Activity", icon: User },
];

function NavLinks({ items, pathname, token }: { items: Array<{ href: string; label: string; icon: (props: { className?: string }) => ReactNode }>; pathname: string; token: string | null }) {
  const isHub = getConsoleMode() === "hub";
  const [pendingCount, setPendingCount] = useState(0);

  useEffect(() => {
    if (!isHub) return;
    if (!token) return;
    const fetchPending = async () => {
      try {
        const m = await getApprovalMetrics(token);
        setPendingCount(m.pending_count ?? 0);
      } catch { /* ignore */ }
    };
    fetchPending();
    const interval = setInterval(fetchPending, 10000);
    return () => clearInterval(interval);
  }, [isHub, token]);

  return (
    <nav className="flex flex-col gap-1">
      {items.map(({ href, label, icon: Icon }) => (
        <Link
          key={href}
          href={href}
          className={`flex items-center gap-2 px-2 py-1.5 rounded text-sm transition-colors ${
            (pathname === href || (href !== "/" && pathname.startsWith(href)))
              ? "text-cyan-400 bg-slate-800"
              : "text-slate-400 hover:text-slate-200 hover:bg-slate-800"
          }`}
        >
          <Icon className="h-4 w-4" />
          {label}
          {isHub && label === "Approvals" && pendingCount > 0 && (
            <span className="ml-auto text-[10px] px-1.5 py-0.5 rounded-full bg-amber-900/60 text-amber-400">{pendingCount}</span>
          )}
        </Link>
      ))}
    </nav>
  );
}

function SidebarContent() {
  const { role, username, token, logout, isAdmin } = useAuth();
  const pathname = usePathname();
  const normalizedPath = pathname.replace(/\/+$/, "") || "/";
  const isHub = getConsoleMode() === "hub";

  // No sidebar on login page
  if (normalizedPath === "/login") return null;

  const visibleItems = isHub ? HUB_NAV_ITEMS : SENTINEL_NAV_ITEMS;

  return (
    <aside className="w-56 bg-slate-900 border-r border-slate-800 flex flex-col py-4 px-3 shrink-0">
      <Link href="/" className="flex items-center gap-2 px-2 mb-6">
        <AALogo className="h-8 w-8" />
        <div>
          <div className="font-bold text-sm text-cyan-400">Enforcer</div>
          <div className="text-[10px] text-slate-500">{consoleLabel()}</div>
        </div>
      </Link>

      <NavLinks items={visibleItems} pathname={pathname} token={token} />

      <div className="mt-auto space-y-2 px-2">
        {/* User info */}
        {username && (
          <div className="flex items-center gap-2 text-xs text-slate-500">
            <User className="h-3 w-3" />
            <span>{username}</span>
            <span className={`px-1 py-0.5 rounded text-[9px] ${isAdmin ? "bg-cyan-900/50 text-cyan-400" : "bg-slate-800 text-slate-400"}`}>
              {role === "admin" ? "Admin" : role === "reviewer" ? "Reviewer" : "Operator"}
            </span>
          </div>
        )}

        {/* Logout */}
        {role !== "none" && (
          <button
            onClick={logout}
            className="flex items-center gap-2 text-xs text-slate-600 hover:text-slate-400 transition-colors w-full"
          >
            <LogOut className="h-3 w-3" />
            Sign out
          </button>
        )}

        <div className="text-[10px] text-slate-700 pt-1">
          v0.1.0
        </div>
      </div>
    </aside>
  );
}

export function AppShell({ children }: { children: React.ReactNode }) {
  return (
    <AuthProvider>
      <AuthGate>
        <div className="min-h-full flex bg-slate-950 text-slate-200">
          <SidebarContent />
          <main className="flex-1 overflow-auto p-6">
            {children}
          </main>
        </div>
      </AuthGate>
    </AuthProvider>
  );
}
