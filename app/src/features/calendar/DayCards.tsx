import type { ReactNode } from "react";
import { View } from "react-native";

import type {
  Checkin,
  DayStatus,
  MealDay,
  MealLog,
  ResolvedDay,
  Unit,
  WorkoutRecord,
} from "../../domain/types";
import { setsSummary } from "../../domain/sets";
import { formatWeight } from "../../domain/units";
import { Button, Card, Chip, ListRow, space, Text } from "../../ui";
import { ExerciseRow } from "../exercises/ExerciseRow";
import { CardMenu, type MenuItem } from "./CardMenu";
import { kcal } from "./format";

// A row with the card's overflow menu beside it.
const MenuRow = ({
  title,
  menu,
  children,
}: {
  title: string;
  menu: MenuItem[];
  children: ReactNode;
}) => (
  <View style={{ flexDirection: "row", alignItems: "center" }}>
    <View style={{ flex: 1 }}>{children}</View>
    <CardMenu title={title} items={menu} />
  </View>
);

export function WorkoutCard({
  record,
  onPress,
  menu,
}: {
  record: WorkoutRecord;
  onPress: () => void;
  menu: MenuItem[];
}) {
  const sets = record.entries.flatMap((e) => e.sets);
  const done = sets.filter((s) => s.completed).length;
  return (
    <Card>
      <MenuRow title="Workout" menu={menu}>
        <ListRow
          icon="barbell-outline"
          title={record.dayName || "Workout"}
          subtitle={`${done} of ${sets.length} sets done`}
          value={record.finalized ? "Done" : record.finishedAt ? "Finished" : "In progress"}
          onPress={onPress}
        />
      </MenuRow>
    </Card>
  );
}

export function PlanCard({ plan, unit }: { plan: ResolvedDay; unit: Unit }) {
  return (
    <Card>
      <Text variant="heading">{plan.name}</Text>
      {plan.desc ? <Text variant="caption">{plan.desc}</Text> : null}
      {plan.exercises.map((e, i) => (
        <ExerciseRow
          key={`${e.name}-${i}`}
          exercise={{ slug: e.slug, name: e.name }}
          subtitle={setsSummary(e.sets, unit) + (e.unilateral ? " · per side" : "")}
        />
      ))}
      {!plan.exercises.length && <Text color="textMuted">Rest day</Text>}
    </Card>
  );
}

export function AddWorkout({ onAdd }: { onAdd: () => void }) {
  return (
    <Card>
      <Button title="Add workout" variant="secondary" icon="add" onPress={onAdd} />
    </Card>
  );
}

const mealKcal = (meal: MealDay["meals"][number]) =>
  meal.items.reduce((t, i) => t + i.macros.kcal, 0);

export function MealsCard({
  log,
  plan,
  onPress,
  onAdd,
  menu = [],
}: {
  log?: MealLog;
  plan?: MealDay;
  onPress: () => void;
  // Set on a past or present date without a log; creates it.
  onAdd?: () => void;
  menu?: MenuItem[];
}) {
  if (log) {
    const target = log.target ? ` of ${Math.round(log.target.kcal)} kcal` : " kcal";
    return (
      <Card>
        <MenuRow title="Meals" menu={menu}>
          <ListRow
            icon="restaurant-outline"
            title="Meals"
            subtitle={`${Math.round(log.totals.kcal)}${target}`}
            onPress={onPress}
          />
        </MenuRow>
      </Card>
    );
  }
  // Nothing is logged yet: the plan shows read-only, but never on a date the user can still fill,
  // where only what they pick counts (P2-R35).
  if (onAdd) {
    return (
      <Card>
        <ListRow icon="restaurant-outline" title="Meals" subtitle="Nothing logged" />
        <Button title="Add meals" variant="secondary" icon="add" onPress={onAdd} />
      </Card>
    );
  }
  if (!plan) return null;
  return (
    <Card>
      <ListRow
        icon="restaurant-outline"
        title="Meals"
        subtitle={plan.target ? `Target ${kcal(plan.target.kcal)}` : plan.name}
        onPress={onPress}
      />
      {plan.meals.map((m) => (
        <ListRow key={m.name} title={m.name} value={kcal(mealKcal(m))} />
      ))}
    </Card>
  );
}

export function CheckinCard({ checkin, unit }: { checkin: Checkin; unit: Unit }) {
  return (
    <Card>
      <ListRow
        icon="scale-outline"
        title="Check-in"
        value={checkin.weight && formatWeight(checkin.weight, unit)}
      />
    </Card>
  );
}

const choices: { status: DayStatus; label: string }[] = [
  { status: "rest", label: "Rest" },
  { status: "skipped", label: "Skipped" },
];

export function StatusEditor({
  status,
  onChange,
}: {
  status?: DayStatus;
  onChange: (status: DayStatus) => void;
}) {
  return (
    <Card>
      <Text variant="label" color="textMuted">
        Mark this day as
      </Text>
      <View style={{ flexDirection: "row", gap: space[2] }}>
        {choices.map((c) => (
          <Chip
            key={c.status}
            label={c.label}
            selected={status === c.status}
            onPress={() => onChange(c.status)}
          />
        ))}
      </View>
    </Card>
  );
}
