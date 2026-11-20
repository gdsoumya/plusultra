import type { Api } from "../api/clients";
import { fromProto } from "../api/convert";
import { addDays, daysBetween } from "../domain/dates";
import { withKnownFlags } from "./exerciseData";
import type { ProgramSummary, ResolvedDay } from "../domain/types";
import { programsRepo } from "./repositories/programs";
import { mealDaysRepo, resolvedDaysRepo } from "./repositories/resolvedDays";
import type { MealDayCache, Store } from "./store";

// What the active programs plan, read from the cached days. A finished program's days may still
// be cached until the next pull, so only days of active programs count.
const activePrograms = (store: Store) =>
  programsRepo(store)
    .all()
    .filter((p) => p.status === "active");

export const activeProgram = (store: Store, kind: ProgramSummary["kind"]) =>
  activePrograms(store).find((p) => p.kind === kind);

const activeIds = (store: Store) => new Set(activePrograms(store).map((p) => p.id));

// Only anchored programs have dated days.
const plannedDayRows = (store: Store, from: string, to: string) => {
  const active = activeIds(store);
  return resolvedDaysRepo(store)
    .range(from, to)
    .filter((r) => active.has(r.programId));
};

export const plannedDays = (store: Store, from: string, to: string) =>
  plannedDayRows(store, from, to).map((r) => r.payload);

export const plannedDay = (store: Store, date: string) => plannedDayRows(store, date, date)[0];

// The workout a date has: an anchored program's day for it, or a floating program's next day,
// which only makes sense for today.
export function plannedWorkout(store: Store, date: string) {
  const program = activeProgram(store, "workout");
  if (!program) return;
  const days = resolvedDaysRepo(store);
  const day =
    program.mode === "floating"
      ? days.at(program.id, program.nextWeek ?? 0, program.nextDay ?? 0)
      : days.byDate(date).find((d) => d.programId === program.id);
  return day && { program, day: day.payload };
}

// The meal day a date has: an anchored meal program's day for it, or a floating program's day
// for the workout planned that day, chosen as the server does.
export function plannedMealDay(store: Store, date: string) {
  const program = activeProgram(store, "meal");
  if (!program) return;
  const days = mealDaysRepo(store);
  if (program.mode !== "floating") {
    return days.byDate(date).find((r) => r.programId === program.id);
  }
  return floatingMealDay(days.floating(program.id), plannedWorkout(store, date)?.day);
}

const isWorkout = (kind?: string) => kind === "training" || kind === "conditioning";

// The workout's numbered day in its week when the plan has one, else the day for its kind in
// the latest week at or before it; a conditioning day without one eats like a training day.
function floatingMealDay(
  rows: MealDayCache[],
  workout?: Pick<ResolvedDay, "week" | "day" | "kind">,
) {
  const week = Math.max(workout?.week ?? 1, 1);
  const kind = workout && isWorkout(workout.kind) ? workout.kind : "rest";
  const numbered = workout && rows.find((r) => r.week === week && r.day === workout.day);
  if (numbered) return numbered;
  const byKind = (k: string) =>
    rows
      .filter((r) => r.day === 0 && r.payload.kind === k && r.week <= week)
      .sort((a, b) => b.week - a.week)[0];
  return byKind(kind) ?? (kind === "conditioning" ? byKind("training") : undefined);
}

// The pull caches only the next four weeks, so the other weeks of a date range are fetched for
// the anchored programs that cover dates in it without a cached day; offline the cache stands.
export async function fetchPlan(api: Api, store: Store, from: string, to = from) {
  for (const p of activePrograms(store)) {
    if (p.mode !== "anchored" || !p.startDate) continue;
    const planned = p.kind === "workout" ? plannedDay : plannedMealDay;
    const weeks = new Set<number>();
    for (let date = from < p.startDate ? p.startDate : from; date <= to; date = addDays(date, 1)) {
      const week = Math.floor(daysBetween(p.startDate, date) / 7) + 1;
      if (week > p.weeks) break;
      if (!planned(store, date)) weeks.add(week);
    }
    for (const week of weeks) {
      const { days, mealDays } = await api.programs.getResolvedDays({ programId: p.id, week });
      resolvedDaysRepo(store).upsertFromServer(
        days
          .map(fromProto.resolvedDay)
          .map((d) => ({ ...d, payload: withKnownFlags(store, d.payload) })),
      );
      mealDaysRepo(store).upsertFromServer(mealDays.map(fromProto.mealDay));
    }
  }
}
