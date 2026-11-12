import { Code, ConnectError } from "@connectrpc/connect";
import { fireEvent, screen, waitFor } from "@testing-library/react-native";
import { Share } from "react-native";

import type { Api } from "../../api/clients";
import type { SessionState } from "../../data/session";
import ExportScreen from "./ExportScreen";
import { demoSession, newMock, renderSettings } from "./test-utils";

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

describe("ExportScreen", () => {
  it("exports everything and hands the zip to the share sheet", async () => {
    const exportAll = jest.spyOn(mockApi.export, "exportAll");
    const share = jest
      .spyOn(Share, "share")
      .mockResolvedValue({ action: "sharedAction", activityType: undefined });
    await renderSettings(<ExportScreen />);
    await fireEvent.press(screen.getByRole("button", { name: "Export everything" }));
    await waitFor(() => expect(exportAll).toHaveBeenCalledTimes(1));
    expect(await screen.findByText(/plusultra-2026-11-11\.zip/)).toBeOnTheScreen();
    expect(share).toHaveBeenCalledTimes(1);
    expect(share.mock.calls[0][0].message).toMatch(/plusultra-2026-11-11\.zip/);
  });

  it("shows an offline message when the export fails", async () => {
    mockFailNext("export.exportAll", new ConnectError("down", Code.Unavailable));
    await renderSettings(<ExportScreen />);
    await fireEvent.press(screen.getByRole("button", { name: "Export everything" }));
    expect(await screen.findByText(/Could not reach the server/)).toBeOnTheScreen();
  });
});
