import type { Item, Macros, Meal, MealDay } from "./types";
import {
  dayTotals,
  disableMeal,
  enableAll,
  enableMeal,
  KCAL_TOLERANCE,
  mealEnabled,
  mealPlanOf,
  onTarget,
  PROTEIN_TOLERANCE_G,
  fractionOf,
  itemMacros,
  mealQuickFill,
  mealTotals,
  newMealLog,
  sumMacros,
  withQty,
} from "./totals";

const macros = (kcal: number, protein: number, carbs: number, fat: number): Macros => ({
  kcal,
  protein,
  carbs,
  fat,
  fiber: 0,
  waterMl: 0,
});

// Scaled macros carry float noise (6.3 * 3), so compare to 6 digits.
const close = (m: Macros) =>
  Object.fromEntries(Object.entries(m).map(([k, v]) => [k, expect.closeTo(v, 6)]));

const chickenPer100g = macros(120, 22.5, 0, 2.6);
const eggPerUnit = macros(71.5, 6.3, 0.4, 4.8);

const chicken = (qty: number): Item => ({
  foodName: "Chicken breast, raw",
  foodRef: "usda:170000",
  per100g: chickenPer100g,
  qty,
  unit: "g",
  macros: itemMacros({ per100g: chickenPer100g, unit: "g" } as Item, qty),
  planned: true,
});

const eggs = (qty: number): Item => ({
  foodName: "Egg, whole, raw",
  per100g: macros(143, 12.6, 0.7, 9.5),
  perUnit: eggPerUnit,
  foodUnit: "egg",
  qty,
  unit: "egg",
  macros: itemMacros({ perUnit: eggPerUnit, foodUnit: "egg", unit: "egg" } as Item, qty),
  planned: true,
});

describe("item macros", () => {
  it("scale the frozen per-100 g macros by the quantity in grams", () => {
    expect(chicken(200).macros).toEqual(close(macros(240, 45, 0, 5.2)));
  });

  it("scale the frozen per-unit macros when counted in the food's unit", () => {
    expect(eggs(2).macros).toEqual(close(macros(143, 12.6, 0.8, 9.6)));
  });

  it("count zero for a planned item that was not eaten", () => {
    expect(withQty(chicken(200), 0)).toMatchObject({ qty: 0, macros: macros(0, 0, 0, 0) });
  });

  it("rescale a hand-typed food from its own macros", () => {
    const typed: Item = {
      foodName: "Canteen dal",
      qty: 200,
      unit: "g",
      macros: macros(180, 10, 24, 4),
      planned: false,
    };
    expect(withQty(typed, 100).macros).toEqual(close(macros(90, 5, 12, 2)));
    expect(withQty({ ...typed, qty: 0 }, 100).macros).toEqual(macros(0, 0, 0, 0));
  });
});

describe("totals", () => {
  const breakfast = { name: "Breakfast", items: [eggs(3), chicken(0)] };
  const lunch = { name: "Lunch", items: [chicken(200)] };

  it("sum a meal and a day", () => {
    expect(mealTotals(breakfast)).toEqual(close(macros(214.5, 18.9, 1.2, 14.4)));
    expect(dayTotals([breakfast, lunch])).toEqual(close(macros(454.5, 63.9, 1.2, 19.6)));
    expect(sumMacros([])).toEqual(macros(0, 0, 0, 0));
  });

  it("compare to a target, with no progress on an unset target", () => {
    const totals = macros(1300, 85, 0, 0);
    expect(fractionOf(totals, macros(2600, 170, 0, 70), "kcal")).toBe(0.5);
    expect(fractionOf(totals, macros(2600, 170, 0, 70), "carbs")).toBe(0);
    expect(fractionOf(totals, undefined, "kcal")).toBe(0);
  });
});

