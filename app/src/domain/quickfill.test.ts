import { ex, sets } from "../api/fixtures/build";
import type { ExerciseEntry, ResolvedDay } from "./types";
import { completeAsPlanned, entries, fill, workoutFromDay } from "./quickfill";

const day: ResolvedDay = {
  week: 3,
  day: 2,
  date: "2026-11-11",
  name: "Upper A",
  desc: "Heavy bench",
  kind: "training",
  exercises: [
    ex("Bench Press", [...sets(1, 5, 40, { warmup: true }), ...sets(3, 5, 80, { rest: 180 })]),
    ex("Lat Pulldown", sets(2, 12, 50)),
  ],
};

const row = { id: "w-1", date: "2026-11-11", userId: "user-1" };
const now = "2026-11-11T07:00:00.000Z";

describe("entries", () => {
  it("snapshots every planned set, completed where the exercise is done", () => {
    const all = entries(day, (i) => i === 1);
    expect(all.map((e) => e.name)).toEqual(["Bench Press", "Lat Pulldown"]);
    expect(all[0]).toMatchObject({ slug: "bench-press-barbell", variation: 0, promptValues: {} });
    expect(all[0].sets.map((s) => [s.completed, s.reps, s.planned.reps])).toEqual([
      [false, 0, 5],
      [false, 0, 5],
      [false, 0, 5],
      [false, 0, 5],
    ]);
    expect(all[1].sets.map((s) => [s.completed, s.reps, s.weight.value])).toEqual([
      [true, 12, 50],
      [true, 12, 50],
    ]);
  });
});

describe("fill", () => {
  it("completes pending sets as planned and keeps done sets as logged", () => {
    const started: ExerciseEntry[] = entries(day, () => false);
    started[0].sets[1] = { ...started[0].sets[1], completed: true, reps: 4, rpe: 9 };
    const filled = fill(started);
    expect(filled[0].sets.map((s) => [s.completed, s.reps, s.rpe])).toEqual([
      [true, 5, 0],
      [true, 4, 9],
      [true, 5, 0],
      [true, 5, 0],
    ]);
    expect(filled[1].sets.every((s) => s.completed && s.reps === 12)).toBe(true);
  });
});

describe("workoutFromDay", () => {
  it("starts a record from the day with nothing done", () => {
    const record = workoutFromDay(day, "program-1", row, now);
    expect(record).toMatchObject({
      id: "w-1",
      date: "2026-11-11",
      userId: "user-1",
      version: 0,
      programId: "program-1",
      week: 3,
      day: 2,
      dayName: "Upper A",
      dayDesc: "Heavy bench",
      kind: "training",
      startedAt: now,
      finalized: false,
      finalizeRequested: false,
    });
    expect(record.finishedAt).toBeUndefined();
    expect(record.entries).toEqual(entries(day, () => false));
  });
});

describe("completeAsPlanned", () => {
  it("fills the record, ends it and asks the server to finalize", () => {
    const done = completeAsPlanned(workoutFromDay(day, "program-1", row, now), now);
    expect(done).toMatchObject({ finishedAt: now, finalizeRequested: true, finalized: false });
    const flat = done.entries.flatMap((e) => e.sets);
    expect(flat).toHaveLength(6);
    expect(flat.every((s) => s.completed && s.reps === s.planned.reps)).toBe(true);
    expect(flat.every((s) => s.weight === s.planned.weight)).toBe(true);
  });
});
