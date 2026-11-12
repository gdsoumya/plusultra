import { createSqliteStore, migrations } from "./sqliteStore";
import { dayLog, nodeSqlite } from "./test-utils";

describe("sqlite store", () => {
  it("creates every table with its indexes and records the schema version", () => {
    const db = nodeSqlite();
    const store = createSqliteStore(db);
    expect(store.getKv("schemaVersion")).toBe(migrations.length);
    const names = db
      .getAllSync<{ name: string }>("SELECT name FROM sqlite_master", [])
      .map((r) => r.name);
    for (const table of [
      "dayLogs",
      "workoutRecords",
      "mealLogs",
      "checkins",
      "programs",
      "resolvedDays",
      "mealDays",
      "kv",
    ]) {
      expect(names).toEqual(
        expect.arrayContaining([table, `${table}_date`, `${table}_dirty`, `${table}_day`]),
      );
    }
  });

  it("reopens without rerunning migrations and keeps its rows", () => {
    const db = nodeSqlite();
    createSqliteStore(db).put("dayLogs", dayLog("a", "2026-11-10"));
    const exec = jest.spyOn(db, "execSync");
    const store = createSqliteStore(db);
    expect(exec.mock.calls.filter(([sql]) => sql.includes("CREATE TABLE"))).toHaveLength(1);
    expect(store.get("dayLogs", "a")).toEqual(dayLog("a", "2026-11-10"));
  });

  it("applies the migrations after the recorded version", () => {
    const db = nodeSqlite();
    createSqliteStore(db).setKv("schemaVersion", 0);
    expect(createSqliteStore(db).getKv("schemaVersion")).toBe(migrations.length);
  });

  it("keeps the schema version when user data is cleared", () => {
    const store = createSqliteStore(nodeSqlite());
    store.setKv("deviceId", "d1");
    store.clearUserData();
    expect(store.getKv("deviceId")).toBeUndefined();
    expect(store.getKv("schemaVersion")).toBe(migrations.length);
  });
});
