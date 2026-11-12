import { timestampMs, type Timestamp } from "@bufbuild/protobuf/wkt";
import { fireEvent, screen, waitFor } from "@testing-library/react-native";

import type { Api } from "../../api/clients";
import type { SessionState } from "../../data/session";
import { ApiKeyScope } from "../../gen/plusultra/v1/apikey_pb";
import ApiKeysScreen from "./ApiKeysScreen";
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
  mockSession = demoSession();
});

describe("ApiKeysScreen", () => {
  it("creates a key, shows it once, then lists and revokes it", async () => {
    const created = jest.spyOn(mockApi.apiKeys, "createApiKey");
    const revoked = jest.spyOn(mockApi.apiKeys, "revokeApiKey");
    await renderSettings(<ApiKeysScreen />);
    expect(await screen.findByRole("button", { name: /^claude desktop/ })).toBeOnTheScreen();
    // A revoked key is listed but cannot be revoked again.
    expect(screen.getByText("old laptop")).toBeOnTheScreen();
    expect(screen.queryByRole("button", { name: /^old laptop/ })).toBeNull();

    await fireEvent.press(screen.getByRole("button", { name: "New key" }));
    await fireEvent.changeText(screen.getByLabelText("Name"), "laptop");
    await fireEvent.press(screen.getByRole("checkbox", { name: "Write" }));
    await fireEvent.press(screen.getByRole("checkbox", { name: "90 days" }));
    const before = Date.now();
    await fireEvent.press(screen.getByRole("button", { name: "Create key" }));

    const secret = await screen.findByText(/^lsk_\w+_\w+$/);
    expect(secret).toBeOnTheScreen();
    expect(screen.getByText(/shown only once/)).toBeOnTheScreen();
    expect(created).toHaveBeenCalledTimes(1);
    const request = created.mock.calls[0][0];
    expect(request).toMatchObject({
      name: "laptop",
      scopes: [ApiKeyScope.READ, ApiKeyScope.WRITE],
    });
    const ahead = timestampMs(request.expiresAt as Timestamp) - before;
    expect(ahead).toBeGreaterThan(89 * 86_400_000);
    expect(ahead).toBeLessThan(91 * 86_400_000);

    await fireEvent.press(screen.getByRole("button", { name: "Done" }));
    await fireEvent.press(screen.getByRole("button", { name: /^laptop/ }));
    expect(screen.getByText(/Revoke laptop\?/)).toBeOnTheScreen();
    await fireEvent.press(screen.getByRole("button", { name: "Revoke laptop" }));
    await waitFor(() => expect(revoked).toHaveBeenCalledTimes(1));
    expect(revoked.mock.calls[0][0].id).toBe((await created.mock.results[0].value).apiKey?.id);
    await waitFor(() => expect(screen.queryByRole("button", { name: /^laptop/ })).toBeNull());
    expect(screen.getAllByText(/ · revoked$/)).toHaveLength(2);
  });

  it("needs a name and a scope", async () => {
    const created = jest.spyOn(mockApi.apiKeys, "createApiKey");
    await renderSettings(<ApiKeysScreen />);
    await screen.findByRole("button", { name: /^claude desktop/ });
    await fireEvent.press(screen.getByRole("button", { name: "New key" }));
    await fireEvent.press(screen.getByRole("checkbox", { name: "Read" }));
    await fireEvent.press(screen.getByRole("button", { name: "Create key" }));
    expect(screen.getByText("Give the key a name")).toBeOnTheScreen();
    await fireEvent.changeText(screen.getByLabelText("Name"), "laptop");
    await fireEvent.press(screen.getByRole("button", { name: "Create key" }));
    expect(screen.getByText("Pick at least one scope")).toBeOnTheScreen();
    expect(created).not.toHaveBeenCalled();
  });
});
