import { daysBetween, formatDate, mondayOf } from "../../domain/dates";
import type { ProgramSummary, ResolvedDay } from "../../domain/types";

type Status = ProgramSummary["status"];

const statusRank: Record<Status, number> = { active: 0, draft: 1, finished: 2, archived: 3 };
const kindRank: Record<ProgramSummary["kind"], number> = { workout: 0, meal: 1 };

export const kindLabel: Record<ProgramSummary["kind"], string> = {
  workout: "Workout",
  meal: "Meals",
};

export const statusLabel: Record<Status, string> = {
  draft: "Draft",
  active: "Active",
  finished: "Finished",
  archived: "Archived",
};

// Catalog levels in listing order.
export const levelLabel: Record<string, string> = {
  beginner: "Beginner",
  intermediate: "Intermediate",
  advanced: "Advanced",
};

export const sortPrograms = (all: ProgramSummary[]) =>
  [...all].sort(
    (a, b) =>
      statusRank[a.status] - statusRank[b.status] ||
      kindRank[a.kind] - kindRank[b.kind] ||
      b.updatedAt.localeCompare(a.updatedAt),
  );

const groups: { title: string; statuses: Status[] }[] = [
  { title: "Active", statuses: ["active"] },
  { title: "Drafts", statuses: ["draft"] },
  { title: "Finished", statuses: ["finished", "archived"] },
];

export const sections = (all: ProgramSummary[]) =>
  groups
    .map(({ title, statuses }) => ({
      title,
      programs: sortPrograms(all.filter((p) => statuses.includes(p.status))),
    }))
    .filter((s) => s.programs.length > 0);

// A date never wraps mid-way; a long range breaks at "to".
const date = (d: string) => formatDate(d).replaceAll(" ", "\u00a0");

export function modeLabel(p: ProgramSummary, today: string): string {
  if (p.mode === "anchored" && p.startDate && p.endDate) {
    if (p.endDate < today) return `Ended ${date(p.endDate)}`;
    return `${date(p.startDate)} to ${date(p.endDate)}`;
  }
  if (p.mode === "floating") {
    return p.nextWeek ? `Next up: week ${p.nextWeek}, day ${p.nextDay}` : "Floating, not started";
  }
  return "Not started";
}

export const plural = (n: number, word: string) => `${n} ${word}${n === 1 ? "" : "s"}`;

// A looping program is a cycle, a fixed one a span.
export const spanLabel = (weeks: number, loop: boolean) =>
  loop ? `${weeks}-week cycle` : plural(weeks, "week");

export const createdByLabel: Record<string, string> = {
  user: "saved in the app",
  api: "over the API",
  mcp: "by an assistant",
};

// A missing 1RM is fixed in Settings, not in the script; the server's text ends without a period.
export const withMaxesHint = (message: string) =>
  message.includes("1RM")
    ? `${message.replace(/\.$/, "")}. Set it under Settings › 1RMs.`
    : message;

// The week today falls in for an active program, clamped to its span; week 1 otherwise.
export function currentWeek(p: ProgramSummary, today: string): number {
  if (p.status !== "active") return 1;
  if (p.mode === "floating") return p.nextWeek ?? 1;
  if (!p.startDate) return 1;
  const week = Math.floor(daysBetween(mondayOf(p.startDate), today) / 7) + 1;
  return Math.min(Math.max(week, 1), p.weeks || 1);
}

// Whether a day already lies behind an active program: before today when dated, before the
// cursor when floating. Past days keep the plan they had then, not the current revision's.
export const isPast =
  (p: ProgramSummary, today: string) =>
  (d: Pick<ResolvedDay, "week" | "day" | "date">): boolean =>
    p.status === "active" &&
    (d.date
      ? d.date < today
      : p.nextWeek !== undefined &&
        (d.week < p.nextWeek || (d.week === p.nextWeek && d.day < (p.nextDay ?? 0))));
