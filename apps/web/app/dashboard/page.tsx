"use client";

import { useEffect } from "react";
import { useQuery } from "@urql/next";
import { BarChart2, DollarSign, ShoppingCart, TrendingUp, Users } from "lucide-react";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { MetricCard, formatCents, formatPct } from "@/components/dashboard/MetricCard";
import { RevenueChart } from "@/components/dashboard/RevenueChart";
import { TopProductsTable } from "@/components/dashboard/TopProductsTable";
import { DeadStockTable } from "@/components/dashboard/DeadStockTable";
import { useActiveStore } from "@/lib/active-store";

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
  query Insight($storeId: ID!, $provider: AIProvider) {
    insight(storeId: $storeId, provider: $provider) {
      summary
      highlights
      actions
    }
  }
`;

export default function DashboardPage() {
  const { activeStoreId, refreshCount, aiProvider } = useActiveStore();

  const [metricsResult, reexecuteMetrics] = useQuery({
    query: METRICS_QUERY,
    variables: { storeId: activeStoreId },
    pause: !activeStoreId,
  });

  const [insightResult, reexecuteInsight] = useQuery({
    query: INSIGHT_QUERY,
    variables: { storeId: activeStoreId, provider: aiProvider },
    pause: !activeStoreId,
  });

  // Re-fetch after CSV import (triggered via ActiveStoreContext.triggerRefresh)
  useEffect(() => {
    if (refreshCount > 0) {
      reexecuteMetrics({ requestPolicy: "network-only" });
      reexecuteInsight({ requestPolicy: "network-only" });
    }
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [refreshCount]);

  const m = metricsResult.data?.metrics;
  const insight = insightResult.data?.insight;

  return (
    <main className="min-h-screen p-6">
      <div className="mb-6">
        <h1 className="text-2xl font-bold tracking-tight">Dashboard</h1>
        <p className="text-sm text-muted-foreground">
          Local-first e-commerce analytics
        </p>
      </div>

      {!activeStoreId ? (
        <div className="flex h-64 flex-col items-center justify-center gap-3 rounded-xl border border-dashed border-white/10 text-center">
          <BarChart2 className="h-8 w-8 text-muted-foreground" />
          <p className="text-muted-foreground text-sm">
            Import a Shopify CSV to get started
          </p>
        </div>
      ) : metricsResult.fetching ? (
        <div className="flex h-64 items-center justify-center text-muted-foreground">
          Loading metrics…
        </div>
      ) : metricsResult.error ? (
        <div className="rounded-xl border border-destructive/30 bg-destructive/10 p-4 text-sm text-destructive">
          Failed to load metrics: {metricsResult.error.message}
        </div>
      ) : m ? (
        <div className="space-y-6">
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

          <Card>
            <CardHeader>
              <CardTitle className="text-base">Revenue Trend</CardTitle>
            </CardHeader>
            <CardContent>
              <RevenueChart data={m.trend} />
            </CardContent>
          </Card>

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
