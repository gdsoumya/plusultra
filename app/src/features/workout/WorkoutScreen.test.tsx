import { act, fireEvent, render, screen, waitFor, within } from "@testing-library/react-native";
import * as Notifications from "expo-notifications";
import { useEffect, useState } from "react";
import { BackHandler, Text } from "react-native";
import { GestureHandlerRootView, State } from "react-native-gesture-handler";
import { fireGestureHandler, getByGestureTestId } from "react-native-gesture-handler/jest-utils";

import { createClients } from "../../api/clients";
import { fromProto } from "../../api/convert";
import { createMockTransport } from "../../api/mockTransport";
import { createMemoryStore } from "../../data/memoryStore";
import { programsRepo } from "../../data/repositories/programs";
import { workoutRecordsRepo } from "../../data/repositories/workoutRecords";
import type { Store } from "../../data/store";
import { StoreProvider, useStore } from "../../data/StoreProvider";
import { SyncProvider } from "../../data/useSync";
import type { Unit } from "../../domain/types";
import { ThemeProvider } from "../../ui";
import { pullToRefresh } from "../../ui/test-utils";
import WorkoutScreen from "./WorkoutScreen";

jest.mock("../../data/uuid", () => ({ uuid: () => "device-1" }));
jest.mock("expo-notifications", () => ({
  SchedulableTriggerInputTypes: { DATE: "date" },
  setNotificationHandler: jest.fn(),
  requestPermissionsAsync: jest.fn(async () => ({ granted: true })),
  scheduleNotificationAsync: jest.fn(async () => "n1"),
  cancelScheduledNotificationAsync: jest.fn(async () => {}),
}));
const mockRouter = { back: jest.fn(), replace: jest.fn(), canGoBack: () => true, push: jest.fn() };
jest.mock("expo-router", () => ({
  useRouter: () => mockRouter,
  useFocusEffect: (effect: () => void) =>
    jest.requireActual<typeof import("react")>("react").useEffect(effect, [effect]),
}));
const mockPlayer = { seekTo: jest.fn(), play: jest.fn() };
jest.mock("expo-audio", () => ({
  setAudioModeAsync: jest.fn(async () => {}),
  createAudioPlayer: jest.fn(() => mockPlayer),
}));
const beeps = () => mockPlayer.play.mock.calls.length;
jest.mock("../../api/useApi", () => ({ useApi: () => mockApi }));
jest.mock("../../data/session", () => ({
  useSession: () => ({ status: "signedIn" }),
  useServerUrl: () => undefined,
}));

const today = "2026-11-11";
const id = `workout-${today}`;
const mockApi = createClients(createMockTransport({ today }).transport);

// Today's fixture record (Wed Lower B: deadlift with warmups and rest, then accessories with
// alternates), with every set still pending.
async function seed(): Promise<Store> {
  const store = createMemoryStore();
  const { record } = await mockApi.workouts.getWorkoutRecord({ id });
  const row = fromProto.workoutRecord(record!, "user-1");
  workoutRecordsRepo(store).upsertFromServer([
    {
      ...row,
      entries: row.entries.map((e) => ({
        ...e,
        sets: e.sets.map((s) => ({ ...s, completed: false, reps: 0 })),
      })),
    },
  ]);
  return store;
}

const mount = (store: Store, unit: Unit = "kg") =>
  render(
    <ThemeProvider>
      <StoreProvider store={store}>
        <GestureHandlerRootView>
          <WorkoutScreen id={id} unit={unit} />
        </GestureHandlerRootView>
      </StoreProvider>
    </ThemeProvider>,
  );

const saved = (store: Store) => store.get("workoutRecords", id)!;
const modifiedChips = () => screen.queryAllByText("Modified");
// The pane shows one exercise; its tile brings it up.
const show = (i: number) => fireEvent.press(screen.getByTestId(`tile-${i}`));
const finishTile = () => fireEvent.press(screen.getByTestId("finish-workout"));

// Registered once when the notifications module loads, before any test clears the mocks.
const [[timerNotices]] = jest.mocked(Notifications.setNotificationHandler).mock.calls;
const notifies = async () =>
  (await timerNotices!.handleNotification({} as Notifications.Notification)).shouldShowBanner;

beforeEach(() => jest.clearAllMocks());

