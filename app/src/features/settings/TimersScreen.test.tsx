import { fireEvent, screen, within } from "@testing-library/react-native";
import { Linking, Platform } from "react-native";

import { createMemoryStore } from "../../data/memoryStore";
import { SETTINGS_KEY, type UserSettings } from "../../data/settings";
import { renderSettings } from "./test-utils";
import TimersScreen from "./TimersScreen";

jest.mock("../../data/session", () => ({
  ...jest.requireActual("../../data/session"),
  useSession: () => ({ user: undefined }),
}));

const stored = (store: ReturnType<typeof createMemoryStore>) =>
  store.getKv<UserSettings>(SETTINGS_KEY);

describe("TimersScreen", () => {
  it("starts from the defaults and flips each switch", async () => {
    const store = createMemoryStore();
    await renderSettings(<TimersScreen />, store);
    expect(within(screen.getByTestId("keep-awake")).getByText("Off")).toBeOnTheScreen();
    expect(within(screen.getByTestId("vibrate")).getByText("On")).toBeOnTheScreen();
    await fireEvent.press(screen.getByTestId("keep-awake"));
    await fireEvent.press(screen.getByTestId("vibrate"));
    expect(stored(store)).toMatchObject({ keepAwake: true, vibrate: false, volume: 1 });
    expect(within(screen.getByTestId("keep-awake")).getByText("On")).toBeOnTheScreen();
  });

  it("picks a volume", async () => {
    const store = createMemoryStore();
    await renderSettings(<TimersScreen />, store);
    expect(screen.getByRole("checkbox", { name: "Loud" })).toBeChecked();
    await fireEvent.press(screen.getByRole("checkbox", { name: "Quiet" }));
    expect(stored(store)?.volume).toBe(0.3);
  });

  it("opens the system's DND access page when alerts in DND are turned on", async () => {
    Platform.OS = "android";
    const intent = jest.spyOn(Linking, "sendIntent").mockResolvedValue(undefined);
    const store = createMemoryStore();
    await renderSettings(<TimersScreen />, store);
    await fireEvent.press(screen.getByTestId("alert-in-dnd"));
    expect(stored(store)?.alertInDnd).toBe(true);
    expect(intent).toHaveBeenCalledWith("android.settings.NOTIFICATION_POLICY_ACCESS_SETTINGS");
    await fireEvent.press(screen.getByTestId("alert-in-dnd"));
    expect(stored(store)?.alertInDnd).toBe(false);
    expect(intent).toHaveBeenCalledTimes(1);
    intent.mockRestore();
  });
});
