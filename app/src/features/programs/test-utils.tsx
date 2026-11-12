import { render } from "@testing-library/react-native";
import type { ReactElement } from "react";

import { createClients, type Api } from "../../api/clients";
import { fromProto } from "../../api/convert";
import { createMockTransport, type MockTransport } from "../../api/mockTransport";
import { createMemoryStore } from "../../data/memoryStore";
import { programsRepo } from "../../data/repositories/programs";
import type { Store } from "../../data/store";
import { StoreProvider } from "../../data/StoreProvider";
import { ThemeProvider } from "../../ui";

export const today = "2026-11-11";

export interface Harness {
  mock: MockTransport;
  api: Api;
  store: Store;
  // Copies the mock server's program list into the store, as the sync engine would.
  seed(): Promise<void>;
}

export async function setup(): Promise<Harness> {
  const mock = createMockTransport({ today });
  const api = createClients(mock.transport);
  const store = createMemoryStore();
  const seed = async () => {
    const { programs } = await api.programs.listPrograms({});
    programsRepo(store).upsertFromServer(programs.map(fromProto.program));
  };
  await seed();
  return { mock, api, store, seed };
}

// Test files mock ../../api/useApi to return the harness api.
export const renderWith = ({ store }: Harness, ui: ReactElement) =>
  render(
    <ThemeProvider>
      <StoreProvider store={store}>{ui}</StoreProvider>
    </ThemeProvider>,
  );

export const draftScript =
  'program("Draft plan", weeks=2)\nday(1, 1, "Full body")\nex("Squat", sets=[s(3, 5, kg(60))])\n';
