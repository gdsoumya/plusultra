import type { ServiceImpl } from "@connectrpc/connect";

import type { DayStatus } from "../../domain/types";
import { ExportFormat, ExportService, ImportKind } from "../../gen/plusultra/v1/export_pb";
import { ProgramKind } from "../../gen/plusultra/v1/program_pb";
import { datesIn } from "../fixtures/dates";
import { slugify } from "../fixtures/exercises";
import { live, newId, onDate, program, required, save, type MockDb } from "./db";
import { createProgram } from "./programs";

// shortcut: the full export is an empty zip (the 22-byte end-of-central-directory record); upgrade
// when the export screen opens the archive.
const emptyZip = () => new Uint8Array([0x50, 0x4b, 0x05, 0x06, ...Array(18).fill(0)]);

const statuses: DayStatus[] = ["planned", "done", "partial", "skipped", "rest"];

export function exportData(db: MockDb): ServiceImpl<typeof ExportService> {
  // shortcut: one line per date (status, workout, kcal, weight), not the server's log format;
  // upgrade when an import or export screen depends on that format.
  const line = (date: string) => {
    const log = onDate(db, "dayLogs", date);
    const workout = onDate(db, "workoutRecords", date);
    const meal = onDate(db, "mealLogs", date);
    const checkin = onDate(db, "checkins", date);
    if (!log && !workout && !meal && !checkin) return undefined;
    return [
      date,
      log?.status ?? "",
      workout ? `W${workout.week}D${workout.day} ${workout.dayName}`.trim() : "",
      meal ? `${Math.round(meal.totals.kcal)} kcal` : "",
      checkin?.weight ? `${checkin.weight.value} ${checkin.weight.unit}` : "",
    ];
  };

  return {
    exportProgram({ id }) {
      const p = program(db, id);
      return {
        script: `${p.script}\nstate({})\ncatalog_snapshot({})\n`,
        filename: `${slugify(p.name)}.star`,
      };
    },
    exportDays({ dates, format }) {
      const { from, to } = required(dates, "dates");
      const rows = [...datesIn(from, to)].map(line).filter((r) => r !== undefined);
      if (format === ExportFormat.JSON) {
        const inRange = <T extends { date: string }>(all: T[]) =>
          all.filter((r) => r.date >= from && r.date <= to);
        const content = {
          dayLogs: inRange(live(db, "dayLogs")),
          workoutRecords: inRange(live(db, "workoutRecords")),
          mealLogs: inRange(live(db, "mealLogs")),
          checkins: inRange(live(db, "checkins")),
        };
        return { content: JSON.stringify(content, null, 2), contentType: "application/json" };
      }
      if (format === ExportFormat.CSV) {
        const csv = [["date", "status", "workout", "meals", "weight"], ...rows].map((r) =>
          r.join(","),
        );
        return { content: csv.join("\n"), contentType: "text/csv" };
      }
      return { content: rows.map((r) => r.join(" | ")).join("\n"), contentType: "text/plain" };
    },
    exportAll: () => ({ zip: emptyZip(), filename: `plusultra-${db.today}.zip` }),
    import({ kind, content }) {
      if (kind === ImportKind.PROGRAM) {
        const programKind = content.includes("meal_program(")
          ? ProgramKind.MEAL
          : ProgramKind.WORKOUT;
        return { program: createProgram(db, programKind, content) };
      }
      const dates: string[] = [];
      for (const [, date, status] of content.matchAll(/^(\d{4}-\d{2}-\d{2}) \| (\w*)/gm)) {
        if (!statuses.includes(status as DayStatus)) continue;
        const log = onDate(db, "dayLogs", date);
        save(db, "dayLogs", {
          notes: "",
          ...log,
          id: log?.id ?? newId(db, "day"),
          date,
          status: status as DayStatus,
        });
        dates.push(date);
      }
      return { dates };
    },
  };
}
