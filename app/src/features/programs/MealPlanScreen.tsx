import { dayTotals, mealTotals } from "../../domain/totals";
import { useStore } from "../../data/StoreProvider";
import { BackButton, Card, EmptyState, ListCard, ListRow, Screen, Text } from "../../ui";
import { macroLine } from "../meals/format";
import { MacroStats } from "../meals/MacroStats";
import { cachedMealDays, mealDaySegment } from "./programStore";

type Props = { programId: string; week: number; day: string; n: number };

// One planned meal: every item with its quantity and macros, the meal's totals, then the whole
// day against its target. Nothing is edited here; the meals screen logs what was eaten.
export default function MealPlanScreen({ programId, week, day, n }: Props) {
  const store = useStore();
  const planned = cachedMealDays(store, programId).find(
    (d) => d.week === week && mealDaySegment(d) === day,
  );
  const meal = planned?.meals[n - 1];
  const back = <BackButton fallback={`/program/${programId}`} />;
  if (!planned || !meal) {
    return (
      <Screen title="Meal" left={back} testID="meal-plan-screen">
        <EmptyState
          icon="cloud-offline-outline"
          title="Plan not available"
          message="Open the program while connected to load its days."
        />
      </Screen>
    );
  }
  const where = planned.day ? `week ${week}, day ${planned.day}` : `week ${week}`;
  return (
    <Screen title={meal.name} left={back} testID="meal-plan-screen">
      <Card>
        <Text variant="caption">
          {[`${planned.name} · ${where}`, meal.time].filter(Boolean).join(" · ")}
        </Text>
        {meal.desc ? <Text>{meal.desc}</Text> : null}
        <MacroStats totals={mealTotals(meal)} />
      </Card>
      <ListCard>
        {meal.items.map((item, i) => (
          <ListRow
            key={`${item.foodName}-${i}`}
            title={item.foodName}
            subtitle={[macroLine(item.macros), item.note].filter(Boolean).join(" · ")}
            value={`${Math.round(item.qty * 10) / 10} ${item.unit}`}
          />
        ))}
      </ListCard>
      <Card>
        <Text variant="heading">Day vs target</Text>
        <MacroStats totals={dayTotals(planned.meals)} target={planned.target} />
      </Card>
    </Screen>
  );
}
