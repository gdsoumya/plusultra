import { clock } from "./dates";
import type { ResolvedSet, SetEntry, Unit, Weight } from "./types";
import { formatWeight, inUnit, toKg } from "./units";

// A timed set with no reps to count, like a plank (weighted or not) or a round of jump rope: its
// time stands in for reps. Reps done against a clock (work for time) keep their reps.
export const isHold = (s: ResolvedSet) => s.work > 0 && s.reps <= 1;

// What a set asks for: "8-12" or "5+" reps, or "0:45" (count up: "0:30+") for a hold.
export const targetLabel = (s: ResolvedSet) =>
  isHold(s)
    ? `${clock(s.work)}${s.countUp ? "+" : ""}`
    : s.amrap
      ? `${s.reps}+`
      : s.minReps && s.minReps !== s.reps
        ? `${s.minReps}-${s.reps}`
        : `${s.reps}`;

// Work sets as "4x5 75 kg · 1x3+ 60 kg · rest 3:00" or "3x0:45": runs of equal sets collapsed, warmups
// skipped, the weight left out for bodyweight work, the rest shown once when every set shares it.
export function setsSummary(sets: ResolvedSet[], unit: Unit): string {
  const work = sets.filter((s) => !s.warmup);
  const parts = runs(
    work.map((s) =>
      s.weight.value ? `${targetLabel(s)} ${formatWeight(s.weight, unit)}` : targetLabel(s),
    ),
  );
  const rest = work[0]?.rest;
  if (rest && work.every((s) => s.rest === rest)) parts.push(`rest ${clock(rest)}`);
  return parts.join(" · ");
}

// Consecutive equal sets collapsed to "3x5 75 kg".
export function runs(texts: string[]): string[] {
  const out: { text: string; n: number }[] = [];
  for (const text of texts) {
    const last = out[out.length - 1];
    if (last?.text === text) last.n++;
    else out.push({ text, n: 1 });
  }
  return out.map((r) => `${r.n}x${r.text}`);
}

// How an exercise went against its plan: met when every set (warmups too, P2-R32) is done at the
// planned weight or more with at least the target reps (the bottom of a range); totals are reps and
// load (tonnage) in `unit`, each set's weight shown in it first so the sum adds up as displayed. A
// unilateral set counts both sides, each against the planned reps. Timed sets add up their seconds,
// logged (workTime) against planned.
export function exerciseTarget(sets: SetEntry[], unilateral = false, unit: Unit = "kg") {
  const target = (p: ResolvedSet) => (unilateral ? 2 : 1) * (p.minReps || p.reps);
  const reps = (s: SetEntry) => (unilateral ? s.reps + (s.repsLeft ?? 0) : s.reps);
  const done = sets.filter((s) => s.completed);
  const sum = (list: SetEntry[], f: (s: SetEntry) => number) => list.reduce((t, s) => t + f(s), 0);
  // Plans arrive in the user's unit and edited sets are stored in kg, so compare in kg.
  const kg = (w: Weight) => toKg(w.value, w.unit);
  return {
    met:
      sets.some((s) => target(s.planned) > 0) &&
      sets.every(
        (s) => s.completed && reps(s) >= target(s.planned) && kg(s.weight) >= kg(s.planned.weight),
      ),
    reps: sum(done, reps),
    plannedReps: sum(sets, (s) => target(s.planned)),
    load: sum(done, (s) => reps(s) * inUnit(s.weight, unit)),
    plannedLoad: sum(sets, (s) => target(s.planned) * inUnit(s.planned.weight, unit)),
    work: sum(done, (s) => s.workTime ?? s.planned.work),
    plannedWork: sum(sets, (s) => s.planned.work),
  };
}
