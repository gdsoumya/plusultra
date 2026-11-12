import { act, fireEvent, render, screen } from "@testing-library/react-native";
import { BackHandler } from "react-native";

import { kg, sets } from "../../api/fixtures/build";
import { createMemoryStore } from "../../data/memoryStore";
import { workoutRecordsRepo } from "../../data/repositories/workoutRecords";
import type { Store } from "../../data/store";
import { StoreProvider } from "../../data/StoreProvider";
import type { ExerciseEntry, SetEntry, Unit, WorkoutRecord } from "../../domain/types";
import { ThemeProvider } from "../../ui";
import { LastTimeRow } from "./LastTimeRow";

const mockRouter = { push: jest.fn() };
let mockSegments: string[] = ["(tabs)", "(today)", "workout", "[id]"];
jest.mock("expo-router", () => ({ useRouter: () => mockRouter, useSegments: () => mockSegments }));
const mockLiftHistory = jest.fn();
jest.mock("../../api/useApi", () => ({
  useApi: () => ({ insights: { liftHistory: mockLiftHistory } }),
}));

const set = (reps: number, weight: number, over: Partial<SetEntry> = {}): SetEntry => ({
  planned: sets(1, reps, weight)[0],
  completed: true,
  reps,
  weight: kg(weight),
  rpe: 0,
  ...over,
});
const planned = (reps: number, weight: number) =>
  set(0, weight, { planned: sets(1, reps, weight)[0], completed: false });

const squat = (all: SetEntry[], over: Partial<ExerciseEntry> = {}): ExerciseEntry => ({
  fromProgram: true,
  name: "Squat",
  slug: "squat",
  equipment: "Barbell",
  label: "",
  alternates: ["Squat"],
  current: 1,
  desc: "",
  superset: "",
  variation: 0,
  promptValues: {},
  sets: all,
  ...over,
});

const record = (id: string, date: string, entries: ExerciseEntry[]): WorkoutRecord => ({
  id,
  userId: "u1",
  updatedAt: `${date}T20:00:00.000Z`,
  version: 1,
  deviceId: "server",
  dirty: false,
  date,
  week: 1,
  day: 1,
  dayName: "Lower",
  dayDesc: "",
  entries,
  finalized: true,
  finalizeRequested: true,
});

const today = { id: "today", date: "2026-11-11" };
const plan = squat([planned(5, 40), planned(5, 82.5), planned(5, 82.5), planned(3, 82.5)]);
plan.sets[0].planned.warmup = true;

const row = (store: Store, entry = plan, unit: Unit = "kg", key = 0) => (
  <ThemeProvider key={key}>
    <StoreProvider store={store}>
      <LastTimeRow entry={entry} record={today} unit={unit} index={0} />
    </StoreProvider>
  </ThemeProvider>
);
const mount = (store: Store, entry = plan, unit: Unit = "kg") => render(row(store, entry, unit));

beforeEach(() => {
  jest.clearAllMocks();
  mockLiftHistory.mockResolvedValue({ points: [] });
});
afterEach(() => jest.restoreAllMocks());

