import { useState, type ReactNode } from "react";
import { View } from "react-native";

import { Button } from "../Button";
import { Text } from "../Text";
import { useTheme } from "../theme";
import { minTouch, space, stroke, tabular } from "../tokens";

export type Table = { columns: string[]; rows: string[][] };

export type ChartFrameProps = {
  // Read by screen readers in place of the drawing.
  summary: string;
  table: Table;
  children: ReactNode;
  testID?: string;
};

// A chart with its screen-reader summary and a toggle to the same data as a table.
export function ChartFrame({ summary, table, children, testID }: ChartFrameProps) {
  const [asTable, setAsTable] = useState(false);
  return (
    <View style={{ gap: space[2] }} testID={testID}>
      {asTable ? (
        <DataTable table={table} />
      ) : (
        <View accessible accessibilityLabel={summary}>
          {children}
        </View>
      )}
      <Button
        variant="ghost"
        title={asTable ? "Show chart" : "Show table"}
        onPress={() => setAsTable((v) => !v)}
        style={{ alignSelf: "flex-end", paddingVertical: 0, minHeight: minTouch }}
      />
    </View>
  );
}

function DataTable({ table: { columns, rows } }: { table: Table }) {
  const { colors } = useTheme();
  const row = (cells: string[], header: boolean) => (
    <View
      style={{
        flexDirection: "row",
        gap: space[2],
        paddingVertical: space[1],
        borderBottomWidth: stroke.control,
        borderColor: colors.border,
      }}
    >
      {cells.map((cell, i) => (
        <Text
          key={i}
          variant={header ? "label" : "body"}
          color={header ? "textMuted" : "text"}
          style={[tabular, { flex: 1, textAlign: i ? "right" : "left" }]}
        >
          {cell}
        </Text>
      ))}
    </View>
  );
  return (
    <View>
      {row(columns, true)}
      {rows.map((cells, i) => (
        <View key={i}>{row(cells, false)}</View>
      ))}
    </View>
  );
}
