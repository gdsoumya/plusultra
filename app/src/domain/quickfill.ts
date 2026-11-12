// Builds workout records and meal logs from resolved days: what a quick fill writes locally.
import type { LocalRow } from "../data/repositories/syncRepo";
import type { ExerciseEntry, ResolvedDay, WorkoutRecord } from "./types";

interface NewRow {
  id: string;
  date: string;
  userId: string;
}

// A workout's entries from its planned day; `done` says which exercises are completed as planned.
export const entries = (day: ResolvedDay, done: (exercise: number) => boolean): ExerciseEntry[] =>
  day.exercises.map(({ sets: planned, ...e }, i) => ({
    ...e,
    fromProgram: true,
    variation: 0,
    promptValues: {},
    sets: planned.map((s) => ({
      planned: s,
      completed: done(i),
      reps: done(i) ? s.reps : 0,
      weight: s.weight,
      rpe: 0,
    })),
  }));

// Completes every set not yet done, as planned.
export const fill = (all: ExerciseEntry[]): ExerciseEntry[] =>
  all.map((e) => ({
    ...e,
    sets: e.sets.map((s) =>
      s.completed ? s : { ...s, completed: true, reps: s.planned.reps, weight: s.planned.weight },
    ),
  }));

export const workoutFromDay = (
  day: ResolvedDay,
  programId: string,
  row: NewRow,
  startedAt: string,
): LocalRow<WorkoutRecord> => ({
  ...row,
  version: 0,
  programId,
  week: day.week,
  day: day.day,
  dayName: day.name,
  dayDesc: day.desc,
  kind: day.kind,
  entries: entries(day, () => false),
  startedAt,
  finalized: false,
  finalizeRequested: false,
});

// An ad hoc workout has no program; the user builds it on the workout screen.
export const adHocWorkout = (row: NewRow, startedAt: string): LocalRow<WorkoutRecord> => ({
  ...row,
  version: 0,
  week: 0,
  day: 0,
  dayName: "Workout",
  dayDesc: "",
  entries: [],
  startedAt,
  finalized: false,
  finalizeRequested: false,
});

// "Done as planned": the server finalizes the record when it is pushed.
export const completeAsPlanned = <R extends Pick<WorkoutRecord, "entries">>(
  record: R,
  finishedAt: string,
): R & Pick<WorkoutRecord, "finishedAt" | "finalizeRequested"> => ({
  ...record,
  entries: fill(record.entries),
  finishedAt,
  finalizeRequested: true,
});
