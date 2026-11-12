import { Code, ConnectError } from "@connectrpc/connect";
import { act, fireEvent, render, screen } from "@testing-library/react-native";

import { createClients } from "../../api/clients";
import { createMockTransport } from "../../api/mockTransport";
import { createMemoryStore } from "../../data/memoryStore";
import { dayLogsRepo } from "../../data/repositories/dayLogs";
import { StoreProvider } from "../../data/StoreProvider";
import { dayLog } from "../../data/test-utils";
import { SyncProvider } from "../../data/useSync";
import { ThemeProvider } from "../../ui";
import { SyncBadge } from "./SyncBadge";

jest.mock("../../data/uuid", () => ({ uuid: jest.fn(() => "device-1") }));
jest.mock("../../data/session", () => ({
  useSession: () => ({ status: "signedIn", serverUrl: "mock://", user: { id: "user-1" } }),
}));
jest.mock("../../api/createApi", () => ({ createApi: () => mockApi }));

const mock = createMockTransport({ today: "2026-11-11", latencyMs: 50 });
const mockApi = createClients(mock.transport);
const settle = (ms = 0) => act(() => jest.advanceTimersByTimeAsync(ms));

beforeEach(() => jest.useFakeTimers());
afterEach(() => jest.useRealTimers());

// The badge over the real engine: offline after a failed pull, then a sync on request.
it("shows offline after a failed pull and syncs on request from the sheet", async () => {
  const store = createMemoryStore();
  dayLogsRepo(store).save(dayLog("6f0f9d2e-2b7a-4a4e-9d2b-1c3e5a7b9d01", "2026-11-12"));
  mock.failNext("sync.pull", new ConnectError("unreachable", Code.Unavailable));
  await render(
    <ThemeProvider>
      <StoreProvider store={store}>
        <SyncProvider>
          <SyncBadge />
        </SyncProvider>
      </StoreProvider>
    </ThemeProvider>,
  );
  await settle(50);
  await fireEvent.press(screen.getByRole("button", { name: "Offline" }));
  expect(screen.getByText("1 change")).toBeOnTheScreen();
  await fireEvent.press(screen.getByRole("button", { name: "Sync now" }));
  expect(screen.getByRole("button", { name: "Syncing" })).toBeOnTheScreen();
  await settle(200);
  expect(screen.getByRole("button", { name: "Synced" })).toBeOnTheScreen();
  expect(screen.getByText("0 changes")).toBeOnTheScreen();
  expect(store.listDirty("dayLogs")).toHaveLength(0);
});
