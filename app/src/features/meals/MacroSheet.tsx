import { View } from "react-native";

import { KCAL_TOLERANCE, PROTEIN_TOLERANCE_G } from "../../domain/totals";
import type { Macros } from "../../domain/types";
import { Badge, ListCard, ListRow, Sheet, space, Text } from "../../ui";

const shown: { key: keyof Macros; label: string; unit: string }[] = [
  { key: "kcal", label: "Calories", unit: "kcal" },
  { key: "protein", label: "Protein", unit: "g" },
  { key: "carbs", label: "Carbs", unit: "g" },
  { key: "fat", label: "Fat", unit: "g" },
  { key: "fiber", label: "Fiber", unit: "g" },
  { key: "waterMl", label: "Water", unit: "ml" },
];

type Props = {
  open: boolean;
  title: string;
  totals: Macros;
  target?: Macros;
  /** The day's verdict, when it has one; a meal has none. */
  onTarget?: boolean;
  onClose: () => void;
};

// "120 over" or "3 short" of the target; "on target" when equal.
const gap = (value: number, goal: number, unit: string) => {
  const diff = Math.round(value) - Math.round(goal);
  if (!diff) return "on target";
  return `${Math.abs(diff)} ${unit} ${diff > 0 ? "over" : "short"}`;
};

// Every macro eaten against its target in full, with how far off each is, for a meal or the day.
export function MacroSheet({ open, title, totals, target, onTarget, onClose }: Props) {
  return (
    <Sheet open={open} onClose={onClose} title={title} snapPoints={[0.6, 0.9]} testID="macro-sheet">
      {onTarget !== undefined && (
        <View style={{ gap: space[2], alignItems: "flex-start" }}>
          <Badge
            label={onTarget ? "On target" : "Off target"}
            icon={onTarget ? "checkmark-circle" : "alert-circle"}
            tone={onTarget ? "accent" : "warning"}
          />
          <Text variant="caption">
            On target means calories within {KCAL_TOLERANCE * 100}% and protein within{" "}
            {PROTEIN_TOLERANCE_G} g of the target.
          </Text>
        </View>
      )}
      <ListCard>
        {shown
          .filter(({ key }) => totals[key] || target?.[key])
          .map(({ key, label, unit }) => {
            const value = Math.round(totals[key]);
            const goal = target?.[key] ? Math.round(target[key]) : undefined;
            return (
              <ListRow
                key={key}
                title={label}
                subtitle={goal === undefined ? undefined : `Target ${goal} ${unit}`}
                value={goal === undefined ? `${value} ${unit}` : gap(value, goal, unit)}
              />
            );
          })}
      </ListCard>
    </Sheet>
  );
}
