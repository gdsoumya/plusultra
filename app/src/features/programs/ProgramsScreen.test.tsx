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
      screen.getByText("Meals, Anchored, 12\u00a0Oct\u00a02026 to 22\u00a0Nov\u00a02026"),
    ).toBeOnTheScreen();
  });

  it("opens a program and the editor", async () => {
    await renderWith(h, <ProgramsScreen />);
    await fireEvent.press(screen.getByRole("button", { name: /^GZCLP 4-week/ }));
    expect(router.push).toHaveBeenCalledWith("/program/program-gzclp");
    await fireEvent.press(screen.getByRole("button", { name: "New" }));
    expect(router.push).toHaveBeenCalledWith("/program/edit");
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

  it("shows an empty state that opens the editor", async () => {
    for (const p of h.store.list("programs")) h.store.delete("programs", p.id);
    await renderWith(h, <ProgramsScreen />);
    await fireEvent.press(screen.getByRole("button", { name: "Write a program" }));
    expect(router.push).toHaveBeenCalledWith("/program/edit");
  });
});
