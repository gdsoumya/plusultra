import { Code, ConnectError } from "@connectrpc/connect";

import { createClients, type Api } from "../api/clients";
import { fromProto, toProto, type ServerRows } from "../api/convert";
import { createMockTransport, type MockTransport } from "../api/mockTransport";
import { adHocWorkout, workoutFromDay } from "../domain/quickfill";
import { dayTotals } from "../domain/totals";
import { ProgramKind } from "../gen/plusultra/v1/program_pb";
import type { Meal, WorkoutRecord } from "../domain/types";
import { createMemoryStore } from "./memoryStore";
import { plannedWorkout } from "./plans";
import { checkinsRepo } from "./repositories/checkins";
import { dayLogsRepo } from "./repositories/dayLogs";
import { mealLogsRepo } from "./repositories/mealLogs";
import { programsRepo } from "./repositories/programs";
import { workoutRecordsRepo } from "./repositories/workoutRecords";
import type { Store, SyncTable } from "./store";
import { CURSOR_KEY, syncOnce } from "./syncEngine";
import { dayLog } from "./test-utils";

jest.mock("./uuid", () => ({ uuid: jest.fn(() => "device-1") }));

const today = "2026-11-11";
const tomorrow = "2026-11-12";
const yesterday = "2026-11-10T09:00:00.000Z";
const user = "user-1";
// Client ids are UUIDs, as the server requires.
const phone = "6f0f9d2e-2b7a-4a4e-9d2b-1c3e5a7b9d01";
const tablet = "6f0f9d2e-2b7a-4a4e-9d2b-1c3e5a7b9d02";

type Position = readonly [number, number];
type Entries = Pick<WorkoutRecord, "entries">;
const pending = (r: Entries): Position[] =>
  r.entries.flatMap((e, i) => e.sets.flatMap((s, j) => (s.completed ? [] : [[i, j] as const])));
const completedAt = (r: Entries, [i, j]: Position) => r.entries[i].sets[j].completed;
const completeAt = <R extends Entries>(r: R, [i, j]: Position): R => ({
  ...r,
  entries: r.entries.map((e, k) =>
    k !== i
      ? e
      : {
          ...e,
          sets: e.sets.map((s, l) =>
            l !== j ? s : { ...s, completed: true, reps: s.planned.reps, weight: s.planned.weight },
          ),
        },
  ),
});

