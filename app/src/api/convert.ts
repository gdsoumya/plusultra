// Maps the server's messages to the local store's rows and back, where the shapes differ: synced
// rows and resolved days. Store rows use ISO strings for timestamps, strings for enums and unset
// fields for proto's "" and 0; nothing is lost on the way back.
import { create } from "@bufbuild/protobuf";
import { timestampDate, timestampFromDate, type Timestamp } from "@bufbuild/protobuf/wkt";

import type { ServerRow } from "../data/repositories/syncRepo";
import type { MealDayCache, ResolvedDayCache, Rows, SyncTable } from "../data/store";
import type * as D from "../domain/types";
import {
  DayLogSchema,
  DayStatus,
  type DayLog,
  type Readiness,
} from "../gen/plusultra/v1/calendar_pb";
import { CheckinSchema, type Checkin } from "../gen/plusultra/v1/checkin_pb";
import {
  ResolvedDaySchema,
  ResolvedSetSchema,
  Unit,
  type Macros,
  type ResolvedDay,
  type ResolvedExercise,
  type ResolvedSet,
  type Weight,
} from "../gen/plusultra/v1/common_pb";
import {
  MealLogSchema,
  ResolvedMealDaySchema,
  type Meal,
  type MealItem,
  type MealLog,
  type ResolvedMealDay,
} from "../gen/plusultra/v1/meal_pb";
import {
  ProgramKind,
  ProgramMode,
  ProgramStatus,
  type Program,
} from "../gen/plusultra/v1/program_pb";
import { SyncRowsSchema, type SyncRows } from "../gen/plusultra/v1/sync_pb";
import {
  WorkoutRecordSchema,
  type ExerciseEntry,
  type SetEntry,
  type WorkoutRecord,
} from "../gen/plusultra/v1/workout_pb";

// Rows of each synced table as the server sends them, in the store's shape.
export type ServerRows = { [T in SyncTable]: ServerRow<Rows[T]>[] };

// Sub-millisecond precision is dropped; nothing the app sends back depends on it.
const iso = (t?: Timestamp) => (t ? timestampDate(t).toISOString() : undefined);
const stamp = (s?: string) => (s ? timestampFromDate(new Date(s)) : undefined);

const plain = <T extends { $typeName: string }>({ $typeName: _, ...rest }: T) => rest;

const dayStatuses = {
  planned: DayStatus.PLANNED,
  done: DayStatus.DONE,
  partial: DayStatus.PARTIAL,
  skipped: DayStatus.SKIPPED,
  rest: DayStatus.REST,
} satisfies Record<D.DayStatus, DayStatus>;
const dayStatusOf = (s: DayStatus) =>
  (Object.keys(dayStatuses) as D.DayStatus[]).find((k) => dayStatuses[k] === s) ?? "planned";

const programStatuses = {
  draft: ProgramStatus.DRAFT,
  active: ProgramStatus.ACTIVE,
  finished: ProgramStatus.FINISHED,
  archived: ProgramStatus.ARCHIVED,
} satisfies Record<D.ProgramSummary["status"], ProgramStatus>;

const weight = (w?: Weight): D.Weight => ({
  value: w?.value ?? 0,
  unit: w?.unit === Unit.LB ? "lb" : "kg",
});
const weightOut = (w: D.Weight) => ({ value: w.value, unit: w.unit === "lb" ? Unit.LB : Unit.KG });

const macros = (m?: Macros): D.Macros =>
  m ? plain(m) : { kcal: 0, protein: 0, carbs: 0, fat: 0, fiber: 0, waterMl: 0 };

const resolvedSet = ({ $typeName: _, weight: w, ...s }: ResolvedSet): D.ResolvedSet => ({
  ...s,
  weight: weight(w),
});
const resolvedSetOut = (s: D.ResolvedSet) => ({ ...s, weight: weightOut(s.weight) });

const exercise = ({ $typeName: _, sets, ...e }: ResolvedExercise): D.ResolvedExercise => ({
  ...e,
  unilateral: e.unilateral || undefined,
  sets: sets.map(resolvedSet),
});
const exerciseOut = (e: D.ResolvedExercise) => ({ ...e, sets: e.sets.map(resolvedSetOut) });

function resolvedDay(d: ResolvedDay): Omit<ResolvedDayCache, "id"> {
  const date = d.date || undefined;
  const { programId, week, day, name, desc } = d;
  const kind = d.kind as D.DayKind;
  return {
    programId,
    week,
    day,
    date,
    payload: { week, day, date, name, desc, kind, exercises: d.exercises.map(exercise) },
  };
}
const resolvedDayOut = ({ date, exercises, ...d }: D.ResolvedDay, programId: string) =>
  create(ResolvedDaySchema, {
    ...d,
    programId,
    date: date ?? "",
    exercises: exercises.map(exerciseOut),
  });

