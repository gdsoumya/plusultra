import { DatabaseSync } from "node:sqlite";

import type { DayLog } from "../domain/types";
import type { SqlExecutor, SqlValue } from "./sqliteStore";

// Runs the store's SQL on Node's built-in SQLite in place of expo-sqlite.
export function nodeSqlite(): SqlExecutor {
  const db = new DatabaseSync(":memory:");
  return {
    execSync: (sql) => db.exec(sql),
    runSync: (sql, params) => db.prepare(sql).run(...params),
    getAllSync: <T>(sql: string, params: SqlValue[]) => db.prepare(sql).all(...params) as T[],
  };
}

export function dayLog(id: string, date: string, over: Partial<DayLog> = {}): DayLog {
  return {
    id,
    userId: "u1",
    updatedAt: "2026-11-10T08:00:00.000Z",
    version: 1,
    deviceId: "server",
    dirty: false,
    date,
    status: "done",
    notes: "",
    ...over,
  };
}

// What a web page served over plain http sees: browsers offer crypto.randomUUID only on secure
// origins.
export function insecureOrigin() {
  const real = globalThis.crypto;
  jest.replaceProperty(globalThis, "crypto", {
    getRandomValues: real.getRandomValues.bind(real),
  } as Crypto);
}
