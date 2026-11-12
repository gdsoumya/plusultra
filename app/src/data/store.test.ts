import type { ProgramSummary, ResolvedDay } from "../domain/types";
import { createMemoryStore } from "./memoryStore";
import { createSqliteStore } from "./sqliteStore";
import type { Store } from "./store";
import { dayLog, nodeSqlite } from "./test-utils";

const program: ProgramSummary = {
  id: "p",
  kind: "workout",
  name: "Strength",
  revision: 1,
  status: "active",
  mode: "floating",
  weeks: 4,
  updatedAt: "2026-11-10T08:00:00.000Z",
};

function cachedDay(programId: string, week: number, day: number, date?: string) {
  const payload: ResolvedDay = {
    week,
    day,
    date,
    name: "",
    desc: "",
    kind: "training",
    exercises: [],
  };
  return { id: `${programId}:${week}:${day}`, programId, week, day, date, payload };
}

describe.each([
  ["memory", createMemoryStore],
  ["sqlite", () => createSqliteStore(nodeSqlite())],
])("%s store", (_name, create: () => Store) => {
  let store: Store;
  beforeEach(() => {
    store = create();
  });

  it("gets a copy of what was put", () => {
    const row = dayLog("a", "2026-11-10", { readiness: { sleep: 4, soreness: 2, stress: 1 } });
    store.put("dayLogs", row);
    const got = store.get("dayLogs", "a")!;
    expect(got).toEqual(row);
    got.notes = "changed";
    expect(store.get("dayLogs", "a")?.notes).toBe("");
    expect(store.get("dayLogs", "missing")).toBeUndefined();
  });

  it("lists an inclusive date range in date order", () => {
    for (const [id, date] of [
      ["c", "2026-11-12"],
      ["a", "2026-11-10"],
      ["d", "2026-11-13"],
      ["b", "2026-11-11"],
    ]) {
      store.put("dayLogs", dayLog(id, date));
    }
    const ids = (filter = {}) => store.list("dayLogs", filter).map((r) => r.id);
    expect(ids({ dateFrom: "2026-11-11", dateTo: "2026-11-12" })).toEqual(["b", "c"]);
    expect(ids()).toEqual(["a", "b", "c", "d"]);
  });

  it("filters by program and orders undated rows by week and day", () => {
    for (const row of [
      cachedDay("p", 2, 1),
      cachedDay("p", 1, 2),
      cachedDay("q", 1, 1),
      cachedDay("p", 1, 1),
    ]) {
      store.put("resolvedDays", row);
    }
    expect(store.list("resolvedDays", { programId: "p" }).map((r) => r.id)).toEqual([
      "p:1:1",
      "p:1:2",
      "p:2:1",
    ]);
    expect(store.list("resolvedDays", { dateFrom: "2026-01-01" })).toEqual([]);
  });

  it("tombstones sync rows and drops cached rows", () => {
    store.put("dayLogs", dayLog("a", "2026-11-10"));
    store.delete("dayLogs", "a");
    store.delete("dayLogs", "missing");
    expect(store.list("dayLogs")).toEqual([]);
    const [tombstone] = store.list("dayLogs", { includeDeleted: true });
    expect(tombstone).toMatchObject({ id: "a", dirty: true });
    expect(tombstone.deletedAt).toBeDefined();
    expect(tombstone.updatedAt).toBe(tombstone.deletedAt);

    store.put("programs", program);
    store.delete("programs", "p");
    expect(store.get("programs", "p")).toBeUndefined();
  });

  it("removes a sync row without a tombstone", () => {
    store.put("dayLogs", dayLog("a", "2026-11-10"));
    store.remove("dayLogs", "a");
    expect(store.list("dayLogs", { includeDeleted: true })).toEqual([]);
  });

  it("tells listeners about local changes until they unsubscribe", () => {
    const listener = jest.fn();
    const unsubscribe = store.onChange(listener);
    store.changed();
    expect(listener).toHaveBeenCalledTimes(1);
    unsubscribe();
    store.changed();
    expect(listener).toHaveBeenCalledTimes(1);
  });

  it("lists dirty rows including tombstones", () => {
    store.put("dayLogs", dayLog("a", "2026-11-10"));
    store.put("dayLogs", dayLog("b", "2026-11-11", { dirty: true }));
    store.put("dayLogs", dayLog("c", "2026-11-12"));
    store.delete("dayLogs", "c");
    expect(store.listDirty("dayLogs").map((r) => r.id)).toEqual(["b", "c"]);
  });

  it("marks a pushed row clean with the server version", () => {
    const pushedAt = "2026-11-10T09:00:00.000Z";
    store.put("dayLogs", dayLog("a", "2026-11-10", { dirty: true, updatedAt: pushedAt }));
    store.markClean("dayLogs", "a", 2, pushedAt);
    expect(store.get("dayLogs", "a")).toMatchObject({ dirty: false, version: 2 });
    expect(store.listDirty("dayLogs")).toEqual([]);
  });

  it("keeps a row dirty when it changed after the push read it", () => {
    store.put(
      "dayLogs",
      dayLog("a", "2026-11-10", { dirty: true, updatedAt: "2026-11-10T09:05:00.000Z" }),
    );
    store.markClean("dayLogs", "a", 2, "2026-11-10T09:00:00.000Z");
    expect(store.get("dayLogs", "a")).toMatchObject({ dirty: true, version: 2 });
  });

  it("stores key values", () => {
    expect(store.getKv("cursor")).toBeUndefined();
    store.setKv("cursor", { at: "c1" });
    expect(store.getKv("cursor")).toEqual({ at: "c1" });
  });

  it("rolls back every write of a transaction that throws", () => {
    store.put("dayLogs", dayLog("a", "2026-11-10"));
    expect(() =>
      store.transaction(() => {
        store.put("dayLogs", dayLog("b", "2026-11-11"));
        store.delete("dayLogs", "a");
        store.setKv("cursor", "c1");
        throw new Error("boom");
      }),
    ).toThrow("boom");
    expect(store.list("dayLogs", { includeDeleted: true })).toEqual([dayLog("a", "2026-11-10")]);
    expect(store.getKv("cursor")).toBeUndefined();

    expect(store.transaction(() => (store.put("dayLogs", dayLog("c", "2026-11-12")), 7))).toBe(7);
    expect(store.get("dayLogs", "c")).toBeDefined();
  });

  it("rolls back only the inner transaction when the outer one catches", () => {
    store.transaction(() => {
      store.put("dayLogs", dayLog("a", "2026-11-10"));
      expect(() =>
        store.transaction(() => {
          store.put("dayLogs", dayLog("b", "2026-11-11"));
          throw new Error("boom");
        }),
      ).toThrow("boom");
    });
    expect(store.list("dayLogs").map((r) => r.id)).toEqual(["a"]);
  });

  it("rolls back nested transactions with the outer one", () => {
    expect(() =>
      store.transaction(() => {
        store.transaction(() => store.put("dayLogs", dayLog("a", "2026-11-10")));
        throw new Error("boom");
      }),
    ).toThrow("boom");
    expect(store.get("dayLogs", "a")).toBeUndefined();
  });

  it("drops every row and setting on clearUserData", () => {
    store.put("dayLogs", dayLog("a", "2026-11-10"));
    store.put("programs", program);
    store.setKv("syncCursor", "7");
    store.clearUserData();
    expect(store.list("dayLogs")).toEqual([]);
    expect(store.get("programs", "p")).toBeUndefined();
    expect(store.getKv("syncCursor")).toBeUndefined();
  });
});