const item = ({ food, qty, unit, note, macros: m, planned, edited }: MealItem): D.Item => ({
  foodName: food?.name ?? "",
  foodRef: food?.ref || undefined,
  per100g: food?.per100g && macros(food.per100g),
  perUnit: food?.perUnit && macros(food.perUnit),
  foodUnit: food?.unit || undefined,
  qty,
  unit,
  note: note || undefined,
  macros: macros(m),
  planned,
  edited: edited || undefined,
});
const itemOut = (i: D.Item) => ({
  food: {
    name: i.foodName,
    ref: i.foodRef ?? "",
    per100g: i.per100g,
    perUnit: i.perUnit,
    unit: i.foodUnit ?? "",
  },
  qty: i.qty,
  unit: i.unit,
  note: i.note ?? "",
  macros: i.macros,
  planned: i.planned,
  edited: !!i.edited,
});

const meal = ({ name, time, desc, items }: Meal): D.Meal => ({
  name,
  time: time || undefined,
  desc: desc || undefined,
  items: items.map(item),
});
const mealOut = (m: D.Meal) => ({
  name: m.name,
  time: m.time ?? "",
  desc: m.desc ?? "",
  items: m.items.map(itemOut),
});

const total = (meals: D.Meal[]) => {
  const items = meals.flatMap((m) => m.items.map((i) => i.macros));
  const sum = (k: keyof D.Macros) => items.reduce((t, m) => t + m[k], 0);
  return {
    kcal: sum("kcal"),
    protein: sum("protein"),
    carbs: sum("carbs"),
    fat: sum("fat"),
    fiber: sum("fiber"),
    waterMl: sum("waterMl"),
  };
};

function mealDay(d: ResolvedMealDay): Omit<MealDayCache, "id"> {
  const { programId, week, day, name, desc } = d;
  const payload: D.MealDay = {
    week,
    day,
    kind: d.kind as D.DayKind,
    name,
    desc,
    target: d.target && macros(d.target),
    meals: d.meals.map(meal),
  };
  return { programId, week, day, date: d.date || undefined, payload };
}
const mealDayOut = (d: D.MealDay, programId: string, date?: string) =>
  create(ResolvedMealDaySchema, {
    ...d,
    programId,
    date: date ?? "",
    meals: d.meals.map(mealOut),
    totals: total(d.meals),
  });

type Meta = {
  id: string;
  version: number;
  deviceId: string;
  updatedAt?: Timestamp;
  deletedAt?: Timestamp;
};
const meta = (r: Meta, userId: string) => ({
  id: r.id,
  userId,
  version: r.version,
  deviceId: r.deviceId,
  updatedAt: iso(r.updatedAt) ?? "",
  deletedAt: iso(r.deletedAt),
});
const metaOut = (r: ServerRow<D.SyncMeta>) => ({
  id: r.id,
  version: r.version,
  deviceId: r.deviceId,
  updatedAt: stamp(r.updatedAt),
  deletedAt: stamp(r.deletedAt),
});

const dayLog = (l: DayLog, userId: string): ServerRow<D.DayLog> => ({
  ...meta(l, userId),
  date: l.date,
  status: dayStatusOf(l.status),
  notes: l.notes,
  readiness: l.readiness && plain(l.readiness),
});
const dayLogOut = (l: ServerRow<D.DayLog>) =>
  create(DayLogSchema, {
    ...metaOut(l),
    date: l.date,
    status: dayStatuses[l.status],
    notes: l.notes,
    readiness: l.readiness,
  });

// Deviation fields the generated client gains with the server's editable-days change: read when
// the server sends them; outgoing they ride along in the spreads and create() keeps them once the
// schema knows them.
type Deviation = { edited?: boolean; fromProgram?: boolean; modified?: boolean };
const deviation = (m: object): Deviation => m as Deviation;

const setEntry = (s: SetEntry): D.SetEntry => ({
  planned: resolvedSet(s.planned ?? create(ResolvedSetSchema)),
  completed: s.completed,
  reps: s.reps,
  weight: weight(s.weight),
  rpe: s.rpe,
  workTime: s.workTime || undefined,
  repsLeft: s.repsLeft || undefined,
  edited: deviation(s).edited || undefined,
});
const setEntryOut = (s: D.SetEntry) => ({
  ...s,
  planned: resolvedSetOut(s.planned),
  weight: weightOut(s.weight),
  workTime: s.workTime ?? 0,
  repsLeft: s.repsLeft ?? 0,
});

const entry = ({ $typeName: _, sets, promptValues, ...e }: ExerciseEntry): D.ExerciseEntry => ({
  ...e,
  fromProgram: deviation(e).fromProgram ?? true,
  unilateral: e.unilateral || undefined,
  promptValues: { ...promptValues },
  sets: sets.map(setEntry),
});
const entryOut = (e: D.ExerciseEntry) => ({ ...e, sets: e.sets.map(setEntryOut) });

