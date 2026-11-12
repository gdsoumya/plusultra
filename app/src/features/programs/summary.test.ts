import type { ProgramSummary } from "../../domain/types";
import { modeLabel, sections, sortPrograms } from "./summary";

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
