import { useEffect, useState } from "react";
import { ActivityIndicator, RefreshControl, ScrollView, StyleSheet, Text, View } from "react-native";
import { SafeAreaView } from "react-native-safe-area-context";
import { client } from "@/lib/graphql";
import { MetricCard, formatCents, formatPct } from "@/components/MetricCard";
import { RevenueChart } from "@/components/RevenueChart";

const STORES_QUERY = `
  query Stores {
    stores { id name platform }
  }
`;

const METRICS_QUERY = `
  query Metrics($storeId: ID!) {
    metrics(storeId: $storeId) {
      revenueCents
      orders
      aovCents
      unitsSold
      newCustomers
      returningCustomers
      returningRate
      trend { date revenueCents orders }
    }
  }
`;

interface Store { id: string; name: string; platform: string }
interface TimePoint { date: string; revenueCents: number; orders: number }
interface Metrics {
  revenueCents: number;
  orders: number;
  aovCents: number;
  unitsSold: number;
  newCustomers: number;
  returningCustomers: number;
  returningRate: number;
  trend: TimePoint[];
}

export default function DashboardTab() {
  const [store, setStore] = useState<Store | null>(null);
  const [metrics, setMetrics] = useState<Metrics | null>(null);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [refreshing, setRefreshing] = useState(false);

  async function load(isRefresh = false) {
    if (isRefresh) setRefreshing(true);
    else setLoading(true);
    setError(null);

    try {
      const storesRes = await client.query(STORES_QUERY, {}).toPromise();
      const stores: Store[] = storesRes.data?.stores ?? [];
      if (stores.length === 0) {
        setLoading(false);
        setRefreshing(false);
        return;
      }
      const s = stores[0];
      setStore(s);

      const mRes = await client.query(METRICS_QUERY, { storeId: s.id }).toPromise();
      if (mRes.error) throw new Error(mRes.error.message);
      setMetrics(mRes.data?.metrics ?? null);
    } catch (e) {
      setError(e instanceof Error ? e.message : String(e));
    } finally {
      setLoading(false);
      setRefreshing(false);
    }
  }

  useEffect(() => { load(); }, []);

  if (loading) {
    return (
      <SafeAreaView style={styles.center}>
        <ActivityIndicator size="large" color="#2f95dc" />
      </SafeAreaView>
    );
  }

  if (error) {
    return (
      <SafeAreaView style={styles.center}>
        <Text style={styles.errorText}>{error}</Text>
      </SafeAreaView>
    );
  }

  if (!store || !metrics) {
    return (
      <SafeAreaView style={styles.center}>
        <Text style={styles.emptyText}>No store data yet.</Text>
        <Text style={styles.emptySubText}>Import a CSV from the web dashboard.</Text>
      </SafeAreaView>
    );
  }

  return (
    <SafeAreaView style={styles.root} edges={["bottom"]}>
      <ScrollView
        contentContainerStyle={styles.scroll}
        refreshControl={
          <RefreshControl refreshing={refreshing} onRefresh={() => load(true)} />
        }
      >
        <Text style={styles.storeName}>{store.name}</Text>
        <Text style={styles.storeLabel}>{store.platform}</Text>

        <View style={styles.grid}>
          <View style={styles.gridItem}>
            <MetricCard title="Revenue" value={formatCents(metrics.revenueCents)} />
          </View>
          <View style={styles.gridItem}>
            <MetricCard title="Orders" value={metrics.orders.toLocaleString()} />
          </View>
          <View style={styles.gridItem}>
            <MetricCard title="Avg Order" value={formatCents(metrics.aovCents)} />
          </View>
          <View style={styles.gridItem}>
            <MetricCard
              title="Returning"
              value={formatPct(metrics.returningRate)}
              sub={`${metrics.newCustomers} new · ${metrics.returningCustomers} ret.`}
            />
          </View>
        </View>

        <View style={styles.chartCard}>
          <Text style={styles.chartTitle}>Revenue Trend</Text>
          <RevenueChart data={metrics.trend} />
        </View>
      </ScrollView>
    </SafeAreaView>
  );
}

const styles = StyleSheet.create({
  root: { flex: 1, backgroundColor: "#f9fafb" },
  center: { flex: 1, alignItems: "center", justifyContent: "center", gap: 8 },
  scroll: { padding: 16, gap: 16 },
  storeName: { fontSize: 20, fontWeight: "700", color: "#111827" },
  storeLabel: { fontSize: 12, color: "#6b7280", marginTop: 2, marginBottom: 4 },
  grid: { flexDirection: "row", flexWrap: "wrap", gap: 12 },
  gridItem: { width: "47%" },
  chartCard: {
    backgroundColor: "#fff",
    borderRadius: 12,
    padding: 16,
    shadowColor: "#000",
    shadowOffset: { width: 0, height: 1 },
    shadowOpacity: 0.06,
    shadowRadius: 4,
    elevation: 2,
  },
  chartTitle: { fontSize: 13, fontWeight: "600", color: "#374151", marginBottom: 4 },
  errorText: { color: "#dc2626", fontSize: 14, textAlign: "center", paddingHorizontal: 24 },
  emptyText: { fontSize: 16, fontWeight: "600", color: "#374151" },
  emptySubText: { fontSize: 13, color: "#6b7280" },
});
