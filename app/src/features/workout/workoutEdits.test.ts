import { ex, sets, warmups } from "../../api/fixtures/build";
import { entries } from "../../domain/quickfill";
import type { ExerciseEntry, WorkoutRecord } from "../../domain/types";
import {
  addExercise,
  addSet,
  completeSet,
  exerciseModified,
  finish,
  markModified,
  nextPending,
  removeExercise,
  removeSet,
  setValues,
  shown,
  switchAlternate,
  tapDone,
} from "./workoutEdits";

const bench = (): ExerciseEntry =>
  entries(
    {
      week: 1,
      day: 1,
      name: "A",
      desc: "",
      kind: "training",
      exercises: [
        ex(
          "Bench Press",
          [...warmups([[10, 20, "kg"]], 80), ...sets(2, 5, 80, { rest: 180, label: "Work" })],
          { alts: ["Bench Press, Dumbbell"] },
        ),
      ],
    },
    () => false,
  )[0];

const record = (): WorkoutRecord => ({
  id: "w",
  userId: "u",
  updatedAt: "",
  version: 0,
  deviceId: "",
  dirty: false,
  date: "2026-11-11",
  week: 1,
  day: 1,
  dayName: "A",
  dayDesc: "",
  entries: [bench()],
  startedAt: "2026-11-11T07:00:00.000Z",
  finalized: false,
  finalizeRequested: false,
});

describe("workout edits", () => {
  it("completes a pending set with its shown values, and a second tap undoes it", () => {
    const done = tapDone(bench(), 1);
    expect(done.sets[1]).toMatchObject({ completed: true, reps: 5, weight: { value: 80 } });
    expect(done.sets[1].edited).toBeFalsy();
    expect(done.sets[2].completed).toBe(false);
    expect(tapDone(done, 1).sets[1]).toMatchObject({ completed: false, reps: 0 });
  });

  it("completes a set with its work time", () => {
    const e = completeSet(bench(), 1, 42);
    expect(e.sets[1]).toMatchObject({ completed: true, reps: 5, workTime: 42 });
    expect(completeSet(bench(), 1).sets[1].workTime).toBeUndefined();
  });

  it("shows the plan until the set is edited or done", () => {
    const e = bench();
    expect(shown(e.sets[1])).toEqual({ reps: 5, weight: { value: 80, unit: "kg" } });
    const edited = setValues(e, 1, { weight: { value: 70, unit: "kg" } });
    expect(edited.sets[1]).toMatchObject({ edited: true, completed: false, reps: 5 });
    expect(shown(edited.sets[1])).toEqual({ reps: 5, weight: { value: 70, unit: "kg" } });
    expect(tapDone(edited, 1).sets[1]).toMatchObject({
      completed: true,
      reps: 5,
      weight: { value: 70 },
    });
    expect(setValues(tapDone(e, 1), 1, { reps: 3 }).sets[1]).toMatchObject({
      completed: true,
      edited: true,
      reps: 3,
    });
  });

  it("logs both sides of a unilateral set, the left one following the plan until edited", () => {
    const e = { ...bench(), unilateral: true };
    expect(shown(e.sets[1], true)).toEqual({
      reps: 5,
      repsLeft: 5,
      weight: { value: 80, unit: "kg" },
    });
    expect(completeSet(e, 1).sets[1]).toMatchObject({ completed: true, reps: 5, repsLeft: 5 });
    expect(completeSet(bench(), 1).sets[1].repsLeft).toBeUndefined();
    const edited = setValues(e, 1, { repsLeft: 4 });
    expect(edited.sets[1]).toMatchObject({ edited: true, reps: 5, repsLeft: 4 });
    expect(setValues(edited, 1, { reps: 6 }).sets[1]).toMatchObject({ reps: 6, repsLeft: 4 });
    expect(completeSet(edited, 1).sets[1]).toMatchObject({ completed: true, reps: 5, repsLeft: 4 });
    expect(addSet(edited).sets[3]).toMatchObject({ reps: 5, repsLeft: 5 });
  });

  it("switches the alternate and keeps completed sets", () => {
    const e = switchAlternate(tapDone(bench(), 1), "Bench Press, Dumbbell");
    expect(e).toMatchObject({ name: "Bench Press, Dumbbell", current: 2, slug: "" });
    expect(e.sets[1].completed).toBe(true);
  });

  it("adds an edited pending set like the last one", () => {
    const added = addSet(setValues(bench(), 2, { reps: 6 }));
    expect(added.sets).toHaveLength(4);
    expect(added.sets[3]).toMatchObject({
      completed: false,
      edited: true,
      reps: 6,
      weight: { value: 80 },
      planned: { reps: 5, warmup: false },
    });
    expect(exerciseModified(added)).toBe(true);
    expect(exerciseModified(bench())).toBe(false);
  });

  it("removes a set or an exercise and marks the day modified", () => {
    const r = record();
    const less = removeSet(r, 0, 2);
    expect(less.entries[0].sets).toHaveLength(2);
    expect(less.modified).toBe(true);
    expect(removeExercise(r, 0)).toMatchObject({ entries: [], modified: true });
  });

  it("adds an ad hoc exercise with one editable set", () => {
    const r = addExercise(record(), {
      name: "Hammer Curl",
      slug: "hammer-curl",
      equipment: "dumbbell",
    });
    expect(r.entries[1]).toMatchObject({
      name: "Hammer Curl",
      slug: "hammer-curl",
      fromProgram: false,
      alternates: ["Hammer Curl"],
      current: 1,
    });
    expect(r.entries[1].sets).toHaveLength(1);
    expect(r.entries[1].sets[0]).toMatchObject({ edited: true, completed: false });
    expect(exerciseModified(r.entries[1])).toBe(true);
    expect(markModified(r).modified).toBe(true);
  });

  it("derives modified from edited sets and ad hoc exercises, never clearing it", () => {
    expect(markModified(record()).modified).toBeFalsy();
    const edited = { ...record(), entries: [setValues(bench(), 1, { reps: 3 })] };
    expect(markModified(edited).modified).toBe(true);
    expect(markModified({ ...record(), modified: true }).modified).toBe(true);
  });

  it("points at the first pending set, warmups included", () => {
    const r = record();
    expect(nextPending(r)).toEqual({ exercise: 0, set: 0 });
    expect(nextPending({ entries: [tapDone(bench(), 0)] })).toEqual({ exercise: 0, set: 1 });
    const done = { ...r, entries: [tapDone(tapDone(tapDone(bench(), 0), 1), 2)] };
    expect(nextPending(done)).toBeUndefined();
  });

  it("finishes a record and asks the server to finalize it", () => {
    expect(finish(record(), "2026-11-11T08:00:00.000Z")).toMatchObject({
      finishedAt: "2026-11-11T08:00:00.000Z",
      finalizeRequested: true,
    });
  });
});
