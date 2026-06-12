import { StyleSheet, Text, View } from "react-native";

interface MetricCardProps {
  title: string;
  value: string;
  sub?: string;
}

export function MetricCard({ title, value, sub }: MetricCardProps) {
  return (
    <View style={styles.card}>
      <Text style={styles.title}>{title}</Text>
      <Text style={styles.value}>{value}</Text>
      {sub && <Text style={styles.sub}>{sub}</Text>}
    </View>
  );
}

export function formatCents(cents: number): string {
  return "$" + (cents / 100).toLocaleString("en-US", { minimumFractionDigits: 2 });
}

export function formatPct(rate: number): string {
  return (rate * 100).toFixed(1) + "%";
}

const styles = StyleSheet.create({
  card: {
    backgroundColor: "#fff",
    borderRadius: 12,
    padding: 16,
    shadowColor: "#000",
    shadowOffset: { width: 0, height: 1 },
    shadowOpacity: 0.06,
    shadowRadius: 4,
    elevation: 2,
  },
  title: {
    fontSize: 12,
    color: "#6b7280",
    textTransform: "uppercase",
    letterSpacing: 0.5,
    marginBottom: 4,
  },
  value: {
    fontSize: 22,
    fontWeight: "700",
    color: "#111827",
  },
  sub: {
    fontSize: 11,
    color: "#9ca3af",
    marginTop: 2,
  },
});
