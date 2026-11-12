import type { ServiceImpl } from "@connectrpc/connect";

import { syncTables, type SyncTable } from "../../data/store";
import { ProgramKind, ProgramMode, ProgramStatus } from "../../gen/plusultra/v1/program_pb";
import { SyncService } from "../../gen/plusultra/v1/sync_pb";
import { fromProto, toProto, type ServerRows } from "../convert";
import { addDays, datesIn } from "../fixtures/dates";
import { invalid, live, put, type MockDb, type Row } from "./db";
import { activeProgram, finalize, mealPlan } from "./plans";

const emptyRows = (): ServerRows => ({
  dayLogs: [],
  workoutRecords: [],
  mealLogs: [],
  checkins: [],
});

// Day logs, meal logs and check-ins are one per date.
const onePerDate = (table: SyncTable) => table !== "workoutRecords";

export function sync(db: MockDb): ServiceImpl<typeof SyncService> {
  function upcoming() {
    const p = activeProgram(db, ProgramKind.WORKOUT);
    const days = (p && db.days[p.id]) || [];
    if (p?.mode === ProgramMode.ANCHORED) {
      const last = addDays(db.today, 27);
      return days.filter((d) => d.date && d.date >= db.today && d.date <= last);
    }
    const next = days.findIndex(
      (d) => d.week === p?.cursor?.nextWeek && d.day === p?.cursor?.nextDay,
    );
    return next < 0 ? [] : days.slice(next, next + 28);
  }

  // A new planned workout takes the date's record of the same program day, as on the server.
  const plannedDay = (row: Row<"workoutRecords">) =>
    row.programId && !row.deletedAt
      ? live(db, "workoutRecords").find(
          (r) =>
            r.date === row.date &&
            r.programId === row.programId &&
            r.week === row.week &&
            r.day === row.day,
        )
      : undefined;

  // The server row a pushed row would replace: same id, or the same date on a one-per-date table.
  const current = <T extends SyncTable>(table: T, row: Row<T>) =>
    db.rows[table].get(row.id)?.row ??
    (onePerDate(table)
      ? live(db, table).find((r) => r.date === row.date)
      : (plannedDay(row as Row<"workoutRecords">) as Row<T> | undefined));

  // The server row and reason a push loses to, or nothing when it is applied.
  function apply<T extends SyncTable>(
    table: T,
    row: Row<T>,
    deviceId: string,
  ): [Row<T>, string] | undefined {
    const server = current(table, row);
    if (!server) return applyRow(table, row, deviceId);
    // As on the server: another row for the date is kept, and so is the server's change when the
    // pushing device had not seen it and it is the later one. An edited finalized record stays
    // finalized with its finish time.
    if (server.id !== row.id) return [server, "date_taken"];
    if (row.version < server.version && row.updatedAt < server.updatedAt)
      return [server, "outdated"];
    if (table === "workoutRecords" && !row.deletedAt) {
      const done = server as Row<"workoutRecords">;
      if (done.finalized) {
        const kept = { finalized: true, finalizeRequested: false, finishedAt: done.finishedAt };
        return applyRow(table, { ...row, ...kept } as Row<T>, deviceId, server.version);
      }
    }
    return applyRow(table, row, deviceId, server.version);
  }

  function applyRow<T extends SyncTable>(table: T, row: Row<T>, deviceId: string, version = 0) {
    put(db, table, {
      ...row,
      userId: db.user.id,
      version: version + 1,
      deviceId: row.deviceId || deviceId,
    });
    return undefined;
  }

  // A push that asks to finalize runs the progression once the record is stored.
  function settle(id: string) {
    const stored = db.rows.workoutRecords.get(id)!;
    if (stored.row.finalized || stored.row.finalizeRequested) stored.row = finalize(db, stored.row);
  }

  return {
    // Up to limit rows per table, oldest change first. When a table is cut short the cursor stops
    // at its last row, so other tables may send rows again on the next pull.
    pull({ cursor, limit }) {
      if (cursor && !/^\d+$/.test(cursor)) throw invalid(`bad cursor ${cursor}`);
      const since = Number(cursor || 0);
      // shortcut: a limit above 500 is not rejected as on the server; upgrade if a client could send one.
      const max = limit || 500;
      const rows = emptyRows();
      let next = db.seq;
      for (const table of syncTables) {
        const changed = [...db.rows[table].values()]
          .filter((s) => s.seq > since)
          .sort((a, b) => a.seq - b.seq);
        if (changed.length > max) next = Math.min(next, changed[max - 1].seq);
        (rows[table] as Row<typeof table>[]).push(...changed.slice(0, max).map((s) => s.row));
      }
      return {
        rows: toProto.syncRows(rows),
        resolvedDays: upcoming().map((d) => toProto.resolvedDay(d, d.programId)),
        resolvedMealDays: [...datesIn(db.today, addDays(db.today, 27))].flatMap((date) => {
          const day = mealPlan(db, date);
          return day ? [toProto.mealDay(day, day.programId, date)] : [];
        }),
        programs: db.programs
          .filter((p) => p.status === ProgramStatus.ACTIVE)
          .map((p) => ({ ...p, script: "" })),
        cursor: String(next),
        hasMore: next < db.seq,
      };
    },
    // shortcut: an empty device id and photo bytes in pushed check-ins are not rejected as on the
    // server (the bytes are dropped); upgrade if the sync engine needs those errors in tests.
    push({ deviceId, rows }) {
      const pushed = fromProto.syncRows(rows, db.user.id);
      const appliedIds: string[] = [];
      const conflicts = emptyRows();
      const reasons: Record<string, string> = {};
      for (const table of syncTables) {
        for (const row of pushed[table]) {
          const wasFinalized = db.rows.workoutRecords.get(row.id)?.row.finalized;
          const lost = apply(table, row as Row<typeof table>, deviceId);
          if (lost) {
            (conflicts[table] as Row<typeof table>[]).push(lost[0]);
            reasons[row.id] = lost[1];
            continue;
          }
          appliedIds.push(row.id);
          if (table === "workoutRecords" && !wasFinalized) settle(row.id);
        }
      }
      return { appliedIds, conflicts: toProto.syncRows(conflicts), reasons };
    },
  };
}
