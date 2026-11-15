import { Code, ConnectError } from "@connectrpc/connect";
import { fireEvent, screen } from "@testing-library/react-native";
import { router } from "expo-router";

import { ProgramKind } from "../../gen/plusultra/v1/program_pb";
import ProgramsScreen from "./ProgramsScreen";
import { fromProto } from "../../api/convert";
import { StoreProvider } from "../../data/StoreProvider";
import { ThemeProvider } from "../../ui";
import type { Api } from "../../api/clients";
import { pullToRefresh } from "../../ui/test-utils";
import { draftScript, renderWith, setup, type Harness } from "./test-utils";

let mockApi: Api;
jest.mock("../../api/useApi", () => ({ useApi: () => mockApi }));
const mockSync = {
  status: "idle",
  pendingCount: 0,
  reasons: {},
  lastSyncedAt: undefined as string | undefined,
  syncNow: jest.fn(async () => {}),
};
jest.mock("../../data/useSync", () => ({ useSync: () => mockSync }));
jest.mock("expo-router", () => {
  const router = { push: jest.fn(), replace: jest.fn(), back: jest.fn(), canGoBack: () => true };
  return {
    router,
    useRouter: () => router,
    useFocusEffect: (effect: () => void) => jest.requireActual("react").useEffect(effect, [effect]),
  };
});

const rowNames = () =>
  screen
    .getAllByRole("button")
    .map((b) => b.props.accessibilityLabel as string)
    .filter((name) => name !== "New" && name !== "Synced");

let h: Harness;
beforeEach(async () => {
  jest.clearAllMocks();
  h = await setup();
  mockApi = h.api;
  await h.api.programs.createProgram({ kind: ProgramKind.WORKOUT, script: draftScript });
  await h.seed();
});

describe("ProgramsScreen", () => {
  it("lists active workout and meal programs, then drafts, then finished", async () => {
    await renderWith(h, <ProgramsScreen />);
    expect(rowNames()).toEqual([
      expect.stringMatching(/^16-Week Strength & Recomp — Bulk block/),
      expect.stringMatching(/^Recomp meals/),
      expect.stringMatching(/^Draft plan/),
      expect.stringMatching(/^GZCLP 4-week/),
    ]);
    expect(screen.getByText("Active")).toBeOnTheScreen();
    expect(screen.getByText("Drafts")).toBeOnTheScreen();
    expect(screen.getByText("Finished")).toBeOnTheScreen();
    expect(
      screen.getByText("Meals, 12\u00a0Oct\u00a02026 to 22\u00a0Nov\u00a02026"),
    ).toBeOnTheScreen();
  });

  it("opens a program and the editor", async () => {
    await renderWith(h, <ProgramsScreen />);
    await fireEvent.press(screen.getByRole("button", { name: /^GZCLP 4-week/ }));
    expect(router.push).toHaveBeenCalledWith("/program/program-gzclp");
    await fireEvent.press(screen.getByRole("button", { name: "New" }));
    await fireEvent.press(await screen.findByRole("button", { name: /^Workout program/ }));
    expect(router.push).toHaveBeenCalledWith("/program/edit");
    await fireEvent.press(screen.getByRole("button", { name: "New" }));
    await fireEvent.press(await screen.findByRole("button", { name: /^Meal plan/ }));
    expect(router.push).toHaveBeenCalledWith("/program/edit?kind=meal");
  });

  it("syncs on pull-to-refresh and shows what the sync stored", async () => {
    mockSync.syncNow.mockImplementation(async () => {
      await h.api.programs.deleteProgram({ id: "program-gzclp" });
      h.store.delete("programs", "program-gzclp");
    });
    await renderWith(h, <ProgramsScreen />);
    await pullToRefresh();
    expect(mockSync.syncNow).toHaveBeenCalledTimes(1);
    expect(screen.queryByText("GZCLP 4-week")).toBeNull();
    expect(screen.queryByText("Finished")).toBeNull();
  });

  it("shows programs a background sync stored, such as a draft made over MCP", async () => {
    const { rerender } = await renderWith(h, <ProgramsScreen />);
    const { program } = await h.api.programs.createProgram({
      kind: ProgramKind.WORKOUT,
      script: draftScript.replace("Draft plan", "Made over MCP"),
    });
    h.store.put("programs", fromProto.program(program!));
    mockSync.lastSyncedAt = "2026-11-11T10:00:00.000Z";
    await rerender(
      <ThemeProvider>
        <StoreProvider store={h.store}>
          <ProgramsScreen />
        </StoreProvider>
      </ThemeProvider>,
    );
    expect(screen.getByRole("button", { name: /^Made over MCP/ })).toBeOnTheScreen();
  });

  it("opens on the catalog without programs, with an empty state under Mine", async () => {
    for (const p of h.store.list("programs")) h.store.delete("programs", p.id);
    await renderWith(h, <ProgramsScreen />);
    expect(await screen.findByText("Beginner")).toBeOnTheScreen();
    expect(screen.getByRole("checkbox", { name: "Catalog" })).toBeChecked();
    await fireEvent.press(screen.getByRole("checkbox", { name: "Mine" }));
    expect(screen.getByText("Pick one from the catalog or write your own.")).toBeOnTheScreen();
    await fireEvent.press(screen.getByRole("button", { name: "Browse the catalog" }));
    expect(screen.getByRole("checkbox", { name: "Catalog" })).toBeChecked();
    expect(screen.getByRole("button", { name: "New" })).toBeOnTheScreen();
  });

  it("lists the catalog by level and opens a program", async () => {
    await renderWith(h, <ProgramsScreen />);
    expect(screen.getByRole("checkbox", { name: "Mine" })).toBeChecked();
    expect(screen.queryByText("Beginner")).toBeNull();
    await fireEvent.press(screen.getByRole("checkbox", { name: "Catalog" }));
    expect(await screen.findByText("Beginner")).toBeOnTheScreen();
    expect(screen.getByText("Intermediate")).toBeOnTheScreen();
    expect(screen.queryByText("Advanced")).toBeNull();
    expect(screen.queryByText("Active")).toBeNull();
    expect(rowNames()).toEqual([
      "GZCLP 4-week, Cody Lefever · 3 days a week, 4-week cycle",
      "5/3/1 for Beginners, Jim Wendler · 3 days a week, 3-week cycle",
      "16-Week Strength & Recomp — Bulk block, plusultra · 4 days a week, 8 weeks",
    ]);
    await fireEvent.press(screen.getByRole("button", { name: /^GZCLP 4-week/ }));
    expect(router.push).toHaveBeenCalledWith("/catalog/gzclp");
    expect(h.store.getKv("programCatalog")).toHaveLength(3);
    await fireEvent.press(screen.getByRole("checkbox", { name: "Mine" }));
    expect(screen.getByText("Active")).toBeOnTheScreen();
  });

  it("shows the cached catalog with the error when the server is unreachable", async () => {
    const { programs } = await h.api.programs.listCatalog({});
    h.store.setKv("programCatalog", programs.slice(0, 1));
    h.mock.failNext("programs.listCatalog", new ConnectError("down", Code.Unavailable));
    await renderWith(h, <ProgramsScreen />);
    await fireEvent.press(screen.getByRole("checkbox", { name: "Catalog" }));
    expect(
      await screen.findByText("Could not reach the server. Check your connection and try again."),
    ).toBeOnTheScreen();
    expect(rowNames()).toEqual(["GZCLP 4-week, Cody Lefever · 3 days a week, 4-week cycle"]);
  });
});
