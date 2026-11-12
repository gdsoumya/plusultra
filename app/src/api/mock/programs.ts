import { create } from "@bufbuild/protobuf";
import { timestampMs } from "@bufbuild/protobuf/wkt";
import { Code, ConnectError, type ServiceImpl } from "@connectrpc/connect";

import {
  ProgramCursorSchema,
  ProgramKind,
  ProgramMode,
  ProgramRevisionSchema,
  ProgramSchema,
  ProgramService,
  ProgramStatus,
  ValidateProgramResponseSchema,
  type Program,
} from "../../gen/plusultra/v1/program_pb";
import { toProto } from "../convert";
import { addDays, mondayOf } from "../fixtures/dates";
import { newId, nowStamp, page, precondition, program, type MockDb } from "./db";
import { activeProgram } from "./plans";
import { nameOf, scriptErrors, structureOf } from "./scripts";

const listed = (p: Program) => ({ ...p, script: "" });

function checked(script: string) {
  const errors = scriptErrors(script);
  if (errors.length) {
    const detail = { desc: ValidateProgramResponseSchema, value: { errors } };
    throw new ConnectError("the script has errors", Code.InvalidArgument, undefined, [detail]);
  }
  return { name: nameOf(script), structure: structureOf(script) };
}

// shortcut: new scripts are not resolved, so one named like a fixture program reuses its days and
// any other has none; upgrade if the editor preview needs other scripts without a server.
const daysNamed = (db: MockDb, name: string) =>
  db.programs.find((p) => p.name === name && (db.days[p.id]?.length || db.mealDays[p.id]?.length))
    ?.id;

export function createProgram(db: MockDb, kind: ProgramKind, script: string): Program {
  const { name, structure } = checked(script);
  const created = nowStamp();
  const p = create(ProgramSchema, {
    id: newId(db, "program"),
    kind,
    name,
    script,
    revision: 1,
    status: ProgramStatus.DRAFT,
    structure,
    createdAt: created,
    updatedAt: created,
  });
  db.programs.push(p);
  db.revisions[p.id] = [
    create(ProgramRevisionSchema, {
      revision: 1,
      script,
      createdBy: "user",
      createdAt: created,
      structure,
    }),
  ];
  const template = daysNamed(db, name);
  if (template) {
    db.days[p.id] = (db.days[template] ?? []).map(({ date: _, ...d }) => ({
      ...d,
      programId: p.id,
    }));
    db.mealDays[p.id] = (db.mealDays[template] ?? []).map((d) => ({ ...d, programId: p.id }));
  }
  return p;
}

export function programs(db: MockDb): ServiceImpl<typeof ProgramService> {
  return {
    listPrograms({ kind, status, cursor, limit }) {
      const all = db.programs
        .filter((p) => (!kind || p.kind === kind) && (!status || p.status === status))
        .sort((a, b) => timestampMs(b.updatedAt!) - timestampMs(a.updatedAt!))
        .map(listed);
      const { items, ...more } = page(all, cursor, limit);
      return { programs: items, ...more };
    },
    getProgram: ({ id }) => ({ program: program(db, id) }),
    createProgram: ({ kind, script }) => ({ program: createProgram(db, kind, script) }),
    updateProgram({ id, script }) {
      const p = program(db, id);
      const { name, structure } = checked(script);
      Object.assign(p, {
        name,
        structure,
        script,
        revision: p.revision + 1,
        updatedAt: nowStamp(),
      });
      db.revisions[id].unshift(
        create(ProgramRevisionSchema, {
          revision: p.revision,
          script,
          createdBy: "user",
          createdAt: p.updatedAt,
          structure,
        }),
      );
      return { program: p };
    },
    deleteProgram({ id }) {
      program(db, id);
      db.programs = db.programs.filter((p) => p.id !== id);
      delete db.days[id];
      delete db.mealDays[id];
      return {};
    },
    validateProgram({ script }) {
      const errors = scriptErrors(script);
      return { errors, structure: errors.length ? undefined : structureOf(script) };
    },
    simulateProgram({ script, weeks }) {
      const errors = scriptErrors(script);
      const template = errors.length ? undefined : daysNamed(db, nameOf(script));
      const days = (template ? (db.days[template] ?? []) : [])
        .filter((d) => d.week <= weeks)
        .map(({ date: _, ...d }) => toProto.resolvedDay(d, ""));
      const mealDays = (template ? (db.mealDays[template] ?? []) : [])
        .filter((d) => d.week <= weeks)
        .map((d) => toProto.mealDay(d, ""));
      return { days, mealDays, errors };
    },
    activateProgram({ id, startDate }) {
      const p = program(db, id);
      const other = activeProgram(db, p.kind);
      if (other && other.id !== id) {
        throw precondition(
          `finish "${other.name}" first: only one program of a kind can be active`,
        );
      }
      const monday = startDate && mondayOf(startDate);
      const days = db.days[id] ?? [];
      // Anchored days take dates from the start week's Monday; days before the start date are skipped.
      db.days[id] = days.map((d) => {
        const date = monday ? addDays(monday, (d.week - 1) * 7 + d.day - 1) : "";
        return { ...d, date: date && date >= startDate ? date : undefined };
      });
      const first = days[0];
      Object.assign(p, {
        status: ProgramStatus.ACTIVE,
        mode: startDate ? ProgramMode.ANCHORED : ProgramMode.FLOATING,
        startDate,
        endDate: monday ? addDays(monday, (p.structure?.weeks ?? 1) * 7 - 1) : "",
        cursor:
          !startDate && first
            ? create(ProgramCursorSchema, { nextWeek: first.week, nextDay: first.day })
            : undefined,
        updatedAt: nowStamp(),
      });
      return { program: p };
    },
    finishProgram({ id }) {
      const p = program(db, id);
      if (p.status !== ProgramStatus.ACTIVE) throw precondition(`"${p.name}" is not active`);
      Object.assign(p, {
        status: ProgramStatus.FINISHED,
        cursor: undefined,
        updatedAt: nowStamp(),
      });
      return { program: p };
    },
    listRevisions({ programId, includeScript, cursor, limit }) {
      program(db, programId);
      const all = db.revisions[programId].map((r) => (includeScript ? r : { ...r, script: "" }));
      const { items, ...more } = page(all, cursor, limit);
      return { revisions: items, ...more };
    },
    getResolvedDays({ programId, week }) {
      const p = program(db, programId);
      if (p.status !== ProgramStatus.ACTIVE) throw precondition(`"${p.name}" is not active`);
      const inWeek = <D extends { week: number }>(all: D[] = []) =>
        all.filter((d) => !week || d.week === week);
      return {
        days: inWeek(db.days[programId]).map((d) => toProto.resolvedDay(d, programId)),
        mealDays: inWeek(db.mealDays[programId]).map((d) => toProto.mealDay(d, programId)),
      };
    },
  };
}
