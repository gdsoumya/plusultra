import { Code, ConnectError } from "@connectrpc/connect";
import { act, fireEvent, render, screen } from "@testing-library/react-native";
import { useState } from "react";
import { AppState, Pressable, Text } from "react-native";

import { createClients } from "../api/clients";
import { createMockTransport, type MockTransport } from "../api/mockTransport";
import { createMemoryStore } from "./memoryStore";
import { dayLogsRepo } from "./repositories/dayLogs";
import type { Store } from "./store";
import { StoreProvider } from "./StoreProvider";
import { dayLog } from "./test-utils";
import { SyncProvider, useSync } from "./useSync";

jest.mock("./uuid", () => ({ uuid: jest.fn(() => "device-1") }));
jest.mock("./session", () => ({
  useSession: () => ({
    status: "signedIn",
    serverUrl: "mock://",
    token: "token",
    user: { id: "user-1" },
  }),
}));
jest.mock("../api/createApi", () => ({ createApi: () => mockApi }));

const today = "2026-11-11";
const phone = "6f0f9d2e-2b7a-4a4e-9d2b-1c3e5a7b9d01";
const tablet = "6f0f9d2e-2b7a-4a4e-9d2b-1c3e5a7b9d02";
const tablet2 = "6f0f9d2e-2b7a-4a4e-9d2b-1c3e5a7b9d03";
let mock: MockTransport;
let mockApi: ReturnType<typeof createClients>;

function Status() {
  const { status, lastSyncedAt, pendingCount, syncNow } = useSync();
  const [settled, setSettled] = useState("no");
  return (
    <>
      <Text testID="settled">{settled}</Text>
      <Text testID="status">{status}</Text>
      <Text testID="synced">{lastSyncedAt ?? "never"}</Text>
      <Text testID="pending">{String(pendingCount)}</Text>
      <Pressable
        accessibilityLabel="Sync now"
        onPress={() => void syncNow().then(() => setSettled("yes"))}
      >
        <Text>Sync now</Text>
      </Pressable>
    </>
  );
}

const text = (id: string) => screen.getByTestId(id).props.children;
const settle = (ms = 0) => act(() => jest.advanceTimersByTimeAsync(ms));
// A sync without local changes is two requests at 50 ms each: the pull and the program list.
const syncMs = 100;

describe("SyncProvider", () => {
  let store: Store;
  beforeEach(() => {
    jest.useFakeTimers();
    mock = createMockTransport({ today });
    mockApi = createClients(mock.transport);
    store = createMemoryStore();
    jest.mocked(AppState.addEventListener).mockClear();
  });
  afterEach(() => jest.useRealTimers());

  const mount = () =>
    render(
      <StoreProvider store={store}>
        <SyncProvider>
          <Status />
        </SyncProvider>
      </StoreProvider>,
    );

  it("syncs on start and reports status, last sync and pending rows", async () => {
    mock = createMockTransport({ today, latencyMs: 50 });
    mockApi = createClients(mock.transport);
    await mount();
    expect(text("status")).toBe("syncing");
    await settle(syncMs);
    expect(text("status")).toBe("idle");
    expect(text("synced")).not.toBe("never");
    expect(text("pending")).toBe("0");
    expect(store.list("workoutRecords")).toHaveLength(21);
  });

  it("pushes two seconds after a local write", async () => {
    await mount();
    await act(async () => {
      dayLogsRepo(store).save(dayLog(phone, "2026-11-12", { version: 0 }));
    });
    expect(text("pending")).toBe("1");
    await settle(1999);
    expect(store.listDirty("dayLogs")).toHaveLength(1);
    await settle(1);
    expect(store.listDirty("dayLogs")).toHaveLength(0);
    expect(text("pending")).toBe("0");
    expect((await mockApi.sync.pull({})).rows!.dayLogs.some((r) => r.id === phone)).toBe(true);
  });

  it("pulls once more after a push, once the server's pull lag has passed", async () => {
    await mount();
    await settle();
    const pull = jest.spyOn(mockApi.sync, "pull");
    await act(async () => {
      dayLogsRepo(store).save(dayLog(phone, "2026-11-12", { version: 0 }));
    });
    await settle(2000);
    const pulls = pull.mock.calls.length;
    expect(store.listDirty("dayLogs")).toHaveLength(0);
    // The server leaves rows stamped in its last 5 seconds out of a pull.
    await settle(6000);
    expect(pull.mock.calls.length).toBe(pulls + 1);
    await settle(60_000);
    expect(pull.mock.calls.length).toBe(pulls + 1);
  });

  it("syncs when the app comes to the foreground and on request", async () => {
    await mount();
    const push = (id: string, date: string) =>
      mockApi.sync.push({ deviceId: "tablet", rows: { dayLogs: [{ id, date }] } });

    await push(tablet, "2026-12-01");
    const onChange = jest.mocked(AppState.addEventListener).mock.calls[0][1];
    await act(async () => {
      onChange("active");
    });
    await settle();
    expect(dayLogsRepo(store).get(tablet)).toBeDefined();

    await push(tablet2, "2026-12-02");
    await fireEvent.press(screen.getByLabelText("Sync now"));
    await settle();
    expect(dayLogsRepo(store).get(tablet2)).toBeDefined();
  });

  it("resolves a request made during a sync once the queued run has settled", async () => {
    mock = createMockTransport({ today, latencyMs: 50 });
    mockApi = createClients(mock.transport);
    await mount();
    expect(text("status")).toBe("syncing");
    await fireEvent.press(screen.getByLabelText("Sync now"));
    await settle(syncMs);
    expect(text("settled")).toBe("no");
    await settle(syncMs);
    expect(text("settled")).toBe("yes");
    expect(text("status")).toBe("idle");
  });

  it("reports offline on a network failure and retries with backoff", async () => {
    mock.failNext("sync.pull", new ConnectError("unreachable", Code.Unavailable));
    await mount();
    expect(text("status")).toBe("offline");
    expect(store.list("workoutRecords")).toHaveLength(0);

    mock.failNext("sync.pull", new ConnectError("unreachable", Code.Unavailable));
    await settle(5000);
    expect(text("status")).toBe("offline");
    await settle(9999);
    expect(text("status")).toBe("offline");
    await settle(1);
    expect(text("status")).toBe("idle");
    expect(store.list("workoutRecords")).toHaveLength(21);
  });

  it("reports other failures as errors", async () => {
    mock.failNext("sync.pull", new ConnectError("expired", Code.Unauthenticated));
    await mount();
    expect(text("status")).toBe("error");
  });

  it("shows a given state without running the engine", async () => {
    await render(
      <SyncProvider value={{ status: "offline", pendingCount: 3 }}>
        <Status />
      </SyncProvider>,
    );
    expect(text("status")).toBe("offline");
    expect(text("pending")).toBe("3");
  });
});
