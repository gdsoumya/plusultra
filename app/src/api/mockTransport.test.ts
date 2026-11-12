import { timestampFromDate } from "@bufbuild/protobuf/wkt";
import { Code, ConnectError } from "@connectrpc/connect";

import type { ServerRow } from "../data/repositories/syncRepo";
import type { DayLog } from "../domain/types";
import { ApiKeyScope } from "../gen/plusultra/v1/apikey_pb";
import { Unit } from "../gen/plusultra/v1/common_pb";
import {
  ProgramKind,
  ProgramMode,
  ProgramStatus,
  ValidateProgramResponseSchema,
} from "../gen/plusultra/v1/program_pb";
import type { SyncRows } from "../gen/plusultra/v1/sync_pb";
import { createClients, type Api } from "./clients";
import { fromProto, toProto, type ServerRows } from "./convert";
import { addDays } from "./fixtures/dates";
import { createMockTransport, type MockTransportOptions } from "./mockTransport";

const today = "2026-11-11";
const earlier = "2098-01-01T00:00:00.000Z";
const later = "2099-01-01T00:00:00.000Z";
const none = { dayLogs: [], workoutRecords: [], mealLogs: [], checkins: [] };
const datesFrom = (from: string, n: number) =>
  Array.from({ length: n }, (_, i) => addDays(from, i));
const history = { from: addDays(today, -30), to: today };

function setup(options: MockTransportOptions = {}) {
  const mock = createMockTransport({ today, ...options });
  return { mock, api: createClients(mock.transport) };
}

const rowsOf = (rows?: SyncRows) => fromProto.syncRows(rows, "user-1");
const push = (api: Api, rows: Partial<ServerRows>) =>
  api.sync.push({ deviceId: "phone-2", rows: toProto.syncRows(rows) });

const newLog = (over: Partial<ServerRow<DayLog>> = {}): ServerRow<DayLog> => ({
  id: "day-new",
  userId: "user-1",
  date: "2026-11-12",
  status: "done",
  notes: "felt good",
  version: 0,
  deviceId: "phone-2",
  updatedAt: "2026-11-12T10:00:00.000Z",
  ...over,
});

async function rejection(call: Promise<unknown>) {
  const error = await call.then(
    () => undefined,
    (e: unknown) => e,
  );
  expect(error).toBeInstanceOf(ConnectError);
  return error as ConnectError;
}

