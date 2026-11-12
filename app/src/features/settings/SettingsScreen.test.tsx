import { fireEvent, screen, waitFor } from "@testing-library/react-native";

import { createMemoryStore } from "../../data/memoryStore";
import type { SessionState } from "../../data/session";
import type { SyncStatus } from "../../data/useSync";
import SettingsScreen from "./SettingsScreen";
import { demoSession, renderSettings } from "./test-utils";

const mockRouter = { push: jest.fn(), replace: jest.fn(), back: jest.fn(), canGoBack: () => true };
let mockSession: SessionState;
let mockSync: SyncStatus;
jest.mock("expo-router", () => ({ useRouter: () => mockRouter }));
jest.mock("../../data/session", () => ({
  ...jest.requireActual("../../data/session"),
  useSession: () => mockSession,
}));
jest.mock("../../data/useSync", () => ({ useSync: () => mockSync }));

beforeEach(() => {
  jest.clearAllMocks();
  mockSession = demoSession();
  mockSync = {
    status: "idle",
    lastSyncedAt: "2026-11-11T09:30:00Z",
    pendingCount: 2,
    reasons: {},
    syncNow: jest.fn(async () => {}),
  };
});

describe("SettingsScreen", () => {
  it.each([
    ["Account", "/settings/account"],
    ["Equipment and plates", "/settings/equipment"],
    ["1RMs", "/settings/maxes"],
    ["Catalog", "/settings/catalog"],
    ["API keys", "/settings/api-keys"],
    ["Export", "/settings/export"],
    ["About", "/settings/about"],
  ])("opens %s", async (name, href) => {
    await renderSettings(<SettingsScreen />);
    expect(screen.getByTestId("settings-screen")).toBeOnTheScreen();
    await fireEvent.press(screen.getByRole("button", { name: new RegExp(`^${name}`) }));
    expect(mockRouter.push).toHaveBeenCalledWith(href);
  });

  it("shows who is signed in", async () => {
    await renderSettings(<SettingsScreen />);
    expect(screen.getByText("Demo")).toBeOnTheScreen();
  });

  it("opens the sync sheet from the Sync now row", async () => {
    await renderSettings(<SettingsScreen />);
    await fireEvent.press(screen.getByRole("button", { name: /^Sync now/ }));
    expect(screen.getByText("2 changes")).toBeOnTheScreen();
    await fireEvent.press(screen.getByRole("button", { name: "Sync now" }));
    expect(mockSync.syncNow).toHaveBeenCalledTimes(1);
  });

  it("changes server after confirmation: signs out, clears the store, lands on the server screen", async () => {
    const store = createMemoryStore();
    store.setKv("k", 1);
    await renderSettings(<SettingsScreen />, store);
    await fireEvent.press(screen.getByRole("button", { name: /^Change server, mock:\/\/demo/ }));
    expect(screen.getByText(/You will be signed out/)).toBeOnTheScreen();
    await fireEvent.press(screen.getByRole("button", { name: "Change server" }));
    await waitFor(() => expect(mockSession.logout).toHaveBeenCalledTimes(1));
    expect(store.getKv("k")).toBeUndefined();
    expect(mockRouter.replace).toHaveBeenCalledWith("/server");
  });

  it("keeps everything when the server change is cancelled", async () => {
    const store = createMemoryStore();
    store.setKv("k", 1);
    await renderSettings(<SettingsScreen />, store);
    await fireEvent.press(screen.getByRole("button", { name: /^Change server/ }));
    await fireEvent.press(screen.getByRole("button", { name: "Cancel" }));
    expect(mockSession.logout).not.toHaveBeenCalled();
    expect(store.getKv("k")).toBe(1);
    expect(mockRouter.replace).not.toHaveBeenCalled();
  });
});
