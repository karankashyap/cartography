"use client";

import { useQuery } from "@urql/next";
import { useState } from "react";
import { BarChart2, DollarSign, ShoppingCart, TrendingUp, Users } from "lucide-react";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { MetricCard, formatCents, formatPct } from "@/components/dashboard/MetricCard";
import { RevenueChart } from "@/components/dashboard/RevenueChart";
import { TopProductsTable } from "@/components/dashboard/TopProductsTable";
import { DeadStockTable } from "@/components/dashboard/DeadStockTable";
import { ImportButton } from "@/components/dashboard/ImportButton";

const STORES_QUERY = `
  query Stores {
    stores { id name platform }
  }
`;

const METRICS_QUERY = `
  query Metrics($storeId: ID!, $from: DateTime, $to: DateTime) {
    metrics(storeId: $storeId, from: $from, to: $to) {
      revenueCents
      orders
      aovCents
      unitsSold
      newCustomers
      returningCustomers
      returningRate
      topProducts { productId title revenueCents unitsSold }
      deadStock { variantId sku title inventoryQty daysSinceLastSale }
      trend { date revenueCents orders }
    }
  }
`;

const INSIGHT_QUERY = `
  query Insight($storeId: ID!) {
    insight(storeId: $storeId) {
      summary
      highlights
      actions
    }
  }
`;

interface Store {
  id: string;
  name: string;
  platform: string;
}

export default function DashboardPage() {
  const [selectedStoreId, setSelectedStoreId] = useState<string | null>(null);
  const [storesResult] = useQuery({ query: STORES_QUERY });
  const stores: Store[] = storesResult.data?.stores ?? [];

  const activeStoreId = selectedStoreId ?? stores[0]?.id ?? null;

  const [metricsResult, reexecuteMetrics] = useQuery({
    query: METRICS_QUERY,
    variables: { storeId: activeStoreId },
    pause: !activeStoreId,
  });

  const [insightResult, reexecuteInsight] = useQuery({
    query: INSIGHT_QUERY,
    variables: { storeId: activeStoreId },
    pause: !activeStoreId,
  });

  const m = metricsResult.data?.metrics;
  const insight = insightResult.data?.insight;

  return (
    <main className="min-h-screen bg-background p-6">
      {/* Header */}
      <div className="mb-6 flex flex-col gap-4 sm:flex-row sm:items-center sm:justify-between">
        <div>
          <h1 className="text-2xl font-bold tracking-tight">Dashboard</h1>
          <p className="text-sm text-muted-foreground">
            Local-first e-commerce analytics
          </p>
        </div>
        <div className="flex items-center gap-3">
          {stores.length > 1 && (
            <select
              value={activeStoreId ?? ""}
              onChange={(e) => setSelectedStoreId(e.target.value)}
              className="rounded-md border bg-background px-3 py-1.5 text-sm"
              aria-label="Select store"
            >
              {stores.map((s) => (
                <option key={s.id} value={s.id}>
                  {s.name} ({s.platform})
                </option>
              ))}
            </select>
          )}
          <ImportButton onComplete={() => {
              reexecuteMetrics({ requestPolicy: "network-only" });
              reexecuteInsight({ requestPolicy: "network-only" });
            }} />
        </div>
      </div>

      {!activeStoreId ? (
        <div className="flex h-64 flex-col items-center justify-center gap-3 rounded-lg border border-dashed text-center">
          <BarChart2 className="h-8 w-8 text-muted-foreground" />
          <p className="text-muted-foreground">
            Import a Shopify CSV to get started
          </p>
        </div>
      ) : metricsResult.fetching ? (
        <div className="flex h-64 items-center justify-center text-muted-foreground">
          Loading metrics…
        </div>
      ) : metricsResult.error ? (
        <div className="rounded-md border border-destructive p-4 text-sm text-destructive">
          Failed to load metrics: {metricsResult.error.message}
        </div>
      ) : m ? (
        <div className="space-y-6">
          {/* KPI row */}
          <div className="grid gap-4 sm:grid-cols-2 lg:grid-cols-4">
            <MetricCard
              title="Total Revenue"
              value={formatCents(m.revenueCents)}
              icon={DollarSign}
              testId="metric-revenue"
            />
            <MetricCard
              title="Orders"
              value={m.orders.toLocaleString()}
              icon={ShoppingCart}
              testId="metric-orders"
            />
            <MetricCard
              title="Avg Order Value"
              value={formatCents(m.aovCents)}
              icon={TrendingUp}
              testId="metric-aov"
            />
            <MetricCard
              title="Returning Rate"
              value={formatPct(m.returningRate)}
              sub={`${m.newCustomers} new · ${m.returningCustomers} returning`}
              icon={Users}
              testId="metric-returning"
            />
          </div>

          {/* Revenue trend */}
          <Card>
            <CardHeader>
              <CardTitle className="text-base">Revenue Trend</CardTitle>
            </CardHeader>
            <CardContent>
              <RevenueChart data={m.trend} />
            </CardContent>
          </Card>

          {/* Products + Dead stock */}
          <div className="grid gap-6 lg:grid-cols-2">
            <Card>
              <CardContent className="pt-6">
                <TopProductsTable products={m.topProducts} title="Top Products" />
              </CardContent>
            </Card>
            <Card>
              <CardContent className="pt-6">
                <DeadStockTable variants={m.deadStock} />
              </CardContent>
            </Card>
          </div>

          {/* AI Insight */}
          {insight && (
            <Card>
              <CardHeader>
                <CardTitle className="text-base">
                  AI Insights
                  <span className="ml-2 rounded-full bg-primary/10 px-2 py-0.5 text-xs font-normal text-primary">
                    AI-generated
                  </span>
                </CardTitle>
              </CardHeader>
              <CardContent className="space-y-4">
                <p className="text-sm">{insight.summary}</p>
                {insight.highlights.length > 0 && (
                  <div>
                    <p className="mb-1 text-xs font-semibold uppercase tracking-wide text-muted-foreground">
                      Highlights
                    </p>
                    <ul className="space-y-1">
                      {insight.highlights.map((h: string, i: number) => (
                        <li key={i} className="flex items-start gap-2 text-sm">
                          <span className="mt-0.5 h-1.5 w-1.5 shrink-0 rounded-full bg-primary" />
                          {h}
                        </li>
                      ))}
                    </ul>
                  </div>
                )}
                {insight.actions.length > 0 && (
                  <div>
                    <p className="mb-1 text-xs font-semibold uppercase tracking-wide text-muted-foreground">
                      Recommended Actions
                    </p>
                    <ol className="space-y-1">
                      {insight.actions.map((a: string, i: number) => (
                        <li key={i} className="flex items-start gap-2 text-sm">
                          <span className="shrink-0 font-mono text-xs text-muted-foreground">
                            {i + 1}.
                          </span>
                          {a}
                        </li>
                      ))}
                    </ol>
                  </div>
                )}
              </CardContent>
            </Card>
          )}
        </div>
      ) : null}
    </main>
  );
}
