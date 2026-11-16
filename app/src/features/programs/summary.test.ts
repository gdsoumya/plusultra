import type { ProgramSummary } from "../../domain/types";
import {
  currentWeek,
  isPast,
  modeLabel,
  sections,
  statusText,
  sortPrograms,
  spanLabel,
  andList,
  withMaxesHint,
} from "./summary";

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
    expect(
      sections([finished, workout], "2026-11-11").map((s) => [s.title, s.programs.length]),
    ).toEqual([
      ["Active", 1],
      ["Finished", 1],
    ]);
    const ended = at({ name: "ended", status: "active", mode: "anchored", endDate: "2026-11-10" });
    expect(sections([ended, workout], "2026-11-11").map((s) => s.title)).toEqual([
      "Active",
      "Ended",
    ]);
    expect(statusText(ended, "2026-11-11")).toBe("Ended");
    expect(statusText(ended, "2026-11-10")).toBe("Active");
  });
});

describe("modeLabel", () => {
  const today = "2026-11-11";
  const anchored = at({ mode: "anchored", startDate: "2026-10-26", endDate: "2026-12-20" });

  it("describes anchored, floating and unstarted programs", () => {
    expect(modeLabel(anchored, today)).toBe("26\u00a0Oct\u00a02026 to 20\u00a0Dec\u00a02026");
    expect(modeLabel(anchored, "2026-12-21")).toBe("Ended 20\u00a0Dec\u00a02026");
    expect(modeLabel(at({ mode: "floating", nextWeek: 2, nextDay: 3 }), today)).toBe(
      "Next up: week 2, day 3",
    );
    expect(modeLabel(at({ mode: "floating" }), today)).toBe("Floating, not started");
    expect(modeLabel(at({}), today)).toBe("Not started");
  });
});

describe("spanLabel", () => {
  it("calls a looping program a cycle and pluralises a fixed one", () => {
    expect(spanLabel(1, true)).toBe("1-week cycle");
    expect(spanLabel(4, true)).toBe("4-week cycle");
    expect(spanLabel(1, false)).toBe("1 week");
    expect(spanLabel(12, false)).toBe("12 weeks");
  });
});

describe("andList", () => {
  it("joins names with commas and a final and", () => {
    expect(andList([])).toBe("");
    expect(andList(["Squat"])).toBe("Squat");
    expect(andList(["Squat", "Bench Press"])).toBe("Squat and Bench Press");
    expect(andList(["Squat", "Bench Press", "Deadlift"])).toBe("Squat, Bench Press and Deadlift");
  });
});

describe("withMaxesHint", () => {
  it("adds the 1RMs pointer as a sentence and leaves other errors alone", () => {
    expect(withMaxesHint("week 1 day 1: no 1RM for Squat")).toBe(
      "Week 1 day 1: no 1RM for Squat. Set it under Settings › 1RMs.",
    );
    expect(withMaxesHint("week 1 day 1: no 1RM for Squat and Deadlift")).toBe(
      "Week 1 day 1: no 1RM for Squat and Deadlift. Set them under Settings › 1RMs.",
    );
    expect(withMaxesHint("Squat needs a 1RM.")).toBe(
      "Squat needs a 1RM. Set it under Settings › 1RMs.",
    );
    expect(withMaxesHint("unknown exercise Squatt")).toBe("unknown exercise Squatt");
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
    expect(currentWeek(at({ status: "active", mode: "floating", nextWeek: 2 }), "2026-11-11")).toBe(
      2,
    );
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
    const floating = isPast(
      at({ status: "active", mode: "floating", nextWeek: 2, nextDay: 2 }),
      "x",
    );
    expect(floating({ week: 1, day: 3 })).toBe(true);
    expect(floating({ week: 2, day: 1 })).toBe(true);
    expect(floating({ week: 2, day: 2 })).toBe(false);
    expect(
      isPast(at({ status: "finished" }), "2026-11-11")({ week: 1, day: 1, date: "2026-01-01" }),
    ).toBe(false);
  });
});
