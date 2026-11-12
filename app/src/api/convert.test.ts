import { create } from "@bufbuild/protobuf";
import { timestampFromDate } from "@bufbuild/protobuf/wkt";

import { DayStatus } from "../gen/plusultra/v1/calendar_pb";
import { ResolvedDaySchema, Unit } from "../gen/plusultra/v1/common_pb";
import {
  ProgramKind,
  ProgramMode,
  ProgramSchema,
  ProgramStatus,
} from "../gen/plusultra/v1/program_pb";
import { MealLogSchema, ResolvedMealDaySchema } from "../gen/plusultra/v1/meal_pb";
import { SyncRowsSchema } from "../gen/plusultra/v1/sync_pb";
import { fromProto, toProto } from "./convert";

const at = (iso: string) => timestampFromDate(new Date(iso));
const kg = (value: number) => ({ value, unit: Unit.KG });
const macros = (kcal: number) => ({ kcal, protein: 10, carbs: 20, fat: 5, fiber: 2, waterMl: 0 });
const set = {
  reps: 5,
  minReps: 3,
  weight: kg(100),
  rpe: 8,
  rest: 180,
  work: 0,
  amrap: true,
  ask: false,
  logRpe: true,
  auto: false,
  countUp: false,
  warmup: false,
  label: "Work",
};
const meta = { version: 4, deviceId: "phone", updatedAt: at("2026-11-11T09:00:00.000Z") };

const rows = create(SyncRowsSchema, {
  dayLogs: [
    {
      id: "d1",
      date: "2026-11-10",
      status: DayStatus.PARTIAL,
      notes: "tired",
      readiness: { sleep: 2, stress: 4 },
      ...meta,
    },
    {
      id: "d2",
      date: "2026-11-09",
      status: DayStatus.REST,
      ...meta,
      deletedAt: at("2026-11-11T10:00:00.000Z"),
    },
  ],
  workoutRecords: [
    {
      id: "w1",
      date: "2026-11-11",
      programId: "p1",
      week: 3,
      day: 2,
      dayName: "Upper A",
      dayDesc: "Bench day",
      kind: "training",
      entries: [
        {
          name: "Bench Press",
          slug: "bench-press-barbell",
          equipment: "barbell",
          label: "t1",
          alternates: ["Bench Press", "Bench Press, Dumbbell"],
          current: 1,
          desc: "Pause on the chest",
          superset: "a",
          variation: 2,
          promptValues: { rm: 100 },
          sets: [
            {
              planned: { ...set, warmup: true, weight: kg(60) },
              completed: true,
              reps: 5,
              weight: kg(60),
            },
            {
              planned: set,
              completed: true,
              reps: 6,
              weight: kg(100),
              rpe: 8.5,
              workTime: 40,
              repsLeft: 1,
            },
          ],
        },
      ],
      startedAt: at("2026-11-11T07:00:00.000Z"),
      finishedAt: at("2026-11-11T08:00:00.000Z"),
      finalized: true,
      ...meta,
    },
    {
      id: "w2",
      date: "2026-11-11",
      dayName: "Hotel gym",
      entries: [
        { name: "Push Up", sets: [{ planned: set, reps: 0, weight: { value: 0, unit: Unit.LB } }] },
      ],
      ...meta,
    },
  ],
  mealLogs: [
    {
      id: "m1",
      date: "2026-11-11",
      programId: "p2",
      target: macros(2600),
      meals: [
        {
          name: "Breakfast",
          time: "07:30",
          desc: "",
          items: [
            {
              food: {
                name: "Egg",
                ref: "usda:1",
                per100g: macros(143),
                perUnit: macros(72),
                unit: "egg",
              },
              qty: 3,
              unit: "egg",
              note: "boiled",
              macros: macros(216),
              planned: true,
              edited: true,
            },
            { food: { name: "Chai" }, qty: 200, unit: "g", macros: macros(80) },
          ],
        },
      ],
      totals: macros(296),
      ...meta,
    },
  ],
  checkins: [
    {
      id: "c1",
      date: "2026-11-11",
      weight: kg(82.4),
      bodyFat: 19.5,
      measurements: { waist: 86 },
      photoId: "ph1",
      ...meta,
    },
    { id: "c2", date: "2026-11-10", measurements: {}, ...meta },
  ],
});

