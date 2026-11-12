import { useState } from "react";
import { Pressable, View } from "react-native";

import { mealEnabled, mealTotals } from "../../domain/totals";
import type { Meal } from "../../domain/types";
import {
  Button,
  Card,
  Chip,
  Icon,
  ListRow,
  disabledOpacity,
  minTouch,
  pressedOpacity,
  space,
  Text,
} from "../../ui";
import { itemLine } from "./format";
import { MacroBars } from "./MacroBars";
import { MealSheet } from "./MealSheet";

type Props = {
  meal: Meal;
  index: number;
  count: number;
  // The same meal in the plan; its totals are the meal's target.
  planned?: Meal;
  onToggle(): void;
  onItem(index: number): void;
  onAdd(): void;
  onMove(to: number): void;
  onDelete(): void;
};

// A disabled meal shows what the plan says, greyed, until "Eaten" logs it. A meal the plan does not
// have, and an item added to a planned one, is eaten as entered: disabling would zero it with no
// plan to restore (P2-R42).
export function MealCard({
  meal,
  index,
  count,
  planned,
  onToggle,
  onItem,
  onAdd,
  onMove,
  onDelete,
}: Props) {
  const [menu, setMenu] = useState(false);
  const enabled = !planned || mealEnabled(meal);
  return (
    <Card>
      <View style={{ flexDirection: "row", alignItems: "center", gap: space[2] }}>
        <View style={{ flex: 1 }}>
          <Text variant="heading" numberOfLines={1}>
            {meal.name}
          </Text>
          {meal.time && <Text variant="caption">{meal.time}</Text>}
        </View>
        {planned && (
          <Chip
            label="Eaten"
            icon="checkmark"
            selected={enabled}
            onPress={onToggle}
            testID={`meal-${index}-eaten`}
          />
        )}
        <Pressable
          accessibilityRole="button"
          accessibilityLabel={`${meal.name} options`}
          onPress={() => setMenu(true)}
          testID={`meal-${index}-menu`}
          style={({ pressed }) => ({
            width: minTouch,
            height: minTouch,
            alignItems: "center",
            justifyContent: "center",
            opacity: pressed ? pressedOpacity : 1,
          })}
        >
          <Icon name="ellipsis-horizontal" color="textMuted" />
        </Pressable>
      </View>
      <View style={{ gap: space[3], opacity: enabled ? 1 : disabledOpacity }}>
        <MacroBars totals={mealTotals(meal)} target={planned && mealTotals(planned)} />
        <View style={{ marginHorizontal: -space[4] }}>
          {meal.items.map((item, i) => (
            <ListRow
              key={`${item.foodName}-${i}`}
              title={item.foodName}
              subtitle={itemLine(
                enabled || !item.planned
                  ? item
                  : (planned?.items.find((p) => p.foodName === item.foodName) ?? item),
              )}
              value={
                [!item.planned && "ad hoc", item.edited && "edited"].filter(Boolean).join(" · ") ||
                undefined
              }
              onPress={enabled || !item.planned ? () => onItem(i) : undefined}
            />
          ))}
        </View>
      </View>
      {enabled && <Button title="Add item" icon="add" variant="ghost" onPress={onAdd} />}
      <MealSheet
        open={menu}
        meal={meal}
        index={index}
        count={count}
        onMove={onMove}
        onDelete={() => {
          setMenu(false);
          onDelete();
        }}
        onClose={() => setMenu(false)}
      />
    </Card>
  );
}