describe("LastTimeRow", () => {
  it("shows the last session from the store, expands with deltas and opens the record", async () => {
    const handlers: Parameters<typeof BackHandler.addEventListener>[1][] = [];
    jest.spyOn(BackHandler, "addEventListener").mockImplementation((_, h) => {
      handlers.push(h);
      return { remove: () => handlers.splice(handlers.indexOf(h), 1) };
    });
    const store = createMemoryStore();
    const warmup = set(5, 40, { planned: sets(1, 5, 40, { warmup: true })[0] });
    workoutRecordsRepo(store).upsertFromServer([
      record("r1", "2026-11-04", [squat([set(5, 77.5), set(5, 77.5)])]),
      record("r2", "2026-11-09", [squat([warmup, set(5, 80), set(5, 80), set(4, 80)])]),
    ]);
    await mount(store);
    const shown = screen.getByTestId("exercise-0-last");
    expect(shown).toHaveTextContent(/Last time · Mon 9 Nov/);
    expect(shown).toHaveTextContent(/2x5 80 kg · 1x4 80 kg/);
    expect(screen.queryByTestId("exercise-0-last-sets")).toBeNull();
    expect(mockLiftHistory).not.toHaveBeenCalled();

    await fireEvent.press(shown);
    expect(screen.getByTestId("exercise-0-last-set-0")).toHaveTextContent("Set 180 kg x 5+2.5 kg");
    expect(screen.getByTestId("exercise-0-last-set-2")).toHaveTextContent(
      "Set 380 kg x 4+2.5 kg -1 rep",
    );
    expect(screen.getByTestId("exercise-0-last-best")).toHaveTextContent(
      "Best 80 kg x 5 · e1RM 93.3 kg",
    );
    await fireEvent.press(screen.getByText("Open session"));
    expect(mockRouter.push).toHaveBeenCalledWith("/workout/r2");
    // From the Calendar tab the session opens at the tab's own path.
    mockSegments = ["(tabs)", "(calendar)", "day", "[date]", "workout", "[id]"];
    await screen.rerender(row(store, plan, "kg", 1));
    await fireEvent.press(screen.getByTestId("exercise-0-last"));
    await fireEvent.press(screen.getByText("Open session"));
    expect(mockRouter.push).toHaveBeenLastCalledWith("/day/2026-11-09/workout/r2");
    mockSegments = ["(tabs)", "(today)", "workout", "[id]"];

    // Hardware back collapses the list instead of leaving the screen.
    await act(() =>
      expect(handlers.at(-1)!({ type: "hardwareBackPress", timeStamp: 0 })).toBe(true),
    );
    expect(screen.queryByTestId("exercise-0-last-sets")).toBeNull();
    expect(handlers).toHaveLength(0);
  });

  it("falls back to the server's lift history, caches it and shows nothing offline without a cache", async () => {
    mockLiftHistory.mockResolvedValue({
      points: [
        { date: "2026-10-28", e1rm: kg(90), bestWeight: kg(75), bestReps: 6 },
        { date: "2026-11-11", e1rm: kg(99), bestWeight: kg(85), bestReps: 5 },
      ],
    });
    const store = createMemoryStore();
    await mount(store);
    expect(mockLiftHistory).toHaveBeenCalledWith({ slug: "squat" });
    const shown = await screen.findByTestId("exercise-0-last");
    expect(shown).toHaveTextContent(/Last time · Wed 28 Oct/);
    expect(shown).toHaveTextContent(/Best 75 kg x 6/);
    await fireEvent.press(shown);
    expect(screen.getByTestId("exercise-0-last-best")).toHaveTextContent(/e1RM\ 90\ kg/);
    expect(screen.queryByText("Open session")).toBeNull();
    expect(screen.queryByTestId("exercise-0-last-set-0")).toBeNull();

    // The cached copy serves the next mount without another call, offline too.
    mockLiftHistory.mockRejectedValue(new Error("offline"));
    await screen.rerender(row(store, plan, "kg", 1));
    expect(screen.getByTestId("exercise-0-last")).toHaveTextContent(/Wed 28 Oct/);
    expect(mockLiftHistory).toHaveBeenCalledTimes(1);
  });

  it("renders nothing without history or a cache", async () => {
    mockLiftHistory.mockRejectedValue(new Error("offline"));
    await mount(createMemoryStore());
    await act(async () => {});
    expect(screen.queryByTestId("exercise-0-last")).toBeNull();
    expect(screen.toJSON()).toBeNull();
  });

  it("shows pounds and both sides of unilateral sets", async () => {
    const store = createMemoryStore();
    const lunge = squat([set(8, 20, { repsLeft: 8 }), set(7, 20, { repsLeft: 8 })], {
      name: "Lunge",
      slug: "lunge",
      unilateral: true,
    });
    workoutRecordsRepo(store).upsertFromServer([record("r1", "2026-11-09", [lunge])]);
    const todayLunge = { ...lunge, sets: [planned(8, 22.5), planned(8, 22.5)] };
    await mount(store, todayLunge, "lb");
    const shown = screen.getByTestId("exercise-0-last");
    expect(shown).toHaveTextContent(/1xL8 R8 44\.1 lb · 1xL8 R7 44\.1 lb/);
    await fireEvent.press(shown);
    expect(screen.getByTestId("exercise-0-last-set-1")).toHaveTextContent(
      "Set 244.1 lb x L 8 R 7+5.5 lb +1 rep",
    );
  });
});