describe("mock sync", () => {
  it("pulls a pushed row back with the next server version", async () => {
    const { api } = setup();
    const { cursor } = await api.sync.pull({});

    const pushed = await push(api, { dayLogs: [newLog()] });
    expect(pushed.appliedIds).toEqual(["day-new"]);
    expect(rowsOf(pushed.conflicts)).toEqual(none);
    const first = await api.sync.pull({ cursor });
    expect(rowsOf(first.rows)).toEqual({ ...none, dayLogs: [newLog({ version: 1 })] });

    const edit = newLog({ version: 1, notes: "edited", updatedAt: later });
    await push(api, { dayLogs: [edit] });
    const second = await api.sync.pull({ cursor: first.cursor });
    expect(rowsOf(second.rows).dayLogs).toEqual([{ ...edit, version: 2 }]);
  });

  it("returns the server row as a conflict when the pushed version is stale and the write older", async () => {
    const { api } = setup();
    const seen = rowsOf((await api.sync.pull({})).rows).dayLogs.find(
      (r) => r.date === "2026-11-10",
    )!;
    const phone = { ...seen, notes: "phone", updatedAt: later };
    await push(api, { dayLogs: [phone] });

    const result = await push(api, { dayLogs: [{ ...seen, notes: "tablet", updatedAt: earlier }] });
    expect(result.appliedIds).toEqual([]);
    expect(rowsOf(result.conflicts).dayLogs).toEqual([{ ...phone, version: seen.version + 1 }]);
  });

  it("applies a stale version whose write is newer, as last write wins", async () => {
    const { api } = setup();
    const { rows, cursor } = await api.sync.pull({});
    const seen = rowsOf(rows).dayLogs.find((r) => r.date === "2026-11-10")!;
    await push(api, { dayLogs: [{ ...seen, notes: "phone", updatedAt: earlier }] });

    const tablet = { ...seen, notes: "tablet", updatedAt: later };
    const result = await push(api, { dayLogs: [tablet] });
    expect(result.appliedIds).toEqual([seen.id]);
    const pulled = rowsOf((await api.sync.pull({ cursor })).rows).dayLogs;
    expect(pulled).toEqual([{ ...tablet, version: seen.version + 2 }]);
  });

  it("returns the server row as a conflict for a second row on a one-per-date table", async () => {
    const { api } = setup();
    const server = rowsOf((await api.sync.pull({})).rows).dayLogs.find(
      (r) => r.date === "2026-11-10",
    );
    const result = await push(api, { dayLogs: [newLog({ date: "2026-11-10", updatedAt: later })] });
    expect(result.appliedIds).toEqual([]);
    expect(rowsOf(result.conflicts)).toEqual({ ...none, dayLogs: [server] });
  });

  it("finalizes a pushed workout record that asks for it", async () => {
    const { api } = setup();
    const { rows, cursor } = await api.sync.pull({});
    const open = rowsOf(rows).workoutRecords.find((r) => !r.finalized)!;
    expect(open.date).toBe(today);

    await push(api, { workoutRecords: [{ ...open, finalizeRequested: true, updatedAt: later }] });
    const changed = rowsOf((await api.sync.pull({ cursor })).rows).workoutRecords;
    expect(changed).toEqual([
      expect.objectContaining({
        id: open.id,
        finalized: true,
        finalizeRequested: false,
        version: open.version + 1,
      }),
    ]);
  });

  it("pulls the next 28 days of plans and the active programs", async () => {
    const { api } = setup();
    const { resolvedDays, resolvedMealDays, programs } = await api.sync.pull({});
    expect(resolvedDays.map((d) => d.date)).toEqual(datesFrom(today, 28));
    // The meal program's sixth and last week ends on 2026-11-22.
    expect(resolvedMealDays.map((d) => d.date)).toEqual(datesFrom(today, 12));
    expect(programs.map((p) => [p.kind, p.status, p.script])).toEqual([
      [ProgramKind.WORKOUT, ProgramStatus.ACTIVE, ""],
      [ProgramKind.MEAL, ProgramStatus.ACTIVE, ""],
    ]);
  });

  it("pages a pull by limit until has_more is false, never sending photo bytes", async () => {
    const { api } = setup();
    const all = rowsOf((await api.sync.pull({})).rows);
    const seen = new Set<string>();
    let cursor = "";
    for (let more = true, pages = 0; more; pages++) {
      expect(pages).toBeLessThan(10);
      const page = await api.sync.pull({ cursor, limit: 10 });
      const rows = rowsOf(page.rows);
      for (const table of Object.values(rows)) {
        expect(table.length).toBeLessThanOrEqual(10);
        for (const row of table) seen.add(row.id);
      }
      expect(page.rows!.checkins.every((c) => c.photo.length === 0)).toBe(true);
      ({ cursor, hasMore: more } = page);
    }
    expect(seen).toEqual(
      new Set(
        Object.values(all)
          .flat()
          .map((r) => r.id),
      ),
    );
  });
});

