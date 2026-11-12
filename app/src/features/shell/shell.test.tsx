import { toJson } from "@bufbuild/protobuf";
import { fireEvent, renderRouter, screen, waitFor } from "expo-router/testing-library";
import * as SecureStore from "expo-secure-store";

import { user } from "../../api/fixtures/user";
import { MOCK_SERVER } from "../../data/session";
import { UserSchema } from "../../gen/plusultra/v1/auth_pb";

jest.mock("expo-secure-store", () => ({ getItemAsync: jest.fn(), setItemAsync: jest.fn() }));
let mockWide = false;
jest.mock("../../ui/useWide", () => ({ useWide: () => mockWide }));

const stored = new Map<string, string>();
const today = "2026-11-11";

beforeEach(() => {
  stored.clear();
  stored.set(
    "plusultra.session",
    JSON.stringify({
      serverUrl: MOCK_SERVER,
      token: "token-1",
      user: toJson(UserSchema, user(today)),
    }),
  );
  jest.mocked(SecureStore.getItemAsync).mockImplementation(async (k) => stored.get(k) ?? null);
  jest.mocked(SecureStore.setItemAsync).mockImplementation(async (k, v) => void stored.set(k, v));
});

const renderApp = (initialUrl: string) =>
  renderRouter({ appDir: "./app", overrides: {} }, { initialUrl });
const at = (pathname: string) => waitFor(() => expect(screen).toHavePathname(pathname));
const shown = (testID: string) =>
  waitFor(() => expect(screen.getByTestId(testID)).toBeOnTheScreen());
const tabs = () => screen.getAllByRole("tab");

describe("navigation shell", () => {
  it("keeps the tab bar on a pushed screen and Back pops within the tab", async () => {
    await renderApp(`/workout/workout-${today}`);
    await shown("workout-screen");
    expect(tabs()).toHaveLength(5);
    expect(screen.getByRole("tab", { name: "Today" })).toBeSelected();
    await fireEvent.press(screen.getByRole("button", { name: "Back" }));
    await at("/");
    await shown("today-screen");
  });

  it("opens a past day's workout inside the Calendar tab and finishes back to the day", async () => {
    await renderApp("/day/2026-01-05");
    await shown("day-screen");
    await fireEvent.press(screen.getByRole("button", { name: "Add workout" }));
    await fireEvent.press(await screen.findByRole("button", { name: "Ad hoc workout" }));
    await shown("workout-screen");
    expect(screen).toHaveSegments(["(tabs)", "(calendar)", "day", "[date]", "workout", "[id]"]);
    expect(screen.getByRole("tab", { name: "Calendar" })).toBeSelected();
    await fireEvent.press(screen.getByRole("button", { name: "Finish" }));
    await at("/day/2026-01-05");
    await shown("day-screen");
  });

  it("opens each tab's sub-routes inside the tab", async () => {
    await renderApp("/");
    await shown("today-screen");
    await fireEvent.press(screen.getByRole("tab", { name: "Settings" }));
    await at("/settings");
    await fireEvent.press(screen.getByRole("button", { name: /^About/ }));
    await at("/settings/about");
    expect(screen.getByRole("tab", { name: "Settings" })).toBeSelected();
    await fireEvent.press(screen.getByRole("tab", { name: "Calendar" }));
    await at("/calendar");
    expect(tabs()).toHaveLength(5);
  });

  it("goes to a tab's root when the tab is tapped, from another tab or from its own stack", async () => {
    await renderApp("/settings/about");
    await shown("about-screen");
    await fireEvent.press(screen.getByRole("tab", { name: "Settings" }));
    await at("/settings");
    await fireEvent.press(screen.getByRole("button", { name: /^About/ }));
    await at("/settings/about");
    await fireEvent.press(screen.getByRole("tab", { name: "Calendar" }));
    await at("/calendar");
    await fireEvent.press(screen.getByRole("tab", { name: "Settings" }));
    await at("/settings");
  });

  it("goes to the root from the top bar too", async () => {
    mockWide = true;
    await renderApp("/settings/about");
    await shown("tab-bar-top");
    await shown("about-screen");
    await fireEvent.press(screen.getByRole("tab", { name: "Calendar" }));
    await at("/calendar");
    await fireEvent.press(screen.getByRole("tab", { name: "Settings" }));
    await at("/settings");
  });

  it("is a bottom bar on a phone", async () => {
    mockWide = false;
    await renderApp("/");
    await shown("tab-bar-bottom");
    expect(screen.queryByTestId("tab-bar-top")).toBeNull();
  });

  it("is a top bar on a wide window", async () => {
    mockWide = true;
    await renderApp("/");
    await shown("tab-bar-top");
    expect(screen.queryByTestId("tab-bar-bottom")).toBeNull();
    // Centred, like the content column under it, not packed to the left of a wider row.
    expect(screen.getByTestId("tab-bar-top-row")).toHaveStyle({
      alignSelf: "center",
      justifyContent: "center",
    });
  });
});