describe("WorkoutScreen", () => {
  it("lists the exercises with their planned sets in the user's unit", async () => {
    const store = await seed();
    const [deadlift, hackSquat] = saved(store).entries;
    await mount(store, "lb");
    expect(screen.getByRole("header", { name: saved(store).dayName })).toBeOnTheScreen();
    expect(within(screen.getByTestId("exercise-0")).getByText(deadlift.name)).toBeOnTheScreen();
    expect(screen.getByTestId("tile-1")).toHaveAccessibleName(
      `${hackSquat.name}, 0 of ${hackSquat.sets.length} sets`,
    );
    expect(screen.queryByTestId("set-1-0-reps")).toBeNull();
    await show(1);
    const planned = hackSquat.sets[0].planned;
    const lb = Math.round((planned.weight.value / 0.45359237) * 10) / 10;
    expect(screen.getByTestId("set-1-0-reps")).toHaveTextContent(`${planned.reps}`);
    expect(screen.getByTestId("set-1-0-weight")).toHaveTextContent(`${lb} lb`);
    expect(modifiedChips()).toHaveLength(0);
  });

  it("marks the shown tile, counts done sets on it and checks it once every set is done", async () => {
    const store = await seed();
    const sets = saved(store).entries[1].sets.length;
    await mount(store);
    expect(screen.getByTestId("tile-0")).toBeSelected();
    expect(screen.getByTestId("tile-1")).not.toBeSelected();
    await show(1);
    expect(screen.getByTestId("tile-1")).toBeSelected();
    expect(screen.queryByTestId("tile-1-check")).toBeNull();
    for (let i = 0; i < sets; i++) await fireEvent.press(screen.getByTestId(`set-1-${i}-done`));
    expect(screen.getByTestId("tile-1")).toHaveAccessibleName(
      new RegExp(`, ${sets} of ${sets} sets$`),
    );
    expect(screen.getByTestId("tile-1-check")).toBeOnTheScreen();
    // The last set done does not jump on by itself.
    expect(screen.getByTestId("exercise-1")).toBeOnTheScreen();
  });

  it("moves between exercises by swipe and with the Next button, which finishes on the last", async () => {
    const store = await seed();
    const last = saved(store).entries.length - 1;
    await mount(store);
    fireGestureHandler(getByGestureTestId("exercise-pane-pan"), [
      { state: State.BEGAN },
      { state: State.ACTIVE },
      { state: State.END, translationX: -80 },
    ]);
    await act(async () => {});
    expect(screen.getByTestId("exercise-1")).toBeOnTheScreen();
    fireGestureHandler(getByGestureTestId("exercise-pane-pan"), [
      { state: State.BEGAN },
      { state: State.ACTIVE },
      { state: State.END, translationX: 80 },
    ]);
    await act(async () => {});
    expect(screen.getByTestId("exercise-0")).toBeOnTheScreen();
    fireGestureHandler(getByGestureTestId("exercise-pane-pan"), [
      { state: State.BEGAN },
      { state: State.ACTIVE },
      { state: State.END, translationX: 80 },
    ]);
    await act(async () => {});
    expect(screen.getByTestId("exercise-0")).toBeOnTheScreen();

    await fireEvent.press(screen.getByTestId("next-exercise"));
    expect(screen.getByTestId("exercise-1")).toBeOnTheScreen();
    expect(screen.getByTestId("next-exercise")).toHaveAccessibleName("Next exercise");
    await show(last);
    expect(screen.getByTestId("next-exercise")).toHaveAccessibleName("Finish");
    await fireEvent.press(screen.getByTestId("next-exercise"));
    expect(saved(store).finalizeRequested).toBe(true);
    expect(mockRouter.back).toHaveBeenCalledTimes(1);
  });

  it("highlights the next pending set, warmups first, and opens on its exercise", async () => {
    const store = await seed();
    expect(saved(store).entries[0].sets[0].planned.warmup).toBe(true);
    await mount(store);
    expect(screen.getByTestId("set-0-0-done")).toHaveAccessibleName("Complete warmup 1, next up");
    expect(screen.getByTestId("set-0-1-done")).not.toHaveAccessibleName(/next up/);
    await fireEvent.press(screen.getByTestId("set-0-0-done"));
    expect(screen.getByTestId("set-0-1-done")).toHaveAccessibleName(/next up/);
    await show(1);
    expect(screen.getByTestId("set-1-0-done")).not.toHaveAccessibleName(/next up/);
  });

  it("ticks a set done with its shown values, and a second tick undoes it", async () => {
    const store = await seed();
    const { planned } = saved(store).entries[1].sets[0];
    await mount(store);
    await show(1);
    await fireEvent.press(screen.getByTestId("set-1-0-done"));
    expect(screen.getByTestId("set-1-0-done")).toBeChecked();
    expect(saved(store)).toMatchObject({ dirty: true, deviceId: "device-1" });
    expect(saved(store).entries[1].sets[0]).toMatchObject({
      completed: true,
      reps: planned.reps,
      weight: planned.weight,
    });
    expect(saved(store).entries[1].sets[1].completed).toBe(false);
    expect(modifiedChips()).toHaveLength(0);

    await fireEvent.press(screen.getByTestId("set-1-0-done"));
    expect(screen.getByTestId("set-1-0-done")).not.toBeChecked();
    expect(saved(store).entries[1].sets[0].completed).toBe(false);
    expect(screen.getByTestId("set-1-0-reps")).toHaveTextContent(`${planned.reps}`);
  });

  it("edits reps and weight inline and marks the set and day modified", async () => {
    const store = await seed();
    const { planned } = saved(store).entries[1].sets[1];
    await mount(store);
    await show(1);
    await fireEvent.press(screen.getByTestId("set-1-1-reps"));
    await fireEvent.changeText(screen.getByLabelText("Reps"), "6");
    expect(saved(store).entries[1].sets[1]).toMatchObject({
      completed: false,
      edited: true,
      reps: 6,
      weight: planned.weight,
    });
    await fireEvent.press(screen.getByLabelText("Weight plus"));
    expect(saved(store).entries[1].sets[1].weight).toEqual({
      value: planned.weight.value + 2.5,
      unit: "kg",
    });
    await fireEvent.press(screen.getByLabelText("Reps minus"));
    expect(saved(store).entries[1].sets[1].reps).toBe(5);
    await fireEvent.changeText(screen.getByLabelText("RPE"), "8.5");
    expect(saved(store).entries[1].sets[1].rpe).toBe(8.5);
    expect(saved(store).modified).toBe(true);
    expect(modifiedChips()).toHaveLength(2);
    expect(screen.getByTestId("exercise-1-modified")).toBeOnTheScreen();

    await fireEvent.press(screen.getByTestId("set-1-1-done"));
    expect(saved(store).entries[1].sets[1]).toMatchObject({ completed: true, reps: 5, rpe: 8.5 });
    expect(saved(store).entries[1].sets[1].weight.value).toBe(planned.weight.value + 2.5);
    // Completing closes the editor.
    expect(screen.queryByLabelText("Reps")).toBeNull();
  });

  it("closes the inline editor when another set completes or the pane is tapped", async () => {
    const store = await seed();
    await mount(store);
    await show(1);
    await fireEvent.press(screen.getByTestId("set-1-1-reps"));
    expect(screen.getByLabelText("Reps")).toBeOnTheScreen();
    await fireEvent.press(screen.getByTestId("set-1-0-done"));
    expect(screen.queryByLabelText("Reps")).toBeNull();

    await fireEvent.press(screen.getByTestId("set-1-1-weight"));
    expect(screen.getByLabelText("Reps")).toBeOnTheScreen();
    await fireEvent.press(screen.getByTestId("exercise-pane"));
    expect(screen.queryByLabelText("Reps")).toBeNull();
  });

  it("edits weight in pounds for a pound user", async () => {
    const store = await seed();
    await mount(store, "lb");
    await show(1);
    await fireEvent.press(screen.getByTestId("set-1-0-weight"));
    await fireEvent.changeText(screen.getByLabelText("Weight (lb)"), "100");
    expect(saved(store).entries[1].sets[0].weight).toEqual({ value: 45.36, unit: "kg" });
    await fireEvent.press(screen.getByLabelText("Weight plus"));
    expect(saved(store).entries[1].sets[0].weight.value).toBeCloseTo(47.63, 2);
    expect(screen.getByLabelText("Weight (lb)")).toHaveDisplayValue("105");
  });

  it("removes one set with its X", async () => {
    const store = await seed();
    const before = saved(store).entries[1].sets.length;
    await mount(store);
    await show(1);
    await fireEvent.press(screen.getByTestId("set-1-1-remove"));
    expect(saved(store).entries[1].sets).toHaveLength(before - 1);
    expect(saved(store).modified).toBe(true);
    expect(saved(store).dirty).toBe(true);
    expect(modifiedChips()).toHaveLength(1);
  });

  it("adds a set copying the last one", async () => {
    const store = await seed();
    const before = saved(store).entries[1].sets;
    await mount(store);
    await show(1);
    await fireEvent.press(screen.getByRole("button", { name: "Add set" }));
    const after = saved(store).entries[1].sets;
    expect(after).toHaveLength(before.length + 1);
    expect(after.at(-1)).toMatchObject({
      edited: true,
      completed: false,
      planned: before.at(-1)!.planned,
      reps: before.at(-1)!.planned.reps,
    });
    expect(screen.getByTestId("exercise-1-modified")).toBeOnTheScreen();
  });

  it("removes an exercise from today after confirming", async () => {
    const store = await seed();
    const [, hackSquat, next] = saved(store).entries;
    await mount(store);
    await show(1);
    await fireEvent.press(screen.getByTestId("exercise-1-menu"));
    await fireEvent.press(screen.getByRole("button", { name: "Remove from today" }));
    await fireEvent.press(screen.getByRole("button", { name: "Remove exercise" }));
    expect(saved(store).entries[1].name).toBe(next.name);
    expect(saved(store).entries.some((e) => e.name === hackSquat.name)).toBe(false);
    expect(saved(store).modified).toBe(true);
    expect(screen.queryByText(hackSquat.name)).toBeNull();
    expect(screen.getByTestId("tile-1")).toHaveAccessibleName(new RegExp(`^${next.name},`));
  });

  it("removes an exercise from the Exercises sheet, opened by long-pressing a tile", async () => {
    const store = await seed();
    const [deadlift, hackSquat] = saved(store).entries;
    await mount(store);
    await fireEvent(screen.getByTestId("tile-0"), "longPress");
    expect(screen.getByTestId("exercises-sheet")).toBeOnTheScreen();
    await fireEvent.press(screen.getByTestId("exercise-1-remove"));
    await fireEvent.press(screen.getByRole("button", { name: "Remove exercise" }));
    expect(saved(store).entries.map((e) => e.name)).not.toContain(hackSquat.name);
    expect(saved(store).entries[0].name).toBe(deadlift.name);
  });

  it("reorders exercises from the menu and by dragging, keeping the record order", async () => {
    const store = await seed();
    const names = () => saved(store).entries.map((e) => e.name);
    const [first, second, third, ...rest] = names();
    await mount(store);
    await show(2);
    await fireEvent.press(screen.getByTestId("exercise-2-menu"));
    await fireEvent.press(screen.getByRole("button", { name: "Move up" }));
    expect(names()).toEqual([first, third, second, ...rest]);
    expect(saved(store).modified).toBeFalsy();
    expect(screen.getByTestId("tile-1")).toHaveAccessibleName(new RegExp(`^${third},`));
    // The pane follows its exercise to the new position.
    expect(within(screen.getByTestId("exercise-1")).getAllByText(third).length).toBeGreaterThan(0);
    expect(screen.getByTestId("tile-1")).toBeSelected();

    await fireEvent.press(screen.getByTestId("reorder-button"));
    for (const i of names().keys()) {
      await fireEvent(screen.getByTestId(`exercises-${i}`), "layout", {
        nativeEvent: { layout: { height: 300 } },
      });
    }
    fireGestureHandler(getByGestureTestId("exercises-0-pan"), [
      { state: State.BEGAN },
      { state: State.ACTIVE },
      { translationY: 400 },
      { state: State.END, translationY: 400 },
    ]);
    await act(async () => {});
    expect(names()).toEqual([third, first, second, ...rest]);
    expect(saved(store).dirty).toBe(true);
    // The pane's exercise (third) moved from 1 to 0 as first passed it.
    expect(screen.getByTestId("tile-0")).toBeSelected();
    expect(screen.getByTestId("exercise-0")).toBeOnTheScreen();
  });

  it("keeps the shown exercise when an earlier one is removed", async () => {
    const store = await seed();
    const [, , third] = saved(store).entries;
    await mount(store);
    await show(2);
    await fireEvent(screen.getByTestId("tile-0"), "longPress");
    await fireEvent.press(screen.getByTestId("exercise-0-remove"));
    await fireEvent.press(screen.getByRole("button", { name: "Remove exercise" }));
    expect(saved(store).entries[1].name).toBe(third.name);
    expect(screen.getByTestId("tile-1")).toBeSelected();
    expect(screen.getByTestId("exercise-1")).toBeOnTheScreen();
  });

  it("sums reps and tonnage against the plan and counts exercises on target", async () => {
    const store = await seed();
    const { entries } = saved(store);
    const all = entries[1].sets;
    const reps = all.reduce((t, s) => t + s.planned.reps, 0);
    const kg = all.reduce((t, s) => t + s.planned.reps * s.planned.weight.value, 0);
    await mount(store);
    await show(1);
    expect(screen.getByText(`0 of ${entries.length} exercises on target`)).toBeOnTheScreen();
    expect(screen.getByTestId("exercise-1-summary")).toHaveTextContent(
      `0 / ${reps} reps · 0 / ${kg} kg`,
    );
    for (const i of all.keys()) await fireEvent.press(screen.getByTestId(`set-1-${i}-done`));
    expect(screen.getByTestId("exercise-1-summary")).toHaveTextContent(
      `On target · ${reps} / ${reps} reps · ${kg} / ${kg} kg`,
    );
    expect(screen.getByText(`1 of ${entries.length} exercises on target`)).toBeOnTheScreen();
    await fireEvent.press(screen.getByTestId("set-1-0-done"));
    expect(screen.getByTestId("exercise-1-summary")).not.toHaveTextContent(/On target/);
    expect(screen.getByText(`0 of ${entries.length} exercises on target`)).toBeOnTheScreen();
  });

  it("logs left and right reps for a unilateral exercise", async () => {
    const store = await seed();
    const split = saved(store).entries[2];
    expect(split).toMatchObject({ name: "Bulgarian Split Squat", unilateral: true });
    const { reps, weight } = split.sets[0].planned;
    const planned = 2 * reps * split.sets.length;
    await mount(store);
    await show(2);
    expect(screen.getByTestId("set-2-0-repsLeft")).toHaveTextContent(`L ${reps}`);
    expect(screen.getByTestId("set-2-0-reps")).toHaveTextContent(`R ${reps}`);
    expect(screen.getByTestId("exercise-2-summary")).toHaveTextContent(
      `0 / ${planned} reps · 0 / ${planned * weight.value} kg`,
    );

    await fireEvent.press(screen.getByTestId("set-2-0-done"));
    expect(saved(store).entries[2].sets[0]).toMatchObject({
      completed: true,
      reps,
      repsLeft: reps,
    });
    expect(screen.getByTestId("exercise-2-summary")).toHaveTextContent(
      `${2 * reps} / ${planned} reps · ${2 * reps * weight.value} / ${planned * weight.value} kg`,
    );

    // Both sides of a done set are edited through the reps bubble; the tick only toggles.
    await fireEvent.press(screen.getByTestId("set-2-0-reps"));
    expect(saved(store).entries[2].sets[0].reps).toBe(reps);
    await fireEvent.changeText(screen.getByLabelText("Left reps"), "6");
    await fireEvent.press(screen.getByLabelText("Right reps minus"));
    expect(saved(store).entries[2].sets[0]).toMatchObject({
      completed: true,
      edited: true,
      reps: reps - 1,
      repsLeft: 6,
    });
    expect(screen.getByTestId("set-2-0-repsLeft")).toHaveTextContent("L 6");
    expect(screen.getByTestId("set-2-0-reps")).toHaveTextContent(`R ${reps - 1}`);
    expect(screen.getByTestId("set-2-0-done")).toHaveAccessibleName(
      `Set 1 done, L 6 R ${reps - 1}, tap to undo`,
    );
  });

  it("reads the unilateral flag when adding an exercise and when switching alternates", async () => {
    const store = await seed();
    await mount(store);
    await fireEvent.press(screen.getByTestId("add-exercise-button"));
    await fireEvent.changeText(screen.getByLabelText("Find an exercise"), "bulgarian");
    await fireEvent(screen.getByLabelText("Find an exercise"), "submitEditing");
    const sheet = within(screen.getByTestId("add-exercise"));
    await fireEvent.press(await sheet.findByRole("button", { name: /Bulgarian Split Squat/ }));
    // In after the shown first exercise, and up in the pane.
    await waitFor(() => expect(saved(store).entries[1]?.unilateral).toBe(true));
    expect(screen.getByTestId("set-1-0-repsLeft")).toBeOnTheScreen();

    // The plan's third exercise is now the fourth.
    await show(3);
    await fireEvent.press(screen.getByTestId("exercise-3-menu"));
    await fireEvent.press(screen.getByLabelText("Lunge, Dumbbell"));
    await waitFor(() =>
      expect(saved(store).entries[3]).toMatchObject({ name: "Lunge, Dumbbell", unilateral: false }),
    );
    expect(screen.queryByTestId("set-3-0-repsLeft")).toBeNull();
    await fireEvent.press(screen.getByTestId("exercise-3-menu"));
    await fireEvent.press(screen.getByLabelText("Bulgarian Split Squat"));
    await waitFor(() => expect(saved(store).entries[3].unilateral).toBe(true));
    expect(screen.getByTestId("set-3-0-repsLeft")).toBeOnTheScreen();
  });

  it("turns an exercise unilateral from its sheet, keeping its 1RM", async () => {
    const store = await seed();
    const spy = jest.spyOn(mockApi.exercises, "setExerciseData");
    const [deadlift] = saved(store).entries;
    const work = deadlift.sets.findIndex((s) => !s.planned.warmup);
    await mount(store);
    expect(screen.queryByTestId(`set-0-${work}-repsLeft`)).toBeNull();
    await fireEvent.press(screen.getByTestId("exercise-0-menu"));
    await fireEvent.press(screen.getByLabelText("Unilateral, Log left and right reps, Off"));
    await waitFor(() => expect(saved(store).entries[0].unilateral).toBe(true));
    expect(spy.mock.calls[0][0].data).toMatchObject({
      slug: deadlift.slug,
      unilateral: true,
      rm1: { value: 140 },
    });
    expect(screen.getByLabelText("Unilateral, Log left and right reps, On")).toBeOnTheScreen();
    expect(screen.getByTestId(`set-0-${work}-repsLeft`)).toBeOnTheScreen();
    expect(saved(store).modified).toBeFalsy();
  });

  it("closes the inline set editor on hardware back instead of leaving", async () => {
    const handlers: Parameters<typeof BackHandler.addEventListener>[1][] = [];
    jest.spyOn(BackHandler, "addEventListener").mockImplementation((_, h) => {
      handlers.push(h);
      return { remove: () => handlers.splice(handlers.indexOf(h), 1) };
    });
    const store = await seed();
    await mount(store);
    expect(handlers).toHaveLength(0);
    await show(1);
    await fireEvent.press(screen.getByTestId("set-1-1-reps"));
    expect(screen.getByLabelText("Reps")).toBeOnTheScreen();
    await act(() =>
      expect(handlers.at(-1)!({ type: "hardwareBackPress", timeStamp: 0 })).toBe(true),
    );
    expect(screen.queryByLabelText("Reps")).toBeNull();
    expect(handlers).toHaveLength(0);
    jest.restoreAllMocks();
  });

  it("keeps Add exercise, Reorder and Finish in a row under the carousel, which holds only exercises", async () => {
    const store = await seed();
    await mount(store);
    const carousel = within(screen.getByTestId("exercise-carousel"));
    expect(carousel.getAllByRole("button").map((b) => b.props.testID)).toEqual(
      saved(store).entries.map((_, i) => `tile-${i}`),
    );
    const actions = within(screen.getByTestId("session-actions"));
    expect(actions.getAllByRole("button").map((b) => b.props.accessibilityLabel)).toEqual([
      "Add exercise",
      "Reorder",
      "Finish",
    ]);
    // With a large system font only "Add exercise" gives way, wrapping between its words, so
    // Finish stays on screen and no word breaks.
    expect(screen.getByTestId("add-exercise-button")).toHaveStyle({ flexGrow: 1, flexShrink: 1 });
    for (const id of ["reorder-button", "finish-workout"]) {
      expect(screen.getByTestId(id)).toHaveStyle({ flexGrow: 1, flexShrink: 0 });
    }
    await fireEvent.press(screen.getByTestId("reorder-button"));
    expect(within(screen.getByTestId("exercises-sheet")).getByText("Reorder")).toBeOnTheScreen();
  });

  it("adds an ad hoc exercise from the catalog search", async () => {
    const store = await seed();
    const before = saved(store).entries.length;
    await mount(store);
    await fireEvent.press(screen.getByTestId("add-exercise-button"));
    jest.useFakeTimers();
    await fireEvent.changeText(screen.getByLabelText("Find an exercise"), "hammer");
    expect(screen.queryByRole("button", { name: /Hammer Curl/ })).toBeNull();
    await act(() => jest.advanceTimersByTime(250));
    await fireEvent.press(await screen.findByRole("button", { name: /Hammer Curl/ }));
    jest.useRealTimers();
    // In after the shown exercise (the first), and shown.
    const entries = saved(store).entries;
    expect(entries).toHaveLength(before + 1);
    const added = entries[1];
    expect(added).toMatchObject({
      name: "Hammer Curl",
      slug: "hammer-curl-dumbbell",
      equipment: "dumbbell",
      fromProgram: false,
    });
    expect(added.sets).toHaveLength(1);
    expect(added.sets[0]).toMatchObject({ edited: true, completed: false });
    expect(saved(store).modified).toBe(true);
    expect(screen.getByTestId("exercise-1-modified")).toBeOnTheScreen();
    expect(screen.getByTestId("set-1-0-reps")).toBeOnTheScreen();
    expect(screen.getByTestId("exercise-1-summary")).toHaveTextContent(/^0 reps$/);
    expect(screen.getByTestId("tile-1")).toBeSelected();
  });

  it("adds an exercise during the shown exercise's rest without stopping it", async () => {
    const store = await seed();
    const work = saved(store).entries[0].sets.findIndex((s) => !s.planned.warmup);
    await mount(store);
    await fireEvent.press(screen.getByTestId(`set-0-${work}-done`));
    expect(screen.getByTestId("rest-timer")).toBeOnTheScreen();
    await fireEvent.press(screen.getByTestId("add-exercise-button"));
    await fireEvent.changeText(screen.getByLabelText("Find an exercise"), "hammer");
    await fireEvent(screen.getByLabelText("Find an exercise"), "submitEditing");
    const sheet = within(screen.getByTestId("add-exercise"));
    await fireEvent.press(await sheet.findByRole("button", { name: /Hammer Curl/ }));
    await waitFor(() => expect(saved(store).entries[1]?.name).toBe("Hammer Curl"));
    expect(screen.getByTestId("rest-timer")).toBeOnTheScreen();
  });

  it("offers recent exercises not in today's workout", async () => {
    const store = await seed();
    const [deadlift] = saved(store).entries;
    workoutRecordsRepo(store).upsertFromServer([
      {
        ...saved(store),
        id: "workout-2026-11-09",
        date: "2026-11-09",
        entries: [
          deadlift,
          { ...deadlift, name: "Bicep Curl", slug: "bicep-curl", equipment: "dumbbell" },
        ],
      },
    ]);
    await mount(store);
    await fireEvent.press(screen.getByTestId("add-exercise-button"));
    expect(screen.queryByLabelText(deadlift.name)).toBeNull();
    await fireEvent.press(screen.getByLabelText("Bicep Curl"));
    expect(saved(store).entries[1]).toMatchObject({
      name: "Bicep Curl",
      slug: "bicep-curl",
      fromProgram: false,
    });
  });

  it("switches to an alternate and keeps the completed sets", async () => {
    const store = await seed();
    const { alternates } = saved(store).entries[1];
    await mount(store);
    await show(1);
    await fireEvent.press(screen.getByTestId("set-1-0-done"));
    await fireEvent.press(screen.getByRole("button", { name: `${alternates[0]} options` }));
    expect(screen.getByLabelText(`${alternates[0]}, Current`)).toBeOnTheScreen();
    expect(screen.getByText("Unilateral")).toBeOnTheScreen();
    await fireEvent.press(screen.getByLabelText(alternates[1]));
    expect(saved(store).entries[1]).toMatchObject({ name: alternates[1], current: 2 });
    expect(saved(store).entries[1].sets[0].completed).toBe(true);
    expect(screen.getByTestId("set-1-0-done")).toBeChecked();
  });

  it("shows the exercise's muscles and opens its details from the menu", async () => {
    const store = await seed();
    const [deadlift] = saved(store).entries;
    await mount(store);
    await fireEvent.press(screen.getByRole("button", { name: `${deadlift.name} options` }));
    expect(await screen.findByText("lower back, hamstrings · barbell")).toBeOnTheScreen();
    await fireEvent.press(screen.getByRole("button", { name: /^Details/ }));
    expect(mockRouter.push).toHaveBeenCalledWith(`/exercise/${deadlift.slug}`);
  });

  it("lists warmups as rows of their own and counts them in the totals", async () => {
    const store = await seed();
    const { sets } = saved(store).entries[0];
    const warmups = sets.filter((s) => s.planned.warmup);
    expect(warmups.length).toBeGreaterThan(0);
    const reps = sets.reduce((t, s) => t + s.planned.reps, 0);
    await mount(store);
    expect(screen.getByTestId("set-0-0-caption")).toHaveTextContent(/^Warmup/);
    expect(screen.getByTestId(`set-0-${warmups.length}-caption`)).toHaveTextContent(/^Work/);
    expect(screen.getByTestId("exercise-0-summary")).toHaveTextContent(
      new RegExp(`^0 / ${reps} reps`),
    );
    await fireEvent.press(screen.getByTestId("set-0-0-done"));
    expect(saved(store).entries[0].sets[0]).toMatchObject({
      completed: true,
      reps: warmups[0].planned.reps,
    });
    expect(screen.getByTestId("exercise-0-summary")).toHaveTextContent(
      new RegExp(`^${warmups[0].planned.reps} / ${reps} reps`),
    );
    expect(screen.getByTestId("tile-0")).toHaveAccessibleName(/, 1 of /);
  });

  it("keeps timer notifications quiet while it is open, since it beeps itself", async () => {
    const { unmount } = await mount(await seed());
    expect(await notifies()).toBe(false);
    await unmount();
    expect(await notifies()).toBe(true);
  });

  it("counts down the planned rest and schedules a notification for its end", async () => {
    jest.useFakeTimers();
    const store = await seed();
    const work = saved(store).entries[0].sets.findIndex((s) => !s.planned.warmup);
    const { rest } = saved(store).entries[0].sets[work].planned;
    expect(rest).toBe(180);
    await mount(store);
    expect(screen.queryByTestId("rest-timer")).toBeNull();

    const startedAt = Date.now();
    await fireEvent.press(screen.getByTestId(`set-0-${work}-done`));
    expect(screen.getByText("3:00")).toBeOnTheScreen();
    await act(() => jest.advanceTimersByTime(1000));
    expect(screen.getByText("2:59")).toBeOnTheScreen();
    expect(Notifications.scheduleNotificationAsync).toHaveBeenCalledWith(
      expect.objectContaining({
        trigger: { type: "date", date: startedAt + rest * 1000 },
      }),
    );

    await fireEvent.press(screen.getByRole("button", { name: "+10 s" }));
    expect(screen.getByText("3:09")).toBeOnTheScreen();
    await fireEvent.press(screen.getByTestId("rest-timer-minus"));
    await fireEvent.press(screen.getByTestId("rest-timer-minus"));
    expect(screen.getByText("2:49")).toBeOnTheScreen();
    await act(async () => {});
    expect(Notifications.scheduleNotificationAsync).toHaveBeenLastCalledWith(
      expect.objectContaining({ trigger: { type: "date", date: startedAt + (rest - 10) * 1000 } }),
    );

    await fireEvent.press(screen.getByRole("button", { name: "Skip" }));
    await act(async () => {});
    expect(screen.queryByTestId("rest-timer")).toBeNull();
    expect(Notifications.cancelScheduledNotificationAsync).toHaveBeenCalledWith("n1");
    jest.useRealTimers();
  });

  it("beeps when a work timer starts and ends and when the rest ends, not on a plain tick", async () => {
    jest.useFakeTimers();
    const store = await seed();
    const r = saved(store);
    workoutRecordsRepo(store).upsertFromServer([
      {
        ...r,
        entries: r.entries.map((e, i) =>
          i === 1
            ? {
                ...e,
                sets: e.sets.map((s) => ({ ...s, planned: { ...s.planned, work: 20, rest: 30 } })),
              }
            : e,
        ),
      },
    ]);
    await mount(store);
    await fireEvent.press(screen.getByTestId("set-0-0-done"));
    await act(async () => {});
    expect(beeps()).toBe(0);

    await show(1);
    await fireEvent.press(screen.getByTestId("set-1-0-done"));
    await act(async () => {});
    expect(beeps()).toBe(1);
    expect(mockPlayer.seekTo).toHaveBeenCalledWith(0);
    await act(() => jest.advanceTimersByTime(20_000));
    await act(async () => {});
    expect(beeps()).toBe(2);
    expect(screen.getByTestId("rest-timer")).toBeOnTheScreen();
    await act(() => jest.advanceTimersByTime(30_000));
    await act(async () => {});
    expect(beeps()).toBe(3);
    jest.useRealTimers();
  });

  it("starts a count-up set with play and stops it with the elapsed work time", async () => {
    jest.useFakeTimers();
    const store = await seed();
    const i = saved(store).entries.findIndex((e) => e.sets[0].planned.countUp);
    const { name, sets } = saved(store).entries[i];
    expect(sets[0].planned).toMatchObject({ work: 30, rest: 60 });
    await mount(store);
    await show(i);
    expect(screen.getByTestId(`set-${i}-0-done`)).toHaveAccessibleName("Start set 1");
    expect(screen.getByTestId(`set-${i}-0`)).toHaveTextContent(/work 0:30 · rest 1:00/);

    await fireEvent.press(screen.getByTestId(`set-${i}-0-done`));
    const bar = within(screen.getByTestId("set-timer"));
    expect(bar.getByText(name)).toBeOnTheScreen();
    expect(bar.getByText("Set 1")).toBeOnTheScreen();
    expect(bar.getByText("0:00 / 0:30")).toBeOnTheScreen();
    expect(screen.getByTestId(`set-${i}-0-done`)).toHaveAccessibleName("Stop set 1");
    expect(Notifications.scheduleNotificationAsync).not.toHaveBeenCalled();
    await act(() => jest.advanceTimersByTime(35_000));
    expect(bar.getByText("0:35 / 0:30")).toBeOnTheScreen();
    expect(saved(store).entries[i].sets[0].completed).toBe(false);

    await fireEvent.press(screen.getByRole("button", { name: "Stop" }));
    expect(saved(store).entries[i].sets[0]).toMatchObject({
      completed: true,
      workTime: 35,
      reps: sets[0].planned.reps,
    });
    expect(screen.queryByTestId("set-timer")).toBeNull();
    expect(within(screen.getByTestId("rest-timer")).getByText("1:00")).toBeOnTheScreen();
    jest.useRealTimers();
  });

  it("cancels a running set timer when exercises are reordered or removed", async () => {
    jest.useFakeTimers();
    const store = await seed();
    const i = saved(store).entries.findIndex((e) => e.sets[0].planned.countUp);
    await mount(store);
    await show(i);
    await fireEvent.press(screen.getByTestId(`set-${i}-0-done`));
    expect(screen.getByTestId("set-timer")).toBeOnTheScreen();

    await fireEvent.press(screen.getByTestId(`exercise-${i}-menu`));
    await fireEvent.press(screen.getByRole("button", { name: i === 0 ? "Move down" : "Move up" }));
    expect(screen.queryByTestId("set-timer")).toBeNull();
    expect(saved(store).entries.every((e) => e.sets.every((s) => !s.completed))).toBe(true);
    jest.useRealTimers();
  });

  it("completes a timed set when its countdown ends and auto chains the next after the rest", async () => {
    jest.useFakeTimers();
    const store = await seed();
    const r = saved(store);
    workoutRecordsRepo(store).upsertFromServer([
      {
        ...r,
        entries: r.entries.map((e, i) =>
          i === 1
            ? {
                ...e,
                sets: e.sets.map((s) => ({
                  ...s,
                  planned: { ...s.planned, work: 45, rest: 30, auto: true },
                })),
              }
            : e,
        ),
      },
    ]);
    await mount(store);
    await show(1);
    const startedAt = Date.now();
    await fireEvent.press(screen.getByTestId("set-1-0-done"));
    expect(within(screen.getByTestId("set-timer")).getByText("0:45")).toBeOnTheScreen();
    expect(Notifications.scheduleNotificationAsync).toHaveBeenCalledWith({
      content: { title: "Set is over", body: "Stop and rest." },
      trigger: { type: "date", date: startedAt + 45_000 },
    });

    await act(() => jest.advanceTimersByTime(45_000));
    expect(saved(store).entries[1].sets[0]).toMatchObject({ completed: true, workTime: 45 });
    expect(screen.queryByTestId("set-timer")).toBeNull();
    expect(within(screen.getByTestId("rest-timer")).getByText("0:30")).toBeOnTheScreen();
    expect(Notifications.scheduleNotificationAsync).toHaveBeenLastCalledWith({
      content: { title: "Rest is over", body: "Time for the next set." },
      trigger: { type: "date", date: startedAt + 75_000 },
    });

    await act(() => jest.advanceTimersByTime(30_000));
    expect(screen.queryByTestId("rest-timer")).toBeNull();
    expect(within(screen.getByTestId("set-timer")).getByText("Set 2")).toBeOnTheScreen();
    expect(screen.getByTestId("set-1-1-done")).toHaveAccessibleName("Stop set 2");

    await finishTile();
    await act(async () => {});
    expect(screen.queryByTestId("set-timer")).toBeNull();
    expect(Notifications.cancelScheduledNotificationAsync).toHaveBeenCalledWith("n1");
    expect(saved(store).entries[1].sets[1].completed).toBe(false);
    jest.useRealTimers();
  });

  it("runs the rest and the auto chain from when the set ended while the app was away", async () => {
    jest.useFakeTimers();
    const store = await seed();
    const r = saved(store);
    workoutRecordsRepo(store).upsertFromServer([
      {
        ...r,
        entries: r.entries.map((e, i) =>
          i === 1
            ? {
                ...e,
                sets: e.sets.map((s) => ({
                  ...s,
                  planned: { ...s.planned, work: 45, rest: 30, auto: true },
                })),
              }
            : e,
        ),
      },
    ]);
    await mount(store);
    await show(1);
    const startedAt = Date.now();
    await fireEvent.press(screen.getByTestId("set-1-0-done"));
    await act(async () => {});
    // Both ends are scheduled now, since no JS runs while the app is in the background.
    expect(jest.mocked(Notifications.scheduleNotificationAsync).mock.calls).toEqual([
      [
        {
          content: { title: "Set is over", body: "Stop and rest." },
          trigger: { type: "date", date: startedAt + 45_000 },
        },
      ],
      [
        {
          content: { title: "Rest is over", body: "Time for the next set." },
          trigger: { type: "date", date: startedAt + 75_000 },
        },
      ],
    ]);

    // Backgrounded: the next tick runs when the app is back, 61 s after play.
    jest.setSystemTime(startedAt + 60_000);
    await act(() => jest.advanceTimersByTime(1000));
    expect(saved(store).entries[1].sets[0]).toMatchObject({ completed: true, workTime: 45 });
    expect(within(screen.getByTestId("rest-timer")).getByText("0:14")).toBeOnTheScreen();
    expect(Notifications.scheduleNotificationAsync).toHaveBeenCalledTimes(2);

    jest.setSystemTime(startedAt + 80_000);
    await act(() => jest.advanceTimersByTime(1000));
    expect(within(screen.getByTestId("set-timer")).getByText("Set 2")).toBeOnTheScreen();
    expect(within(screen.getByTestId("set-timer")).getByText("0:39")).toBeOnTheScreen();
    jest.useRealTimers();
  });

  it("cancels the rest scheduled with a timed set when the set is stopped early", async () => {
    jest.useFakeTimers();
    const store = await seed();
    const r = saved(store);
    workoutRecordsRepo(store).upsertFromServer([
      {
        ...r,
        entries: r.entries.map((e, i) =>
          i === 1
            ? {
                ...e,
                sets: e.sets.map((s) => ({ ...s, planned: { ...s.planned, work: 45, rest: 30 } })),
              }
            : e,
        ),
      },
    ]);
    jest
      .mocked(Notifications.scheduleNotificationAsync)
      .mockResolvedValueOnce("work")
      .mockResolvedValueOnce("rest");
    await mount(store);
    await show(1);
    await fireEvent.press(screen.getByTestId("set-1-0-done"));
    await act(() => jest.advanceTimersByTime(10_000));
    const stoppedAt = Date.now();
    await fireEvent.press(screen.getByRole("button", { name: "Stop" }));
    await act(async () => {});
    expect(jest.mocked(Notifications.cancelScheduledNotificationAsync).mock.calls).toEqual([
      ["work"],
      ["rest"],
    ]);
    expect(Notifications.scheduleNotificationAsync).toHaveBeenLastCalledWith({
      content: { title: "Rest is over", body: "Time for the next set." },
      trigger: { type: "date", date: stoppedAt + 30_000 },
    });
    jest.useRealTimers();
  });

  it("finishes the workout, asks for finalize and goes back to Today", async () => {
    jest.useFakeTimers({ now: new Date("2026-11-11T08:30:00.000Z") });
    const store = await seed();
    await mount(store);
    await show(1);
    await fireEvent.press(screen.getByTestId("set-1-0-done"));
    await finishTile();
    expect(saved(store)).toMatchObject({
      finishedAt: "2026-11-11T08:30:00.000Z",
      finalizeRequested: true,
      finalized: false,
      dirty: true,
    });
    expect(saved(store).entries[1].sets[0].completed).toBe(true);
    expect(saved(store).entries[1].sets[1].completed).toBe(false);
    expect(mockRouter.back).toHaveBeenCalledTimes(1);
    jest.useRealTimers();
  });

  it("leaves out an empty equipment line and the tonnage of a bodyweight exercise", async () => {
    const store = await seed();
    const r = saved(store);
    workoutRecordsRepo(store).upsertFromServer([
      {
        ...r,
        entries: r.entries.map((e, i) =>
          i === 1
            ? {
                ...e,
                equipment: "",
                label: "",
                sets: e.sets.map((s) => ({
                  ...s,
                  weight: { value: 0, unit: "kg" as const },
                  planned: { ...s.planned, weight: { value: 0, unit: "kg" as const } },
                })),
              }
            : e,
        ),
      },
    ]);
    await mount(store);
    await show(1);
    const reps = r.entries[1].sets.reduce((n, s) => n + s.planned.reps, 0);
    expect(screen.getByTestId("exercise-1-summary")).toHaveTextContent(`0 / ${reps} reps`, {
      exact: true,
    });
    expect(screen.queryByTestId("exercise-1-equipment")).toBeNull();
  });

  it("keeps a finished workout editable without a Finish button and says edits count", async () => {
    const store = await seed();
    const r = saved(store);
    const done = { ...r.entries[1].sets[0], completed: true, reps: 7 };
    workoutRecordsRepo(store).upsertFromServer([
      {
        ...r,
        finalized: true,
        finishedAt: "2026-11-11T08:30:00.000Z",
        entries: r.entries.map((e, i) =>
          i === 1 ? { ...e, sets: [done, ...e.sets.slice(1)] } : e,
        ),
      },
    ]);
    await mount(store);
    expect(
      screen.getByText(/^Finished 11 Nov 2026, .*\. Edits update the program's next sessions\.$/),
    ).toBeOnTheScreen();
    expect(screen.queryByTestId("finish-workout")).toBeNull();
    expect(screen.getByTestId("add-exercise-button")).toBeOnTheScreen();
    expect(screen.getByTestId("reorder-button")).toBeOnTheScreen();

    await show(1);
    expect(screen.getByTestId("set-1-0-done")).toHaveAccessibleName(
      "Set 1 done, 7 reps, tap to undo",
    );
    await fireEvent.press(screen.getByTestId("set-1-1-done"));
    expect(saved(store)).toMatchObject({ dirty: true, finalized: true });
    expect(saved(store).entries[1].sets[1].completed).toBe(true);
    expect(screen.getByRole("button", { name: "Add set" })).toBeOnTheScreen();
    expect(screen.getByTestId("set-1-0-remove")).toBeOnTheScreen();
    await fireEvent.press(screen.getByTestId("set-1-1-reps"));
    expect(screen.getByTestId("set-1-1-editor")).toBeOnTheScreen();

    await fireEvent.press(screen.getByTestId("exercise-1-menu"));
    expect(screen.getByRole("button", { name: "Remove from today" })).toBeOnTheScreen();
  });

  it("notes that finishing a past workout of an active program counts for it", async () => {
    const note = "Finishing this past workout counts for the program.";
    const store = await seed();
    const record = saved(store);
    workoutRecordsRepo(store).upsertFromServer([{ ...record, date: "2020-01-01" }]);
    const { unmount } = await mount(store);
    expect(screen.queryByText(note)).toBeNull();
    unmount();

    const { programs } = await mockApi.programs.listPrograms({});
    programsRepo(store).upsertFromServer(programs.map((p) => fromProto.program(p)));
    expect(programsRepo(store).get(record.programId!)?.status).toBe("active");
    await mount(store);
    expect(screen.getByText(note)).toBeOnTheScreen();
  });

  it("shows the record from the store again after a remount", async () => {
    const store = await seed();
    const first = await mount(store);
    await show(1);
    await fireEvent.press(screen.getByTestId("set-1-0-done"));
    await first.unmount();

    await mount(store);
    await show(1);
    expect(screen.getByTestId("set-1-0-done")).toBeChecked();
    expect(screen.getByTestId("set-1-1-done")).not.toBeChecked();
  });

  it("writes the record outside React's render, so store listeners may set state", async () => {
    // The sync runner sets state from store.onChange; a save inside a state updater runs
    // during render and React rejects the nested update.
    function Listener() {
      const store = useStore();
      const [n, setN] = useState(0);
      useEffect(() => store.onChange(() => setN((x) => x + 1)), [store]);
      return <Text testID="changes">{n}</Text>;
    }
    const errors = jest.spyOn(console, "error").mockImplementation(() => {});
    const store = await seed();
    await render(
      <ThemeProvider>
        <StoreProvider store={store}>
          <Listener />
          <GestureHandlerRootView>
            <WorkoutScreen id={id} unit="kg" />
          </GestureHandlerRootView>
        </StoreProvider>
      </ThemeProvider>,
    );
    await show(1);
    // Two taps in one batch: React cannot run the second updater eagerly, so it runs in render.
    await act(() => {
      fireEvent.press(screen.getByTestId("set-1-0-done"));
      fireEvent.press(screen.getByTestId("set-1-1-done"));
    });
    expect(screen.getByTestId("changes")).toHaveTextContent("2");
    expect(errors.mock.calls.flat().join(" ")).not.toMatch(/Cannot update a component/);
    errors.mockRestore();
  });

  it("syncs on pull-to-refresh and shows the sets the sync stored", async () => {
    const store = await seed();
    // The sync completes the second exercise's first set on the server's copy and moves
    // lastSyncedAt, as a real run does.
    function Harness() {
      const [syncedAt, setSyncedAt] = useState("2026-11-11T09:00:00Z");
      const syncNow = async () => {
        const r = saved(store);
        const entries = r.entries.map((e, i) =>
          i === 1
            ? { ...e, sets: e.sets.map((s, j) => (j === 0 ? { ...s, completed: true } : s)) }
            : e,
        );
        workoutRecordsRepo(store).upsertFromServer([{ ...r, version: r.version + 1, entries }]);
        setSyncedAt("2026-11-11T09:01:00Z");
      };
      return (
        <SyncProvider value={{ lastSyncedAt: syncedAt, syncNow }}>
          <WorkoutScreen id={id} unit="kg" />
        </SyncProvider>
      );
    }
    await render(
      <ThemeProvider>
        <StoreProvider store={store}>
          <GestureHandlerRootView>
            <Harness />
          </GestureHandlerRootView>
        </StoreProvider>
      </ThemeProvider>,
    );
    await show(1);
    expect(screen.getByTestId("set-1-0-done")).not.toBeChecked();
    await pullToRefresh();
    expect(screen.getByTestId("set-1-0-done")).toBeChecked();
  });

  it("keeps the add sheet and a new exercise through sync pulls (P2-R36)", async () => {
    const store = await seed();
    const before = saved(store);
    // A pull lands rows in the store and moves lastSyncedAt; the screen element itself stays.
    function Harness() {
      const [syncedAt, setSyncedAt] = useState("2026-11-11T09:00:00Z");
      const [child] = useState(() => <WorkoutScreen id={id} unit="kg" />);
      return (
        <SyncProvider value={{ lastSyncedAt: syncedAt }}>
          {child}
          <Text onPress={() => setSyncedAt(new Date().toISOString())}>pulled</Text>
        </SyncProvider>
      );
    }
    await render(
      <ThemeProvider>
        <StoreProvider store={store}>
          <GestureHandlerRootView>
            <Harness />
          </GestureHandlerRootView>
        </StoreProvider>
      </ThemeProvider>,
    );
    await fireEvent.press(screen.getByTestId("add-exercise-button"));
    await fireEvent.changeText(screen.getByLabelText("Find an exercise"), "bulgarian");
    // A newer server copy of a clean record is taken, and the sheet keeps its search.
    workoutRecordsRepo(store).upsertFromServer([{ ...before, version: before.version + 1 }]);
    await fireEvent.press(screen.getByText("pulled"));
    expect(saved(store).version).toBe(before.version + 1);
    expect(screen.getByTestId("add-exercise")).toBeOnTheScreen();
    expect(screen.getByLabelText("Find an exercise")).toHaveDisplayValue("bulgarian");

    await fireEvent(screen.getByLabelText("Find an exercise"), "submitEditing");
    const sheet = within(screen.getByTestId("add-exercise"));
    await fireEvent.press(await sheet.findByRole("button", { name: /Bulgarian Split Squat/ }));
    await waitFor(() => expect(saved(store).entries).toHaveLength(before.entries.length + 1));
    expect(saved(store).dirty).toBe(true);
    await fireEvent.press(screen.getByTestId("set-1-0-reps"));
    await fireEvent.changeText(screen.getByLabelText("Right reps"), "7");

    // An older server copy, and then a newer one, never replace the dirty local record; the
    // editor stays open on the added exercise.
    workoutRecordsRepo(store).upsertFromServer([{ ...before, version: before.version + 1 }]);
    await fireEvent.press(screen.getByText("pulled"));
    workoutRecordsRepo(store).upsertFromServer([{ ...before, version: before.version + 2 }]);
    await fireEvent.press(screen.getByText("pulled"));
    expect(saved(store).entries).toHaveLength(before.entries.length + 1);
    expect(saved(store).entries[1]).toMatchObject({ name: "Bulgarian Split Squat" });
    expect(screen.getByTestId("exercise-1")).toBeOnTheScreen();
    expect(screen.getByLabelText("Right reps")).toHaveDisplayValue("7");
  });

  it("picks the record up when a sync lands it after a deep link", async () => {
    const empty = createMemoryStore();
    // Changes the sync context without re-rendering the screen element, as a finished sync does.
    function Harness() {
      const [syncedAt, setSyncedAt] = useState("2026-11-11T09:00:00Z");
      const [child] = useState(() => <WorkoutScreen id={id} unit="kg" />);
      return (
        <SyncProvider value={{ lastSyncedAt: syncedAt }}>
          {child}
          <Text onPress={() => setSyncedAt("2026-11-11T09:01:00Z")}>synced</Text>
        </SyncProvider>
      );
    }
    await render(
      <ThemeProvider>
        <StoreProvider store={empty}>
          <GestureHandlerRootView>
            <Harness />
          </GestureHandlerRootView>
        </StoreProvider>
      </ThemeProvider>,
    );
    expect(screen.getByText("Workout not found")).toBeOnTheScreen();
    workoutRecordsRepo(empty).upsertFromServer([saved(await seed())]);
    await fireEvent.press(screen.getByText("synced"));
    expect(screen.getByTestId("set-0-0")).toBeOnTheScreen();
  });

  it("follows the record a sync merged into another device's record of the day (P2-R41)", async () => {
    const store = await seed();
    function Harness() {
      const [syncedAt, setSyncedAt] = useState("2026-11-11T09:00:00Z");
      const [child] = useState(() => <WorkoutScreen id={id} unit="kg" />);
      return (
        <SyncProvider value={{ lastSyncedAt: syncedAt }}>
          {child}
          <Text onPress={() => setSyncedAt("2026-11-11T09:01:00Z")}>synced</Text>
        </SyncProvider>
      );
    }
    await render(
      <ThemeProvider>
        <StoreProvider store={store}>
          <GestureHandlerRootView>
            <Harness />
          </GestureHandlerRootView>
        </StoreProvider>
      </ThemeProvider>,
    );
    const mine = saved(store);
    store.remove("workoutRecords", id);
    store.put("workoutRecords", { ...mine, id: "theirs", version: 4 });
    await fireEvent.press(screen.getByText("synced"));
    expect(screen.queryByText("Workout not found")).toBeNull();

    await fireEvent.press(screen.getByTestId("set-0-0-done"));
    expect(store.get("workoutRecords", id)).toBeUndefined();
    expect(store.get("workoutRecords", "theirs")).toMatchObject({ version: 4, dirty: true });
  });

  it("explains when the record is missing", async () => {
    await mount(createMemoryStore());
    expect(screen.getByText("Workout not found")).toBeOnTheScreen();
  });
});
