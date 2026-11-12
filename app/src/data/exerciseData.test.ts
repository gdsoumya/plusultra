import { createClients, type Api } from "../api/clients";
import { createMockTransport } from "../api/mockTransport";
import { create } from "@bufbuild/protobuf";

import { ExerciseDataSchema } from "../gen/plusultra/v1/exercise_pb";
import { fetchUnilateral, rememberExerciseData } from "./exerciseData";
import { workoutDays } from "../features/calendar/programDays";
import { fetchPastPlan } from "./plans";
import { programsRepo } from "./repositories/programs";
import { resolvedDaysRepo } from "./repositories/resolvedDays";
import { seedStore } from "./seedStore";
import { createMemoryStore } from "./memoryStore";
import type { Store } from "./store";

let api: Api;
let store: Store;
beforeEach(() => {
  api = createClients(createMockTransport({ today: "2026-11-11" }).transport);
  store = createMemoryStore();
});

describe("fetchUnilateral", () => {
  it("reads the flag by slug or through the catalog by name, then from the cache", async () => {
    expect(await fetchUnilateral(api, store, { slug: "bulgarian-split-squat-dumbbell" })).toBe(
      true,
    );
    expect(await fetchUnilateral(api, store, { name: "Bulgarian Split Squat" })).toBe(true);
    expect(await fetchUnilateral(api, store, { name: "Hack Squat" })).toBe(false);
    expect(await fetchUnilateral(api, store, { name: "Not In The Catalog" })).toBe(false);
    const spy = jest.spyOn(api.exercises, "getExerciseData");
    expect(await fetchUnilateral(api, store, { name: "Bulgarian Split Squat" })).toBe(true);
    expect(spy).not.toHaveBeenCalled();
  });
});

describe("rememberExerciseData", () => {
  it("writes the flag into cached plan days, past ones too, which are never pulled again", async () => {
    store = await seedStore("2026-11-11");
    const slug = "bulgarian-split-squat-dumbbell";
    const flags = () =>
      resolvedDaysRepo(store)
        .range("2026-10-01", "2026-12-31")
        .flatMap((d) => d.payload.exercises.filter((e) => e.slug === slug))
        .map((e) => !!e.unilateral);
    const past = resolvedDaysRepo(store).range("2026-10-01", "2026-11-10");
    expect(past.some((d) => d.payload.exercises.some((e) => e.slug === slug))).toBe(true);
    expect(flags()).toContain(true);
    const others = resolvedDaysRepo(store)
      .range("2026-10-01", "2026-12-31")
      .flatMap((d) => d.payload.exercises.filter((e) => e.slug !== slug));

    rememberExerciseData(store, create(ExerciseDataSchema, { slug, unilateral: false }));
    expect(flags().every((f) => !f)).toBe(true);
    expect(
      resolvedDaysRepo(store)
        .range("2026-10-01", "2026-12-31")
        .flatMap((d) => d.payload.exercises.filter((e) => e.slug !== slug)),
    ).toEqual(others);
    expect(await fetchUnilateral(api, store, { slug })).toBe(false);
  });
});

describe("days fetched from the server", () => {
  // The server re-resolves from today on when a flag changes, so a past week comes back stale.
  it("carry the flag the app knows, for a past date and in the program day picker", async () => {
    store = await seedStore("2026-11-11");
    const slug = "bulgarian-split-squat-dumbbell";
    const [past] = resolvedDaysRepo(store)
      .range("2026-10-01", "2026-11-10")
      .filter((d) => d.payload.exercises.some((e) => e.slug === slug && e.unilateral));
    store.delete("resolvedDays", past.id);
    rememberExerciseData(store, create(ExerciseDataSchema, { slug, unilateral: false }));

    await fetchPastPlan(api, store, past.date!);
    const fetched = resolvedDaysRepo(store).byDate(past.date!)[0].payload.exercises;
    expect(fetched.filter((e) => e.slug === slug).map((e) => !!e.unilateral)).toEqual([false]);

    const program = programsRepo(store).get(past.programId)!;
    const days = await workoutDays(api, store, program);
    const flags = days.flatMap((d) => d.exercises.filter((e) => e.slug === slug));
    expect(flags.length).toBeGreaterThan(0);
    expect(flags.every((e) => !e.unilateral)).toBe(true);
  });
});
