import { storeFrom, tables, type ListFilter, type Row, type Store, type Table } from "./store";

export type SqlValue = string | number | null;

// The part of expo-sqlite's SQLiteDatabase the store uses.
export interface SqlExecutor {
  execSync(sql: string): void;
  runSync(sql: string, params: SqlValue[]): unknown;
  getAllSync<T>(sql: string, params: SqlValue[]): T[];
}

// Every table has the same shape: indexed columns copied from the row, and the row as JSON.
const createTable = (t: Table) => `
CREATE TABLE IF NOT EXISTS ${t} (
  id TEXT PRIMARY KEY NOT NULL, date TEXT, program_id TEXT, week INTEGER, day INTEGER,
  dirty INTEGER NOT NULL DEFAULT 0, deleted_at TEXT, data TEXT NOT NULL
);
CREATE INDEX IF NOT EXISTS ${t}_date ON ${t} (date);
CREATE INDEX IF NOT EXISTS ${t}_dirty ON ${t} (dirty);
CREATE INDEX IF NOT EXISTS ${t}_day ON ${t} (program_id, week, day);`;

// Append only: migrations[i] upgrades schema version i to i + 1.
export const migrations = [(Object.keys(tables) as Table[]).map(createTable).join("\n")];

const columns = (r: Row): SqlValue[] => [
  r.id,
  r.date ?? null,
  r.programId ?? null,
  r.week ?? null,
  r.day ?? null,
  r.dirty ? 1 : 0,
  r.deletedAt || null,
  JSON.stringify(r),
];

function where(f: ListFilter): [string, SqlValue[]] {
  const terms: [string, SqlValue][] = [];
  if (f.dateFrom) terms.push(["date >= ?", f.dateFrom]);
  if (f.dateTo) terms.push(["date <= ?", f.dateTo]);
  if (f.programId) terms.push(["program_id = ?", f.programId]);
  const sql = terms.map(([term]) => term);
  if (!f.includeDeleted) sql.push("deleted_at IS NULL");
  if (f.dirty) sql.push("dirty = 1");
  return [sql.length ? ` WHERE ${sql.join(" AND ")}` : "", terms.map(([, value]) => value)];
}

export function createSqliteStore(db: SqlExecutor): Store {
  const select = (sql: string, params: SqlValue[]) =>
    db.getAllSync<{ data: string }>(sql, params).map((r) => JSON.parse(r.data));
  const store = storeFrom({
    get: (table, id) => select(`SELECT data FROM ${table} WHERE id = ?`, [id])[0],
    put(table, row) {
      db.runSync(
        `INSERT OR REPLACE INTO ${table} (id, date, program_id, week, day, dirty, deleted_at, data) VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
        columns(row),
      );
    },
    remove(table, id) {
      db.runSync(`DELETE FROM ${table} WHERE id = ?`, [id]);
    },
    // The schema version stays, or the next start would rerun every migration.
    clear() {
      for (const table of Object.keys(tables) as Table[]) {
        db.runSync(`DELETE FROM ${table} WHERE id <> ?`, ["schemaVersion"]);
      }
    },
    list(table, filter = {}) {
      const [sql, params] = where(filter);
      return select(`SELECT data FROM ${table}${sql} ORDER BY date, week, day, id`, params);
    },
    // Savepoints nest, and outside a transaction one begins it.
    transaction(fn) {
      db.execSync("SAVEPOINT tx");
      try {
        const result = fn();
        db.execSync("RELEASE tx");
        return result;
      } catch (e) {
        db.execSync("ROLLBACK TO tx; RELEASE tx");
        throw e;
      }
    },
  });
  migrate(db, store);
  return store;
}

function migrate(db: SqlExecutor, store: Store) {
  db.execSync(createTable("kv"));
  const from = store.getKv<number>("schemaVersion") ?? 0;
  migrations.slice(from).forEach((sql, i) =>
    store.transaction(() => {
      db.execSync(sql);
      store.setKv("schemaVersion", from + i + 1);
    }),
  );
}
