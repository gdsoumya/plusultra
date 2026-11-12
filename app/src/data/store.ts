import type {
  Checkin,
  DayLog,
  MealDay,
  MealLog,
  ProgramSummary,
  ResolvedDay,
  SyncMeta,
  WorkoutRecord,
} from "../domain/types";

export interface ResolvedDayCache {
  id: string;
  programId: string;
  week: number;
  day: number;
  date?: string;
  payload: ResolvedDay;
}

export interface MealDayCache {
  id: string;
  programId: string;
  week: number;
  day: number;
  date?: string;
  payload: MealDay;
}

export interface KvEntry {
  id: string;
  value: unknown;
}

export interface Rows {
  dayLogs: DayLog;
  workoutRecords: WorkoutRecord;
  mealLogs: MealLog;
  checkins: Checkin;
  programs: ProgramSummary;
  resolvedDays: ResolvedDayCache;
  mealDays: MealDayCache;
  kv: KvEntry;
}

export type Table = keyof Rows;
export type SyncTable = { [T in Table]: Rows[T] extends SyncMeta ? T : never }[Table];

// true marks the syncable tables; the rest cache server-owned data.
export const tables = {
  dayLogs: true,
  workoutRecords: true,
  mealLogs: true,
  checkins: true,
  programs: false,
  resolvedDays: false,
  mealDays: false,
  kv: false,
} satisfies Record<Table, boolean>;

const isSyncTable = (table: Table): table is SyncTable => tables[table];
export const syncTables = (Object.keys(tables) as Table[]).filter(isSyncTable);

// The fields a store indexes and filters on.
export interface Row {
  id: string;
  date?: string;
  programId?: string;
  week?: number;
  day?: number;
  dirty?: boolean;
  deletedAt?: string;
}

export interface ListFilter {
  dateFrom?: string;
  dateTo?: string;
  programId?: string;
  includeDeleted?: boolean;
  dirty?: boolean;
}

export interface Store {
  get<T extends Table>(table: T, id: string): Rows[T] | undefined;
  put<T extends Table>(table: T, row: Rows[T]): void;
  // Tombstones a sync row so the delete is pushed; drops a cached row.
  delete(table: Table, id: string): void;
  // Drops a row without a tombstone: for rows the server never had.
  remove(table: Table, id: string): void;
  // Ordered by date, week, day, id with missing values first; date bounds are inclusive.
  list<T extends Table>(table: T, filter?: ListFilter): Rows[T][];
  listDirty<T extends SyncTable>(table: T): Rows[T][];
  // The row stays dirty when it changed after the push read it at pushedUpdatedAt.
  markClean(table: SyncTable, id: string, version: number, pushedUpdatedAt: string): void;
  getKv<V>(key: string): V | undefined;
  setKv(key: string, value: unknown): void;
  // Rolls back every write made in fn when fn throws.
  transaction<R>(fn: () => R): R;
  // Local writes call changed() so the sync engine can schedule a push; onChange returns the
  // unsubscribe.
  onChange(listener: () => void): () => void;
  changed(): void;
  // Empties every table on logout, so the next user starts from nothing.
  clearUserData(): void;
}

export type StorePrimitives = Pick<Store, "get" | "put" | "remove" | "list" | "transaction"> & {
  clear(): void;
};

export function storeFrom({ clear, ...p }: StorePrimitives): Store {
  const listeners = new Set<() => void>();
  return {
    ...p,
    clearUserData: () => p.transaction(clear),
    delete(table, id) {
      if (!isSyncTable(table)) return p.remove(table, id);
      const row = p.get(table, id);
      const now = new Date().toISOString();
      if (row) p.put(table, { ...row, deletedAt: now, updatedAt: now, dirty: true });
    },
    listDirty: (table) => p.list(table, { dirty: true, includeDeleted: true }),
    markClean(table, id, version, pushedUpdatedAt) {
      const row = p.get(table, id);
      if (!row) return;
      p.put(table, { ...row, version, dirty: row.dirty && row.updatedAt !== pushedUpdatedAt });
    },
    getKv: <V>(key: string) => p.get("kv", key)?.value as V | undefined,
    setKv: (key, value) => p.put("kv", { id: key, value }),
    onChange(listener) {
      listeners.add(listener);
      return () => void listeners.delete(listener);
    },
    changed: () => listeners.forEach((listener) => listener()),
  };
}
