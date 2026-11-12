import { View } from "react-native";

import { Text } from "../Text";
import { useTheme } from "../theme";
import { space, tabular } from "../tokens";
import { ChartFrame } from "./ChartFrame";

export type Bar = { label: string; value: number };

export type BarChartProps = {
  // In display order; the longest bar fills the row.
  bars: Bar[];
  // Table headings for the label and the value.
  columns: [string, string];
  format?: (value: number) => string;
  summary: string;
  testID?: string;
};

export function BarChart({ bars, columns, format = String, summary, testID }: BarChartProps) {
  const { colors } = useTheme();
  const max = Math.max(0, ...bars.map((b) => b.value));
  return (
    <ChartFrame
      summary={summary}
      table={{ columns, rows: bars.map((b) => [b.label, format(b.value)]) }}
      testID={testID}
    >
      <View style={{ gap: space[2] }}>
        {bars.map((b) => (
          <View key={b.label} style={{ flexDirection: "row", alignItems: "center", gap: space[2] }}>
            <Text variant="caption" color="text" numberOfLines={1} style={{ width: 96 }}>
              {b.label}
            </Text>
            <View style={{ flex: 1, height: 12, borderRadius: 4, backgroundColor: colors.border }}>
              <View
                testID={`bar-${b.label}`}
                style={{
                  width: `${max ? (b.value / max) * 100 : 0}%`,
                  height: "100%",
                  backgroundColor: colors.accent,
                  borderRadius: 4,
                }}
              />
            </View>
            <Text variant="label" style={[tabular, { width: 36, textAlign: "right" }]}>
              {format(b.value)}
            </Text>
          </View>
        ))}
      </View>
    </ChartFrame>
  );
}
