import type { Api } from "../api/clients";
import type { ResolvedDay, ResolvedExercise } from "../domain/types";
import type { ExerciseData } from "../gen/plusultra/v1/exercise_pb";
import { fetchExercise, type ExerciseRef } from "./exerciseInfo";
import type { Store } from "./store";

const flagKey = (slug: string) => `exerciseUnilateral:${slug}`;

// Keeps flags by slug, and writes them into the cached plan days: those carry the flag the server
// resolved them with, and past ones are never pulled again.
export function rememberUnilateral(store: Store, flags: Map<string, boolean>) {
  const stale = (e: ResolvedExercise) => flags.has(e.slug) && flags.get(e.slug) !== !!e.unilateral;
  store.transaction(() => {
    for (const row of store.list("resolvedDays")) {
      if (!row.payload.exercises.some(stale)) continue;
      const exercises = row.payload.exercises.map((e) =>
        stale(e) ? { ...e, unilateral: flags.get(e.slug) } : e,
      );
      store.put("resolvedDays", { ...row, payload: { ...row.payload, exercises } });
    }
    for (const [slug, flag] of flags) store.setKv(flagKey(slug), flag);
  });
}

// The server re-resolves from today on when a flag changes, so a day it sends for a past week may
// carry the old one; the flags the app knows win.
export const withKnownFlags = (store: Store, day: ResolvedDay): ResolvedDay => ({
  ...day,
  exercises: day.exercises.map((e) => {
    const known = e.slug ? store.getKv<boolean>(flagKey(e.slug)) : undefined;
    return known === undefined ? e : { ...e, unilateral: known };
  }),
});

// Only the flag is kept: the rest of the data (1RM, rounding) is read live where it is edited.
export const rememberExerciseData = (store: Store, data: ExerciseData) =>
  rememberUnilateral(store, new Map([[data.slug, data.unilateral]]));

// The sync refreshes the flags of exercises in the pulled plan days; one changed elsewhere (MCP,
// another device) for an exercise outside them shows up after this app writes its data.
export async function fetchUnilateral(api: Api, store: Store, ref: ExerciseRef) {
  const slug = ref.slug || (await fetchExercise(api, store, ref))?.slug;
  if (!slug) return false;
  const cached = store.getKv<boolean>(flagKey(slug));
  if (cached !== undefined) return cached;
  const { data } = await api.exercises.getExerciseData({ slug });
  if (data) rememberExerciseData(store, data);
  return !!data?.unilateral;
}
