/**
 * Author: Deepankar Das
 */

"use client";

import { useState } from "react";
import { useRouter } from "next/navigation";
import { useAuth } from "@/lib/auth-context";
import { consoleLabel, getConsoleMode } from "../../lib/console-mode";
import { AALogo } from "@/components/aa-logo";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";

export default function LoginPage() {
  const mode = getConsoleMode();
  const isHub = mode === "hub";
  const [username, setUsername] = useState("");
  const [accessToken, setAccessToken] = useState("");
  const [error, setError] = useState("");
  const [loading, setLoading] = useState(false);
  const router = useRouter();
  const { login } = useAuth();

  async function handleLogin(e: React.FormEvent) {
    e.preventDefault();
    if (!username) {
      setError("Username is required");
      return;
    }
    setLoading(true);
    setError("");

    const role = await login(accessToken, username);
    if (role === "none") {
      setError(isHub ? "Invalid Hub token. Use admin or reviewer token." : "Invalid Sentinel token. Use operator token.");
    } else {
      router.push("/");
    }
    setLoading(false);
  }

  return (
    <div className="min-h-screen bg-slate-950 flex items-center justify-center p-4">
      <div className="w-full max-w-sm">
        <div className="text-center mb-8">
          <AALogo className="h-16 w-16 mx-auto mb-4" />
          <h1 className="text-xl font-bold text-slate-100">Enforcer</h1>
          <p className="text-sm text-slate-500">{consoleLabel()}</p>
        </div>

        <form onSubmit={handleLogin} className="bg-slate-900 border border-slate-800 rounded-lg p-6 space-y-4">
          <div>
            <label className="text-xs text-slate-500 uppercase tracking-wider block mb-1.5">Username</label>
            <Input
              placeholder="Enter your username"
              value={username}
              onChange={e => setUsername(e.target.value)}
              className="bg-slate-950 border-slate-700 text-slate-300"
              autoFocus
            />
          </div>

          <div>
            <label className="text-xs text-slate-500 uppercase tracking-wider block mb-1.5">
              Access Token <span className="text-slate-600">(required)</span>
            </label>
            <Input
              type="password"
              placeholder={isHub ? "Access token (admin/reviewer)" : "Access token (operator)"}
              value={accessToken}
              onChange={e => setAccessToken(e.target.value)}
              className="bg-slate-950 border-slate-700 text-slate-300 font-mono"
            />
          </div>

          {error && (
            <div className="text-xs text-amber-400 bg-amber-950/30 border border-amber-900/50 rounded px-3 py-2">
              {error}
            </div>
          )}

          <Button
            type="submit"
            disabled={loading}
            className="w-full bg-cyan-900 hover:bg-cyan-800 text-cyan-300"
          >
            {loading ? "Verifying..." : "Sign In"}
          </Button>

          <div className="text-[10px] text-slate-600 text-center mt-2">
            {isHub ? "Hub accepts only admin/reviewer tokens." : "Sentinel accepts only operator tokens."}<br/>
            Sessions are isolated per console mode.
          </div>
        </form>
      </div>
    </div>
  );
}
