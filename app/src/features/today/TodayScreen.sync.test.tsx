import { createClients } from "../../api/clients";
import { createMockTransport } from "../../api/mockTransport";
import { fireEvent, render, screen } from "@testing-library/react-native";
import { useState } from "react";
import { Button } from "react-native";

import { workoutRecordsRepo } from "../../data/repositories/workoutRecords";
import { seedStore } from "../../data/seedStore";
import { StoreProvider } from "../../data/StoreProvider";
import { SyncProvider } from "../../data/useSync";
import { ThemeProvider } from "../../ui";
import TodayScreen from "./TodayScreen";

jest.mock("../../data/uuid", () => ({ uuid: () => "uuid-1" }));
jest.mock("expo-router", () => ({ useRouter: () => ({ push: jest.fn() }) }));
jest.mock("../../data/session", () => ({
  useSession: () => ({ status: "signedIn" }),
  useServerUrl: () => undefined,
}));

const today = "2026-11-11";
const mockApi = createClients(createMockTransport({ today: "2026-11-11" }).transport);
jest.mock("../../api/useApi", () => ({ useApi: () => mockApi }));

// Changes the sync context without re-rendering the screen element, as a finished sync does.
function Harness() {
  const [syncedAt, setSyncedAt] = useState("2026-11-11T09:00:00Z");
  const [child] = useState(() => <TodayScreen today={today} unit="kg" userId="user-1" />);
  return (
    <SyncProvider value={{ lastSyncedAt: syncedAt }}>
      {child}
      <Button title="synced" onPress={() => setSyncedAt("2026-11-11T09:01:00Z")} />
    </SyncProvider>
  );
}

it("re-reads the store when a sync finishes", async () => {
  const store = await seedStore(today);
  const repo = workoutRecordsRepo(store);
  const record = repo.byDate(today)[0];
  repo.save({ ...record, finishedAt: "2026-11-11T10:00:00Z", finalizeRequested: true });
  await render(
    <ThemeProvider>
      <StoreProvider store={store}>
        <Harness />
      </StoreProvider>
    </ThemeProvider>,
  );
  expect(screen.getByText("Finished, waiting to sync")).toBeOnTheScreen();

  // What a pull lands in the store once the server finalized the record.
  const row = repo.byDate(today)[0];
  store.put("workoutRecords", { ...row, finalizeRequested: false, finalized: true, dirty: false });
  await fireEvent.press(screen.getByText("synced"));
  expect(screen.getByText("Finished")).toBeOnTheScreen();
});
