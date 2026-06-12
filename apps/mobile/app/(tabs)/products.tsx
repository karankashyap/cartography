import { useEffect, useState } from "react";
import {
  ActivityIndicator,
  FlatList,
  RefreshControl,
  StyleSheet,
  Text,
  View,
} from "react-native";
import { SafeAreaView } from "react-native-safe-area-context";
import { client } from "@/lib/graphql";

const STORES_QUERY = `
  query Stores {
    stores { id name }
  }
`;

const METRICS_QUERY = `
  query TopProducts($storeId: ID!) {
    metrics(storeId: $storeId) {
      topProducts { productId title revenueCents unitsSold }
    }
  }
`;

interface ProductStat {
  productId: string;
  title: string;
  revenueCents: number;
  unitsSold: number;
}

function formatCents(cents: number): string {
  return "$" + (cents / 100).toLocaleString("en-US", { minimumFractionDigits: 0 });
}

export default function ProductsTab() {
  const [products, setProducts] = useState<ProductStat[]>([]);
  const [loading, setLoading] = useState(true);
  const [refreshing, setRefreshing] = useState(false);
  const [error, setError] = useState<string | null>(null);

  async function load(isRefresh = false) {
    if (isRefresh) setRefreshing(true);
    else setLoading(true);
    setError(null);

    try {
      const storesRes = await client.query(STORES_QUERY, {}).toPromise();
      const stores = storesRes.data?.stores ?? [];
      if (stores.length === 0) return;

      const mRes = await client
        .query(METRICS_QUERY, { storeId: stores[0].id })
        .toPromise();
      if (mRes.error) throw new Error(mRes.error.message);
      setProducts(mRes.data?.metrics?.topProducts ?? []);
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

  return (
    <SafeAreaView style={styles.root} edges={["bottom"]}>
      <FlatList
        data={products}
        keyExtractor={(item) => item.productId}
        contentContainerStyle={styles.list}
        refreshControl={
          <RefreshControl refreshing={refreshing} onRefresh={() => load(true)} />
        }
        ListEmptyComponent={
          <View style={styles.center}>
            <Text style={styles.emptyText}>No product data yet.</Text>
          </View>
        }
        ListHeaderComponent={
          <Text style={styles.heading}>Top Products by Revenue</Text>
        }
        renderItem={({ item, index }) => (
          <View style={styles.row}>
            <View style={styles.rank}>
              <Text style={styles.rankText}>{index + 1}</Text>
            </View>
            <View style={styles.info}>
              <Text style={styles.title} numberOfLines={1}>{item.title}</Text>
              <Text style={styles.sub}>{item.unitsSold} units sold</Text>
            </View>
            <Text style={styles.revenue}>{formatCents(item.revenueCents)}</Text>
          </View>
        )}
        ItemSeparatorComponent={() => <View style={styles.sep} />}
      />
    </SafeAreaView>
  );
}

const styles = StyleSheet.create({
  root: { flex: 1, backgroundColor: "#f9fafb" },
  center: { flex: 1, alignItems: "center", justifyContent: "center", paddingVertical: 40 },
  list: { padding: 16, gap: 0 },
  heading: { fontSize: 17, fontWeight: "700", color: "#111827", marginBottom: 12 },
  row: {
    flexDirection: "row",
    alignItems: "center",
    backgroundColor: "#fff",
    paddingHorizontal: 14,
    paddingVertical: 12,
    borderRadius: 10,
    gap: 12,
  },
  rank: {
    width: 28,
    height: 28,
    borderRadius: 14,
    backgroundColor: "#eff6ff",
    alignItems: "center",
    justifyContent: "center",
  },
  rankText: { fontSize: 12, fontWeight: "700", color: "#2f95dc" },
  info: { flex: 1 },
  title: { fontSize: 14, fontWeight: "600", color: "#111827" },
  sub: { fontSize: 11, color: "#6b7280", marginTop: 1 },
  revenue: { fontSize: 14, fontWeight: "700", color: "#111827" },
  sep: { height: 8 },
  errorText: { color: "#dc2626", fontSize: 14, textAlign: "center", paddingHorizontal: 24 },
  emptyText: { fontSize: 14, color: "#6b7280" },
});
