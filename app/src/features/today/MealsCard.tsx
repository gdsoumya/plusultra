import { View } from "react-native";

import { mealEnabled, mealTotals } from "../../domain/totals";
import type { Meal, MealDay, MealLog } from "../../domain/types";
import { Button, Card, Progress, Text, space, tabular } from "../../ui";
import { CardMenu, type MenuItem } from "../calendar/CardMenu";

type Props = {
  plan?: MealDay;
  log?: MealLog;
  onLog: () => void;
  onOpen: () => void;
  menu: MenuItem[];
};

// The log's meals once there is one (deleted meals are gone, added ones show); a meal not eaten
// yet shows what the plan has for it. Without a plan (an ad hoc log) the day shows its totals.
export function MealsCard({ plan, log, onLog, onOpen, menu }: Props) {
  const target = log?.target ?? plan?.target;
  const eaten = log?.totals;
  const meals = log?.meals ?? plan?.meals ?? [];
  const kcalOf = (m: Meal) =>
    mealTotals(mealEnabled(m) ? m : (plan?.meals.find((p) => p.name === m.name) ?? m)).kcal;
  return (
    <Card>
      <View style={{ flexDirection: "row", alignItems: "flex-start", gap: space[3] }}>
        <View style={{ flex: 1 }}>
          {plan && <Text variant="caption">Meals</Text>}
          <Text variant="heading">{plan?.name ?? "Meals"}</Text>
        </View>
        <CardMenu title="Meals" items={menu} />
      </View>
      {meals.length > 0 && (
        <View style={{ gap: space[2] }}>
          {meals.map((m, i) => (
            <View key={i} style={{ flexDirection: "row", gap: space[3] }}>
              <Text numberOfLines={1} style={{ flex: 1 }}>
                {m.time ? `${m.name} · ${m.time}` : m.name}
              </Text>
              <Text color="textMuted" style={tabular}>{`${Math.round(kcalOf(m))} kcal`}</Text>
            </View>
          ))}
        </View>
      )}
      {target ? (
        <>
          <Progress value={(eaten?.kcal ?? 0) / target.kcal} />
          <Text variant="label" style={tabular}>
            {`${Math.round(eaten?.kcal ?? 0)} / ${Math.round(target.kcal)} kcal · ${Math.round(eaten?.protein ?? 0)} / ${Math.round(target.protein)} g protein`}
          </Text>
        </>
      ) : (
        eaten && (
          <Text variant="label" style={tabular}>
            {`${Math.round(eaten.kcal)} kcal · ${Math.round(eaten.protein)} g protein · ${Math.round(eaten.carbs)} g carbs · ${Math.round(eaten.fat)} g fat`}
          </Text>
        )
      )}
      {plan && !log?.meals.every(mealEnabled) && (
        <Button title="Log as planned" icon="checkmark-done" onPress={onLog} />
      )}
      <Button title="Open meals" variant="secondary" onPress={onOpen} />
    </Card>
  );
}
