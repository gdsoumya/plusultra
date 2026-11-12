import { create } from "@bufbuild/protobuf";

import { entries } from "../../domain/quickfill";
import type { DayKind } from "../../domain/types";
import {
  ProgramCursorSchema,
  ProgramKind,
  ProgramMode,
  ProgramStatus,
  type Program,
} from "../../gen/plusultra/v1/program_pb";
import { daysBetween } from "../fixtures/dates";
import {
  now,
  nowStamp,
  onDate,
  type MockDb,
  type PlannedDay,
  type PlannedMealDay,
  type Row,
} from "./db";

// Resolved days carry the user's unilateral flag, so they are rebuilt when exercise data changes.
export function applyExerciseData(db: MockDb) {
  const unilateral = new Set(db.exerciseData.filter((d) => d.unilateral).map((d) => d.slug));
  for (const id of Object.keys(db.days)) {
    db.days[id] = db.days[id].map((d) => ({
      ...d,
      exercises: d.exercises.map((e) => ({
        ...e,
        unilateral: unilateral.has(e.slug) || undefined,
      })),
    }));
  }
}

export const activeProgram = (db: MockDb, kind: ProgramKind) =>
  db.programs.find((p) => p.kind === kind && p.status === ProgramStatus.ACTIVE);

export const dayAt = (db: MockDb, programId: string, week: number, day: number) =>
  db.days[programId]?.find((d) => d.week === week && d.day === day);

// The day a workout program has next: by date when anchored, at the cursor when floating.
export const nextDay = (db: MockDb, p: Program, date: string) =>
  p.mode === ProgramMode.ANCHORED
    ? db.days[p.id]?.find((d) => d.date === date)
    : p.cursor && dayAt(db, p.id, p.cursor.nextWeek, p.cursor.nextDay);

// Floating programs plan only today: future dates show plans for anchored programs only.
export function workoutPlan(db: MockDb, date: string): PlannedDay | undefined {
  const p = activeProgram(db, ProgramKind.WORKOUT);
  return p && (p.mode === ProgramMode.ANCHORED || date === db.today)
    ? nextDay(db, p, date)
    : undefined;
}

export const recordDay = (db: MockDb, r: Row<"workoutRecords">) =>
  r.programId ? dayAt(db, r.programId, r.week, r.day) : undefined;

export function plannedOrDone(db: MockDb, date: string) {
  const record = onDate(db, "workoutRecords", date);
  return workoutPlan(db, date) ?? (record && recordDay(db, record));
}

// shortcut: only anchored meal programs pick days, a floating one plans nothing; upgrade when a
// fixture or test needs a floating meal program. Conditioning, note and rest days eat as rest days.
export function mealPlan(
  db: MockDb,
  date: string,
  workoutKind: DayKind | undefined = plannedOrDone(db, date)?.kind,
): PlannedMealDay | undefined {
  const p = activeProgram(db, ProgramKind.MEAL);
  if (!p?.startDate || date < p.startDate) return;
  const week = Math.floor(daysBetween(p.startDate, date) / 7) + 1;
  const kind = workoutKind === "training" ? "training" : "rest";
  const day = db.mealDays[p.id]?.find((d) => d.week === week && d.kind === kind);
  return day && { ...day, date };
}

export const startRecord = (id: string, plan: PlannedDay, date: string) => ({
  id,
  date,
  programId: plan.programId,
  week: plan.week,
  day: plan.day,
  dayName: plan.name,
  dayDesc: plan.desc,
  kind: plan.kind,
  entries: entries(plan, () => false),
  startedAt: now(),
  finalized: false,
  finalizeRequested: false,
});

type Unsaved = Omit<Row<"workoutRecords">, "version" | "updatedAt" | "deviceId" | "userId">;

// Marks a record finalized and, for the active floating program, moves the cursor past its day.
// shortcut: no progression runs, so later days keep their weights; upgrade when a screen shows
// progressed weights after finishing a workout against the mock.
export function finalize<R extends Unsaved>(db: MockDb, record: R): R {
  const p = db.programs.find((x) => x.id === record.programId && x.status === ProgramStatus.ACTIVE);
  if (p?.mode === ProgramMode.FLOATING) {
    const days = db.days[p.id] ?? [];
    const i = days.findIndex((d) => d.week === record.week && d.day === record.day);
    const next = days[i + 1] ?? (p.structure?.loop ? days[0] : undefined);
    p.cursor = next && create(ProgramCursorSchema, { nextWeek: next.week, nextDay: next.day });
    if (!next) p.status = ProgramStatus.FINISHED;
    p.updatedAt = nowStamp();
  }
  return {
    ...record,
    finalized: true,
    finalizeRequested: false,
    finishedAt: record.finishedAt ?? now(),
  };
}
