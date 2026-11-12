// Pure macro math for meal logs: scaling items by quantity, summing meals and days, and comparing
// to a target. Nothing here rounds; screens round for display.
import type { LocalRow } from "../data/repositories/syncRepo";
import type { Item, Macros, Meal, MealDay, MealLog } from "./types";

const macroKeys = ["kcal", "protein", "carbs", "fat", "fiber", "waterMl"] as const;
export type MacroKey = (typeof macroKeys)[number];

const combine = (f: (k: MacroKey) => number) =>
  Object.fromEntries(macroKeys.map((k) => [k, f(k)])) as unknown as Macros;

export const scaleMacros = (m: Macros, factor: number) => combine((k) => m[k] * factor);

export const sumMacros = (all: Macros[]) => combine((k) => all.reduce((t, m) => t + m[k], 0));

// Macros of qty of the item's food, from the macros frozen when it was planned or logged: per unit
// when counted in the food's unit, else per 100 g. A hand-typed food has neither and rescales its
// own macros. A quantity of 0 is a planned item that was not eaten.
export function itemMacros(item: Item, qty: number): Macros {
  if (item.perUnit && item.unit === item.foodUnit) return scaleMacros(item.perUnit, qty);
  if (item.per100g && item.unit === "g") return scaleMacros(item.per100g, qty / 100);
  return item.qty > 0 ? scaleMacros(item.macros, qty / item.qty) : scaleMacros(item.macros, 0);
}

export const withQty = (item: Item, qty: number): Item => ({
  ...item,
  qty,
  macros: itemMacros(item, qty),
});

// The item is counted in its food's unit, so its macros come per unit rather than per 100 g.
export const countedPerUnit = (item: Item) => !!item.perUnit && item.unit === item.foodUnit;

// The item with its own macros per unit or per 100 g, as it is counted, in place of its food's;
// it keeps the food's ref for provenance (P2-R44).
export const withMacros = (item: Item, per: Macros): Item =>
  withQty(
    { ...item, ...(countedPerUnit(item) ? { perUnit: per } : { per100g: per }), edited: true },
    item.qty,
  );

export const mealTotals = (meal: Meal) => sumMacros(meal.items.map((i) => i.macros));

export const dayTotals = (meals: Meal[]) => sumMacros(meals.map(mealTotals));

// Share of the target reached; 0 when there is no target for key.
export const fractionOf = (totals: Macros, target: Macros | undefined, key: MacroKey) =>
  target?.[key] ? totals[key] / target[key] : 0;

interface MealLogMeta {
  id: string;
  userId: string;
  date: string;
  programId?: string;
}

// The meal log for a date as planned: every item planned, at its planned quantity.
export function mealQuickFill(day: MealDay, meta: MealLogMeta): LocalRow<MealLog> {
  const meals = plannedMeals(day);
  return { ...meta, version: 0, target: day.target, meals, totals: dayTotals(meals) };
}

const plannedMeals = (day: MealDay) =>
  day.meals.map((m) => ({ ...m, items: m.items.map((i) => ({ ...i, planned: true })) }));

// The plan's meals before anything is eaten: every item planned, at qty 0 (R44).
export const uneatenMeals = (day?: MealDay) => (day ? plannedMeals(day).map(disableMeal) : []);

// The log for a date with nothing eaten yet: the plan's meals disabled, or none without a plan.
// Meals and items are then added or enabled on the meals screen.
export function newMealLog(meta: MealLogMeta, day?: MealDay): LocalRow<MealLog> {
  const meals = uneatenMeals(day);
  return { ...meta, version: 0, target: day?.target, meals, totals: dayTotals(meals) };
}

// The plan a date's meals follow: its planned meal day, unless the log was made without it, ad hoc
// or from another program (P2-R42).
export const mealPlanOf = (
  log: { programId?: string } | undefined,
  programId: string | undefined,
  plan: MealDay | undefined,
) => (!log || log.programId === programId ? plan : undefined);

// The plan semantics (R44) cover planned items only: a meal counts as eaten (enabled) once one has
// a quantity, and one without any (ad hoc, or being filled in) is eaten as entered (P2-R42).
export const mealEnabled = (meal: Meal) =>
  !meal.items.some((i) => i.planned) || meal.items.some((i) => i.planned && i.qty > 0);

// Not eaten: its planned items stay in the log at qty 0 (R44); added items keep their quantities.
export const disableMeal = (meal: Meal): Meal => ({
  ...meal,
  items: meal.items.map((i) => (i.planned ? withQty(i, 0) : i)),
});

// Eaten as planned: zeroed planned items take the plan's item back.
export const enableMeal = (meal: Meal, planned: Meal | undefined): Meal => ({
  ...meal,
  items: meal.items.map((i) => {
    const p = i.planned && i.qty === 0 && planned?.items.find((x) => x.foodName === i.foodName);
    return p ? { ...p, planned: true } : i;
  }),
});

export const enableAll = (meals: Meal[], plan: MealDay | undefined) =>
  meals.map((m) =>
    mealEnabled(m)
      ? m
      : enableMeal(
          m,
          plan?.meals.find((p) => p.name === m.name),
        ),
  );

// On target means kcal within 10% and protein within 15 g of the target; undefined without one.
export const KCAL_TOLERANCE = 0.1;
export const PROTEIN_TOLERANCE_G = 15;
export const onTarget = (totals: Macros, target: Macros | undefined) =>
  target &&
  Math.abs(totals.kcal - target.kcal) <= KCAL_TOLERANCE * target.kcal &&
  Math.abs(totals.protein - target.protein) <= PROTEIN_TOLERANCE_G;
