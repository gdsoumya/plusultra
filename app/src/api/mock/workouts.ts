import type { ServiceImpl } from "@connectrpc/connect";

import { ProgramKind } from "../../gen/plusultra/v1/program_pb";
import { WorkoutService } from "../../gen/plusultra/v1/workout_pb";
import { fromProto, toProto } from "../convert";
import {
  exists,
  find,
  inDates,
  notFound,
  now,
  page,
  precondition,
  program,
  remove,
  required,
  save,
  type MockDb,
} from "./db";
import { activeProgram, dayAt, finalize, nextDay, startRecord } from "./plans";

export function workouts(db: MockDb): ServiceImpl<typeof WorkoutService> {
  const out = toProto.workoutRecord;

  return {
    listWorkoutRecords({ dates, programId, cursor, limit }) {
      const all = inDates(db, "workoutRecords", dates).filter(
        (r) => !programId || r.programId === programId,
      );
      const { items, ...more } = page(all, cursor, limit);
      return { records: items.map(out), ...more };
    },
    getWorkoutRecord: ({ id }) => ({ record: out(find(db, "workoutRecords", id)) }),
    createWorkoutRecord({ id, date, source }) {
      required(id, "id");
      if (db.rows.workoutRecords.has(id)) throw exists(`workout record ${id}`);
      if (source.case === "adHoc") {
        const { dayName, entries } = source.value;
        const record = {
          id,
          date,
          week: 0,
          day: 0,
          dayName,
          dayDesc: "",
          entries: entries.map(fromProto.entry),
          startedAt: now(),
          finalized: false,
          finalizeRequested: false,
        };
        return { record: out(save(db, "workoutRecords", record)) };
      }
      const chosen = source.case === "programDay" ? source.value : undefined;
      const p = chosen?.programId
        ? program(db, chosen.programId)
        : activeProgram(db, ProgramKind.WORKOUT);
      if (!p) throw precondition("there is no active workout program");
      const plan = chosen ? dayAt(db, p.id, chosen.week, chosen.day) : nextDay(db, p, date);
      if (!plan)
        throw notFound(
          chosen ? `week ${chosen.week} day ${chosen.day}` : `a planned day on ${date}`,
        );
      return { record: out(save(db, "workoutRecords", startRecord(id, plan, date))) };
    },
    updateWorkoutRecord({ record }) {
      const { id, entries, startedAt, finishedAt } = fromProto.workoutRecord(
        required(record, "record"),
        db.user.id,
      );
      const stored = find(db, "workoutRecords", id);
      // As on the server: a finalized record takes the edit and keeps its finish time.
      return {
        record: out(
          save(db, "workoutRecords", {
            ...stored,
            entries,
            startedAt,
            finishedAt: stored.finalized ? stored.finishedAt : finishedAt,
          }),
        ),
      };
    },
    deleteWorkoutRecord({ id }) {
      if (find(db, "workoutRecords", id).finalized) {
        throw precondition("a finalized workout cannot be deleted");
      }
      return remove(db, "workoutRecords", id);
    },
    finalizeWorkoutRecord({ id }) {
      const record = find(db, "workoutRecords", id);
      if (record.finalized) throw precondition("the workout is already finalized");
      return { record: out(save(db, "workoutRecords", finalize(db, record))), output: [] };
    },
  };
}
