import { uuid } from "../../data/uuid";
import { useMemo, useReducer, useState } from "react";
import { View } from "react-native";

import type { Api } from "../../api/clients";
import { plannedMealDay } from "../../data/plans";
import { mealLogsRepo } from "../../data/repositories/mealLogs";
import type { Store } from "../../data/store";
import { useRefresh } from "../../data/useRefresh";
import { dateLabel, today as todayLocal } from "../../domain/dates";
import { moveItem } from "../../domain/order";
import {
  dayTotals,
  disableMeal,
  enableAll,
  enableMeal,
  mealEnabled,
  mealPlanOf,
  newMealLog,
  onTarget,
  uneatenMeals,
  withMacros,
  withQty,
} from "../../domain/totals";
import type { Item, Meal } from "../../domain/types";
import {
  BackButton,
  Badge,
  Button,
  Card,
  ConfirmSheet,
  DraggableList,
  EmptyState,
  ErrorLine,
  Screen,
  space,
  Text,
} from "../../ui";
import { SyncBadge } from "../sync/SyncBadge";
import { AddItemSheet } from "./AddItemSheet";
import { ItemSheet } from "./ItemSheet";
import { MacroBars } from "./MacroBars";
import { MealCard } from "./MealCard";
import { MealNameSheet } from "./MealNameSheet";

type Props = {
  date: string;
  today?: string;
  store: Store;
  // Unset while offline; the log is edited locally either way.
  api?: Api;
  userId?: string;
};

