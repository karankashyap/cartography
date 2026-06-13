"use client";

import {
  Area,
  AreaChart,
  CartesianGrid,
  ResponsiveContainer,
  Tooltip,
  XAxis,
  YAxis,
} from "recharts";
import { formatCents } from "./MetricCard";

interface TimePoint {
  date: string;
  revenueCents: number;
  orders: number;
}

interface RevenueChartProps {
  data: TimePoint[];
}

export function RevenueChart({ data }: RevenueChartProps) {
  if (!data || data.length === 0) {
    return (
      <div className="flex h-48 items-center justify-center text-sm text-muted-foreground">
        No trend data for selected period
      </div>
    );
  }

  const chartData = data.map((d) => ({
    ...d,
    revenue: d.revenueCents / 100,
  }));

  return (
    <div
      role="img"
      aria-label={`Revenue trend chart showing ${data.length} data points`}
    >
      <ResponsiveContainer width="100%" height={220}>
        <AreaChart
          data={chartData}
          margin={{ top: 4, right: 4, left: 0, bottom: 0 }}
        >
          <defs>
            <linearGradient id="revenueGrad" x1="0" y1="0" x2="0" y2="1">
              <stop offset="5%" stopColor="oklch(0.72 0.19 192)" stopOpacity={0.35} />
              <stop offset="95%" stopColor="oklch(0.72 0.19 192)" stopOpacity={0} />
            </linearGradient>
          </defs>
          <CartesianGrid strokeDasharray="3 3" className="stroke-border" />
          <XAxis
            dataKey="date"
            tick={{ fontSize: 11 }}
            tickFormatter={(v: string) => v.slice(5)} // MM-DD
            className="text-muted-foreground"
          />
          <YAxis
            tick={{ fontSize: 11 }}
            tickFormatter={(v: number) => `$${Math.round(v / 1000)}k`}
            className="text-muted-foreground"
          />
          <Tooltip
            formatter={(v) => [formatCents(Number(v) * 100), "Revenue"]}
            labelFormatter={(l) => `Date: ${String(l)}`}
          />
          <Area
            type="monotone"
            dataKey="revenue"
            stroke="oklch(0.72 0.19 192)"
            strokeWidth={2}
            fill="url(#revenueGrad)"
          />
        </AreaChart>
      </ResponsiveContainer>
    </div>
  );
}
