import type { ExerciseEntry, SetEntry, Weight, WorkoutRecord } from "../../domain/types";

type Values = Partial<Pick<SetEntry, "reps" | "repsLeft" | "weight" | "rpe">>;
type Shown = Pick<SetEntry, "reps" | "repsLeft" | "weight">;
type Entries = Pick<WorkoutRecord, "entries" | "modified">;

const mapSets = (e: ExerciseEntry, i: number, fn: (s: SetEntry) => SetEntry) => ({
  ...e,
  sets: e.sets.map((s, j) => (j === i ? fn(s) : s)),
});

// What a set shows and completes with: the user's values once edited or done, else the plan. A
// unilateral set carries the left side too; left out, it follows the plan like the right side.
export function shown(s: SetEntry, unilateral = false): Shown {
  const fromPlan = !s.edited && !s.completed;
  const values = fromPlan
    ? { reps: s.planned.reps, weight: s.planned.weight }
    : { reps: s.reps, weight: s.weight };
  if (!unilateral) return values;
  return { ...values, repsLeft: fromPlan ? s.planned.reps : (s.repsLeft ?? s.planned.reps) };
}

const done = (s: SetEntry, unilateral?: boolean, workTime?: number): SetEntry => ({
  ...s,
  ...shown(s, unilateral),
  completed: true,
  workTime: workTime ?? s.workTime,
});
const pending = (s: SetEntry): SetEntry => ({ ...s, completed: false, reps: 0 });

// Completes with the shown values; workTime is the seconds a timed set ran.
export const completeSet = (e: ExerciseEntry, i: number, workTime?: number) =>
  mapSets(e, i, (s) => done(s, e.unilateral, workTime));

// The tick toggles: a pending set completes with its shown values, a done set goes back to pending.
// Reps are changed through the editor, not by tapping (P2-R45).
export const tapDone = (e: ExerciseEntry, i: number) =>
  mapSets(e, i, (s) => (!s.completed ? done(s, e.unilateral) : pending(s)));

export const setValues = (e: ExerciseEntry, i: number, values: Values) =>
  mapSets(e, i, (s) => ({ ...s, ...shown(s, e.unilateral), ...values, edited: true }));

// The slug belongs to the old name; the server fills it in for the new one.
export const switchAlternate = (e: ExerciseEntry, name: string): ExerciseEntry => ({
  ...e,
  name,
  slug: "",
  current: e.alternates.indexOf(name) + 1,
});

const emptyWeight: Weight = { value: 0, unit: "kg" };
const emptyPlan: SetEntry["planned"] = {
  reps: 0,
  minReps: 0,
  weight: emptyWeight,
  rpe: 0,
  rest: 0,
  work: 0,
  amrap: false,
  ask: false,
  auto: false,
  countUp: false,
  warmup: false,
  label: "",
  logRpe: false,
};

const newSet = (planned: SetEntry["planned"], values: Shown) => ({
  planned,
  completed: false,
  rpe: 0,
  edited: true,
  ...values,
});

export function addSet(e: ExerciseEntry): ExerciseEntry {
  const last = e.sets.at(-1);
  const set = last
    ? newSet({ ...last.planned, warmup: false }, shown(last, e.unilateral))
    : newSet(emptyPlan, { reps: 0, weight: emptyWeight });
  return { ...e, sets: [...e.sets, set] };
}

export const removeSet = <R extends Entries>(r: R, exercise: number, set: number): R => ({
  ...mapExercise(r, exercise, (e) => ({ ...e, sets: e.sets.filter((_, j) => j !== set) })),
  modified: true,
});

export const removeExercise = <R extends Entries>(r: R, i: number): R => ({
  ...r,
  entries: r.entries.filter((_, j) => j !== i),
  modified: true,
});

export type Picked = Pick<ExerciseEntry, "name" | "slug" | "equipment" | "unilateral">;

// Inserted at `at`, at the end by default.
export const addExercise = <R extends Entries>(r: R, picked: Picked, at = r.entries.length): R =>
  markModified({
    ...r,
    entries: [
      ...r.entries.slice(0, at),
      {
        ...picked,
        fromProgram: false,
        label: "",
        alternates: [picked.name],
        current: 1,
        desc: "",
        superset: "",
        variation: 0,
        promptValues: {},
        sets: [newSet(emptyPlan, { reps: 0, weight: emptyWeight })],
      },
      ...r.entries.slice(at),
    ],
  });

export const exerciseModified = (e: ExerciseEntry) =>
  !e.fromProgram || e.sets.some((s) => s.edited);

// Removed sets and exercises leave no trace in the entries, so modified only ever turns on; the
// server recomputes it against the plan.
export const markModified = <R extends Entries>(r: R): R => ({
  ...r,
  modified: r.modified || r.entries.some(exerciseModified),
});

export function nextPending(r: Pick<WorkoutRecord, "entries">) {
  for (const [exercise, e] of r.entries.entries()) {
    const set = e.sets.findIndex((s) => !s.completed);
    if (set >= 0) return { exercise, set };
  }
}

export const mapExercise = <R extends Pick<WorkoutRecord, "entries">>(
  r: R,
  i: number,
  fn: (e: ExerciseEntry) => ExerciseEntry,
): R => ({ ...r, entries: r.entries.map((e, j) => (j === i ? fn(e) : e)) });

// Pending sets stay not done; the server finalizes when the record is pushed.
export const finish = <R extends Pick<WorkoutRecord, "finishedAt" | "finalizeRequested">>(
  r: R,
  now: string,
): R => ({ ...r, finishedAt: now, finalizeRequested: true });
