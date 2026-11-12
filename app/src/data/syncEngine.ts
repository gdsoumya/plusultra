import type { Api } from "../api/clients";
import { fromProto, toProto } from "../api/convert";
import { merge } from "../domain/sync";
import type { ProgramSummary } from "../domain/types";
import { deviceId } from "./deviceId";
import { rememberUnilateral } from "./exerciseData";
import { programsRepo } from "./repositories/programs";
import { mealDaysRepo, resolvedDaysRepo } from "./repositories/resolvedDays";
import { syncRepo, type Conflict } from "./repositories/syncRepo";
import { syncTables, type Row, type Rows, type Store, type SyncTable } from "./store";

export const CURSOR_KEY = "syncCursor";
const BATCH = 500;

export interface SyncResult {
  pushed: number;
  pulled: number;
  conflicts: number;
  // Why the server did not apply a row, by row id: the server's push reasons.
  reasons: Record<string, string>;
}

export interface SyncOptions {
  userId?: string;
  // Rows per table and pull page, at most 500.
  limit?: number;
}

type Context = { api: Api; store: Store; userId: string };
type Dirty = { [T in SyncTable]: Rows[T][] };

// Pulls every page of server changes, pushes the dirty rows, merges what conflicts and pushes
// those once more, then pulls again so finalized records and re-resolved days land in the store.
export async function syncOnce(
  api: Api,
  store: Store,
  { userId = "", limit = BATCH }: SyncOptions = {},
): Promise<SyncResult> {
  const ctx = { api, store, userId };
  const result = await pullAll(ctx, limit);
  await listPrograms(api, store);
  const { pushed, conflicts, reasons } = await pushAll(ctx);
  result.pushed = pushed;
  result.conflicts += conflicts;
  result.reasons = reasons;
  if (pushed > 0) result.pulled += (await pullAll(ctx, limit)).pulled;
  return result;
}

async function pullAll({ api, store, userId }: Context, limit: number): Promise<SyncResult> {
  const result: SyncResult = { pushed: 0, pulled: 0, conflicts: 0, reasons: {} };
  for (let more = true; more;) {
    const page = await api.sync.pull({ cursor: store.getKv<string>(CURSOR_KEY) ?? "", limit });
    const rows = fromProto.syncRows(page.rows, userId);
    store.transaction(() => {
      for (const table of syncTables) {
        result.pulled += rows[table].length;
        // A dirty row keeps its local copy: the push that follows carries it, and the server's
        // answer to that push decides (P2-R36).
        syncRepo(store, table).upsertFromServer(rows[table]);
      }
      // Only active programs are pulled, so one that left the list is no longer active.
      const programs = page.programs.map(fromProto.program);
      const active = new Set(programs.map((p) => p.id));
      for (const p of store.list("programs")) {
        if (p.status === "active" && !active.has(p.id)) store.delete("programs", p.id);
      }
      programsRepo(store).upsertFromServer(programs);
      const days = page.resolvedDays.map(fromProto.resolvedDay);
      resolvedDaysRepo(store).replaceFromServer(days);
      rememberUnilateral(
        store,
        new Map(
          days
            .flatMap((d) => d.payload.exercises.filter((e) => e.slug))
            .map((e) => [e.slug, !!e.unilateral]),
        ),
      );
      mealDaysRepo(store).replaceFromServer(page.resolvedMealDays.map(fromProto.mealDay));
      store.setKv(CURSOR_KEY, page.cursor);
    });
    more = page.hasMore;
  }
  return result;
}

// The pull carries only active programs; drafts and finished ones, and programs made or deleted on
// another client, come from the full list.
async function listPrograms(api: Api, store: Store) {
  const programs: ProgramSummary[] = [];
  for (let cursor = "", more = true; more;) {
    const page = await api.programs.listPrograms({ cursor, limit: BATCH });
    programs.push(...page.programs.map(fromProto.program));
    [cursor, more] = [page.nextCursor, page.hasMore];
  }
  const ids = new Set(programs.map((p) => p.id));
  store.transaction(() => {
    for (const p of store.list("programs")) if (!ids.has(p.id)) store.delete("programs", p.id);
    programsRepo(store).upsertFromServer(programs);
  });
}

async function pushAll(ctx: Context) {
  const { api, store, userId } = ctx;
  const result = { pushed: 0, conflicts: 0, reasons: {} as Record<string, string> };
  // A merged row is pushed again once; whatever still conflicts waits for the next sync.
  for (let round = 0; round < 2; round++) {
    const dirty = Object.fromEntries(syncTables.map((t) => [t, store.listDirty(t)])) as Dirty;
    const longest = Math.max(...syncTables.map((t) => dirty[t].length));
    for (let from = 0; from < longest; from += BATCH) {
      const batch = Object.fromEntries(
        syncTables.map((t) => [t, dirty[t].slice(from, from + BATCH)]),
      ) as Dirty;
      const response = await api.sync.push({
        deviceId: deviceId(store),
        rows: toProto.syncRows(batch),
      });
      const applied = new Set(response.appliedIds);
      const conflicts = fromProto.syncRows(response.conflicts, userId);
      store.transaction(() => {
        for (const table of syncTables) {
          for (const row of batch[table]) {
            // The server assigns the next version to an applied row.
            if (applied.has(row.id)) store.markClean(table, row.id, row.version + 1, row.updatedAt);
            delete result.reasons[row.id];
          }
          result.conflicts += resolve(
            store,
            table,
            settle(store, table, batch[table], conflicts[table]),
            response.reasons,
          );
        }
      });
      Object.assign(result.reasons, response.reasons);
      result.pushed += applied.size;
    }
  }
  return result;
}

// Pairs each conflict row with the current local row it answers: the row of the same id, or the
// pushed row the server knows under another id: on a one-per-date table the one for its date, and
// a workout of the same planned day on its date.
function settle<T extends SyncTable>(
  store: Store,
  table: T,
  pushed: Rows[T][],
  servers: Omit<Rows[T], "dirty">[],
): Conflict<Rows[T]>[] {
  return servers.flatMap((row) => {
    const server = { ...row, dirty: false } as Rows[T];
    const match =
      pushed.find((r) => r.id === server.id) ??
      pushed.find(
        (r) => r.date === server.date && (table !== "workoutRecords" || samePlannedDay(r, server)),
      );
    const local = match && store.get(table, match.id);
    if (local) return [{ local, server }];
    store.put(table, server);
    return [];
  });
}

const samePlannedDay = (a: Row, b: Row) =>
  a.programId === b.programId && a.week === b.week && a.day === b.day;

// Merges conflicts in place: a row to push again is written dirty under the server's id and
// version, otherwise the server row stands. A row the server knows under another id is re-keyed.
// A finalized record cannot change any more, so the server's copy replaces local edits.
function resolve<T extends SyncTable>(
  store: Store,
  table: T,
  conflicts: Conflict<Rows[T]>[],
  reasons: Record<string, string> = {},
) {
  for (const { local, server } of conflicts) {
    if (server.version < local.version) continue;
    const merged = reasons[local.id] === "finalized" ? undefined : merge(table, local, server);
    if (local.id !== server.id) store.remove(table, local.id);
    store.put(
      table,
      merged
        ? {
            ...merged,
            id: server.id,
            version: server.version,
            userId: server.userId,
            updatedAt: new Date().toISOString(),
            deviceId: deviceId(store),
            dirty: true,
          }
        : server,
    );
  }
  return conflicts.length;
}
