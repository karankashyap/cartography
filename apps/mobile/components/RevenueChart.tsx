import { VictoryChart, VictoryLine, VictoryAxis, VictoryTheme } from "victory-native";
import { View } from "react-native";

interface TimePoint {
  date: string;
  revenueCents: number;
}

interface RevenueChartProps {
  data: TimePoint[];
}

export function RevenueChart({ data }: RevenueChartProps) {
  if (data.length === 0) return null;

  const chartData = data.map((d, i) => ({
    x: i,
    y: d.revenueCents / 100,
  }));

  return (
    <View>
      <VictoryChart
        theme={VictoryTheme.material}
        height={180}
        padding={{ top: 10, bottom: 30, left: 50, right: 20 }}
      >
        <VictoryAxis
          tickCount={4}
          tickFormat={(i: number) => {
            const pt = data[Math.round(i)];
            return pt ? pt.date.slice(5) : "";
          }}
          style={{ tickLabels: { fontSize: 9, fill: "#9ca3af" }, axis: { stroke: "#e5e7eb" } }}
        />
        <VictoryAxis
          dependentAxis
          tickFormat={(v: number) => `$${(v / 1000).toFixed(0)}k`}
          style={{ tickLabels: { fontSize: 9, fill: "#9ca3af" }, axis: { stroke: "#e5e7eb" } }}
        />
        <VictoryLine
          data={chartData}
          style={{ data: { stroke: "#2f95dc", strokeWidth: 2 } }}
          interpolation="monotoneX"
        />
      </VictoryChart>
    </View>
  );
}
