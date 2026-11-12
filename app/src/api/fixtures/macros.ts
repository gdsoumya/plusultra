import type { Macros } from "../../domain/types";

const keys = ["kcal", "protein", "carbs", "fat", "fiber", "waterMl"] as const;
const round = (v: number) => Math.round(v * 10) / 10;

export const macros = (kcal: number, protein: number, carbs: number, fat: number, fiber = 0) => ({
  kcal,
  protein,
  carbs,
  fat,
  fiber,
  waterMl: 0,
});

const combine = (f: (k: (typeof keys)[number]) => number) =>
  Object.fromEntries(keys.map((k) => [k, round(f(k))])) as unknown as Macros;

export const scale = (m: Macros, factor: number) => combine((k) => m[k] * factor);

export const sum = (all: Macros[]) => combine((k) => all.reduce((t, m) => t + m[k], 0));

export const average = (all: Macros[]) => (all.length ? scale(sum(all), 1 / all.length) : sum([]));