describe("mock failures and latency", () => {
  it("fails the next call of a method with the injected error, once", async () => {
    const { api, mock } = setup();
    mock.failNext("sync.push", new ConnectError("server unreachable", Code.Unavailable));
    const error = await rejection(api.sync.push({ deviceId: "phone" }));
    expect(error.code).toBe(Code.Unavailable);
    expect(error.rawMessage).toBe("server unreachable");
    await expect(api.sync.push({ deviceId: "phone" })).resolves.toMatchObject({ appliedIds: [] });
  });

  it("raises its own errors with Connect codes", async () => {
    const { api } = setup();
    expect((await rejection(api.workouts.getWorkoutRecord({ id: "missing" }))).code).toBe(
      Code.NotFound,
    );
  });

  it("answers after the configured latency", async () => {
    jest.useFakeTimers();
    try {
      const { api } = setup({ latencyMs: 300 });
      let answered = false;
      void api.auth.getMe({}).then(() => (answered = true));
      await jest.advanceTimersByTimeAsync(299);
      expect(answered).toBe(false);
      await jest.advanceTimersByTimeAsync(1);
      expect(answered).toBe(true);
    } finally {
      jest.useRealTimers();
    }
  });
});

describe("mock programs", () => {
  it("reports an error on each line that says error", async () => {
    const { api } = setup();
    const script = 'program("X", weeks=1)\nday(1, 1, "A")\nex("Squat", sets=error)\n';
    const result = await api.programs.validateProgram({ kind: ProgramKind.WORKOUT, script });
    expect(result.errors.map(({ line, col }) => ({ line, col }))).toEqual([{ line: 3, col: 18 }]);
    expect(result.structure).toBeUndefined();
  });

  it("rejects creating an invalid script with its line errors as details", async () => {
    const { api } = setup();
    const call = api.programs.createProgram({
      kind: ProgramKind.WORKOUT,
      script: 'program("X", weeks=1)\nerror\n',
    });
    const error = await rejection(call);
    expect(error.code).toBe(Code.InvalidArgument);
    expect(error.findDetails(ValidateProgramResponseSchema)[0].errors[0].line).toBe(2);
  });

  it("validates the fixture scripts", async () => {
    const { api } = setup();
    for (const { id, kind } of (await api.programs.listPrograms({})).programs) {
      const { program } = await api.programs.getProgram({ id });
      const result = await api.programs.validateProgram({ kind, script: program!.script });
      expect(result.errors).toEqual([]);
      expect(result.structure?.weeks).toBeGreaterThan(0);
    }
  });

  it("refuses a second active workout program", async () => {
    const { api } = setup();
    const { programs } = await api.programs.listPrograms({ status: ProgramStatus.FINISHED });
    const error = await rejection(api.programs.activateProgram({ id: programs[0].id }));
    expect(error.code).toBe(Code.FailedPrecondition);
  });

  it("pages program lists", async () => {
    const { api } = setup();
    const first = await api.programs.listPrograms({ limit: 2 });
    expect(first).toMatchObject({ hasMore: true, nextCursor: "2" });
    expect(first.programs).toHaveLength(2);
    const second = await api.programs.listPrograms({ limit: 2, cursor: first.nextCursor });
    expect(second).toMatchObject({ hasMore: false, nextCursor: "" });
    expect(second.programs).toHaveLength(1);
  });
});

