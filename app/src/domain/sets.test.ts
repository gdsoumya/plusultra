import { sets, warmups } from "../api/fixtures/build";
import { exerciseTarget, setsSummary } from "./sets";
import type { ExerciseEntry, SetEntry } from "./types";

describe("setsSummary", () => {
  it("groups equal work sets, shows them in the user's unit and skips warmups", () => {
    const all = [
      ...warmups([[10, 20, "kg"]], 75),
      ...sets(4, 5, 75),
      ...sets(2, 5, 62.5),
      ...sets(1, 8, 62.5),
    ];
    expect(setsSummary(all, "kg")).toBe("4x5 75 kg · 2x5 62.5 kg · 1x8 62.5 kg");
    expect(setsSummary(all, "lb")).toBe("4x5 165.3 lb · 2x5 137.8 lb · 1x8 137.8 lb");
    expect(setsSummary([], "kg")).toBe("");
  });

  it("shows rep ranges, AMRAP sets and bodyweight work", () => {
    const [range, amrap] = sets(2, 12, 60);
    range.minReps = 8;
    amrap.amrap = true;
    expect(setsSummary([range, amrap], "kg")).toBe("1x8-12 60 kg · 1x12+ 60 kg");
    expect(setsSummary(sets(3, 10, 0), "kg")).toBe("3x10");
  });

  it("shows a timed set by its time, with a plus when it counts up", () => {
    expect(setsSummary(sets(3, 1, 0, { work: 45, rest: 30 }), "kg")).toBe("3x0:45 · rest 0:30");
    expect(setsSummary(sets(2, 1, 0, { work: 30, countUp: true }), "kg")).toBe("2x0:30+");
    expect(setsSummary(sets(1, 1, 20, { work: 60 }), "kg")).toBe("1x1:00 20 kg");
  });

  it("shows the rest once when every work set has the same", () => {
    const all = [...warmups([[10, 20, "kg"]], 75), ...sets(3, 5, 75, { rest: 180 })];
    expect(setsSummary(all, "kg")).toBe("3x5 75 kg · rest 3:00");
    expect(setsSummary([...all, ...sets(1, 5, 60, { rest: 90 })], "kg")).toBe(
      "3x5 75 kg · 1x5 60 kg",
    );
  });
});

describe("exerciseTarget", () => {
  const entry = (done: Partial<SetEntry>[]): ExerciseEntry["sets"] =>
    [...warmups([[10, 20, "kg"]], 100), ...sets(3, 5, 100)].map((planned, i) => ({
      planned,
      completed: false,
      reps: 0,
      weight: planned.weight,
      rpe: 0,
      ...done[i],
    }));
  const did = (reps: number, kg = 100): Partial<SetEntry> => ({
    completed: true,
    reps,
    weight: { value: kg, unit: "kg" },
  });

  it("is met when every set, warmups included, is done at the planned weight with the target reps or more", () => {
    const t = exerciseTarget(entry([did(10, 20), did(5), did(6), did(5, 102.5)]));
    expect(t).toMatchObject({
      met: true,
      reps: 26,
      plannedReps: 25,
      load: 1812.5,
      plannedLoad: 1700,
    });
  });

  it("misses on fewer reps, less weight, or a pending set, warmups included", () => {
    expect(exerciseTarget(entry([did(10, 20), did(5), did(4), did(5)])).met).toBe(false);
    expect(exerciseTarget(entry([did(10, 20), did(5), did(5, 95), did(5)])).met).toBe(false);
    expect(exerciseTarget(entry([{}, did(5), did(5), did(5)])).met).toBe(false);
    expect(exerciseTarget(entry([did(10, 20), did(5), did(5)])).met).toBe(false);
    expect(exerciseTarget(entry([{}, did(5), did(5), did(5)])).reps).toBe(15);
  });

  it("is never met without a plan, as for an ad hoc exercise", () => {
    const [planned] = sets(1, 0, 0);
    const set = {
      planned,
      completed: true,
      reps: 10,
      weight: { value: 100, unit: "kg" as const },
      rpe: 0,
    };
    expect(exerciseTarget([set])).toMatchObject({
      met: false,
      reps: 10,
      plannedReps: 0,
      load: 1000,
    });
  });

  it("uses the bottom of a rep range as the target", () => {
    const [range] = sets(1, 12, 60);
    range.minReps = 8;
    const set = { planned: range, completed: true, reps: 8, weight: range.weight, rpe: 0 };
    expect(exerciseTarget([set]).met).toBe(true);
    expect(exerciseTarget([{ ...set, reps: 7 }]).met).toBe(false);
  });

  it("compares and totals in kilograms when the plan arrived in pounds", () => {
    // An lb user's plan comes from the server in lb; an edited set is stored in kg.
    const [planned] = sets(1, 5, 110);
    planned.weight = { value: 242.50848840336533, unit: "lb" };
    const set = { planned, completed: true, reps: 5, weight: planned.weight, rpe: 0 };
    expect(exerciseTarget([set])).toMatchObject({
      met: true,
      reps: 5,
      plannedReps: 5,
      load: 550,
      plannedLoad: 550,
    });
    expect(exerciseTarget([{ ...set, weight: { value: 110, unit: "kg" } }]).met).toBe(true);
  });

  it("counts both sides of a unilateral set against twice the planned reps", () => {
    const [planned] = sets(1, 8, 10);
    const set = { planned, completed: true, reps: 8, repsLeft: 8, weight: planned.weight, rpe: 0 };
    expect(exerciseTarget([set], true)).toMatchObject({
      met: true,
      reps: 16,
      plannedReps: 16,
      load: 160,
      plannedLoad: 160,
    });
    expect(exerciseTarget([{ ...set, repsLeft: 7 }], true)).toMatchObject({ met: false, reps: 15 });
    expect(exerciseTarget([{ ...set, repsLeft: undefined }], true).reps).toBe(8);
    expect(exerciseTarget([set])).toMatchObject({ met: true, reps: 8, plannedReps: 8, load: 80 });
  });

  it("totals the time of timed sets, logged against planned", () => {
    const timed = sets(3, 1, 0, { work: 60 }).map((planned, i) => ({
      planned,
      completed: i < 2,
      reps: i < 2 ? 1 : 0,
      weight: planned.weight,
      rpe: 0,
      workTime: i === 0 ? 50 : undefined,
    }));
    expect(exerciseTarget(timed)).toMatchObject({ work: 110, plannedWork: 180 });
    expect(exerciseTarget(entry([]))).toMatchObject({ work: 0, plannedWork: 0 });
  });

  it("sums the load in the user's unit, so pound plans add up to whole pounds", () => {
    // 265 lb is stored as 120.2 kg; summing in kg and converting back gave 105998.3 lb.
    const lb = sets(4, 100, 0).map((p) => ({
      planned: { ...p, weight: { value: 265, unit: "lb" as const } },
      completed: true,
      reps: 100,
      weight: { value: 120.2, unit: "kg" as const },
      rpe: 0,
    }));
    expect(exerciseTarget(lb, false, "lb")).toMatchObject({ load: 106000, plannedLoad: 106000 });
  });
});