// The meal log for one date, from the local store. Without a log the plan's meals show disabled
// (every item at qty 0, R44); the first change creates the log. Every edit is saved through the
// repository, so sync picks it up.
export default function MealsScreen({
  date,
  today = todayLocal(),
  store,
  api,
  userId = "",
}: Props) {
  const repo = useMemo(() => mealLogsRepo(store), [store]);
  const planRow = useMemo(() => plannedMealDay(store, date), [store, date]);
  // Read on every render, which a finished sync causes, so an edit lands on the row the sync
  // stored: a merged log may have another id and version (P2-R41).
  const log = repo.byDate(date)[0];
  const plan = mealPlanOf(log, planRow?.programId, planRow?.payload);
  const [, changed] = useReducer((n: number) => n + 1, 0);
  const refresh = useRefresh();
  const [editing, setEditing] = useState<{ meal: number; item: number }>();
  const [adding, setAdding] = useState<number>();
  const [naming, setNaming] = useState(false);
  const [deleting, setDeleting] = useState<number>();
  // Why the last custom item was not saved to the user's foods.
  const [notice, setNotice] = useState<string>();

  const meals = useMemo(() => log?.meals ?? uneatenMeals(plan), [log, plan]);
  const target = log ? log.target : plan?.target;
  const totals = log?.totals ?? dayTotals(meals);

  const save = (meals: Meal[]) => {
    const base =
      log ?? newMealLog({ id: uuid(), userId, date, programId: planRow?.programId }, plan);
    repo.save({ ...base, meals, totals: dayTotals(meals) });
    changed();
  };
  const updateMeal = (at: number, change: (meal: Meal) => Meal) =>
    save(meals.map((m, i) => (i === at ? change(m) : m)));
  const updateItems = (at: number, change: (items: Item[]) => Item[]) =>
    updateMeal(at, (m) => ({ ...m, items: change(m.items) }));

  // The plan's meal for one that has the plan's items; any other meal is eaten as entered (P2-R42).
  const plannedMeal = (meal: Meal) =>
    meal.items.some((i) => i.planned) ? plan?.meals.find((m) => m.name === meal.name) : undefined;
  // A verdict on the day waits for its last planned meal, or for the day to pass: an earlier
  // meal still disabled by then was skipped.
  const lastPlanned = meals.findLast(plannedMeal);
  const dayDone = date < today || !lastPlanned || mealEnabled(lastPlanned);
  const onTargetNow = dayDone ? onTarget(totals, target) : undefined;
  const edited = editing && meals[editing.meal];
  const editedItem = edited?.items[editing!.item];

  return (
    <Screen
      title="Meals"
      testID="meals-screen"
      left={<BackButton />}
      right={<SyncBadge />}
      refresh={refresh}
    >
      <Text variant="caption" numberOfLines={1}>
        {plan ? `${dateLabel(date)} · ${plan.name}` : dateLabel(date)}
      </Text>
      <ErrorLine message={notice} />
      {meals.length > 0 ? (
        <>
          <Card>
            <View style={{ flexDirection: "row", alignItems: "center", gap: space[2] }}>
              <Text variant="heading" style={{ flex: 1 }}>
                Day total
              </Text>
              {onTargetNow !== undefined && (
                <Badge
                  label={onTargetNow ? "On target" : "Off target"}
                  icon={onTargetNow ? "checkmark-circle" : "alert-circle"}
                  tone={onTargetNow ? "accent" : "warning"}
                />
              )}
            </View>
            <MacroBars totals={totals} target={target} />
            {meals.some((m) => plannedMeal(m) && !mealEnabled(m)) && (
              <Button
                title="Enable all"
                icon="checkmark-done"
                variant="secondary"
                onPress={() => save(enableAll(meals, plan))}
              />
            )}
          </Card>
          <DraggableList
            items={meals}
            keyOf={(meal, i) => `${meal.name}-${i}`}
            testID="meals"
            onMove={(from, to) => save(moveItem(meals, from, to))}
            renderItem={(meal, i) => (
              <MealCard
                meal={meal}
                index={i}
                count={meals.length}
                planned={plannedMeal(meal)}
                planDay={!!plan}
                onToggle={() =>
                  updateMeal(i, (m) =>
                    mealEnabled(m) ? disableMeal(m) : enableMeal(m, plannedMeal(m)),
                  )
                }
                onItem={(item) => setEditing({ meal: i, item })}
                onAdd={() => setAdding(i)}
                onMove={(to) => save(moveItem(meals, i, to))}
                onDelete={() => setDeleting(i)}
              />
            )}
          />
          <Button title="Add meal" icon="add" variant="secondary" onPress={() => setNaming(true)} />
        </>
      ) : (
        <EmptyState
          icon="restaurant-outline"
          title="Nothing logged"
          message={`${planRow ? "This day is logged ad hoc." : "No meal plan for this day."} Add a meal to log what you ate.`}
          action={{ title: "Add meal", onPress: () => setNaming(true) }}
        />
      )}
      {edited && editedItem && (
        <ItemSheet
          item={editedItem}
          plannedQty={
            editedItem.planned
              ? plannedMeal(edited)?.items.find((i) => i.foodName === editedItem.foodName)?.qty
              : undefined
          }
          onSave={(qty) => {
            updateItems(editing.meal, (items) =>
              items.map((it, i) => (i === editing.item ? withQty(it, qty) : it)),
            );
            setEditing(undefined);
          }}
          onSaveMacros={(per) => {
            updateItems(editing.meal, (items) =>
              items.map((it, i) => (i === editing.item ? withMacros(it, per) : it)),
            );
            setEditing(undefined);
          }}
          onRemove={() => {
            updateItems(editing.meal, (items) => items.filter((_, i) => i !== editing.item));
            setEditing(undefined);
          }}
          onClose={() => setEditing(undefined)}
        />
      )}
      {adding !== undefined && (
        <AddItemSheet
          meal={meals[adding].name}
          store={store}
          api={api}
          onAdd={(item, why) => {
            updateItems(adding, (items) => [...items, item]);
            setNotice(why);
            setAdding(undefined);
          }}
          onClose={() => setAdding(undefined)}
        />
      )}
      {naming && (
        <MealNameSheet
          onAdd={(name) => {
            save([...meals, { name, items: [] }]);
            setNaming(false);
          }}
          onClose={() => setNaming(false)}
        />
      )}
      <ConfirmSheet
        open={deleting !== undefined}
        title="Delete meal"
        message={
          deleting === undefined
            ? ""
            : `${meals[deleting]?.name} leaves this day's log.${plannedMeal(meals[deleting]) ? " The plan keeps it." : ""}`
        }
        action="Delete"
        onConfirm={() => {
          if (deleting !== undefined) save(meals.filter((_, i) => i !== deleting));
          setDeleting(undefined);
        }}
        onClose={() => setDeleting(undefined)}
      />
    </Screen>
  );
}
