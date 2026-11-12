import { fireEvent, screen, waitFor } from "@testing-library/react-native";

import type { Api } from "../../api/clients";
import type { SessionState } from "../../data/session";
import EquipmentScreen from "./EquipmentScreen";
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

describe("EquipmentScreen", () => {
  it("edits one piece and sends the whole list", async () => {
    const spy = jest.spyOn(mockApi.exercises, "setEquipment");
    await renderSettings(<EquipmentScreen />);
    expect(await screen.findByText("Bar 20, plates 25/20/15/10/5/2.5/1.25")).toBeOnTheScreen();

    await fireEvent.press(screen.getByRole("button", { name: /^barbell/ }));
    expect(
      screen.getByText("Rounding values are in kg. Changing units does not convert them."),
    ).toBeOnTheScreen();
    await fireEvent.changeText(screen.getByLabelText("Bar (kg)"), "15");
    await fireEvent.changeText(screen.getByLabelText("Plates per side (kg)"), "20, 10, 5");
    await fireEvent.press(screen.getByRole("button", { name: "Save" }));

    await waitFor(() => expect(spy).toHaveBeenCalledTimes(1));
    const equipment = spy.mock.calls[0][0].equipment!;
    expect(equipment).toHaveLength(8);
    expect(equipment.find((e) => e.name === "barbell")?.rounding).toMatchObject({
      kind: "plates",
      bar: 15,
      plates: [20, 10, 5],
    });
    expect(await screen.findByText("Bar 15, plates 20/10/5")).toBeOnTheScreen();
  });

  it("adds and removes equipment", async () => {
    const spy = jest.spyOn(mockApi.exercises, "setEquipment");
    await renderSettings(<EquipmentScreen />);
    await screen.findByRole("button", { name: /^barbell/ });

    await fireEvent.press(screen.getByRole("button", { name: "Add equipment" }));
    await fireEvent.changeText(screen.getByLabelText("Name"), "trap bar");
    await fireEvent.press(screen.getByRole("checkbox", { name: "stack" }));
    await fireEvent.changeText(screen.getByLabelText("Increment (kg)"), "2.5");
    await fireEvent.press(screen.getByRole("button", { name: "Save" }));
    await waitFor(() => expect(spy).toHaveBeenCalledTimes(1));
    expect(spy.mock.calls[0][0].equipment!.map((e) => e.name)).toContain("trap bar");
    expect(spy.mock.calls[0][0].equipment!.at(-1)?.rounding).toMatchObject({
      kind: "stack",
      increment: 2.5,
    });

    await fireEvent.press(await screen.findByRole("button", { name: /^trap bar/ }));
    await fireEvent.press(screen.getByRole("button", { name: "Remove" }));
    await waitFor(() => expect(spy).toHaveBeenCalledTimes(2));
    expect(spy.mock.calls[1][0].equipment!.map((e) => e.name)).not.toContain("trap bar");
  });

  it("keeps the sheet open and explains a bad number", async () => {
    const spy = jest.spyOn(mockApi.exercises, "setEquipment");
    await renderSettings(<EquipmentScreen />);
    await fireEvent.press(await screen.findByRole("button", { name: /^barbell/ }));
    await fireEvent.changeText(screen.getByLabelText("Bar (kg)"), "twenty");
    await fireEvent.press(screen.getByRole("button", { name: "Save" }));
    expect(screen.getByText("Bar must be a number")).toBeOnTheScreen();
    expect(spy).not.toHaveBeenCalled();
  });
});
