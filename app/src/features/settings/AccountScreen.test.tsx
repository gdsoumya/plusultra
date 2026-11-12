import { Code, ConnectError } from "@connectrpc/connect";
import { fireEvent, screen, waitFor } from "@testing-library/react-native";

import type { Api } from "../../api/clients";
import { createMemoryStore } from "../../data/memoryStore";
import type { SessionState } from "../../data/session";
import AccountScreen from "./AccountScreen";
import { demoSession, newMock, renderSettings } from "./test-utils";
import { SETTINGS_KEY, type UserSettings } from "../../data/settings";

let mockApi: Api;
let mockFailNext: ReturnType<typeof newMock>["failNext"];
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
  ({ api: mockApi, failNext: mockFailNext } = newMock());
  mockSession = demoSession();
});

describe("AccountScreen", () => {
  it("persists the unit change and shows weights in it", async () => {
    const store = createMemoryStore();
    const first = await renderSettings(<AccountScreen />, store);
    expect(screen.getByText("Weights show as 100 kg")).toBeOnTheScreen();

    await fireEvent.press(screen.getByRole("checkbox", { name: "lb" }));
    expect(screen.getByText("Weights show as 220.5 lb")).toBeOnTheScreen();
    await fireEvent.press(screen.getByRole("checkbox", { name: "in" }));
    expect(store.getKv<UserSettings>(SETTINGS_KEY)).toMatchObject({
      units: "lb",
      measurementUnit: "in",
    });
    await first.unmount();

    await renderSettings(<AccountScreen />, store);
    expect(screen.getByText("Weights show as 220.5 lb")).toBeOnTheScreen();
    expect(screen.getByRole("checkbox", { name: "lb" })).toBeChecked();
  });

  it("starts from the signed-in user's settings", async () => {
    await renderSettings(<AccountScreen />);
    expect(screen.getByText("demo")).toBeOnTheScreen();
    expect(screen.getByLabelText("Display name")).toHaveDisplayValue("Demo");
    expect(screen.getByRole("checkbox", { name: "kg" })).toBeChecked();
    expect(screen.getByRole("checkbox", { name: "cm" })).toBeChecked();
  });

  it("changes the password through the server", async () => {
    const spy = jest.spyOn(mockApi.auth, "changePassword");
    await renderSettings(<AccountScreen />);
    await fireEvent.changeText(screen.getByLabelText("Current password"), "demo-password");
    await fireEvent.changeText(screen.getByLabelText("New password"), "a-new-password");
    await fireEvent.press(screen.getByRole("button", { name: "Change password" }));
    await waitFor(() => expect(screen.getByText("Password changed")).toBeOnTheScreen());
    expect(spy).toHaveBeenCalledWith({
      currentPassword: "demo-password",
      newPassword: "a-new-password",
    });
    expect(screen.getByLabelText("New password")).toHaveDisplayValue("");
  });

  it("shows the server's error and an offline message", async () => {
    await renderSettings(<AccountScreen />);
    await fireEvent.changeText(screen.getByLabelText("Current password"), "wrong");
    await fireEvent.changeText(screen.getByLabelText("New password"), "a-new-password");
    await fireEvent.press(screen.getByRole("button", { name: "Change password" }));
    expect(await screen.findByText("current password is wrong")).toBeOnTheScreen();

    mockFailNext("auth.changePassword", new ConnectError("down", Code.Unavailable));
    await fireEvent.press(screen.getByRole("button", { name: "Change password" }));
    expect(await screen.findByText(/Could not reach the server/)).toBeOnTheScreen();
  });

  it("logs out through the session and clears the store", async () => {
    const store = createMemoryStore();
    store.setKv(SETTINGS_KEY, { displayName: "Demo", units: "lb", measurementUnit: "cm" });
    await renderSettings(<AccountScreen />, store);
    await fireEvent.press(screen.getByRole("button", { name: "Log out" }));
    await waitFor(() => expect(mockSession.logout).toHaveBeenCalled());
    expect(store.getKv(SETTINGS_KEY)).toBeUndefined();
  });
});
