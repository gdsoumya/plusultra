import type { Rows, SyncTable } from "../data/store";
import { dayTotals } from "./totals";
import type { Item, Meal, MealLog, WorkoutRecord } from "./types";

// The row to push again after a conflict, or undefined when the server row stands. The last
// write wins and a server delete always stands, except that a device never loses its own set
// completions or finalize request to a newer server record.
export function merge<T extends SyncTable>(
  table: T,
  local: Rows[T],
  server: Rows[T],
): Rows[T] | undefined {
  if (server.deletedAt) return undefined;
  // Another device's meal log or planned workout of the date: one row with what each logged,
  // whichever is newer (P2-R41).
  if (local.id !== server.id && table === "mealLogs") {
    return combineMealLogs(local as MealLog, server as MealLog) as Rows[T] | undefined;
  }
  const another = local.id !== server.id && table === "workoutRecords";
  if (local.updatedAt > server.updatedAt && !another) return local;
  if (table !== "workoutRecords" || local.deletedAt) return undefined;
  return keepCompletions(local as WorkoutRecord, server as WorkoutRecord) as Rows[T] | undefined;
}

const same = (a: unknown, b: unknown) => JSON.stringify(a) === JSON.stringify(b);

function keepCompletions(local: WorkoutRecord, server: WorkoutRecord) {
  let kept = false;
  const entries = server.entries.map((entry, i) => {
    const mine = local.entries[i]?.sets ?? [];
    const sets = entry.sets.map((set, j) => (mine[j]?.completed ? mine[j] : set));
    const extra = mine.slice(entry.sets.length).filter((set) => set.completed);
    kept ||= extra.length > 0 || !same(sets, entry.sets);
    return { ...entry, sets: [...sets, ...extra] };
  });
  const finalizeRequested = local.finalizeRequested && !server.finalized;
  return kept || finalizeRequested ? { ...server, entries, finalizeRequested } : undefined;
}

// Neither log saw the other, so the server's gets what only the local one has: its meals and items
// paired by name, and its eaten items where the server's are not eaten.
function combineMealLogs(local: MealLog, server: MealLog) {
  const meals = pair(server.meals, local.meals, (m) => m.name, combineMeal);
  const target = server.target?.kcal ? server.target : (local.target ?? server.target);
  const programId = server.programId ?? local.programId;
  if (same([meals, target, programId], [server.meals, server.target, server.programId])) return;
  return { ...server, meals, totals: dayTotals(meals), target, programId };
}

const combineMeal = (server: Meal, local: Meal): Meal => ({
  ...server,
  items: pair(server.items, local.items, (i) => i.foodName, eatenOf),
});

const eatenOf = (server: Item, local: Item) => (server.qty === 0 && local.qty > 0 ? local : server);

// Each of theirs with the first unpaired one of mine of the same key, then mine left unpaired.
function pair<T>(theirs: T[], mine: T[], key: (x: T) => string, both: (t: T, m: T) => T) {
  const left = [...mine];
  const paired = theirs.map((t) => {
    const i = left.findIndex((m) => key(m) === key(t));
    return i < 0 ? t : both(t, left.splice(i, 1)[0]);
  });
  return [...paired, ...left];
}
