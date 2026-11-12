import { renderHook, waitFor } from "@testing-library/react-native";
import type { ReactNode } from "react";

import { createClients, type Api } from "../api/clients";
import { exerciseNamed } from "../api/fixtures/exercises";
import { createMockTransport } from "../api/mockTransport";
import { cachedExercise, rememberExercise, useExerciseInfo, withImageHost } from "./exerciseInfo";
import { createMemoryStore } from "./memoryStore";
import type { Store } from "./store";
import { StoreProvider } from "./StoreProvider";

let mockApi: Api;
jest.mock("../api/useApi", () => ({ useApi: () => mockApi }));

let store: Store;
beforeEach(() => {
  mockApi = createClients(createMockTransport({ today: "2026-11-11" }).transport);
  store = createMemoryStore();
});

const wrapper = ({ children }: { children: ReactNode }) => (
  <StoreProvider store={store}>{children}</StoreProvider>
);

describe("useExerciseInfo", () => {
  it("returns cached info without a network call", async () => {
    rememberExercise(store, exerciseNamed("Squat"));
    const get = jest.spyOn(mockApi.exercises, "getExercise");
    const { result } = await renderHook(() => useExerciseInfo({ slug: "squat-barbell" }), {
      wrapper,
    });
    expect(result.current).toMatchObject({ loading: false, info: { name: "Squat" } });
    expect(get).not.toHaveBeenCalled();
  });

  it("refreshes a week-old entry behind the cached one, so reseeded images arrive", async () => {
    rememberExercise(store, { ...exerciseNamed("Squat"), category: "stale" });
    const key = "exerciseInfo:squat-barbell";
    const old = store.getKv<{ fetchedAt: string }>(key)!;
    store.setKv(key, { ...old, fetchedAt: "2026-11-01T00:00:00.000Z" });
    jest.spyOn(Date, "now").mockReturnValue(Date.parse("2026-11-11T00:00:00Z"));
    const { result } = await renderHook(() => useExerciseInfo({ slug: "squat-barbell" }), {
      wrapper,
    });
    // Never a loading state: the stale entry shows until the fetch replaces it.
    expect(result.current.loading).toBe(false);
    await waitFor(() => expect(result.current.info?.category).not.toBe("stale"));
    expect(store.getKv<{ fetchedAt: string }>(key)?.fetchedAt).not.toBe(old.fetchedAt);
  });

  it("keeps only the image path and serves it from the session's server", async () => {
    const squat = exerciseNamed("Squat");
    rememberExercise(store, {
      ...squat,
      images: ["https://public.example/api/v1/exercises/images/squat-barbell/0.jpg"],
    });
    expect(cachedExercise(store, { slug: "squat-barbell" })?.exercise.images).toEqual([
      "/api/v1/exercises/images/squat-barbell/0.jpg",
    ]);
    const cached = cachedExercise(store, { slug: "squat-barbell" })!.exercise;
    expect(withImageHost(cached, "http://lift.example.com:9797").images).toEqual([
      "http://lift.example.com:9797/api/v1/exercises/images/squat-barbell/0.jpg",
    ]);
    expect(withImageHost(cached, "http://lift.example.com:9797/api/v1").images[0]).toBe(
      "http://lift.example.com:9797/api/v1/exercises/images/squat-barbell/0.jpg",
    );
    // A fresh search result still carries the server's public host; it is replaced too.
    const fresh = {
      ...squat,
      images: ["https://public.example/api/v1/exercises/images/squat-barbell/0.jpg"],
    };
    expect(withImageHost(fresh, "http://lift.example.com:9797").images[0]).toBe(
      "http://lift.example.com:9797/api/v1/exercises/images/squat-barbell/0.jpg",
    );
  });

  it("fetches a slug once and keeps it in the store", async () => {
    const get = jest.spyOn(mockApi.exercises, "getExercise");
    const { result } = await renderHook(() => useExerciseInfo({ slug: "squat-barbell" }), {
      wrapper,
    });
    await waitFor(() => expect(result.current.info?.name).toBe("Squat"));
    expect(result.current.loading).toBe(false);
    expect(get).toHaveBeenCalledTimes(1);
    expect(cachedExercise(store, { slug: "squat-barbell" })).toMatchObject({
      exercise: { equipment: "barbell" },
      fetchedAt: expect.stringMatching(/^2\d{3}-/),
    });
  });

  it("finds an alternate by name and remembers its slug", async () => {
    const { result } = await renderHook(() => useExerciseInfo({ name: "Hack Squat" }), {
      wrapper,
    });
    await waitFor(() => expect(result.current.info?.slug).toBe("hack-squat-leverage-machine"));
    expect(cachedExercise(store, { name: "Hack Squat" })?.exercise.name).toBe("Hack Squat");
  });

  it("gives up quietly on an unknown exercise", async () => {
    const { result } = await renderHook(() => useExerciseInfo({ slug: "nope" }), { wrapper });
    await waitFor(() => expect(result.current.loading).toBe(false));
    expect(result.current.info).toBeUndefined();
  });
});