describe("sync row conversion", () => {
  it("round-trips every synced row without loss", () => {
    const store = fromProto.syncRows(rows, "u1");
    expect(JSON.stringify(store)).not.toContain("$typeName");
    expect(toProto.syncRows(store)).toEqual(rows);
  });

  it("gives store rows with dates, strings and unset fields in the domain's form", () => {
    const { dayLogs, workoutRecords, checkins } = fromProto.syncRows(rows, "u1");
    expect(dayLogs[0]).toMatchObject({
      userId: "u1",
      status: "partial",
      readiness: { sleep: 2, stress: 4 },
      updatedAt: "2026-11-11T09:00:00.000Z",
    });
    expect(dayLogs[1].deletedAt).toBe("2026-11-11T10:00:00.000Z");
    expect(workoutRecords[0]).toMatchObject({
      finalized: true,
      finalizeRequested: false,
      kind: "training",
    });
    expect(workoutRecords[0].entries[0].sets[0].planned.warmup).toBe(true);
    expect(workoutRecords[1]).toMatchObject({ programId: undefined, kind: undefined, week: 0 });
    expect(workoutRecords[1].entries[0].sets[0].weight).toEqual({ value: 0, unit: "lb" });
    expect(checkins[1].weight).toBeUndefined();
  });

  it("asks the server to finalize a record finalized offline", () => {
    const [record] = fromProto.syncRows(rows, "u1").workoutRecords;
    const pushed = toProto.workoutRecord({ ...record, finalized: false, finalizeRequested: true });
    expect(pushed.finalized).toBe(true);
  });

  it("reads the all-zero target the server sends for an ad hoc meal log as none (P2-R42)", () => {
    const zero = { ...macros(0), protein: 0, carbs: 0, fat: 0, fiber: 0 };
    const adHoc = create(MealLogSchema, {
      id: "m",
      date: "2026-11-11",
      target: zero,
      totals: zero,
    });
    expect(fromProto.mealLog(adHoc, "u1").target).toBeUndefined();
    const planned = create(MealLogSchema, { id: "m", date: "2026-11-11", target: macros(2200) });
    expect(fromProto.mealLog(planned, "u1").target).toEqual(macros(2200));
  });
});

describe("plan conversion", () => {
  it("turns resolved days into store cache rows and back", () => {
    const anchored = create(ResolvedDaySchema, {
      programId: "p1",
      date: "2026-11-11",
      week: 3,
      day: 3,
      name: "Lower",
      desc: "",
      kind: "training",
      exercises: [
        { name: "Squat", slug: "squat-barbell", alternates: ["Squat"], current: 1, sets: [set] },
        { name: "Lunge", slug: "lunge", alternates: ["Lunge"], current: 1, unilateral: true },
      ],
    });
    const floating = create(ResolvedDaySchema, { programId: "p2", week: 1, day: 1, kind: "rest" });
    const cached = [anchored, floating].map(fromProto.resolvedDay);
    expect(cached[0]).toMatchObject({ programId: "p1", week: 3, day: 3, date: "2026-11-11" });
    expect(cached[1].date).toBeUndefined();
    expect(cached[0].payload.exercises.map((e) => e.unilateral)).toEqual([undefined, true]);
    expect(cached.map((c) => toProto.resolvedDay(c.payload, c.programId))).toEqual([
      anchored,
      floating,
    ]);
  });

  it("turns meal days into store cache rows and back, totalling their items", () => {
    const item = {
      food: { name: "Rice", ref: "usda:2" },
      qty: 100,
      unit: "g",
      macros: macros(365),
      planned: true,
    };
    const day = create(ResolvedMealDaySchema, {
      programId: "p3",
      date: "2026-11-11",
      week: 5,
      day: 0,
      kind: "training",
      name: "Training day",
      target: macros(2600),
      meals: [{ name: "Lunch", items: [item, item] }],
      totals: { kcal: 730, protein: 20, carbs: 40, fat: 10, fiber: 4, waterMl: 0 },
    });
    const cached = fromProto.mealDay(day);
    expect(cached).toMatchObject({ programId: "p3", date: "2026-11-11", week: 5, day: 0 });
    expect(toProto.mealDay(cached.payload, cached.programId, cached.date)).toEqual(day);
  });

  it("summarises a program for the store", () => {
    const program = create(ProgramSchema, {
      id: "p1",
      kind: ProgramKind.WORKOUT,
      name: "GZCLP",
      revision: 2,
      status: ProgramStatus.ACTIVE,
      mode: ProgramMode.FLOATING,
      cursor: { nextWeek: 1, nextDay: 3 },
      structure: { weeks: 4, loop: true },
      updatedAt: at("2026-11-10T08:00:00.000Z"),
    });
    expect(fromProto.program(program)).toEqual({
      id: "p1",
      kind: "workout",
      name: "GZCLP",
      revision: 2,
      status: "active",
      mode: "floating",
      nextWeek: 1,
      nextDay: 3,
      weeks: 4,
      updatedAt: "2026-11-10T08:00:00.000Z",
    });
    const draft = fromProto.program(
      create(ProgramSchema, { id: "p2", status: ProgramStatus.DRAFT }),
    );
    expect(draft).toMatchObject({ status: "draft", mode: undefined, startDate: undefined });
  });
});
