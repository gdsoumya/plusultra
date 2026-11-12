import { uuid } from "../uuid";

import { createMemoryStore } from "../memoryStore";
import type { Store } from "../store";
import { dayLog } from "../test-utils";
import { dayLogsRepo } from "./dayLogs";

jest.mock("../uuid", () => ({ uuid: jest.fn(() => "device-1") }));

describe("sync repository", () => {
  let store: Store;
  let repo: ReturnType<typeof dayLogsRepo>;
  beforeEach(() => {
    jest.mocked(uuid).mockClear();
    store = createMemoryStore();
    repo = dayLogsRepo(store);
  });

  it("stamps local writes dirty from this device and keeps the server version", () => {
    const before = dayLog("a", "2026-11-10", { version: 3 });
    const saved = repo.save(before);
    expect(saved).toMatchObject({ dirty: true, deviceId: "device-1", version: 3 });
    expect(saved.updatedAt > before.updatedAt).toBe(true);
    expect(repo.get("a")).toEqual(saved);

    repo.save(dayLog("b", "2026-11-11"));
    expect(uuid).toHaveBeenCalledTimes(1);
    expect(store.getKv("deviceId")).toBe("device-1");
  });

  it("finds rows by date and by range in date order", () => {
    for (const [id, date] of [
      ["c", "2026-11-12"],
      ["a", "2026-11-10"],
      ["b", "2026-11-11"],
    ]) {
      store.put("dayLogs", dayLog(id, date));
    }
    expect(repo.byDate("2026-11-11").map((r) => r.id)).toEqual(["b"]);
    expect(repo.range("2026-11-10", "2026-11-11").map((r) => r.id)).toEqual(["a", "b"]);
  });

  it("removes rows as dirty tombstones", () => {
    repo.save(dayLog("a", "2026-11-10"));
    repo.remove("a");
    expect(repo.byDate("2026-11-10")).toEqual([]);
    expect(repo.get("a")).toMatchObject({ dirty: true, deletedAt: expect.any(String) });
  });

  it("takes server rows over clean local rows", () => {
    store.put("dayLogs", dayLog("a", "2026-11-10", { notes: "local" }));
    const { dirty: _, ...server } = dayLog("a", "2026-11-10", { notes: "server", version: 2 });
    const { dirty: __, ...added } = dayLog("b", "2026-11-11");
    expect(repo.upsertFromServer([server, added])).toEqual([]);
    expect(repo.get("a")).toEqual({ ...server, dirty: false });
    expect(repo.get("b")).toEqual({ ...added, dirty: false });
  });

  it("ignores server rows older than the local row", () => {
    store.put("dayLogs", dayLog("a", "2026-11-10", { notes: "v3", version: 3 }));
    const { dirty: _, ...stale } = dayLog("a", "2026-11-10", { notes: "v2", version: 2 });
    expect(repo.upsertFromServer([stale])).toEqual([]);
    expect(repo.get("a")).toMatchObject({ notes: "v3", version: 3 });
  });

  it("ignores a re-delivered server row a dirty local row is based on", () => {
    store.put("dayLogs", dayLog("a", "2026-11-10", { notes: "server", version: 2 }));
    const edited = repo.save(dayLog("a", "2026-11-10", { notes: "local", version: 2 }));
    const { dirty: _, ...same } = dayLog("a", "2026-11-10", { notes: "server", version: 2 });
    expect(repo.upsertFromServer([same])).toEqual([]);
    expect(repo.get("a")).toEqual(edited);
  });

  it("applies a server tombstone to a clean row and reports it against a dirty one", () => {
    store.put("dayLogs", dayLog("clean", "2026-11-10"));
    const edited = repo.save(dayLog("edited", "2026-11-11"));
    const deletedAt = "2026-11-12T08:00:00.000Z";
    const tombstone = (id: string, date: string) => {
      const { dirty: _, ...row } = dayLog(id, date, {
        version: 2,
        deletedAt,
        updatedAt: deletedAt,
      });
      return row;
    };

    const conflicts = repo.upsertFromServer([
      tombstone("clean", "2026-11-10"),
      tombstone("edited", "2026-11-11"),
    ]);
    expect(repo.get("clean")).toMatchObject({ deletedAt, version: 2, dirty: false });
    expect(repo.byDate("2026-11-10")).toEqual([]);
    expect(conflicts).toEqual([
      { local: edited, server: { ...tombstone("edited", "2026-11-11"), dirty: false } },
    ]);
    expect(repo.get("edited")).toEqual(edited);
  });

  it("never overwrites dirty local rows and reports them as conflicts", () => {
    const newer = repo.save(dayLog("a", "2026-11-10", { notes: "newer local" }));
    const older = { ...dayLog("b", "2026-11-11", { notes: "older local" }), dirty: true };
    store.put("dayLogs", older);
    const later = "2099-01-01T00:00:00.000Z";
    const { dirty: _, ...serverA } = dayLog("a", "2026-11-10", { notes: "server", version: 2 });
    const { dirty: __, ...serverB } = dayLog("b", "2026-11-11", { updatedAt: later, version: 2 });

    expect(repo.upsertFromServer([serverA, serverB])).toEqual([
      { local: newer, server: { ...serverA, dirty: false } },
      { local: older, server: { ...serverB, dirty: false } },
    ]);
    expect(repo.get("a")).toEqual(newer);
    expect(repo.get("b")).toEqual(older);
  });
});
