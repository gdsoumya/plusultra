import { screen } from "@testing-library/react-native";

import type { Api } from "../../api/clients";
import type { SessionState } from "../../data/session";
import AboutScreen from "./AboutScreen";
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
  mockSession = demoSession({ serverUrl: "https://lift.example.com" });
});

afterEach(() => jest.restoreAllMocks());

describe("AboutScreen", () => {
  it("shows the server address and the version from healthz", async () => {
    const fetch = jest.spyOn(globalThis, "fetch").mockResolvedValue(new Response("ok 0.3.0\n"));
    await renderSettings(<AboutScreen />);
    expect(screen.getByText("https://lift.example.com")).toBeOnTheScreen();
    expect(await screen.findByText("0.3.0")).toBeOnTheScreen();
    expect(fetch).toHaveBeenCalledWith("https://lift.example.com/healthz");
  });

  it("says when the server cannot be reached", async () => {
    jest.spyOn(globalThis, "fetch").mockRejectedValue(new TypeError("Network request failed"));
    await renderSettings(<AboutScreen />);
    expect(await screen.findByText(/Could not reach the server/)).toBeOnTheScreen();
  });

  it("names the demo server", async () => {
    mockSession = demoSession();
    await renderSettings(<AboutScreen />);
    expect(await screen.findByText("demo")).toBeOnTheScreen();
  });
});
