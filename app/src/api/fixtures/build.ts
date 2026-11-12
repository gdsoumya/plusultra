import type { ResolvedExercise, ResolvedSet } from "../../domain/types";
import { exerciseNamed } from "./exercises";

export const kg = (value: number) => ({ value, unit: "kg" as const });

// Rounds to what 2.5 kg plates can load.
const round = (value: number) => Math.round(value / 2.5) * 2.5;

export function sets(n: number, reps: number, weight: number, over: Partial<ResolvedSet> = {}) {
  return Array.from({ length: n }, (): ResolvedSet => ({
    reps,
    minReps: reps,
    weight: kg(weight),
    rpe: 0,
    rest: 0,
    work: 0,
    amrap: false,
    ask: false,
    auto: false,
    countUp: false,
    warmup: false,
    label: "",
    logRpe: false,
    ...over,
  }));
}

// Each step is [reps, kg] or [reps, percent of the first work set].
export type Warmup = [number, number, "kg" | "%"][];

export const warmups = (steps: Warmup, work: number) =>
  steps.flatMap(([reps, v, unit]) =>
    sets(1, reps, unit === "kg" ? v : round((work * v) / 100), { warmup: true }),
  );

export type ExerciseOptions = Partial<Pick<ResolvedExercise, "label" | "desc" | "superset">> & {
  alts?: string[];
};

export function ex(name: string, all: ResolvedSet[], { alts = [], ...over }: ExerciseOptions = {}) {
  const { slug, equipment } = exerciseNamed(name);
  return {
    name,
    slug,
    equipment,
    label: "",
    alternates: [name, ...alts],
    current: 1,
    desc: "",
    superset: "",
    sets: all,
    ...over,
  } satisfies ResolvedExercise;
}

// Double progression after `step` sessions, all completed: reps climb to max, then weight goes up.
export function dp(step: number, weight: number, [inc, min, max]: [number, number, number]) {
  const span = max - min + 1;
  return { reps: min + (step % span), weight: weight + inc * Math.floor(step / span) };
}
