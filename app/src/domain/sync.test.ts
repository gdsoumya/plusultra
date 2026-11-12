import { sets } from "../api/fixtures/build";
import { dayLog } from "../data/test-utils";
import { merge } from "./sync";
import { dayTotals } from "./totals";
import type { Item, Meal, MealLog, SetEntry, WorkoutRecord } from "./types";

const earlier = "2026-11-10T08:00:00.000Z";
const later = "2026-11-10T09:00:00.000Z";

const set = (completed: boolean, reps = 5): SetEntry => ({
  planned: sets(1, 5, 100)[0],
  completed,
  reps: completed ? reps : 0,
  weight: { value: completed ? 100 : 0, unit: "kg" },
  rpe: 0,
});

const record = (updatedAt: string, done: boolean[], over: Partial<WorkoutRecord> = {}) =>
  ({
    ...dayLog("w", "2026-11-10"),
    updatedAt,
    week: 1,
    day: 1,
    dayName: "A",
    dayDesc: "",
    entries: [{ name: "Squat", sets: done.map((d) => set(d)) }],
    finalized: false,
    finalizeRequested: false,
    ...over,
  }) as unknown as WorkoutRecord;

describe("merge", () => {
  it("re-pushes a newer local row and lets a newer server row stand", () => {
    const local = dayLog("a", "2026-11-10", { notes: "local", updatedAt: later, dirty: true });
    const server = dayLog("a", "2026-11-10", { notes: "server", updatedAt: earlier, version: 2 });
    expect(merge("dayLogs", local, server)).toBe(local);
    expect(
      merge("dayLogs", { ...local, updatedAt: earlier }, { ...server, updatedAt: later }),
    ).toBe(undefined);
  });

  it("lets a server delete stand and re-pushes a newer local delete", () => {
    const local = dayLog("a", "2026-11-10", { updatedAt: later, dirty: true });
    const gone = dayLog("a", "2026-11-10", { deletedAt: earlier, updatedAt: earlier, version: 2 });
    expect(merge("dayLogs", local, gone)).toBe(undefined);

    const removed = { ...local, deletedAt: later };
    const server = dayLog("a", "2026-11-10", { updatedAt: earlier, version: 2 });
    expect(merge("dayLogs", removed, server)).toBe(removed);
    expect(
      merge("dayLogs", { ...removed, updatedAt: earlier }, { ...server, updatedAt: later }),
    ).toBe(undefined);
  });

  it("keeps local set completions and the finalize request over a newer server record", () => {
    const local = record(earlier, [true, false, true], { finalizeRequested: true, dirty: true });
    const server = record(later, [false, true, false], { dayDesc: "server", version: 2 });
    const merged = merge("workoutRecords", local, server)!;
    expect(merged.entries[0].sets.map((s) => s.completed)).toEqual([true, true, true]);
    expect(merged.entries[0].sets[0]).toEqual(local.entries[0].sets[0]);
    expect(merged.entries[0].sets[1]).toEqual(server.entries[0].sets[1]);
    expect(merged).toMatchObject({ dayDesc: "server", version: 2, finalizeRequested: true });
  });

  it("drops the finalize request once the server has finalized", () => {
    const local = record(earlier, [true], { finalizeRequested: true, dirty: true });
    const server = record(later, [true], { finalized: true, version: 2 });
    expect(merge("workoutRecords", local, server)).toBe(undefined);
  });

  it("keeps completed sets the server record does not have", () => {
    const local = record(earlier, [true, true, true], { dirty: true });
    const server = record(later, [true, false], { version: 2 });
    const merged = merge("workoutRecords", local, server)!;
    expect(merged.entries[0].sets.map((s) => s.completed)).toEqual([true, true, true]);
  });

  it("lets a server record stand when it already has every local completion", () => {
    const local = record(earlier, [true, false], { dirty: true });
    const server = record(later, [true, true], { version: 2 });
    expect(merge("workoutRecords", local, server)).toBe(undefined);
  });

  it("keeps both devices' completions of one planned day, whichever record is newer", () => {
    const local = record(later, [true, false, false], { id: "l", dirty: true });
    const server = record(earlier, [false, true, false], { version: 2 });
    const merged = merge("workoutRecords", local, server)!;
    expect(merged.id).toBe(server.id);
    expect(merged.entries[0].sets.map((s) => s.completed)).toEqual([true, true, false]);
  });

  it("re-pushes a newer local record as it is", () => {
    const local = record(later, [false, true], { dirty: true });
    const server = record(earlier, [true, false], { version: 2 });
    expect(merge("workoutRecords", local, server)).toBe(local);
  });

  describe("two meal logs of one date", () => {
    const item = (foodName: string, qty: number, planned = true): Item => ({
      foodName,
      qty,
      unit: "g",
      planned,
      macros: { kcal: qty, protein: 0, carbs: 0, fat: 0, fiber: 0, waterMl: 0 },
    });
    const log = (id: string, updatedAt: string, meals: Meal[], over: Partial<MealLog> = {}) =>
      ({
        ...dayLog(id, "2026-11-10"),
        updatedAt,
        meals,
        totals: dayTotals(meals),
        ...over,
      }) as unknown as MealLog;
    const eaten = (l: MealLog) => l.meals.map((m) => [m.name, m.items.map((i) => i.qty)]);

    it("keeps what each logged: an eaten item over a not eaten one, and meals only one has", () => {
      const server = log("s", earlier, [
        { name: "Breakfast", items: [item("Oats", 80), item("Milk", 0)] },
        { name: "Lunch", items: [item("Rice", 0)] },
      ]);
      const local = log("l", later, [
        { name: "Breakfast", items: [item("Oats", 0), item("Milk", 250), item("Egg", 2, false)] },
        { name: "meal", items: [item("Chicken", 100, false)] },
      ]);
      const merged = merge("mealLogs", local, server)!;
      expect(eaten(merged)).toEqual([
        ["Breakfast", [80, 250, 2]],
        ["Lunch", [0]],
        ["meal", [100]],
      ]);
      expect(merged.totals).toEqual(dayTotals(merged.meals));
    });

    it("pairs repeated foods and meals one to one", () => {
      const server = log("s", later, [{ name: "Snack", items: [item("Banana", 0)] }]);
      const local = log("l", earlier, [
        { name: "Snack", items: [item("Banana", 120), item("Banana", 60)] },
        { name: "Snack", items: [item("Nuts", 30)] },
      ]);
      expect(eaten(merge("mealLogs", local, server)!)).toEqual([
        ["Snack", [120, 60]],
        ["Snack", [30]],
      ]);
    });

    it("takes the plan's target and program from the log that has them", () => {
      const meals = [{ name: "meal", items: [item("Chicken", 100, false)] }];
      const zero = { kcal: 0, protein: 0, carbs: 0, fat: 0, fiber: 0, waterMl: 0 };
      const target = { ...zero, kcal: 2200, protein: 170 };
      const server = log("s", earlier, meals, { target: zero });
      const local = log("l", later, [], { target, programId: "p" });
      expect(merge("mealLogs", local, server)).toMatchObject({ target, programId: "p" });
    });

    it("lets the server log stand when the other adds nothing to it", () => {
      const server = log("s", earlier, [{ name: "Lunch", items: [item("Rice", 150)] }]);
      expect(merge("mealLogs", log("l", later, []), server)).toBe(undefined);
      const planned = log("l", later, [{ name: "Lunch", items: [item("Rice", 0)] }]);
      expect(merge("mealLogs", planned, server)).toBe(undefined);
    });

    it("lets a deleted server log stand", () => {
      const server = log("s", earlier, [], { deletedAt: earlier });
      expect(merge("mealLogs", log("l", later, [{ name: "meal", items: [] }]), server)).toBe(
        undefined,
      );
    });
  });
});
