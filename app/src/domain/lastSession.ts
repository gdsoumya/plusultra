import { clock } from "./dates";
import { isHold, runs } from "./sets";
import type { ExerciseEntry, ResolvedSet, SetEntry, Unit, Weight, WorkoutRecord } from "./types";
import { formatWeight, inUnit, round1, toKg } from "./units";

export type LastSession = {
  date: string;
  // Completed work sets; empty when the session comes from the server's history.
  sets: SetEntry[];
  best: Lifted;
  e1rm?: Weight;
  // Set when the record is in the local store.
  recordId?: string;
};

// Epley estimate of the one-rep max, in kilograms.
const epley = (s: SetEntry) => toKg(s.weight.value, s.weight.unit) * (1 + s.reps / 30);

// Ties (holds, bodyweight sets) go to the longer hold, then the more reps.
export function bestSet(sets: SetEntry[]) {
  const better = (s: SetEntry, b: SetEntry) =>
    epley(s) !== epley(b)
      ? epley(s) > epley(b)
      : (s.workTime ?? 0) !== (b.workTime ?? 0)
        ? (s.workTime ?? 0) > (b.workTime ?? 0)
        : s.reps > b.reps;
  const best = sets.reduce((b, s) => (better(s, b) ? s : b));
  return { ...best, e1rm: round1(epley(best)) };
}

const same = (entry: ExerciseEntry, e: ExerciseEntry) =>
  (!!entry.slug && entry.slug === e.slug) ||
  [entry.name, ...entry.alternates].some((n) => n.toLowerCase() === e.name.toLowerCase());

// The most recent record on or before the current one (another record of the same day counts) with
// completed work sets of the exercise, by slug, else by name or one of the alternates.
export function findLastSession(
  records: WorkoutRecord[],
  entry: ExerciseEntry,
  current: Pick<WorkoutRecord, "id" | "date">,
): LastSession | undefined {
  for (const r of [...records].reverse()) {
    if (r.id === current.id || r.date > current.date || r.deletedAt) continue;
    for (const e of r.entries) {
      if (!same(entry, e)) continue;
      const sets = e.sets.filter((s) => s.completed && !s.planned.warmup);
      if (!sets.length) continue;
      const { e1rm, ...best } = bestSet(sets);
      return {
        date: r.date,
        sets,
        best,
        e1rm: e1rm ? { value: e1rm, unit: "kg" } : undefined,
        recordId: r.id,
      };
    }
  }
  return undefined;
}

export type Lifted = Pick<SetEntry, "reps" | "repsLeft" | "weight"> &
  Partial<Pick<SetEntry, "planned" | "workTime">>;

const reps = (s: Lifted, unilateral: boolean, sep: string) =>
  unilateral ? `L${sep}${s.repsLeft ?? 0} R${sep}${s.reps}` : `${s.reps}`;
const holdTime = (s: Lifted) =>
  s.planned && isHold(s.planned) ? clock(s.workTime ?? s.planned.work) : undefined;

// "80 kg x 5", "12 reps", or "20 kg x L 7 R 8" for a unilateral set.
// "80 kg x 5", "L 8 R 8", "5 reps"; a hold is its time, "0:45" or "10 kg x 0:45".
export const setLine = (s: Lifted, unit: Unit, unilateral = false) =>
  s.weight.value
    ? `${formatWeight(s.weight, unit)} x ${holdTime(s) ?? reps(s, unilateral, " ")}`
    : (holdTime(s) ?? (unilateral ? reps(s, unilateral, " ") : `${s.reps} reps`));

// "2x5 80 kg · 1x5 65 kg", like setsSummary for the plan.
// A hold is its time ("0:45"), any other set its reps.
const effort = (s: SetEntry, unilateral: boolean) =>
  isHold(s.planned) ? clock(s.workTime ?? s.planned.work) : reps(s, unilateral, "");

export const doneSummary = (sets: SetEntry[], unilateral: boolean, unit: Unit) =>
  runs(
    sets.map((s) =>
      s.weight.value
        ? `${effort(s, unilateral)} ${formatWeight(s.weight, unit)}`
        : effort(s, unilateral),
    ),
  ).join(" · ");

const signed = (v: number) => (v > 0 ? `+${v}` : `${v}`);

// Today's planned set against the last session's: "+2.5 kg", "-1 rep", "same". The weight leads the
// tone; deltas are computed in kilograms and shown rounded to the unit's display step.
export function setDelta(planned: ResolvedSet, last: SetEntry, unit: Unit) {
  const kg =
    toKg(planned.weight.value, planned.weight.unit) - toKg(last.weight.value, last.weight.unit);
  const weight = inUnit({ value: kg, unit: "kg" }, unit);
  const reps = planned.reps - last.reps;
  const parts = [
    weight ? `${signed(weight)} ${unit}` : "",
    reps ? `${signed(reps)} rep${Math.abs(reps) === 1 ? "" : "s"}` : "",
  ].filter(Boolean);
  const lead = weight || reps;
  return {
    text: parts.join(" ") || "same",
    tone: lead > 0 ? "accent" : lead < 0 ? "warning" : "textMuted",
  } as const;
}
