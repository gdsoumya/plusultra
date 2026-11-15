import { Pressable, View } from "react-native";

import { fractionOf, type MacroKey } from "../../domain/totals";
import type { Macros } from "../../domain/types";
import { pressedOpacity, Progress, space, tabular, Text } from "../../ui";

const shown: { key: MacroKey; label: string; unit: string }[] = [
  { key: "kcal", label: "kcal", unit: "" },
  { key: "protein", label: "Protein", unit: " g" },
  { key: "carbs", label: "Carbs", unit: " g" },
  { key: "fat", label: "Fat", unit: " g" },
];

type Props = {
  totals: Macros;
  // Bars are hidden without a target; the numbers still show.
  target?: Macros;
  /** Opens the full figures; the row is a button. */
  onPress?: () => void;
};

// kcal, protein, carbs and fat eaten against the target, one column each.
export function MacroBars({ totals, target, onPress }: Props) {
  return (
    <Pressable
      accessibilityRole={onPress && "button"}
      accessibilityLabel={onPress && "Macro details"}
      disabled={!onPress}
      onPress={onPress}
      style={({ pressed }) => ({
        flexDirection: "row",
        gap: space[2],
        opacity: pressed ? pressedOpacity : 1,
      })}
    >
      {shown.map(({ key, label, unit }) => (
        <View key={key} style={{ flex: 1, gap: space[1] }}>
          <Text variant="caption">{label}</Text>
          {/* A day's "3289 / 2900" wraps on a phone; flex keeps the bars level across columns. */}
          <Text variant="label" style={[tabular, { flex: 1 }]}>
            {Math.round(totals[key])}
            <Text variant="caption" style={tabular}>
              {`${target?.[key] ? ` / ${Math.round(target[key])}` : ""}${unit}`}
            </Text>
          </Text>
          {target?.[key] ? <Progress value={fractionOf(totals, target, key)} /> : null}
        </View>
      ))}
    </Pressable>
  );
}
