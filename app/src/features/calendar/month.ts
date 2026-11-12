import type { Api } from "../../api/clients";
import { checkinsRepo } from "../../data/repositories/checkins";
import { dayLogsRepo } from "../../data/repositories/dayLogs";
import { workoutRecordsRepo } from "../../data/repositories/workoutRecords";
import type { Store } from "../../data/store";
import { monthGrid } from "../../domain/dates";
import type { DayStatus, ResolvedDay, WorkoutRecord } from "../../domain/types";
import { activeProgram, plannedDays } from "../../data/plans";
import { resolvedDaysRepo } from "../../data/repositories/resolvedDays";

export interface MonthDay {
  status?: DayStatus;
  checkin: boolean;
}

export interface MonthData {
  weeks: string[][];
  // Only dates with a marker.
  days: Record<string, MonthDay>;
  next?: { week: number; day: number; name: string };
}

const byDate = <R extends { date: string }>(rows: R[]) => new Map(rows.map((r) => [r.date, r]));

const kindsKey = (month: string) => `calendarKinds:${month}`;

// The pull only caches days from today on, so a shown month asks the server what it planned on
// its other dates; the answer is kept per month so the grid marks them offline too.
export async function fetchMonth(api: Api, store: Store, month: string) {
  const weeks = monthGrid(month);
  const { days } = await api.calendar.getCalendar({
    dates: { from: weeks[0][0], to: weeks[weeks.length - 1][6] },
  });
  const kinds: Record<string, string> = {};
  for (const d of days) if (d.workoutWeek) kinds[d.date] = d.kind;
  store.setKv(kindsKey(month), kinds);
}

const isWorkout = (kind?: string) => kind === "training" || kind === "conditioning";

// The log is the user's word on a day; a record, the cached plan or the server's calendar speaks
// when there is none.
const statusOf = (
  log?: { status: DayStatus },
  record?: WorkoutRecord,
  plan?: ResolvedDay,
  kind?: string,
): DayStatus | undefined =>
  log?.status ??
  (record && (record.finalized ? "done" : "partial")) ??
  (plan && (plan.exercises.length ? "planned" : "rest")) ??
  (isWorkout(kind) ? "planned" : undefined);

// The floating workout program's next day, which no date carries.
function floatingNext(store: Store) {
  const p = activeProgram(store, "workout");
  if (p?.mode !== "floating" || !p.nextWeek || !p.nextDay) return;
  const day = resolvedDaysRepo(store).at(p.id, p.nextWeek, p.nextDay)?.payload;
  return day && { week: day.week, day: day.day, name: day.name };
}

export function buildMonth(store: Store, month: string): MonthData {
  const weeks = monthGrid(month);
  const from = weeks[0][0];
  const to = weeks[weeks.length - 1][6];
  const logs = byDate(dayLogsRepo(store).range(from, to));
  const records = byDate(workoutRecordsRepo(store).range(from, to));
  const checkins = byDate(checkinsRepo(store).range(from, to));
  const plans = new Map(plannedDays(store, from, to).map((d) => [d.date, d]));
  const kinds = store.getKv<Record<string, string>>(kindsKey(month)) ?? {};
  const days: Record<string, MonthDay> = {};
  for (const date of weeks.flat()) {
    const status = statusOf(logs.get(date), records.get(date), plans.get(date), kinds[date]);
    const checkin = checkins.has(date);
    if (status || checkin) days[date] = { status, checkin };
  }
  return { weeks, days, next: floatingNext(store) };
}
