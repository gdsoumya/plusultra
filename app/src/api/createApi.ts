import { Code, ConnectError, type Interceptor } from "@connectrpc/connect";
import { createConnectTransport } from "@connectrpc/connect-web";

import { MOCK_SERVER, type Session } from "../data/session";
import { createClients, type Api } from "./clients";
import { createMockTransport } from "./mockTransport";

// One demo server per process, so every screen and the sync engine see the same state.
let demo: Api | undefined;

// The server mounts the Connect services behind this prefix; /healthz stays at the origin.
// A stored address that already ends in the prefix (older sessions) is not doubled.
export const connectBaseUrl = (serverUrl: string) =>
  `${serverUrl.replace(/\/api\/v1$/i, "")}/api/v1`;

// Clients for the session's server: the in-memory mock for mock://demo, Connect over fetch
// otherwise. A token the server rejects ends the session through expire.
export function createApi({ serverUrl, token, expire }: Session & { expire?(): void }): Api {
  if (serverUrl === MOCK_SERVER) return (demo ??= createClients(createMockTransport().transport));
  if (!serverUrl) throw new Error("no server is set");
  const bearer: Interceptor = (next) => async (req) => {
    if (!token) return next(req);
    req.header.set("Authorization", `Bearer ${token}`);
    try {
      return await next(req);
    } catch (e) {
      if (ConnectError.from(e).code === Code.Unauthenticated) expire?.();
      throw e;
    }
  };
  // A server that never answers, such as a wrong address, fails instead of hanging.
  return createClients(
    createConnectTransport({
      baseUrl: connectBaseUrl(serverUrl),
      interceptors: [bearer],
      defaultTimeoutMs: 10_000,
    }),
  );
}
