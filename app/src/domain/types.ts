// Dates are local calendar dates (YYYY-MM-DD); timestamps are ISO 8601.

export type Unit = "kg" | "lb";
export type DayKind = "training" | "conditioning" | "rest" | "note";
export type DayStatus = "planned" | "done" | "partial" | "skipped" | "rest";

export interface Weight {
  value: number;
  unit: Unit;
}

export interface Macros {
  kcal: number;
  protein: number;
  carbs: number;
  fat: number;
  fiber: number;
  waterMl: number;
}

export interface ResolvedSet {
  reps: number;
  minReps: number;
  weight: Weight;
  rpe: number;
  rest: number;
  work: number;
  amrap: boolean;
  ask: boolean;
  auto: boolean;
  countUp: boolean;
  warmup: boolean;
  label: string;
  logRpe: boolean;
}

// name is alternates[current - 1]; slug is empty when name is not in the catalog; sets list warmups first.
// unilateral comes from the user's exercise data: the sets are then logged per side.
// progression is the server's one-line summary of how the exercise progresses; the proto conversion
// spreads it through once the generated ResolvedExercise carries the field (server Task 24).
export interface ResolvedExercise {
  name: string;
  slug: string;
  equipment: string;
  label: string;
  alternates: string[];
  current: number;
  desc: string;
  superset: string;
  unilateral?: boolean;
  sets: ResolvedSet[];
  progression?: string;
}

// date is set only for anchored programs.
export interface ResolvedDay {
  week: number;
  day: number;
  date?: string;
  name: string;
  desc: string;
  kind: DayKind;
  exercises: ResolvedExercise[];
}

export interface ProgramSummary {
  id: string;
  kind: "workout" | "meal";
  name: string;
  revision: number;
  status: "draft" | "active" | "finished" | "archived";
  startDate?: string;
  endDate?: string;
  // Unset until the program is activated.
  mode?: "anchored" | "floating";
  nextWeek?: number;
  nextDay?: number;
  weeks: number;
  updatedAt: string;
}

// The food's macros are frozen when it is planned or logged: per100g, and perUnit for foods counted
// in foodUnit (for example "egg").
export interface Item {
  foodName: string;
  foodRef?: string;
  per100g?: Macros;
  perUnit?: Macros;
  foodUnit?: string;
  qty: number;
  unit: string;
  note?: string;
  macros: Macros;
  planned: boolean;
  // The user overrode the food's macros for this item (P2-R44).
  edited?: boolean;
}

export interface Meal {
  name: string;
  time?: string;
  desc?: string;
  items: Item[];
}

export interface MealDay {
  week: number;
  day: number;
  kind: DayKind;
  name: string;
  desc: string;
  target?: Macros;
  meals: Meal[];
}

// version is server assigned. dirty is local only: set by local writes, cleared after a push.
export interface SyncMeta {
  id: string;
  userId: string;
  updatedAt: string;
  version: number;
  deviceId: string;
  deletedAt?: string;
  dirty: boolean;
}

// Each from 1 (bad) to 5 (good); unset when not given.
export interface Readiness {
  sleep?: number;
  soreness?: number;
  stress?: number;
}

export interface DayLog extends SyncMeta {
  date: string;
  status: DayStatus;
  notes: string;
  readiness?: Readiness;
}

// For a unilateral exercise reps is the right side and repsLeft the left.
export interface SetEntry {
  planned: ResolvedSet;
  completed: boolean;
  reps: number;
  weight: Weight;
  rpe: number;
  workTime?: number;
  repsLeft?: number;
  // The user changed or added this set; hooks leave it alone.
  edited?: boolean;
}

// variation is the 1-based set variation the sets come from, 0 without variations; promptValues
// answer the progression's prompted state by name. fromProgram is false for an exercise the user
// added to the day. unilateral is a snapshot of the user's exercise data when the entry was made.
export interface ExerciseEntry {
  fromProgram: boolean;
  name: string;
  slug: string;
  equipment: string;
  label: string;
  alternates: string[];
  current: number;
  desc: string;
  superset: string;
  unilateral?: boolean;
  variation: number;
  sets: SetEntry[];
  promptValues: Record<string, number>;
}

// programId is unset and week and day are 0 for an ad hoc workout. modified says the day deviates
// from the program (edited or added sets, removed sets, added or removed exercises).
export interface WorkoutRecord extends SyncMeta {
  modified?: boolean;
  date: string;
  programId?: string;
  week: number;
  day: number;
  dayName: string;
  dayDesc: string;
  kind?: DayKind;
  entries: ExerciseEntry[];
  startedAt?: string;
  finishedAt?: string;
  finalized: boolean;
  finalizeRequested: boolean;
}

export interface MealLog extends SyncMeta {
  date: string;
  programId?: string;
  target?: Macros;
  meals: Meal[];
  totals: Macros;
}

export interface Checkin extends SyncMeta {
  date: string;
  weight?: Weight;
  bodyFat?: number;
  measurements: Record<string, number>;
  photoId?: string;
}
