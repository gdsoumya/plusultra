import type { DayKind, MealDay, ProgramSummary, ResolvedDay } from "../domain/types";
import { createMemoryStore } from "./memoryStore";
import { plannedMealDay } from "./plans";
import { programsRepo } from "./repositories/programs";
import { mealDaysRepo, resolvedDaysRepo } from "./repositories/resolvedDays";

const today = "2026-11-11";

const program = (id: string, kind: ProgramSummary["kind"], over: Partial<ProgramSummary> = {}) =>
  ({
    id,
    kind,
    name: id,
    revision: 1,
    status: "active",
    mode: "floating",
    nextWeek: 1,
    nextDay: 1,
    weeks: 2,
    updatedAt: "2026-11-01T00:00:00.000Z",
    ...over,
  }) satisfies ProgramSummary;

const workoutDay = (week: number, day: number, kind: DayKind) => {
  const payload: ResolvedDay = { week, day, kind, name: "", desc: "", exercises: [] };
  return { programId: "w", week, day, payload };
};

// A pulled floating meal day: the server sends one row per week for each numbered day and kind.
const mealDay = (week: number, day: number, kind: DayKind, name: string) => {
  const payload: MealDay = { week, day, kind, name, desc: "", meals: [] };
  return { programId: "m", week, day, payload };
};

function seed(workout: ProgramSummary | undefined, nextWeek: number, nextDay: number) {
  const store = createMemoryStore();
  const programs: ProgramSummary[] = [program("m", "meal")];
  if (workout) programs.push({ ...workout, nextWeek, nextDay });
  programsRepo(store).upsertFromServer(programs);
  resolvedDaysRepo(store).upsertFromServer([
    workoutDay(1, 1, "training"),
    workoutDay(1, 2, "conditioning"),
    workoutDay(2, 1, "training"),
    workoutDay(3, 1, "training"),
  ]);
  mealDaysRepo(store).upsertFromServer([
    mealDay(1, 0, "training", "Lift wk1"),
    mealDay(1, 0, "rest", "Rest wk1"),
    mealDay(2, 1, "training", "Day 1 wk2"),
    mealDay(2, 0, "training", "Lift wk2"),
    mealDay(2, 0, "rest", "Rest wk2"),
  ]);
  return store;
}

describe("plannedMealDay for a floating meal program", () => {
  const workout = program("w", "workout", { weeks: 3 });
  const name = (store: ReturnType<typeof seed>) => plannedMealDay(store, today)?.payload.name;

  it("picks the workout's week and kind", () => {
    expect(name(seed(workout, 1, 1))).toBe("Lift wk1");
  });

  it("prefers the week's numbered day", () => {
    expect(name(seed(workout, 2, 1))).toBe("Day 1 wk2");
  });

  it("falls back from conditioning to training", () => {
    expect(name(seed(workout, 1, 2))).toBe("Lift wk1");
  });

  it("takes the latest meal week at or below a workout week the meal plan does not reach", () => {
    expect(name(seed(workout, 3, 1))).toBe("Lift wk2");
  });

  it("eats like a rest day in week 1 without a workout program", () => {
    expect(name(seed(undefined, 1, 1))).toBe("Rest wk1");
  });
});
