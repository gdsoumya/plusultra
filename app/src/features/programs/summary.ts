import { formatDate } from "../../domain/dates";
import type { ProgramSummary } from "../../domain/types";

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

export function modeLabel(p: ProgramSummary): string {
  if (p.mode === "anchored" && p.startDate && p.endDate) {
    return `Anchored, ${formatDate(p.startDate)} to ${formatDate(p.endDate)}`;
  }
  if (p.mode === "floating") {
    return p.nextWeek ? `Floating, next week ${p.nextWeek} day ${p.nextDay}` : "Floating";
  }
  return "Not started";
}
