import { useRouter } from "expo-router";
import { Platform } from "react-native";

import { createApi } from "../../api/createApi";
import { OFFLINE } from "../../api/errors";
import { MOCK_SERVER, normalizeServerUrl, useSession, type SessionState } from "../../data/session";
import type { Store } from "../../data/store";
import { useStore } from "../../data/StoreProvider";
import type { User } from "../../gen/plusultra/v1/auth_pb";

// The hosted web build is served by the server itself, so the page's origin is the address.
// Native builds have no default.
export const defaultServerUrl = () =>
  Platform.OS === "web" ? globalThis.location?.origin : undefined;

// Checks that url is a reachable plusultra server and returns its version.
export async function probe(url: string, timeoutMs = 5000): Promise<string> {
  const abort = new AbortController();
  const timer = setTimeout(() => abort.abort(), timeoutMs);
  try {
    // Android rejects with a plain Error, not the TypeError isOffline expects.
    const res = await fetch(`${url}/healthz`, { signal: abort.signal }).catch(() => {
      throw new Error(OFFLINE);
    });
    if (!res.ok) throw new Error(`${url} answered with HTTP ${res.status}`);
    const body = await res.text();
    if (!body.startsWith("ok")) throw new Error(`${url} is not a PlusUltra server`);
    return body.slice(2).trim();
  } catch (e) {
    if (abort.signal.aborted) {
      throw new Error(`${url} did not answer within ${timeoutMs / 1000} seconds`);
    }
    throw e;
  } finally {
    clearTimeout(timer);
  }
}

// Server choice for the auth screens. Choosing another server signs out of the previous one and
// drops its rows, so nothing of it is ever pushed elsewhere.
export function useServer() {
  const session = useSession();
  const store = useStore();
  const router = useRouter();
  const choose = (url: string) => {
    if (url !== session.serverUrl) store.clearUserData();
    session.setServer(url);
    router.replace("/login");
  };
  return {
    serverUrl: session.serverUrl,
    async connect(input: string) {
      const url = normalizeServerUrl(input);
      if (url !== MOCK_SERVER) await probe(url);
      choose(url);
    },
    tryDemo: () => choose(MOCK_SERVER),
  };
}

// Stores a login or register response as the session.
export function signIn(session: SessionState, { token, user }: { token: string; user?: User }) {
  if (!user) throw new Error("the server sent no user");
  session.login(token, user);
}

// Ends the session on the server when it can, then forgets the token and every local row.
export async function signOut(session: SessionState, store: Store) {
  if (session.token)
    await createApi(session)
      .auth.logout({})
      .catch(() => {});
  store.clearUserData();
  session.logout();
}