describe("mock calendar", () => {
  const dates = { from: "2026-11-12", to: "2026-11-14" };

  it("plans future dates for the anchored program only", async () => {
    const { api } = setup();
    const anchored = await api.calendar.getCalendar({ dates });
    expect(anchored.days.map((d) => [d.date, d.kind, d.workoutWeek, d.workoutDay])).toEqual([
      ["2026-11-12", "conditioning", 3, 4],
      ["2026-11-13", "training", 3, 5],
      ["2026-11-14", "training", 3, 6],
    ]);

    const { programs } = await api.programs.listPrograms({ kind: ProgramKind.WORKOUT });
    const active = programs.find((p) => p.status === ProgramStatus.ACTIVE)!;
    const gzclp = programs.find((p) => p.status === ProgramStatus.FINISHED)!;
    await api.programs.finishProgram({ id: active.id });
    const { program } = await api.programs.activateProgram({ id: gzclp.id });
    expect(program).toMatchObject({
      mode: ProgramMode.FLOATING,
      cursor: { nextWeek: 1, nextDay: 1 },
    });

    const after = await api.calendar.getCalendar({ dates });
    expect(after.days.map((d) => [d.kind, d.workoutWeek])).toEqual([
      ["rest", 0],
      ["rest", 0],
      ["rest", 0],
    ]);
    const { day } = await api.calendar.getDay({ date: today });
    expect(day?.plan).toMatchObject({ programId: gzclp.id, week: 1, day: 1, name: "A1" });
  });

  it("quick fills a day with every planned set completed as planned", async () => {
    const { api } = setup();
    const { day } = await api.calendar.quickFillDay({
      date: "2026-11-12",
      workout: true,
      meals: true,
    });
    const sets = day!.workout!.entries.flatMap((e) => e.sets);
    expect(sets.length).toBeGreaterThan(0);
    for (const set of sets) {
      expect(set).toMatchObject({
        completed: true,
        reps: set.planned!.reps,
        weight: set.planned!.weight,
      });
    }
    expect(day!.workout).toMatchObject({
      finalized: false,
      dayName: "Thu conditioning",
      kind: "conditioning",
    });
    expect(fromProto.dayStatus(day!.log!.status)).toBe("done");
    const items = day!.meal!.meals.flatMap((m) => m.items);
    expect(items.length).toBeGreaterThan(0);
    expect(items.every((i) => i.planned)).toBe(true);
    expect(day!.meal!.totals!.kcal).toBeGreaterThan(0);
  });

  it("finishes today's started workout and finalizes it on request", async () => {
    const { api } = setup();
    const before = (await api.calendar.getDay({ date: today })).day!.workout!;
    expect(before.entries.flatMap((e) => e.sets).some((s) => !s.completed)).toBe(true);

    const { day } = await api.calendar.quickFillDay({ date: today, workout: true, finalize: true });
    expect(day!.workout).toMatchObject({ id: before.id, finalized: true });
    expect(day!.workout!.entries.flatMap((e) => e.sets).every((s) => s.completed)).toBe(true);
  });

  it("refuses quick fill when nothing is planned", async () => {
    const { api } = setup();
    const workout = await rejection(
      api.calendar.quickFillDay({ date: "2026-12-16", workout: true }),
    );
    expect(workout.code).toBe(Code.FailedPrecondition);
    const meals = await rejection(api.calendar.quickFillDay({ date: "2026-11-30", meals: true }));
    expect(meals.code).toBe(Code.FailedPrecondition);
  });
});

