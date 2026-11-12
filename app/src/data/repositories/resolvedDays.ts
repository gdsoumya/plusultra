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
    // The server sends its whole window from today on, so days it no longer plans are dropped;
    // dated days before the window stay, since past dates on the calendar still show their plan.
    replaceFromServer: (rows: (Omit<Rows[T], "id"> & DayPosition)[]) =>
      store.transaction(() => {
        const next = rows.map(withId);
        const keep = new Set(next.map((row) => row.id));
        const from = next.reduce<string | undefined>(
          (min, { date }) => (date && (!min || date < min) ? date : min),
          undefined,
        );
        for (const { id, date } of store.list(table)) {
          if (!keep.has(id) && !(from && date && date < from)) store.delete(table, id);
        }
        next.forEach((row) => store.put(table, row));
      }),
  };
}

export const resolvedDaysRepo = (store: Store) => dayCacheRepo(store, "resolvedDays");
export const mealDaysRepo = (store: Store) => dayCacheRepo(store, "mealDays");
