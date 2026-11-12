import { sets } from "../api/fixtures/build";
import { bestSet, doneSummary, findLastSession, setDelta, setLine } from "./lastSession";
import type { ExerciseEntry, SetEntry, WorkoutRecord } from "./types";

const done = (reps: number, kg: number, over: Partial<SetEntry> = {}): SetEntry => ({
  planned: sets(1, reps, kg)[0],
  completed: true,
  reps,
  weight: { value: kg, unit: "kg" },
  rpe: 0,
  ...over,
});

const entry = (
  name: string,
  all: SetEntry[],
  over: Partial<ExerciseEntry> = {},
): ExerciseEntry => ({
  fromProgram: true,
  name,
  slug: name.toLowerCase().replace(/ /g, "-"),
  equipment: "",
  label: "",
  alternates: [name],
  current: 1,
  desc: "",
  superset: "",
  variation: 0,
  promptValues: {},
  sets: all,
  ...over,
});

const record = (id: string, date: string, entries: ExerciseEntry[]): WorkoutRecord => ({
  id,
  userId: "u1",
  updatedAt: `${date}T20:00:00.000Z`,
  version: 1,
  deviceId: "d1",
  dirty: false,
  date,
  week: 1,
  day: 1,
  dayName: "Lower",
  dayDesc: "",
  entries,
  finalized: true,
  finalizeRequested: true,
});

describe("findLastSession", () => {
  const squat = entry(
    "Squat",
    sets(3, 5, 82.5).map((p) => done(0, 82.5, { planned: p })),
  );
  const current = { id: "today", date: "2026-11-11" };

  it("takes the latest earlier record with completed work sets of the same exercise", () => {
    const older = record("r1", "2026-11-04", [entry("Squat", [done(5, 77.5), done(5, 77.5)])]);
    const warmup = done(5, 40, { planned: sets(1, 5, 40, { warmup: true })[0] });
    const latest = record("r2", "2026-11-09", [entry("Squat", [warmup, done(5, 80), done(4, 80)])]);
    const pending = record("r3", "2026-11-10", [
      entry("Squat", [done(5, 85, { completed: false })]),
    ]);
    const other = record("r4", "2026-11-10", [entry("Bench Press", [done(5, 60)])]);
    const self = record("today", "2026-11-11", [squat]);
    const future = record("r5", "2026-11-12", [entry("Squat", [done(5, 90)])]);
    const last = findLastSession([older, latest, pending, other, self, future], squat, current);
    expect(last).toMatchObject({ date: "2026-11-09", recordId: "r2" });
    expect(last?.sets).toHaveLength(2);
    expect(last?.best).toEqual({ weight: { value: 80, unit: "kg" }, reps: 5 });
    expect(last?.e1rm).toEqual({ value: 93.3, unit: "kg" });
  });

  it("matches by name when the slug is empty and by an alternate's name", () => {
    const custom = entry("My Row", [done(8, 20)], { slug: "" });
    const past = record("r1", "2026-11-01", [entry("my row", [done(8, 20)], { slug: "" })]);
    expect(findLastSession([past], custom, current)?.recordId).toBe("r1");
    const alt = entry("Hack Squat", [done(8, 20)], { alternates: ["Hack Squat", "Leg Press"] });
    const legPress = record("r2", "2026-11-02", [entry("Leg Press", [done(10, 120)])]);
    expect(findLastSession([legPress], alt, current)?.recordId).toBe("r2");
    expect(findLastSession([legPress], squat, current)).toBeUndefined();
  });
});

describe("bestSet", () => {
  it("prefers the highest estimated 1RM, then the most reps for bodyweight work", () => {
    expect(bestSet([done(5, 80), done(8, 75), done(3, 85)])).toMatchObject({ reps: 8 });
    expect(bestSet([done(10, 0), done(12, 0)])).toMatchObject({ reps: 12, e1rm: 0 });
  });
});

describe("setLine and doneSummary", () => {
  it("shows weight x reps, reps alone for bodyweight and both sides of a unilateral set", () => {
    expect(setLine(done(5, 80), false, "kg")).toBe("80 kg x 5");
    expect(setLine(done(5, 80), false, "lb")).toBe("176.4 lb x 5");
    expect(setLine(done(12, 0), false, "kg")).toBe("12 reps");
    expect(setLine(done(8, 20, { repsLeft: 7 }), true, "kg")).toBe("20 kg x L 7 R 8");
  });

  it("collapses equal sets like the plan summary", () => {
    const all = [done(5, 80), done(5, 80), done(5, 65), done(10, 0)];
    expect(doneSummary(all, false, "kg")).toBe("2x5 80 kg · 1x5 65 kg · 1x10");
    expect(doneSummary([done(8, 20, { repsLeft: 8 })], true, "kg")).toBe("1xL8 R8 20 kg");
  });
});

describe("setDelta", () => {
  const plan = (reps: number, kg: number) => sets(1, reps, kg)[0];

  it("compares today's plan with the last set in the user's unit", () => {
    expect(setDelta(plan(5, 82.5), done(5, 80), "kg")).toEqual({ text: "+2.5 kg", tone: "accent" });
    expect(setDelta(plan(5, 82.5), done(5, 80), "lb")).toEqual({ text: "+5.5 lb", tone: "accent" });
    expect(setDelta(plan(4, 80), done(5, 80), "kg")).toEqual({ text: "-1 rep", tone: "warning" });
    expect(setDelta(plan(5, 80), done(5, 80), "kg")).toEqual({ text: "same", tone: "textMuted" });
    expect(setDelta(plan(3, 85), done(5, 80), "kg")).toEqual({
      text: "+5 kg -2 reps",
      tone: "accent",
    });
  });
});
