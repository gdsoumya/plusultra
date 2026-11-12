import { useEffect, useState } from "react";

import type { Api } from "../api/clients";
import { useApi } from "../api/useApi";
import { connectBaseUrl } from "../api/createApi";
import type { Exercise } from "../gen/plusultra/v1/exercise_pb";
import { useServerUrl } from "./session";
import type { Store } from "./store";
import { useStore } from "./StoreProvider";

// A workout names its exercise; the slug is empty after switching to an alternate.
export interface ExerciseRef {
  slug?: string;
  name?: string;
}

export interface CachedExercise {
  exercise: Exercise;
  fetchedAt: string;
}

const infoKey = (slug: string) => `exerciseInfo:${slug}`;
const slugKey = (name: string) => `exerciseSlug:${name.toLowerCase()}`;

// A catalog reseed changes images and instructions; a cached entry older than this is shown as is
// and fetched again behind it.
const maxAge = 7 * 24 * 60 * 60 * 1000;
const fresh = (c?: CachedExercise) => !!c && Date.now() - Date.parse(c.fetchedAt) < maxAge;

export function cachedExercise(store: Store, { slug, name }: ExerciseRef) {
  const key = slug || (name && store.getKv<string>(slugKey(name)));
  return key ? store.getKv<CachedExercise>(infoKey(key)) : undefined;
}

// The server builds image links from its PUBLIC_URL, which may not be the address this client
// reaches it on. Only the path is kept; the session's server fills in the host (P2-R48).
const imagePath = (u: string) => (/^https?:\/\//i.test(u) ? new URL(u).pathname : u);

export function rememberExercise(store: Store, exercise: Exercise) {
  const stored = { ...exercise, images: exercise.images.map(imagePath) };
  store.setKv(infoKey(exercise.slug), { exercise: stored, fetchedAt: new Date().toISOString() });
  store.setKv(slugKey(exercise.name), exercise.slug);
}

// Any image link, cached path or fresh from the server, served by the session's server.
export const withImageHost = (exercise: Exercise, serverUrl?: string): Exercise => {
  if (!serverUrl) return exercise;
  const origin = connectBaseUrl(serverUrl).replace(/\/api\/v1$/, "");
  return { ...exercise, images: exercise.images.map((u) => origin + imagePath(u)) };
};

// For exercises rendered straight from a search or catalog response, not the cache.
export function useResolvedExercise(exercise: Exercise) {
  const serverUrl = useServerUrl();
  return withImageHost(exercise, serverUrl);
}

const byName = (api: Api, name: string) =>
  api.exercises
    .searchExercises({ query: name, limit: 5 })
    .then(({ exercises }) => exercises.find((e) => e.name.toLowerCase() === name.toLowerCase()));

const inFlight = new Map<string, Promise<Exercise | undefined>>();

// One request per exercise however many rows show it; the result lands in the store's kv so the
// screens render offline afterwards.
export function fetchExercise(api: Api, store: Store, { slug, name }: ExerciseRef) {
  const key = slug || name;
  if (!key) return Promise.resolve(undefined);
  let pending = inFlight.get(key);
  if (!pending) {
    const lookup = slug
      ? api.exercises.getExercise({ slug }).then((r) => r.exercise)
      : byName(api, name!);
    pending = lookup
      .then((exercise) => {
        if (exercise) rememberExercise(store, exercise);
        return exercise;
      })
      .finally(() => inFlight.delete(key));
    inFlight.set(key, pending);
  }
  return pending;
}

// shortcut: a failed lookup is not cached, so an unknown name is asked again on every mount; cache
// misses when that shows up in the logs.
export function useExerciseInfo({ slug, name }: ExerciseRef) {
  const api = useApi();
  const store = useStore();
  const serverUrl = useServerUrl();
  const key = slug || name || "";
  // A fetch fills the store, so the cache is the source after it; `done` only ends the loading.
  const cached = cachedExercise(store, { slug, name })?.exercise;
  const [done, setDone] = useState("");

  useEffect(() => {
    if (!key || fresh(cachedExercise(store, { slug, name }))) return;
    let live = true;
    fetchExercise(api, store, { slug, name })
      .catch(() => {})
      .finally(() => live && setDone(key));
    return () => {
      live = false;
    };
  }, [api, store, slug, name, key]);

  return {
    info: cached && withImageHost(cached, serverUrl),
    loading: !!key && !cached && done !== key,
  };
}
