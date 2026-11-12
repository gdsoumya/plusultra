import { act, fireEvent, screen, waitFor, within } from "@testing-library/react-native";

import type { Api } from "../../api/clients";
import type { SessionState } from "../../data/session";
import { mealFlags } from "../meals/flags";
import CatalogScreen from "./CatalogScreen";
import { demoSession, newMock, renderSettings } from "./test-utils";

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

jest.mock("expo-camera", () => {
  const { View } = jest.requireActual<typeof import("react-native")>("react-native");
  return {
    CameraView: (props: object) => <View testID="camera" {...props} />,
    useCameraPermissions: () => [{ granted: true }, jest.fn()],
  };
});

const search = async (text: string) => {
  const field = screen.getByLabelText("Search");
  await fireEvent.changeText(field, text);
  await fireEvent(field, "submitEditing");
};

describe("CatalogScreen", () => {
  it("searches as you type", async () => {
    jest.useFakeTimers();
    await renderSettings(<CatalogScreen />);
    await fireEvent.changeText(screen.getByLabelText("Search"), "flat bench");
    expect(screen.queryByRole("button", { name: /^Bench Press/ })).toBeNull();
    await act(() => jest.advanceTimersByTime(250));
    expect(await screen.findByRole("button", { name: /^Bench Press/ })).toBeOnTheScreen();
    jest.useRealTimers();
  });

  it("searches exercises and shows the detail", async () => {
    await renderSettings(<CatalogScreen />);
    await search("flat bench");
    await fireEvent.press(await screen.findByRole("button", { name: /^Bench Press/ }));
    for (const muscle of ["chest", "triceps", "shoulders"]) {
      expect(screen.getByRole("checkbox", { name: muscle })).toBeOnTheScreen();
    }
    expect(screen.getByText("Equipment")).toBeOnTheScreen();
    expect(screen.queryByRole("button", { name: "Delete" })).toBeNull();
  });

  it("creates a custom exercise", async () => {
    const spy = jest.spyOn(mockApi.exercises, "createCustomExercise");
    await renderSettings(<CatalogScreen />);
    await fireEvent.press(screen.getByRole("button", { name: "New exercise" }));
    await fireEvent.changeText(screen.getByLabelText("Name"), "Zercher Squat");
    await fireEvent.changeText(screen.getByLabelText("Equipment"), "barbell");
    const primary = within(screen.getByTestId("primary-muscles"));
    await fireEvent.changeText(primary.getByLabelText("Primary muscles"), "quad");
    expect(primary.queryByLabelText("biceps")).toBeNull();
    await fireEvent.press(primary.getByLabelText("quadriceps"));
    await fireEvent.changeText(primary.getByLabelText("Primary muscles"), "");
    await fireEvent.press(primary.getByLabelText("abdominals"));
    const secondary = within(screen.getByTestId("secondary-muscles"));
    await fireEvent.press(secondary.getByLabelText("glutes"));
    await fireEvent.changeText(secondary.getByLabelText("Secondary muscles"), "delt");
    expect(secondary.getByText("Shoulders")).toBeOnTheScreen();
    expect(secondary.queryByText("Chest")).toBeNull();
    await fireEvent.press(secondary.getByLabelText("front delts"));
    await fireEvent.changeText(
      screen.getByLabelText("Instructions"),
      "Set the bar in the crook of the elbows.\n\nSquat to depth.",
    );
    await fireEvent.press(screen.getByRole("button", { name: "Create" }));
    await waitFor(() => expect(spy).toHaveBeenCalledTimes(1));
    expect(spy.mock.calls[0][0].exercise).toMatchObject({
      name: "Zercher Squat",
      equipment: "barbell",
      primaryMuscles: ["quadriceps", "abdominals"],
      secondaryMuscles: ["glutes", "front delts"],
      instructions: ["Set the bar in the crook of the elbows.", "Squat to depth."],
    });
    expect(await screen.findByRole("button", { name: /^Zercher Squat/ })).toBeOnTheScreen();
    expect(screen.getByText("barbell · custom")).toBeOnTheScreen();
  });

  it("creates a custom food and deletes it after confirming", async () => {
    const created = jest.spyOn(mockApi.foods, "createCustomFood");
    const deleted = jest.spyOn(mockApi.foods, "deleteCustomFood");
    await renderSettings(<CatalogScreen />);
    await fireEvent.press(screen.getByRole("checkbox", { name: "Foods" }));
    await fireEvent.press(screen.getByRole("button", { name: "New food" }));
    await fireEvent.changeText(screen.getByLabelText("Name"), "Protein bar");
    await fireEvent.changeText(screen.getByLabelText("kcal"), "400");
    await fireEvent.changeText(screen.getByLabelText("Protein"), "30");
    await fireEvent.changeText(screen.getByLabelText("Carbs"), "40");
    await fireEvent.changeText(screen.getByLabelText("Fat"), "12");
    await fireEvent.press(screen.getByRole("button", { name: "Create" }));
    await waitFor(() => expect(created).toHaveBeenCalledTimes(1));
    expect(created.mock.calls[0][0].food).toMatchObject({
      name: "Protein bar",
      per100g: { kcal: 400, protein: 30, carbs: 40, fat: 12 },
    });

    await fireEvent.press(await screen.findByRole("button", { name: /^Protein bar/ }));
    expect(screen.getByText("400 kcal, 30 protein, 40 carbs, 12 fat")).toBeOnTheScreen();
    await fireEvent.press(screen.getByRole("button", { name: "Delete" }));
    expect(screen.getByText(/Delete Protein bar\?/)).toBeOnTheScreen();
    await fireEvent.press(screen.getByRole("button", { name: "Delete Protein bar" }));
    await waitFor(() => expect(deleted).toHaveBeenCalledTimes(1));
    expect(deleted.mock.calls[0][0].id).toBe((await created.mock.results[0].value).food?.id);
    await waitFor(() => expect(screen.queryByRole("button", { name: /^Protein bar/ })).toBeNull());
  });

  it("marks catalog foods by source", async () => {
    await renderSettings(<CatalogScreen />);
    await fireEvent.press(screen.getByRole("checkbox", { name: "Foods" }));
    await search("paneer");
    expect(await screen.findByText(/indb/)).toBeOnTheScreen();
    await fireEvent.press(screen.getByRole("button", { name: /^Paneer/ }));
    expect(screen.queryByRole("button", { name: "Delete" })).toBeNull();
  });

  it("prefills a new food from a scanned barcode", async () => {
    mealFlags.scanBarcodes = true;
    await renderSettings(<CatalogScreen />);
    await fireEvent.press(screen.getByRole("checkbox", { name: "Foods" }));
    await fireEvent.press(screen.getByRole("button", { name: "New food" }));
    await fireEvent.press(screen.getByRole("button", { name: "Scan barcode" }));
    await act(async () =>
      fireEvent(screen.getByTestId("camera"), "barcodeScanned", { data: "8901234567895" }),
    );
    await waitFor(() => expect(screen.getByLabelText("Name")).toHaveDisplayValue("Whey protein"));
    expect(screen.getByText(/Found .*; adjust it and create your copy\./)).toBeOnTheScreen();
    mealFlags.scanBarcodes = false;
  });
});
