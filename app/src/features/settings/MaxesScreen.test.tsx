import { act, fireEvent, screen, waitFor } from "@testing-library/react-native";

import type { Api } from "../../api/clients";
import { createMemoryStore } from "../../data/memoryStore";
import type { SessionState } from "../../data/session";
import { Unit } from "../../gen/plusultra/v1/common_pb";
import MaxesScreen from "./MaxesScreen";
import { demoSession, newMock, renderSettings } from "./test-utils";
import { SETTINGS_KEY } from "../../data/settings";

let mockApi: Api;
let mockSession: SessionState;
jest.mock("../../api/useApi", () => ({ useApi: () => mockApi }));
jest.mock("../../data/session", () => ({
  ...jest.requireActual("../../data/session"),
  useSession: () => mockSession,
}));
jest.mock("expo-router", () => ({
  useRouter: () => ({ back: jest.fn(), push: jest.fn(), canGoBack: () => true }),
}));

beforeEach(() => {
  ({ api: mockApi } = newMock());
  mockSession = demoSession();
});

describe("MaxesScreen", () => {
  it("sets and clears a 1RM", async () => {
    const spy = jest.spyOn(mockApi.exercises, "setExerciseData");
    await renderSettings(<MaxesScreen />);
    expect(await screen.findByText("102.5 kg")).toBeOnTheScreen();

    await fireEvent.press(screen.getByRole("button", { name: /^Squat Barbell/ }));
    await fireEvent.changeText(screen.getByLabelText("1RM (kg)"), "110");
    await fireEvent.press(screen.getByRole("button", { name: "Save" }));
    await waitFor(() => expect(spy).toHaveBeenCalledTimes(1));
    expect(spy.mock.calls[0][0].data).toMatchObject({
      slug: "squat-barbell",
      rm1: { value: 110, unit: Unit.KG },
    });
    expect(await screen.findByText("110 kg")).toBeOnTheScreen();

    await fireEvent.press(screen.getByRole("button", { name: /^Squat Barbell/ }));
    await fireEvent.press(screen.getByRole("button", { name: "Clear 1RM" }));
    await waitFor(() => expect(spy).toHaveBeenCalledTimes(2));
    expect(spy.mock.calls[1][0].data?.slug).toBe("squat-barbell");
    expect(spy.mock.calls[1][0].data?.rm1).toBeUndefined();
    expect(screen.queryByText("110 kg")).toBeNull();
  });

  it("marks an exercise unilateral next to its 1RM", async () => {
    const spy = jest.spyOn(mockApi.exercises, "setExerciseData");
    await renderSettings(<MaxesScreen />);
    await fireEvent.press(await screen.findByRole("button", { name: /^Squat Barbell/ }));
    await fireEvent.press(screen.getByRole("checkbox", { name: /^Unilateral/ }));
    await fireEvent.press(screen.getByRole("button", { name: "Save" }));
    await waitFor(() => expect(spy).toHaveBeenCalledTimes(1));
    expect(spy.mock.calls[0][0].data).toMatchObject({
      slug: "squat-barbell",
      rm1: { value: 102.5 },
      unilateral: true,
    });
  });

  it("saves the unilateral flag for an exercise without a 1RM", async () => {
    const spy = jest.spyOn(mockApi.exercises, "setExerciseData");
    await renderSettings(<MaxesScreen />);
    jest.useFakeTimers();
    await fireEvent.changeText(screen.getByLabelText("Find an exercise"), "incline bench");
    await act(() => jest.advanceTimersByTime(250));
    await fireEvent.press(await screen.findByRole("button", { name: /^Incline Bench Press/ }));
    jest.useRealTimers();
    await fireEvent.press(screen.getByRole("checkbox", { name: /^Unilateral/ }));
    await fireEvent.press(screen.getByRole("button", { name: "Save" }));
    await waitFor(() => expect(spy).toHaveBeenCalledTimes(1));
    expect(spy.mock.calls[0][0].data).toMatchObject({
      slug: "incline-bench-press-barbell",
      unilateral: true,
    });
    expect(spy.mock.calls[0][0].data?.rm1).toBeUndefined();
  });

  it("finds an exercise and works in the user's unit", async () => {
    const store = createMemoryStore();
    store.setKv(SETTINGS_KEY, { displayName: "Demo", units: "lb", measurementUnit: "cm" });
    const spy = jest.spyOn(mockApi.exercises, "setExerciseData");
    await renderSettings(<MaxesScreen />, store);
    expect(await screen.findByText("226 lb")).toBeOnTheScreen();

    jest.useFakeTimers();
    await fireEvent.changeText(screen.getByLabelText("Find an exercise"), "incline bench");
    await act(() => jest.advanceTimersByTime(250));
    await fireEvent.press(await screen.findByRole("button", { name: /^Incline Bench Press/ }));
    jest.useRealTimers();
    await fireEvent.changeText(screen.getByLabelText("1RM (lb)"), "200");
    await fireEvent.press(screen.getByRole("checkbox", { name: "Custom rounding" }));
    await fireEvent.press(screen.getByRole("checkbox", { name: "stack" }));
    await fireEvent.changeText(screen.getByLabelText("Increment (lb)"), "5");
    await fireEvent.press(screen.getByRole("button", { name: "Save" }));
    await waitFor(() => expect(spy).toHaveBeenCalledTimes(1));
    expect(spy.mock.calls[0][0].data).toMatchObject({
      slug: "incline-bench-press-barbell",
      rm1: { value: 200, unit: Unit.LB },
      rounding: { kind: "stack", increment: 5 },
    });
    expect(await screen.findByText("200 lb")).toBeOnTheScreen();
  });
});
