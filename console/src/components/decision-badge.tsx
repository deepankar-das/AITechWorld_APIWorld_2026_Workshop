/**
 * Author: Deepankar Das
 */

import { Badge } from "@/components/ui/badge";

const DECISION_STYLES: Record<string, { variant: "default" | "secondary" | "destructive" | "outline"; className: string }> = {
  allow: { variant: "default", className: "bg-emerald-900/50 text-emerald-400 border-emerald-800 hover:bg-emerald-900/70" },
  deny: { variant: "destructive", className: "bg-red-900/50 text-red-400 border-red-800 hover:bg-red-900/70" },
  require_approval: { variant: "default", className: "bg-amber-900/50 text-amber-400 border-amber-800 hover:bg-amber-900/70" },
  blocked: { variant: "destructive", className: "bg-red-900/50 text-red-400 border-red-800 line-through hover:bg-red-900/70" },
  approved: { variant: "default", className: "bg-emerald-900/50 text-emerald-400 border-emerald-800 hover:bg-emerald-900/70" },
  denied: { variant: "destructive", className: "bg-red-900/50 text-red-400 border-red-800 hover:bg-red-900/70" },
  pending: { variant: "default", className: "bg-blue-900/50 text-blue-400 border-blue-800 animate-pulse hover:bg-blue-900/70" },
  bypass_detected: { variant: "destructive", className: "bg-red-900/50 text-red-400 border-red-800 animate-pulse hover:bg-red-900/70" },
};

const DEFAULT_STYLE = { variant: "outline" as const, className: "text-slate-400" };

export function DecisionBadge({ decision }: { decision: string }) {
  const style = DECISION_STYLES[decision] || DEFAULT_STYLE;
  return (
    <Badge variant={style.variant} className={style.className}>
      {decision}
    </Badge>
  );
}