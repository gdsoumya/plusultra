import { render } from "@testing-library/react-native";
import type { ReactElement } from "react";

import { createClients, type Api } from "../../api/clients";
import { user } from "../../api/fixtures/user";
import { createMockTransport, type MockTransport } from "../../api/mockTransport";
import { createMemoryStore } from "../../data/memoryStore";
import { MOCK_SERVER, type SessionState } from "../../data/session";
import { SettingsProvider } from "../../data/settings";
import { StoreProvider } from "../../data/StoreProvider";
import { ThemeProvider } from "../../ui";

export const today = "2026-11-11";

export const newMock = (): { api: Api; failNext: MockTransport["failNext"] } => {
  const { transport, failNext } = createMockTransport({ today });
  return { api: createClients(transport), failNext };
};

export const demoSession = (over: Partial<SessionState> = {}): SessionState => ({
  status: "signedIn",
  serverUrl: MOCK_SERVER,
  token: "token-1",
  user: user(today),
  setServer: jest.fn(),
  login: jest.fn(),
  logout: jest.fn(),
  expire: jest.fn(),
  ...over,
});

export const renderSettings = (ui: ReactElement, store = createMemoryStore()) =>
  render(
    <ThemeProvider>
      <StoreProvider store={store}>
        <SettingsProvider>{ui}</SettingsProvider>
      </StoreProvider>
    </ThemeProvider>,
  );
