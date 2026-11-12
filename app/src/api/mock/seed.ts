import { create } from "@bufbuild/protobuf";

import type { ResolvedDay } from "../../domain/types";
import {
  ProgramKind,
  ProgramMode,
  ProgramRevisionSchema,
  ProgramSchema,
  ProgramStatus,
  type Program,
} from "../../gen/plusultra/v1/program_pb";
import { addDays, mondayOf, stampAt } from "../fixtures/dates";
import { equipment, exerciseData } from "../fixtures/equipment";
import { exercises } from "../fixtures/exercises";
import { foods } from "../fixtures/foods";
import { gzclpDays, gzclpScript } from "../fixtures/gzclp";
import { history } from "../fixtures/history";
import { mealDays, mealScript } from "../fixtures/meals";
import { strength16Days } from "../fixtures/strength16";
import { strength16Script } from "../fixtures/strength16Script";
import { apiKeys, credentials, invites, user } from "../fixtures/user";
import { syncTables } from "../../data/store";
import { put, type MockDb, type Row } from "./db";
import { applyExerciseData, mealPlan } from "./plans";
import { loops, nameOf } from "./scripts";

type Day = Pick<ResolvedDay, "week" | "day" | "name" | "kind">;

type Over = Partial<Pick<Program, "mode" | "startDate" | "endDate" | "status">>;

function fixture(
  id: string,
  kind: ProgramKind,
  script: string,
  days: Day[],
  updated: string,
  over: Over,
) {
  const weeks = Math.max(...days.map((d) => d.week));
  const updatedAt = stampAt(updated, "06:00");
  return create(ProgramSchema, {
    id,
    kind,
    name: nameOf(script),
    script,
    revision: 1,
    status: ProgramStatus.ACTIVE,
    structure: {
      weeks,
      loop: loops(script),
      days: days.map(({ week, day, name, kind }) => ({ week, day, name, kind })),
    },
    createdAt: updatedAt,
    updatedAt,
    ...over,
  });
}

// The 16-week program started on the Monday two weeks before today, after two weeks of GZCLP.
// The meal program started on the Monday on or before the first day of history.
export function seed(today: string): MockDb {
  const start = addDays(mondayOf(today), -14);
  const mealStart = mondayOf(addDays(today, -30));
  const strength = strength16Days();
  const gzclp = gzclpDays();
  const meals = mealDays();
  const anchored = (from: string, weeks: number) => ({
    mode: ProgramMode.ANCHORED,
    startDate: from,
    endDate: addDays(from, weeks * 7 - 1),
  });
  const programs = [
    fixture(
      "program-strength16",
      ProgramKind.WORKOUT,
      strength16Script,
      strength,
      start,
      anchored(start, 8),
    ),
    fixture("program-gzclp", ProgramKind.WORKOUT, gzclpScript, gzclp, addDays(start, -1), {
      status: ProgramStatus.FINISHED,
    }),
    fixture(
      "program-meals",
      ProgramKind.MEAL,
      mealScript,
      meals,
      mealStart,
      anchored(mealStart, 6),
    ),
  ];

  const db: MockDb = {
    today,
    seq: 0,
    // Above every fixture id, so a created row never shares a key with one.
    ids: 1000,
    user: user(today),
    password: credentials.password,
    invites: invites(today),
    apiKeys: apiKeys(today),
    programs,
    revisions: Object.fromEntries(
      programs.map(({ id, script, structure, createdAt }) => [
        id,
        [
          create(ProgramRevisionSchema, {
            revision: 1,
            script,
            createdBy: "user",
            createdAt,
            structure,
          }),
        ],
      ]),
    ),
    days: {
      "program-strength16": strength.map((d) => ({
        ...d,
        programId: "program-strength16",
        date: addDays(start, (d.week - 1) * 7 + d.day - 1),
      })),
      "program-gzclp": gzclp.map((d) => ({ ...d, programId: "program-gzclp" })),
    },
    mealDays: { "program-meals": meals.map((d) => ({ ...d, programId: "program-meals" })) },
    rows: {
      dayLogs: new Map(),
      workoutRecords: new Map(),
      mealLogs: new Map(),
      checkins: new Map(),
    },
    photos: new Map(),
    exercises: [...exercises],
    exerciseData: exerciseData(today),
    equipment: equipment(),
    foods: [...foods],
  };
  applyExerciseData(db);

  const rows = history(today, db.user.id, {
    start,
    anchored: (date) => db.days["program-strength16"].find((d) => d.date === date),
    floating: db.days["program-gzclp"],
    meal: (date, kind) => mealPlan(db, date, kind),
  });
  for (const table of syncTables) {
    for (const row of rows[table]) put(db, table, row as Row<typeof table>);
  }
  return db;
}
