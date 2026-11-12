import { fromJson, toJson, type JsonValue } from "@bufbuild/protobuf";
import * as SecureStore from "expo-secure-store";
import {
  createContext,
  createElement,
  useContext,
  useEffect,
  useMemo,
  useState,
  type ReactNode,
} from "react";
import { Platform } from "react-native";

import { UserSchema, type User } from "../gen/plusultra/v1/auth_pb";

export const MOCK_SERVER = "mock://demo";

export interface Session {
  serverUrl?: string;
  token?: string;
  user?: User;
  // Set when the server rejected the token, until the next login.
  expired?: boolean;
}

export interface SessionState extends Session {
  status: "loading" | "signedOut" | "signedIn";
  // Throws when url is not an http(s) address or the mock server.
  setServer(url: string): void;
  login(token: string, user: User): void;
  logout(): void;
  // Drops the token the server rejected; the API layer calls this on Unauthenticated.
  expire(): void;
}

const KEY = "plusultra.session";

// The user goes through protobuf JSON: its timestamps hold bigints, which JSON.stringify rejects.
const encode = ({ user, ...rest }: Session) =>
  JSON.stringify({ ...rest, user: user && toJson(UserSchema, user) });

function decode(json: string): Session {
  const { user, ...rest } = JSON.parse(json) as Omit<Session, "user"> & { user?: JsonValue };
  return {
    ...rest,
    user: user ? fromJson(UserSchema, user, { ignoreUnknownFields: true }) : undefined,
  };
}

// Native keeps the session in the keychain or keystore, the web in localStorage. Either can be
// missing or fail (static web rendering, a locked keystore); the session then lives in memory.
async function read(): Promise<Session> {
  try {
    const json =
      Platform.OS === "web"
        ? typeof localStorage === "undefined"
          ? null
          : localStorage.getItem(KEY)
        : await SecureStore.getItemAsync(KEY);
    return json ? decode(json) : {};
  } catch {
    return {};
  }
}

let writes = Promise.resolve();

// Queued so an older write never lands after a newer one.
function write(session: Session) {
  const json = encode(session);
  writes = writes
    .then(() =>
      Platform.OS === "web"
        ? globalThis.localStorage?.setItem(KEY, json)
        : SecureStore.setItemAsync(KEY, json),
    )
    .catch(() => {});
}

export function normalizeServerUrl(input: string): string {
  const url = input.trim();
  if (url === MOCK_SERVER) return url;
  // The app adds the API prefix itself; accept an address typed with it.
  const trimmed = url.replace(/\/+$/, "").replace(/\/api\/v1$/i, "");
  if (!/^https?:\/\/[^\s/?#]+(\/\S*)?$/i.test(trimmed)) {
    throw new Error("Enter the server address, starting with http:// or https://");
  }
  return trimmed;
}

const SessionContext = createContext<SessionState | null>(null);

export function SessionProvider({ children }: { children: ReactNode }) {
  const [session, setSession] = useState<Session>();

  useEffect(() => {
    let mounted = true;
    // A change made while loading wins over what was stored.
    read().then((stored) => mounted && setSession((current) => current ?? stored));
    return () => {
      mounted = false;
    };
  }, []);

  const value = useMemo<SessionState>(() => {
    const update = (next: Session) => {
      setSession(next);
      write(next);
    };
    return {
      ...session,
      status: !session ? "loading" : session.token && session.user ? "signedIn" : "signedOut",
      setServer(url) {
        const serverUrl = normalizeServerUrl(url);
        // A token is only ever sent to the server that issued it.
        update(serverUrl === session?.serverUrl ? { ...session, serverUrl } : { serverUrl });
      },
      login: (token, user) => update({ serverUrl: session?.serverUrl, token, user }),
      logout: () => update({ serverUrl: session?.serverUrl }),
      expire: () => update({ serverUrl: session?.serverUrl, expired: true }),
    };
  }, [session]);

  return createElement(SessionContext.Provider, { value }, children);
}

export function useSession(): SessionState {
  const session = useContext(SessionContext);
  if (!session) throw new Error("useSession must be used inside a SessionProvider");
  return session;
}

// The signed-in user; undefined before sign-in and outside a SessionProvider.
export const useUser = () => useContext(SessionContext)?.user;
// The configured server; undefined outside a SessionProvider.
export const useServerUrl = () => useContext(SessionContext)?.serverUrl;