const workoutRecord = (r: WorkoutRecord, userId: string): ServerRow<D.WorkoutRecord> => ({
  ...meta(r, userId),
  date: r.date,
  programId: r.programId || undefined,
  week: r.week,
  day: r.day,
  dayName: r.dayName,
  dayDesc: r.dayDesc,
  kind: (r.kind || undefined) as D.DayKind | undefined,
  entries: r.entries.map(entry),
  startedAt: iso(r.startedAt),
  finishedAt: iso(r.finishedAt),
  finalized: r.finalized,
  finalizeRequested: false,
  modified: deviation(r).modified || undefined,
});
// A record finalized offline goes out with finalized set, which asks the server to finalize it.
const workoutRecordOut = (r: ServerRow<D.WorkoutRecord>) =>
  create(WorkoutRecordSchema, {
    ...metaOut(r),
    date: r.date,
    programId: r.programId ?? "",
    week: r.week,
    day: r.day,
    dayName: r.dayName,
    dayDesc: r.dayDesc,
    kind: r.kind ?? "",
    entries: r.entries.map(entryOut),
    startedAt: stamp(r.startedAt),
    finishedAt: stamp(r.finishedAt),
    finalized: r.finalized || r.finalizeRequested,
  });

// The server stores no target for an ad hoc log and sends it as zeros.
const target = (m?: Macros) => (m && Object.values(plain(m)).some(Boolean) ? plain(m) : undefined);

const mealLog = (l: MealLog, userId: string): ServerRow<D.MealLog> => ({
  ...meta(l, userId),
  date: l.date,
  programId: l.programId || undefined,
  target: target(l.target),
  meals: l.meals.map(meal),
  totals: macros(l.totals),
});
const mealLogOut = (l: ServerRow<D.MealLog>) =>
  create(MealLogSchema, {
    ...metaOut(l),
    date: l.date,
    programId: l.programId ?? "",
    target: l.target,
    meals: l.meals.map(mealOut),
    totals: l.totals,
  });

// Photos never travel with synced check-ins, only their photoId.
const checkin = (c: Checkin, userId: string): ServerRow<D.Checkin> => ({
  ...meta(c, userId),
  date: c.date,
  weight: c.weight && weight(c.weight),
  bodyFat: c.bodyFat,
  measurements: { ...c.measurements },
  photoId: c.photoId || undefined,
});
const checkinOut = (c: ServerRow<D.Checkin>) =>
  create(CheckinSchema, {
    ...metaOut(c),
    date: c.date,
    weight: c.weight && weightOut(c.weight),
    bodyFat: c.bodyFat,
    measurements: c.measurements,
    photoId: c.photoId ?? "",
  });

const syncRows = (rows: SyncRows | undefined, userId: string): ServerRows => ({
  dayLogs: (rows?.dayLogs ?? []).map((r) => dayLog(r, userId)),
  workoutRecords: (rows?.workoutRecords ?? []).map((r) => workoutRecord(r, userId)),
  mealLogs: (rows?.mealLogs ?? []).map((r) => mealLog(r, userId)),
  checkins: (rows?.checkins ?? []).map((r) => checkin(r, userId)),
});
const syncRowsOut = (rows: Partial<ServerRows>) =>
  create(SyncRowsSchema, {
    dayLogs: rows.dayLogs?.map(dayLogOut),
    workoutRecords: rows.workoutRecords?.map(workoutRecordOut),
    mealLogs: rows.mealLogs?.map(mealLogOut),
    checkins: rows.checkins?.map(checkinOut),
  });

const program = (p: Program): D.ProgramSummary => ({
  id: p.id,
  kind: p.kind === ProgramKind.MEAL ? "meal" : "workout",
  name: p.name,
  revision: p.revision,
  status:
    (Object.keys(programStatuses) as D.ProgramSummary["status"][]).find(
      (k) => programStatuses[k] === p.status,
    ) ?? "draft",
  startDate: p.startDate || undefined,
  endDate: p.endDate || undefined,
  mode:
    p.mode === ProgramMode.ANCHORED
      ? "anchored"
      : p.mode === ProgramMode.FLOATING
        ? "floating"
        : undefined,
  nextWeek: p.cursor?.nextWeek,
  nextDay: p.cursor?.nextDay,
  weeks: p.structure?.weeks ?? 0,
  updatedAt: iso(p.updatedAt) ?? "",
});

export const fromProto = {
  syncRows,
  weight,
  dayStatus: dayStatusOf,
  readiness: (r: Readiness): D.Readiness => plain(r),
  workoutRecord,
  entry,
  mealLog,
  checkin,
  resolvedDay,
  mealDay,
  program,
};

export const toProto = {
  syncRows: syncRowsOut,
  dayLog: dayLogOut,
  workoutRecord: workoutRecordOut,
  mealLog: mealLogOut,
  checkin: checkinOut,
  resolvedDay: resolvedDayOut,
  mealDay: mealDayOut,
  weight: weightOut,
  dayStatus: (s: D.DayStatus) => dayStatuses[s],
};
