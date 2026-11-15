import type { ProgramSummary } from "../../domain/types";
import { currentWeek, isPast, modeLabel, sections, sortPrograms } from "./summary";

const at = (over: Partial<ProgramSummary>): ProgramSummary => ({
  id: over.name ?? "p",
  kind: "workout",
  name: "p",
  revision: 1,
  status: "draft",
  weeks: 4,
  updatedAt: "2026-11-01T00:00:00.000Z",
  ...over,
});

const finished = at({ name: "old", status: "finished", updatedAt: "2026-11-10T00:00:00.000Z" });
const meal = at({ name: "meals", kind: "meal", status: "active" });
const draft = at({ name: "draft", updatedAt: "2026-11-09T00:00:00.000Z" });
const olderDraft = at({ name: "older draft", updatedAt: "2026-11-02T00:00:00.000Z" });
const workout = at({ name: "lifting", status: "active" });
const archived = at({ name: "archived", status: "archived" });

describe("sortPrograms", () => {
  it("puts active workout then meal programs first, then drafts, then finished and archived", () => {
    const order = sortPrograms([finished, meal, olderDraft, archived, draft, workout]);
    expect(order.map((p) => p.name)).toEqual([
      "lifting",
      "meals",
      "draft",
      "older draft",
      "old",
      "archived",
    ]);
  });

  it("groups into non-empty sections", () => {
    expect(sections([finished, workout]).map((s) => [s.title, s.programs.length])).toEqual([
      ["Active", 1],
      ["Finished", 1],
    ]);
  });
});

describe("modeLabel", () => {
  it("describes anchored, floating and unstarted programs", () => {
    expect(
      modeLabel(at({ mode: "anchored", startDate: "2026-10-26", endDate: "2026-12-20" })),
    ).toBe("Anchored, 26\u00a0Oct\u00a02026 to 20\u00a0Dec\u00a02026");
    expect(modeLabel(at({ mode: "floating", nextWeek: 2, nextDay: 3 }))).toBe(
      "Floating, next week 2 day 3",
    );
    expect(modeLabel(at({ mode: "floating" }))).toBe("Floating");
    expect(modeLabel(at({}))).toBe("Not started");
  });
});

describe("currentWeek", () => {
  const anchored = at({ status: "active", mode: "anchored", startDate: "2026-10-28", weeks: 4 });

  it("counts Monday-based weeks from the start date and clamps to the program", () => {
    expect(currentWeek(anchored, "2026-10-29")).toBe(1);
    expect(currentWeek(anchored, "2026-11-11")).toBe(3);
    expect(currentWeek(anchored, "2026-12-25")).toBe(4);
    expect(currentWeek(anchored, "2026-10-01")).toBe(1);
  });

  it("follows a floating program's cursor and opens inactive programs on week 1", () => {
    expect(currentWeek(at({ status: "active", mode: "floating", nextWeek: 2 }), "2026-11-11")).toBe(2);
    expect(currentWeek(at({ status: "active", mode: "floating" }), "2026-11-11")).toBe(1);
    expect(currentWeek(at({ mode: "anchored", startDate: "2026-10-26" }), "2026-11-11")).toBe(1);
  });
});

describe("isPast", () => {
  it("dates a day before today or before a floating cursor", () => {
    const anchored = isPast(at({ status: "active", mode: "anchored" }), "2026-11-11");
    expect(anchored({ week: 1, day: 1, date: "2026-11-10" })).toBe(true);
    expect(anchored({ week: 1, day: 3, date: "2026-11-11" })).toBe(false);
    expect(anchored({ week: 1, day: 1 })).toBe(false);
    const floating = isPast(at({ status: "active", mode: "floating", nextWeek: 2, nextDay: 2 }), "x");
    expect(floating({ week: 1, day: 3 })).toBe(true);
    expect(floating({ week: 2, day: 1 })).toBe(true);
    expect(floating({ week: 2, day: 2 })).toBe(false);
    expect(isPast(at({ status: "finished" }), "2026-11-11")({ week: 1, day: 1, date: "2026-01-01" })).toBe(false);
  });
});
