/**
 * Author: Deepankar Das
 */

import { FileText, Terminal, Globe, Puzzle, ShieldCheck, AlertTriangle } from "lucide-react";

const ICON_MAP: Record<string, typeof FileText> = {
  "file.read": FileText,
  "file.write": FileText,
  "file.delete": FileText,
  "file.move": FileText,
  "shell.exec": Terminal,
  "network.request": Globe,
  "mcp.invoke": Puzzle,
  "approval": ShieldCheck,
};

export function ActionIcon({ actionType, className = "h-4 w-4" }: { actionType: string; className?: string }) {
  const Icon = ICON_MAP[actionType] || AlertTriangle;
  return <Icon className={className} />;
}