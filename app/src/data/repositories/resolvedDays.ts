import { addDays } from "../../domain/dates";
import type { Rows, Store } from "../store";

type DayTable = "resolvedDays" | "mealDays";
type DayPosition = Pick<Rows[DayTable], "programId" | "week" | "day" | "date" | "payload">;

const dayKey = (programId: string, week: number, day: number) => `${programId}:${week}:${day}`;

// Meal days picked by kind share week and day 0, so a dated one is keyed by date and a floating
// one by kind.
function cacheId(table: DayTable, { programId, week, day, date, payload }: DayPosition) {
  if (table !== "mealDays") return dayKey(programId, week, day);
  return date ? `${programId}:${date}` : `${dayKey(programId, week, day)}:${payload.kind}`;
}

function dayCacheRepo<T extends DayTable>(store: Store, table: T) {
  const withId = (row: Omit<Rows[T], "id"> & DayPosition) =>
    ({ ...row, id: cacheId(table, row) }) as Rows[T];
  return {
    at: (programId: string, week: number, day: number) =>
      store.get(table, dayKey(programId, week, day)),
    floating: (programId: string) => store.list(table, { programId }).filter((r) => !r.date),
    byDate: (date: string) => store.list(table, { dateFrom: date, dateTo: date }),
    range: (from: string, to: string) => store.list(table, { dateFrom: from, dateTo: to }),
    upsertFromServer: (rows: (Omit<Rows[T], "id"> & DayPosition)[]) =>
      store.transaction(() => rows.forEach((row) => store.put(table, withId(row)))),
    // The server sends its whole window from today on, so days in it that it no longer plans are
    // dropped; dated days outside it stay, fetched for the calendar's past and later months.
    replaceFromServer: (rows: (Omit<Rows[T], "id"> & DayPosition)[]) =>
      store.transaction(() => {
        const next = rows.map(withId);
        const keep = new Set(next.map((row) => row.id));
        const from = next.flatMap(({ date }) => (date ? [date] : [])).sort()[0];
        // shortcut: mirrors the server's 28-day pull window, counted from the first day it sent.
        const to = from && addDays(from, 27);
        for (const { id, date } of store.list(table)) {
          const outside = from && date && (date < from || date > to);
          if (!keep.has(id) && !outside) store.delete(table, id);
        }
        next.forEach((row) => store.put(table, row));
      }),
    dropProgram: (programId: string) =>
      store.transaction(() =>
        store.list(table, { programId }).forEach(({ id }) => store.delete(table, id)),
      ),
  };
}

export const resolvedDaysRepo = (store: Store) => dayCacheRepo(store, "resolvedDays");
export const mealDaysRepo = (store: Store) => dayCacheRepo(store, "mealDays");
