/**
 * Author: Deepankar Das
 */

import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";

interface MetricCardProps {
  title: string;
  value: string | number;
  subtitle?: string;
  color?: string;
}

export function MetricCard({ title, value, subtitle, color = "text-cyan-400" }: MetricCardProps) {
  return (
    <Card className="bg-slate-900 border-slate-800">
      <CardHeader className="pb-2">
        <CardTitle className="text-[10px] uppercase tracking-wider text-slate-500 font-medium">
          {title}
        </CardTitle>
      </CardHeader>
      <CardContent>
        <div className={`text-2xl font-bold ${color}`}>{value}</div>
        {subtitle && <div className="text-[11px] text-slate-500 mt-1">{subtitle}</div>}
      </CardContent>
    </Card>
  );
}