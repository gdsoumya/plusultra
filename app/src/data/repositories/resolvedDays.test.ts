import type { ResolvedDay } from "../../domain/types";
import { createMemoryStore } from "../memoryStore";
import { mealDaysRepo, resolvedDaysRepo } from "./resolvedDays";

function day(week: number, n: number, date?: string, name = "") {
  const payload: ResolvedDay = {
    week,
    day: n,
    date,
    name,
    desc: "",
    kind: "training",
    exercises: [],
  };
  return { programId: "p", week, day: n, date, payload };
}

describe("resolved days repository", () => {
  it("finds cached days by program position, date and range", () => {
    const repo = resolvedDaysRepo(createMemoryStore());
    repo.upsertFromServer([day(1, 2, "2026-11-12"), day(1, 1, "2026-11-11")]);
    expect(repo.at("p", 1, 2)?.payload.date).toBe("2026-11-12");
    expect(repo.at("p", 2, 1)).toBeUndefined();
    expect(repo.byDate("2026-11-11").map((r) => r.day)).toEqual([1]);
    expect(repo.range("2026-11-01", "2026-11-30").map((r) => r.day)).toEqual([1, 2]);
  });

  it("drops cached days the server no longer plans and keeps days before its window", () => {
    const repo = resolvedDaysRepo(createMemoryStore());
    repo.upsertFromServer([
      day(1, 1, "2026-11-11"),
      day(1, 2, "2026-11-12"),
      day(1, 4, "2026-11-14"),
    ]);
    // The window starts today, the 12th: the 11th is past and stays for the calendar.
    repo.replaceFromServer([day(1, 2, "2026-11-12"), day(1, 3, "2026-11-13")]);
    expect(repo.range("2026-11-01", "2026-11-30").map((r) => r.day)).toEqual([1, 2, 3]);
    repo.replaceFromServer([]);
    expect(repo.range("2026-11-01", "2026-11-30")).toEqual([]);
  });

  it("keys dated meal days by date, since kind-picked days share day 0", () => {
    const repo = mealDaysRepo(createMemoryStore());
    const mealDay = (date: string, name: string) => ({
      programId: "m",
      week: 1,
      day: 0,
      date,
      payload: { week: 1, day: 0, kind: "training" as const, name, desc: "", meals: [] },
    });
    repo.upsertFromServer([mealDay("2026-11-11", "Mon"), mealDay("2026-11-12", "Tue")]);
    expect(repo.byDate("2026-11-11")[0]?.payload.name).toBe("Mon");
    expect(repo.byDate("2026-11-12")[0]?.payload.name).toBe("Tue");
    expect(repo.range("2026-11-01", "2026-11-30")).toHaveLength(2);
  });

  it("keys floating meal days by kind, so a week's training and rest days both stay", () => {
    const repo = mealDaysRepo(createMemoryStore());
    const mealDay = (kind: "training" | "rest", name: string) => ({
      programId: "m",
      week: 1,
      day: 0,
      payload: { week: 1, day: 0, kind, name, desc: "", meals: [] },
    });
    repo.replaceFromServer([mealDay("training", "Lift"), mealDay("rest", "Rest")]);
    expect(
      repo
        .floating("m")
        .map((r) => r.payload.name)
        .sort(),
    ).toEqual(["Lift", "Rest"]);
  });

  it("replaces a cached day the server resolved again", () => {
    const repo = resolvedDaysRepo(createMemoryStore());
    repo.upsertFromServer([day(1, 1, undefined, "Heavy")]);
    repo.upsertFromServer([day(1, 1, undefined, "Light")]);
    expect(repo.at("p", 1, 1)?.payload.name).toBe("Light");
  });
});
