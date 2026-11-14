import { Pressable, View } from "react-native";

import { fractionOf, type MacroKey } from "../../domain/totals";
import type { Macros } from "../../domain/types";
import { pressedOpacity, Progress, space, tabular, Text } from "../../ui";

const shown: { key: MacroKey; label: string }[] = [
  { key: "kcal", label: "kcal" },
  { key: "protein", label: "Protein" },
  { key: "carbs", label: "Carbs" },
  { key: "fat", label: "Fat" },
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
      {shown.map(({ key, label }) => (
        <View key={key} style={{ flex: 1, gap: space[1] }}>
          <Text variant="caption">{label}</Text>
          <Text variant="label" numberOfLines={1} style={tabular}>
            {Math.round(totals[key])}
            {target?.[key] ? (
              <Text variant="caption" style={tabular}>{` / ${Math.round(target[key])}`}</Text>
            ) : null}
          </Text>
          {target?.[key] ? <Progress value={fractionOf(totals, target, key)} /> : null}
        </View>
      ))}
    </Pressable>
  );
}
