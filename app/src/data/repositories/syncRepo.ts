import { deviceId } from "../deviceId";
import type { Rows, Store, SyncTable } from "../store";

export type LocalRow<R> = Omit<R, "dirty" | "updatedAt" | "deviceId">;
export type ServerRow<R> = Omit<R, "dirty">;

export interface Conflict<R> {
  local: R;
  server: R;
}

export function syncRepo<T extends SyncTable>(store: Store, table: T) {
  type R = Rows[T];
  return {
    get: (id: string) => store.get(table, id),
    byDate: (date: string) => store.list(table, { dateFrom: date, dateTo: date }),
    range: (from: string, to: string) => store.list(table, { dateFrom: from, dateTo: to }),
    // Local writes keep the last server version; the server assigns the next one.
    save(row: LocalRow<R>): R {
      const saved = {
        ...row,
        dirty: true,
        updatedAt: new Date().toISOString(),
        deviceId: deviceId(store),
      } as R;
      store.put(table, saved);
      store.changed();
      return saved;
    },
    remove(id: string) {
      store.delete(table, id);
      store.changed();
    },
    // A server row older than the local one is stale or re-delivered by a paged pull, and one at
    // the version a dirty local row is based on has nothing new: both are skipped. Any other server
    // row never overwrites a dirty local row; it is only reported.
    upsertFromServer: (rows: ServerRow<R>[]): Conflict<R>[] =>
      store.transaction(() =>
        rows.flatMap((row) => {
          const server = { ...row, dirty: false } as R;
          const local = store.get(table, server.id);
          if (
            local &&
            (row.version < local.version || (local.dirty && row.version === local.version))
          ) {
            return [];
          }
          if (local?.dirty) return [{ local, server }];
          store.put(table, server);
          return [];
        }),
      ),
  };
}