describe("syncOnce", () => {
  let mock: MockTransport;
  let api: Api;
  let store: Store;
  const sync = (limit?: number) => syncOnce(api, store, { userId: user, limit });
  const onServer = async <T extends SyncTable>(table: T) =>
    fromProto.syncRows((await api.sync.pull({})).rows, user)[table];
  const fromTablet = (rows: Partial<ServerRows>) =>
    api.sync.push({ deviceId: "tablet", rows: toProto.syncRows(rows) });
  const dirtyCount = () =>
    (["dayLogs", "workoutRecords", "mealLogs", "checkins"] as const).reduce(
      (n, t) => n + store.listDirty(t).length,
      0,
    );

  beforeEach(() => {
    mock = createMockTransport({ today });
    api = createClients(mock.transport);
    store = createMemoryStore();
  });

  it("fills the store from the server, caches the plans and stores the cursor", async () => {
    const result = await sync();
    const all = await Promise.all(
      (["dayLogs", "workoutRecords", "mealLogs", "checkins"] as const).map(onServer),
    );
    expect(result).toEqual({ pushed: 0, pulled: all.flat().length, conflicts: 0, reasons: {} });
    expect(store.list("workoutRecords")).toHaveLength(21);
    expect(store.list("checkins")).toHaveLength(25);
    expect(dirtyCount()).toBe(0);
    expect(store.getKv<string>(CURSOR_KEY)).toEqual(expect.any(String));

    expect(
      store
        .list("programs")
        .filter((p) => p.status === "active")
        .map((p) => p.kind),
    ).toEqual(["meal", "workout"]);
    expect(store.list("resolvedDays")).toHaveLength(28);
    expect(store.list("resolvedDays")[0].date).toBe(today);
    // Kind-picked meal days share week and day 0; each date must still be cached.
    const mealDays = store.list("mealDays");
    expect(mealDays).toHaveLength(12);
    expect(new Set(mealDays.map((d) => d.date)).size).toBe(12);
  });

  it("lists every program, so a draft made on another client shows and a deleted one goes", async () => {
    await sync();
    const { programs } = await api.programs.listPrograms({});
    expect(
      store
        .list("programs")
        .map((p) => p.id)
        .sort(),
    ).toEqual(programs.map((p) => p.id).sort());
    const script =
      'program("Draft plan", weeks=2)\nday(1, 1, "Full body")\nex("Squat", sets=[s(3, 5, kg(60))])\n';
    const { program } = await api.programs.createProgram({ kind: ProgramKind.WORKOUT, script });
    await sync();
    expect(programsRepo(store).get(program!.id)).toMatchObject({
      name: "Draft plan",
      status: "draft",
    });

    await api.programs.deleteProgram({ id: program!.id });
    await sync();
    expect(programsRepo(store).get(program!.id)).toBeUndefined();
  });

  it("brings the unilateral flag of pulled days to cached past days", async () => {
    await sync();
    const slug = "bulgarian-split-squat-dumbbell";
    const [upcoming] = store
      .list("resolvedDays")
      .filter((d) => d.payload.exercises.some((e) => e.slug === slug && e.unilateral));
    expect(upcoming).toBeDefined();
    const past = {
      ...upcoming,
      id: "past-day",
      date: "2026-11-02",
      payload: {
        ...upcoming.payload,
        exercises: upcoming.payload.exercises.map((e) => ({ ...e, unilateral: false })),
      },
    };
    store.put("resolvedDays", past);
    await sync();
    const exercises = store.get("resolvedDays", "past-day")!.payload.exercises;
    expect(exercises.map((e) => !!e.unilateral)).toEqual(
      upcoming.payload.exercises.map((e) => !!e.unilateral),
    );
  });

  it("pulls in pages until the server has no more", async () => {
    const whole = await sync();
    const paged = createMemoryStore();
    const result = await syncOnce(api, paged, { userId: user, limit: 10 });
    expect(result.pulled).toBeGreaterThanOrEqual(whole.pulled);
    expect(paged.list("workoutRecords")).toHaveLength(21);
    expect(paged.getKv(CURSOR_KEY)).toEqual(store.getKv(CURSOR_KEY));
  });

  it("resumes from the stored cursor and pulls only what changed", async () => {
    await sync();
    await fromTablet({ dayLogs: [dayLog(tablet, tomorrow, { version: 0 })] });
    expect(await sync()).toEqual({ pushed: 0, pulled: 1, conflicts: 0, reasons: {} });
    expect(dayLogsRepo(store).get(tablet)).toMatchObject({ version: 1, dirty: false });
  });

  it("pushes a set logged offline once, with the completion kept, across app restarts", async () => {
    await sync();
    const repo = workoutRecordsRepo(store);
    const open = repo.byDate(today)[0];
    const at = pending(open)[0];
    repo.save(completeAt(open, at));
    expect(dirtyCount()).toBe(1);

    expect(await sync()).toMatchObject({ pushed: 1, conflicts: 0 });
    // The app is reopened: a new sync over the same store pushes nothing again.
    expect(await sync()).toMatchObject({ pushed: 0, conflicts: 0 });

    const records = (await onServer("workoutRecords")).filter((r) => r.date === today);
    expect(records).toHaveLength(1);
    expect(completedAt(records[0], at)).toBe(true);
    expect(records[0].version).toBe(open.version + 1);
    expect(repo.get(open.id)).toMatchObject({ dirty: false, version: open.version + 1 });
    expect(completedAt(repo.get(open.id)!, at)).toBe(true);
  });

  it("asks the server to finalize a workout finished offline", async () => {
    await sync();
    const repo = workoutRecordsRepo(store);
    const open = repo.byDate(today)[0];
    const done = pending(open).reduce(completeAt, open);
    repo.save({ ...done, finalizeRequested: true, finishedAt: new Date().toISOString() });

    await sync();
    const [server] = (await onServer("workoutRecords")).filter((r) => r.id === open.id);
    expect(server).toMatchObject({ finalized: true, finalizeRequested: false });
    expect(repo.get(open.id)).toMatchObject({
      finalized: true,
      finalizeRequested: false,
      dirty: false,
      version: server.version,
    });
  });

  it("pushes a later local edit of a finalized record, which stays finalized", async () => {
    await sync();
    const repo = workoutRecordsRepo(store);
    const open = repo.byDate(today)[0];
    repo.save({ ...open, finalizeRequested: true, finishedAt: new Date().toISOString() });
    await sync();
    const final = repo.get(open.id)!;
    repo.save({ ...final, dayName: "edited after finalizing" });

    const result = await sync();
    expect(result).toMatchObject({ pushed: 1, conflicts: 0 });
    const [server] = (await onServer("workoutRecords")).filter((r) => r.id === open.id);
    expect(server).toMatchObject({ dayName: "edited after finalizing", finalized: true });
    expect(repo.get(open.id)).toMatchObject({
      dayName: "edited after finalizing",
      finalized: true,
      dirty: false,
    });
    expect(dirtyCount()).toBe(0);
  });

  it("deletes a finalized record the user reset, keeping nothing of it locally", async () => {
    await sync();
    const repo = workoutRecordsRepo(store);
    const open = repo.byDate(today)[0];
    repo.save({ ...open, finalizeRequested: true, finishedAt: new Date().toISOString() });
    await sync();
    expect(repo.get(open.id)).toMatchObject({ finalized: true });

    repo.remove(open.id);
    expect(await sync()).toMatchObject({ pushed: 1, conflicts: 0, reasons: {} });
    const [server] = (await onServer("workoutRecords")).filter((r) => r.id === open.id);
    expect(server.deletedAt).toEqual(expect.any(String));
    expect(repo.get(open.id)).toMatchObject({ deletedAt: expect.any(String), dirty: false });
    expect(repo.byDate(today)).toEqual([]);
  });

  it("keeps a reset finalized record deleted when an ad hoc workout takes its date", async () => {
    await sync();
    const repo = workoutRecordsRepo(store);
    const old = repo.byDate(today)[0];
    repo.save({ ...old, finalizeRequested: true, finishedAt: new Date().toISOString() });
    await sync();
    expect(repo.get(old.id)).toMatchObject({ finalized: true });

    repo.remove(old.id);
    const adHoc = adHocWorkout({ id: phone, date: today, userId: user }, new Date().toISOString());
    repo.save({ ...adHoc, entries: [{ ...old.entries[0], fromProgram: false }] });
    await sync();
    await sync();

    const server = (await onServer("workoutRecords")).filter((r) => r.date === today);
    expect(server.filter((r) => !r.deletedAt).map((r) => r.id)).toEqual([phone]);
    expect(repo.get(old.id)).toMatchObject({ deletedAt: expect.any(String), dirty: false });
    expect(repo.byDate(today).map((r) => r.id)).toEqual([phone]);
    expect(dirtyCount()).toBe(0);
  });

  it("keeps local completions when the server has a newer record", async () => {
    await sync();
    const repo = workoutRecordsRepo(store);
    const open = repo.byDate(today)[0];
    const [mine, theirs] = pending(open);
    store.put("workoutRecords", { ...completeAt(open, mine), dirty: true, updatedAt: yesterday });
    const tablet = { ...completeAt(open, theirs), updatedAt: new Date().toISOString() };
    await fromTablet({ workoutRecords: [tablet] });

    const result = await sync();
    expect(result.conflicts).toBe(1);
    const records = (await onServer("workoutRecords")).filter((r) => r.date === today);
    expect(records).toHaveLength(1);
    expect([completedAt(records[0], mine), completedAt(records[0], theirs)]).toEqual([true, true]);
    const local = repo.get(open.id)!;
    expect([completedAt(local, mine), completedAt(local, theirs)]).toEqual([true, true]);
    expect(local).toMatchObject({ dirty: false, version: records[0].version });
    expect(dirtyCount()).toBe(0);
  });

  it("never lets a pull replace a dirty row; the push's answer does", async () => {
    await sync();
    const repo = workoutRecordsRepo(store);
    const open = repo.byDate(today)[0];
    store.put("workoutRecords", { ...open, dayName: "mine", dirty: true, updatedAt: yesterday });
    await fromTablet({
      workoutRecords: [{ ...open, dayName: "theirs", updatedAt: new Date().toISOString() }],
    });

    mock.failNext("sync.push", new ConnectError("offline", Code.Unavailable));
    await expect(sync()).rejects.toMatchObject({ code: Code.Unavailable });
    expect(repo.get(open.id)).toMatchObject({ dayName: "mine", dirty: true });

    await sync();
    const [server] = (await onServer("workoutRecords")).filter((r) => r.id === open.id);
    expect(server.dayName).toBe("theirs");
    expect(repo.get(open.id)).toMatchObject({ dayName: "theirs", dirty: false });
  });

  it("adopts the server's row when another device already logged the date", async () => {
    await sync();
    const repo = dayLogsRepo(store);
    await fromTablet({
      dayLogs: [dayLog(tablet, tomorrow, { version: 0, status: "done", updatedAt: yesterday })],
    });
    repo.save(dayLog(phone, tomorrow, { status: "skipped", notes: "phone", version: 0 }));

    const result = await sync();
    expect(result.conflicts).toBe(1);
    const server = (await onServer("dayLogs")).filter((r) => r.date === tomorrow);
    expect(server).toHaveLength(1);
    expect(server[0]).toMatchObject({ id: tablet, status: "skipped", notes: "phone" });
    expect(repo.get(phone)).toBeUndefined();
    expect(repo.byDate(tomorrow)).toEqual([
      expect.objectContaining({ id: tablet, status: "skipped", dirty: false }),
    ]);
    expect(repo.byDate(tomorrow)[0].version).toBe(server[0].version);
  });

  it("makes one meal log of two devices' logs for the same date, losing neither (P2-R41)", async () => {
    await sync();
    const repo = mealLogsRepo(store);
    const item = (foodName: string, qty: number, planned = true) => ({
      foodName,
      qty,
      unit: "g",
      planned,
      macros: { kcal: qty, protein: 0, carbs: 0, fat: 0, fiber: 0, waterMl: 0 },
    });
    const log = (id: string, meals: Meal[], updatedAt: string) => ({
      id,
      userId: user,
      date: tomorrow,
      version: 0,
      updatedAt,
      deviceId: "",
      dirty: false,
      meals,
      totals: dayTotals(meals),
    });
    // The tablet logged breakfast; the phone, which had not seen that, a later lunch and a snack.
    await fromTablet({
      mealLogs: [
        log(
          tablet,
          [
            { name: "Breakfast", items: [item("Oats", 80)] },
            { name: "Lunch", items: [item("Rice", 0)] },
          ],
          yesterday,
        ),
      ],
    });
    repo.save(
      log(
        phone,
        [
          { name: "Breakfast", items: [item("Oats", 0)] },
          { name: "Lunch", items: [item("Rice", 150)] },
          { name: "meal", items: [item("Chicken", 100, false)] },
        ],
        new Date().toISOString(),
      ),
    );

    expect(await sync()).toMatchObject({ conflicts: 1 });
    const server = (await onServer("mealLogs")).filter((r) => r.date === tomorrow);
    expect(server).toHaveLength(1);
    const eaten = (l: { meals: Meal[] }) =>
      l.meals.map((m) => [m.name, m.items.map((i) => [i.foodName, i.qty])]);
    const both = [
      ["Breakfast", [["Oats", 80]]],
      ["Lunch", [["Rice", 150]]],
      ["meal", [["Chicken", 100]]],
    ];
    expect(server[0].id).toBe(tablet);
    expect(eaten(server[0])).toEqual(both);
    expect(repo.get(phone)).toBeUndefined();
    const [local] = repo.byDate(tomorrow);
    expect(local).toMatchObject({ id: tablet, dirty: false, version: server[0].version });
    expect(eaten(local)).toEqual(both);
  });

  it("shares one record when two devices start the same planned day, keeping both's sets", async () => {
    await sync();
    const { program, day } = plannedWorkout(store, tomorrow)!;
    const fresh = (id: string, updatedAt: string) => ({
      ...workoutFromDay(day, program.id, { id, userId: user, date: tomorrow }, updatedAt),
      updatedAt,
      deviceId: "",
      dirty: false,
    });
    const tabletRecord = fresh(tablet, yesterday);
    const [theirs, mine] = pending(tabletRecord);
    await fromTablet({ workoutRecords: [completeAt(tabletRecord, theirs)] });
    const repo = workoutRecordsRepo(store);
    repo.save(completeAt(fresh(phone, new Date().toISOString()), mine));

    expect(await sync()).toMatchObject({ conflicts: 1 });
    const server = (await onServer("workoutRecords")).filter((r) => r.date === tomorrow);
    expect(server.map((r) => r.id)).toEqual([tablet]);
    expect([completedAt(server[0], theirs), completedAt(server[0], mine)]).toEqual([true, true]);
    expect(repo.get(phone)).toBeUndefined();
    expect(repo.byDate(tomorrow)).toEqual([
      expect.objectContaining({ id: tablet, dirty: false, version: server[0].version }),
    ]);
  });

  it("syncs tombstones both ways", async () => {
    await sync();
    const checkins = checkinsRepo(store);
    const removed = store.list("checkins").at(-1)!;
    checkins.remove(removed.id);
    const deleted = store.list("dayLogs").at(-1)!;
    await fromTablet({
      dayLogs: [
        { ...deleted, deletedAt: new Date().toISOString(), updatedAt: new Date().toISOString() },
      ],
    });

    await sync();
    const onServerCheckin = (await onServer("checkins")).find((r) => r.id === removed.id)!;
    expect(onServerCheckin.deletedAt).toEqual(expect.any(String));
    expect(checkins.get(removed.id)).toMatchObject({ dirty: false, version: removed.version + 1 });
    expect(dayLogsRepo(store).byDate(deleted.date)).toEqual([]);
    expect(dayLogsRepo(store).get(deleted.id)).toMatchObject({
      deletedAt: expect.any(String),
      dirty: false,
    });
  });

  it("keeps rows dirty and the cursor when a call fails, and retries on the next sync", async () => {
    await sync();
    const cursor = store.getKv(CURSOR_KEY);
    const repo = dayLogsRepo(store);
    repo.save(dayLog(phone, tomorrow, { version: 0 }));

    mock.failNext("sync.pull", new ConnectError("offline", Code.Unavailable));
    await expect(sync()).rejects.toMatchObject({ code: Code.Unavailable });
    mock.failNext("sync.push", new ConnectError("offline", Code.Unavailable));
    await expect(sync()).rejects.toMatchObject({ code: Code.Unavailable });
    expect(store.getKv(CURSOR_KEY)).toEqual(cursor);
    expect(dirtyCount()).toBe(1);

    expect(await sync()).toMatchObject({ pushed: 1 });
    expect(dirtyCount()).toBe(0);
    expect((await onServer("dayLogs")).some((r) => r.id === phone)).toBe(true);
  });

  it("replaces the cached plans when the active program changes", async () => {
    await sync();
    const { programs } = await api.programs.listPrograms({});
    const strength = programs.find((p) => p.id === "program-strength16")!;
    const gzclp = programs.find((p) => p.id === "program-gzclp")!;
    await api.programs.finishProgram({ id: strength.id });
    await api.programs.activateProgram({ id: gzclp.id });

    await sync();
    const active = store.list("programs").filter((p) => p.status === "active");
    expect(active.map((p) => [p.id, p.mode, p.nextWeek, p.nextDay])).toEqual([
      ["program-gzclp", "floating", 1, 1],
      ["program-meals", "anchored", undefined, undefined],
    ]);
    const days = store.list("resolvedDays");
    expect(days.every((d) => d.programId === gzclp.id && d.date === undefined)).toBe(true);
    expect(days).toHaveLength(16);
  });
});