describe("meal quick fill", () => {
  const day: MealDay = {
    week: 2,
    day: 0,
    kind: "training",
    name: "Training day",
    desc: "",
    target: macros(2600, 170, 300, 70),
    meals: [
      { name: "Breakfast", time: "07:30", items: [{ ...eggs(3), planned: false }] },
      { name: "Lunch", time: "13:00", items: [chicken(200)] },
    ],
  };

  it("logs every planned item at its planned quantity", () => {
    const log = mealQuickFill(day, {
      id: "meal-1",
      userId: "user-1",
      date: "2026-11-11",
      programId: "program-meals",
    });
    expect(log).toMatchObject({
      id: "meal-1",
      userId: "user-1",
      date: "2026-11-11",
      programId: "program-meals",
      version: 0,
      target: day.target,
      totals: close(macros(454.5, 63.9, 1.2, 19.6)),
    });
    expect(log.meals.map((m) => [m.name, m.time])).toEqual([
      ["Breakfast", "07:30"],
      ["Lunch", "13:00"],
    ]);
    expect(log.meals.flatMap((m) => m.items.map((i) => [i.qty, i.planned]))).toEqual([
      [3, true],
      [200, true],
    ]);
  });

  it("starts a new log with the plan's meals disabled and their items still planned", () => {
    const log = newMealLog({ id: "meal-1", userId: "user-1", date: "2026-11-11" }, day);
    expect(log.meals.flatMap((m) => m.items.map((i) => [i.qty, i.planned]))).toEqual([
      [0, true],
      [0, true],
    ]);
    expect(newMealLog({ id: "meal-2", userId: "user-1", date: "2026-11-11" }).meals).toEqual([]);
  });
});

describe("meal enabling", () => {
  const planned: Meal = { name: "Lunch", items: [chicken(200), eggs(2)] };
  const adHoc: Item = { ...chicken(50), foodName: "Rice", planned: false };

  it("a meal is enabled when a planned item has a quantity; one without any is eaten as entered", () => {
    expect(mealEnabled(planned)).toBe(true);
    expect(mealEnabled(disableMeal(planned))).toBe(false);
    expect(mealEnabled({ name: "Extra", items: [] })).toBe(true);
    expect(mealEnabled({ name: "Extra", items: [{ ...adHoc, qty: 0 }] })).toBe(true);
    expect(mealEnabled(disableMeal({ ...planned, items: [...planned.items, adHoc] }))).toBe(false);
  });

  it("disabling zeroes the planned items without deleting them; added items keep theirs", () => {
    const off = disableMeal({ ...planned, items: [...planned.items, adHoc] });
    expect(off.items.map((i) => i.qty)).toEqual([0, 0, 50]);
    expect(off.items[0].macros.kcal).toBe(0);
    expect(mealTotals(off)).toEqual(adHoc.macros);
    expect(disableMeal({ name: "Extra", items: [adHoc] }).items).toEqual([adHoc]);
  });

  it("enabling restores the planned quantities and leaves ad hoc items alone", () => {
    const off = disableMeal({ ...planned, items: [...planned.items, { ...adHoc, qty: 0 }] });
    const on = enableMeal(off, planned);
    expect(on.items.map((i) => i.qty)).toEqual([200, 2, 0]);
    expect(on.items[0].macros).toEqual(close(chicken(200).macros));
    expect(on.items.every((i) => i.planned || i.foodName === "Rice")).toBe(true);
    expect(enableMeal(off, undefined).items.every((i) => i.qty === 0)).toBe(true);
  });

  it("follows the plan only for a log made from it (P2-R42)", () => {
    const day = { meals: [planned] } as MealDay;
    expect(mealPlanOf(undefined, "p", day)).toBe(day);
    expect(mealPlanOf({ programId: "p" }, "p", day)).toBe(day);
    expect(mealPlanOf({}, "p", day)).toBeUndefined();
    expect(mealPlanOf({ programId: "other" }, "p", day)).toBeUndefined();
  });

  it("enableAll only touches disabled meals", () => {
    const eaten = { ...planned, name: "Breakfast", items: [chicken(100)] };
    const plan = { meals: [eaten, planned] } as MealDay;
    const out = enableAll([{ ...eaten, items: [chicken(50)] }, disableMeal(planned)], plan);
    expect(out[0].items[0].qty).toBe(50);
    expect(out[1].items.map((i) => i.qty)).toEqual([200, 2]);
  });
});

describe("onTarget", () => {
  const target = macros(2000, 150, 200, 70);
  it("is within 10% kcal and 15 g protein of the target", () => {
    expect(onTarget(macros(1800, 135, 0, 0), target)).toBe(true);
    expect(onTarget(macros(2200, 165, 0, 0), target)).toBe(true);
    expect(onTarget(macros(1799, 150, 0, 0), target)).toBe(false);
    expect(onTarget(macros(2000, 134, 0, 0), target)).toBe(false);
    expect(KCAL_TOLERANCE).toBe(0.1);
    expect(PROTEIN_TOLERANCE_G).toBe(15);
  });
  it("is undefined without a target", () => {
    expect(onTarget(macros(1, 1, 1, 1), undefined)).toBeUndefined();
  });
});
