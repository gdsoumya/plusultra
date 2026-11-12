import { ProgramKind, ProgramMode, ProgramStatus } from "../../gen/plusultra/v1/program_pb";
import { createClients } from "../clients";
import { fromProto } from "../convert";
import { createMockTransport } from "../mockTransport";
import { addDays } from "./dates";

const today = "2026-11-11";
const api = () => createClients(createMockTransport({ today }).transport);

describe("mock fixtures", () => {
  it("are the same for the same day", async () => {
    expect(await api().sync.pull({})).toEqual(await api().sync.pull({}));
  });

  it("hold 30 days of history with one workout still in progress", async () => {
    const rows = fromProto.syncRows((await api().sync.pull({})).rows, "user-1");
    const dates = rows.dayLogs.map((r) => r.date);
    expect(dates[0]).toBe(addDays(today, -30));
    expect(dates.at(-1)).toBe(addDays(today, -1));
    expect(new Set(rows.dayLogs.map((r) => r.status))).toEqual(
      new Set(["done", "partial", "skipped", "rest"]),
    );
    expect(rows.workoutRecords.filter((r) => !r.finalized).map((r) => r.date)).toEqual([today]);
    expect(
      rows.workoutRecords.every((r) => r.dayName && r.kind && r.entries.every((e) => e.slug)),
    ).toBe(true);
    expect(rows.mealLogs.length).toBeGreaterThan(25);
    expect(rows.checkins.length).toBeGreaterThan(20);
  });

  it("anchor the 16-week program on the Monday two weeks before today", async () => {
    const client = api();
    const { programs } = await client.programs.listPrograms({
      kind: ProgramKind.WORKOUT,
      status: ProgramStatus.ACTIVE,
    });
    expect(programs).toEqual([
      expect.objectContaining({
        mode: ProgramMode.ANCHORED,
        startDate: "2026-10-26",
        structure: expect.objectContaining({ weeks: 8 }),
      }),
    ]);

    const { days } = await client.programs.getResolvedDays({ programId: programs[0].id, week: 1 });
    expect(days.map((d) => [d.date, d.kind])).toEqual([
      ["2026-10-26", "conditioning"],
      ["2026-10-27", "training"],
      ["2026-10-28", "training"],
      ["2026-10-29", "conditioning"],
      ["2026-10-30", "training"],
      ["2026-10-31", "training"],
      ["2026-11-01", "conditioning"],
    ]);
    const bench = fromProto.resolvedDay(days[1]).payload.exercises[0];
    expect(bench).toMatchObject({ name: "Bench Press", slug: "bench-press-barbell" });
    expect(bench.sets.map((s) => [s.warmup, s.label, s.reps, s.weight.value])).toEqual([
      [true, "", 10, 20],
      [true, "", 8, 37.5],
      [true, "", 5, 52.5],
      [true, "", 3, 65],
      [true, "", 1, 70],
      ...Array(4).fill([false, "Work", 5, 75]),
      ...Array(2).fill([false, "Backoff", 5, 62.5]),
    ]);
  });

  it("include a 4-week floating program and a 6-week meal program", async () => {
    const client = api();
    const { programs } = await client.programs.listPrograms({});
    const gzclp = programs.find((p) => p.id === "program-gzclp")!;
    expect(gzclp).toMatchObject({
      kind: ProgramKind.WORKOUT,
      status: ProgramStatus.FINISHED,
      mode: ProgramMode.UNSPECIFIED,
    });
    expect(gzclp.structure).toMatchObject({ weeks: 4, loop: true });
    expect(gzclp.structure!.days).toHaveLength(16);

    const meals = programs.find((p) => p.kind === ProgramKind.MEAL)!;
    expect(meals).toMatchObject({
      status: ProgramStatus.ACTIVE,
      structure: expect.objectContaining({ weeks: 6 }),
    });
    const { mealDays } = await client.programs.getResolvedDays({ programId: meals.id });
    expect(mealDays.map((d) => d.kind)).toEqual(Array(6).fill(["training", "rest"]).flat());
    const items = mealDays[0].meals.flatMap((m) => m.items);
    const names = items.map((i) => i.food!.name).join(" ");
    for (const food of ["Chicken", "Rice", "Egg", "Oats", "Paneer", "dal"])
      expect(names).toContain(food);
    expect(items.every((i) => i.food!.ref && i.food!.per100g)).toBe(true);
    expect(mealDays[0].totals!.kcal).toBeGreaterThan(2000);
  });

  it("search about 40 foods and 60 exercises", async () => {
    const client = api();
    const { foods } = await client.foods.searchFoods({ query: "", limit: 500 });
    expect(foods.length).toBeGreaterThanOrEqual(40);
    const dal = await client.foods.searchFoods({ query: "dal" });
    expect(dal.foods.map((f) => f.name)).toEqual(expect.arrayContaining(["Toor dal, raw"]));
    expect((await client.foods.searchFoods({ query: "chicken" })).foods[0].name).toBe(
      "Chicken breast, raw",
    );
    const concept = await client.foods.listConceptMembers({ conceptId: dal.foods[0].conceptId });
    expect(concept.preferredFoodId).toBe(dal.foods[0].id);

    const { exercises } = await client.exercises.searchExercises({ limit: 500 });
    expect(exercises.length).toBeGreaterThanOrEqual(60);
    expect(new Set(exercises.map((e) => e.slug)).size).toBe(exercises.length);
    expect((await client.exercises.searchExercises({ query: "bench" })).exercises[0].name).toBe(
      "Bench Press",
    );
    const chest = await client.exercises.searchExercises({
      muscle: "chest",
      equipment: "dumbbell",
    });
    expect(chest.exercises.map((e) => e.name)).toEqual(["Bench Press, Dumbbell"]);
    expect((await client.exercises.listExerciseData({})).items.map((d) => d.slug)).toContain(
      "squat-barbell",
    );
  });

  it("mark Bulgarian Split Squat unilateral and re-resolve days when exercise data changes", async () => {
    const client = api();
    const slug = "bulgarian-split-squat-dumbbell";
    const flag = async (name: string) => {
      const { days } = await client.programs.getResolvedDays({ programId: "program-strength16" });
      return days.flatMap((d) => d.exercises).find((e) => e.name === name)?.unilateral;
    };
    expect(await flag("Bulgarian Split Squat")).toBe(true);
    expect(await flag("Hack Squat")).toBe(false);
    const { record } = await client.workouts.getWorkoutRecord({ id: `workout-${today}` });
    expect(record!.entries.map((e) => e.unilateral)).toEqual([
      false,
      false,
      true,
      false,
      false,
      false,
      false,
    ]);

    await client.exercises.setExerciseData({ data: { slug, unilateral: false } });
    expect(await flag("Bulgarian Split Squat")).toBe(false);
    const hack = (await client.exercises.getExerciseData({ slug: "hack-squat-leverage-machine" }))
      .data!;
    await client.exercises.setExerciseData({ data: { ...hack, unilateral: true } });
    expect(await flag("Hack Squat")).toBe(true);
  });

  it("compute insights from the history", async () => {
    const client = api();
    const trend = await client.insights.weightTrend({
      dates: { from: addDays(today, -30), to: today },
    });
    expect(trend.points.length).toBeGreaterThan(20);
    expect(trend.weeklyRate!.value).toBeLessThan(0);

    for (const slug of ["squat-barbell", "bench-press-barbell", "deadlift-barbell"]) {
      const history = await client.insights.liftHistory({ slug });
      expect(history.slug).toBe(slug);
      expect(history.points.length).toBeGreaterThan(1);
      expect(history.points[0].e1rm!.value).toBeGreaterThan(history.points[0].bestWeight!.value);
    }

    const week = await client.insights.weeklySummary({ weekStart: "2026-11-02" });
    expect(week.adherence!.planned).toBe(7);
    expect(week.adherence!.done + week.adherence!.partial + week.adherence!.skipped).toBe(7);
    expect(week.setsPerMuscle.chest).toBeGreaterThan(0);
    const bench = week.e1rm.find((e) => e.slug === "bench-press-barbell");
    expect(bench).toMatchObject({ name: "Bench Press" });
    expect(bench!.e1rm!.value).toBeGreaterThan(80);
    expect(week.macros!.actual!.kcal).toBeGreaterThan(1500);
    expect(week.weight!.average!.value).toBeGreaterThan(70);
  });
});
