import { Code, ConnectError } from "@connectrpc/connect";

import { MOCK_SERVER } from "../data/session";
import { timestampFromDate } from "@bufbuild/protobuf/wkt";

import { ApiKeyScope } from "../gen/plusultra/v1/apikey_pb";
import { createApi } from "./createApi";

const connectError = (status: number, code: string) =>
  new Response(JSON.stringify({ code, message: code }), {
    status,
    headers: { "Content-Type": "application/json" },
  });

describe("createApi", () => {
  afterEach(() => jest.restoreAllMocks());

  it("serves the mock server from fixtures", async () => {
    const { user } = await createApi({ serverUrl: MOCK_SERVER }).auth.getMe({});
    expect(user?.username).toBe("demo");
  });

  it("serves one demo server to every caller", async () => {
    const { apiKey } = await createApi({ serverUrl: MOCK_SERVER }).apiKeys.createApiKey({
      name: "shared",
      scopes: [ApiKeyScope.READ],
      expiresAt: timestampFromDate(new Date(Date.now() + 86_400_000)),
    });
    const other = createApi({ serverUrl: MOCK_SERVER, token: "t" });
    const { apiKeys } = await other.apiKeys.listApiKeys({});
    expect(apiKeys.map((k) => k.id)).toContain(apiKey?.id);
  });

  it("calls a real server over Connect with the session token", async () => {
    const fetch = jest.spyOn(globalThis, "fetch").mockResolvedValue(
      new Response(JSON.stringify({ user: { username: "lifter" } }), {
        headers: { "Content-Type": "application/json" },
      }),
    );
    const api = createApi({ serverUrl: "https://lift.example.com", token: "token-1" });
    const { user } = await api.auth.getMe({});
    expect(user?.username).toBe("lifter");
    const [url, init] = fetch.mock.calls[0];
    // The typed address is the origin; the Connect services live under /api/v1.
    expect(url).toBe("https://lift.example.com/api/v1/plusultra.v1.AuthService/GetMe");
    const headers = new Headers(init?.headers);
    expect(headers.get("Authorization")).toBe("Bearer token-1");
    // A server that never answers fails after 10 s instead of hanging.
    expect(headers.get("Connect-Timeout-Ms")).toBe("10000");
  });

  it("expires the session when the server rejects its token", async () => {
    jest.spyOn(globalThis, "fetch").mockResolvedValue(connectError(401, "unauthenticated"));
    const expire = jest.fn();
    const api = createApi({ serverUrl: "https://lift.example.com", token: "token-1", expire });
    await expect(api.auth.getMe({})).rejects.toMatchObject({ code: Code.Unauthenticated });
    expect(expire).toHaveBeenCalledTimes(1);
  });

  it("leaves the session alone when no token was sent or another error comes back", async () => {
    const fetch = jest.spyOn(globalThis, "fetch");
    const expire = jest.fn();
    fetch.mockResolvedValueOnce(connectError(401, "unauthenticated"));
    const anonymous = createApi({ serverUrl: "https://lift.example.com", expire });
    await expect(anonymous.auth.login({ username: "a", password: "b" })).rejects.toBeInstanceOf(
      ConnectError,
    );
    fetch.mockResolvedValueOnce(connectError(403, "permission_denied"));
    const signedIn = createApi({ serverUrl: "https://lift.example.com", token: "token-1", expire });
    await expect(signedIn.auth.createInvite({})).rejects.toMatchObject({
      code: Code.PermissionDenied,
    });
    expect(expire).not.toHaveBeenCalled();
  });

  it("needs a server", () => {
    expect(() => createApi({})).toThrow("no server is set");
  });
});