describe("mock workouts", () => {
  it("starts an ad hoc workout outside any program and finalizes it without progression", async () => {
    const { api } = setup();
    const before = (await api.programs.listPrograms({ status: ProgramStatus.ACTIVE })).programs;
    const pushUps = { reps: 20, minReps: 20, weight: { value: 0, unit: Unit.KG } };
    const { record } = await api.workouts.createWorkoutRecord({
      id: "w-adhoc",
      date: today,
      source: {
        case: "adHoc",
        value: {
          dayName: "Hotel gym",
          entries: [
            {
              name: "Push Up",
              slug: "push-up-bodyweight",
              sets: [{ planned: pushUps }, { planned: pushUps }],
            },
          ],
        },
      },
    });
    expect(record).toMatchObject({
      id: "w-adhoc",
      programId: "",
      week: 0,
      dayName: "Hotel gym",
      finalized: false,
    });
    expect(record!.entries[0]).toMatchObject({ name: "Push Up", slug: "push-up-bodyweight" });
    expect(record!.entries[0].sets.map((s) => s.planned!.reps)).toEqual([20, 20]);

    const duplicate = api.workouts.createWorkoutRecord({
      id: "w-adhoc",
      date: today,
      source: { case: "adHoc", value: {} },
    });
    expect((await rejection(duplicate)).code).toBe(Code.AlreadyExists);

    const { record: done } = await api.workouts.finalizeWorkoutRecord({ id: "w-adhoc" });
    expect(done!.finalized).toBe(true);
    expect((await api.programs.listPrograms({ status: ProgramStatus.ACTIVE })).programs).toEqual(
      before,
    );
  });

  it("starts a workout from a chosen program day", async () => {
    const { api } = setup();
    const { record } = await api.workouts.createWorkoutRecord({
      id: "w-chosen",
      date: today,
      source: { case: "programDay", value: { programId: "program-gzclp", week: 2, day: 3 } },
    });
    expect(record).toMatchObject({ programId: "program-gzclp", week: 2, day: 3, dayName: "A2" });
    expect(record!.entries.every((e) => e.sets.every((s) => !s.completed))).toBe(true);
  });

  it("pages workout records", async () => {
    const { api } = setup();
    const first = await api.workouts.listWorkoutRecords({ dates: history, limit: 5 });
    expect(first.records).toHaveLength(5);
    expect(first.hasMore).toBe(true);
    let { nextCursor: cursor, hasMore } = first;
    let total = first.records.length;
    while (hasMore) {
      const next = await api.workouts.listWorkoutRecords({ dates: history, limit: 5, cursor });
      total += next.records.length;
      ({ nextCursor: cursor, hasMore } = next);
    }
    expect(total).toBe(21);
  });
});

describe("mock check-in photos", () => {
  const png = new Uint8Array([0x89, 0x50, 0x4e, 0x47]);
  const dates = { from: today, to: today };
  const photoOf = async (api: Api) =>
    (await api.checkins.listCheckins({ dates, includePhotos: true })).checkins[0];

  it("keeps a photo on an update without one, clears it on request, and never syncs it", async () => {
    const { api } = setup();
    const checkin = { id: "c-today", date: today, weight: { value: 82.1, unit: Unit.KG } };
    await api.checkins.createCheckin({
      checkin: { ...checkin, photo: png, photoContentType: "image/png" },
    });
    expect(await photoOf(api)).toMatchObject({ photo: png, photoContentType: "image/png" });
    expect((await api.checkins.listCheckins({ dates })).checkins[0].photo).toHaveLength(0);
    expect((await api.sync.pull({})).rows!.checkins.every((c) => c.photo.length === 0)).toBe(true);

    const { checkin: kept } = await api.checkins.updateCheckin({
      checkin: { ...checkin, weight: { value: 82, unit: Unit.KG } },
    });
    expect(kept!.photoId).not.toBe("");
    expect(await photoOf(api)).toMatchObject({ photo: png, weight: { value: 82 } });

    const { checkin: cleared } = await api.checkins.updateCheckin({ checkin, clearPhoto: true });
    expect(cleared!.photoId).toBe("");
    expect((await photoOf(api)).photo).toHaveLength(0);
  });

  it("replaces the values of a date's check-in and keeps its id and photo", async () => {
    const { api } = setup();
    await api.checkins.createCheckin({
      checkin: { id: "c-1", date: today, photo: png, photoContentType: "image/png" },
    });
    const { checkin } = await api.checkins.createCheckin({
      checkin: { id: "c-2", date: today, bodyFat: 18 },
    });
    expect(checkin).toMatchObject({ id: "c-1", bodyFat: 18 });
    expect((await photoOf(api)).photo).toEqual(png);
  });
});

it("gives created rows ids no fixture uses", async () => {
  const { api } = setup();
  const { apiKey } = await api.apiKeys.createApiKey({
    name: "n",
    scopes: [ApiKeyScope.READ],
    expiresAt: timestampFromDate(new Date(Date.now() + 86_400_000)),
  });
  const { apiKeys } = await api.apiKeys.listApiKeys({});
  expect(apiKeys.filter((k) => k.id === apiKey!.id)).toHaveLength(1);
});
