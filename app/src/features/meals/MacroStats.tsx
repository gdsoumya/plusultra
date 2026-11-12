import { View } from "react-native";

import type { MacroKey } from "../../domain/totals";
import type { Macros } from "../../domain/types";
import { space, tabular, Text } from "../../ui";

const shown: { key: MacroKey; label: string; unit: string }[] = [
  { key: "kcal", label: "kcal", unit: "" },
  { key: "protein", label: "Protein", unit: "g" },
  { key: "carbs", label: "Carbs", unit: "g" },
  { key: "fat", label: "Fat", unit: "g" },
  { key: "fiber", label: "Fiber", unit: "g" },
  { key: "waterMl", label: "Water", unit: "ml" },
];

type Props = {
  totals: Macros;
  /** Each value reads against it, e.g. "165 / 170". */
  target?: Macros;
  testID?: string;
};

// A day's or a meal's macros as labelled values: the number first, the target and the label muted.
// Cells take a sixth of the row but at least 92 px, so six fit one row on a wide window and three a
// row on a phone, in columns that line up when the last row is short.
export function MacroStats({ totals, target, testID }: Props) {
  return (
    <View style={{ flexDirection: "row", flexWrap: "wrap", gap: space[3] }} testID={testID}>
      {shown
        .filter(({ key }) => totals[key] || target?.[key])
        .map(({ key, label, unit }) => {
          const value = Math.round(totals[key]);
          const goal = target?.[key] ? Math.round(target[key]) : undefined;
          const spoken = [label, value, goal !== undefined && `of ${goal}`, unit];
          return (
            <View
              key={key}
              accessible
              accessibilityLabel={spoken.filter((w) => w !== false && w !== "").join(" ")}
              style={{ flexBasis: "15%", minWidth: 92, gap: space[1] }}
            >
              <Text variant="heading" numberOfLines={1} style={tabular}>
                {value}
                {goal !== undefined ? (
                  <Text variant="caption" color="textMuted" style={tabular}>{` / ${goal}`}</Text>
                ) : null}
              </Text>
              <Text variant="caption" color="textMuted" numberOfLines={1}>
                {unit ? `${label} (${unit})` : label}
              </Text>
            </View>
          );
        })}
    </View>
  );
}
