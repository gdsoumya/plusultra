import {
  createContext,
  createElement,
  useCallback,
  useContext,
  useEffect,
  useMemo,
  useRef,
  useState,
  type ReactNode,
} from "react";
import { AppState } from "react-native";

import { isOffline } from "../api/errors";
import { useApi } from "../api/useApi";
import { useSession } from "./session";
import { syncTables, type Store } from "./store";
import { useStore } from "./StoreProvider";
import { syncOnce } from "./syncEngine";

// What screens read from the sync engine.
export interface SyncStatus {
  status: "idle" | "syncing" | "offline" | "error";
  lastSyncedAt?: string;
  // Local rows not yet on the server.
  pendingCount: number;
  // Rows the server refused on the last sync, by row id, with the server's reason.
  reasons: Record<string, string>;
  syncNow(): Promise<void>;
}

const DEBOUNCE_MS = 2_000;
// The server leaves rows stamped in its last 5 seconds out of a pull, so what a push changed there
// (a finalized record, re-resolved days) comes back on a pull this long after it.
const SERVER_PULL_LAG_MS = 6_000;
const RETRY_MS = 5_000;
const MAX_RETRY_MS = 5 * 60_000;

const idle: SyncStatus = { status: "idle", pendingCount: 0, reasons: {}, syncNow: async () => {} };
const SyncContext = createContext<SyncStatus>(idle);

export const useSync = () => useContext(SyncContext);

// Runs the engine for a signed-in session; tests pass value to show a state without one.
export function SyncProvider({
  value,
  children,
}: {
  value?: Partial<SyncStatus>;
  children: ReactNode;
}) {
  const { status } = useSession();
  if (value) return createElement(SyncContext.Provider, { value: { ...idle, ...value } }, children);
  if (status !== "signedIn") return createElement(SyncContext.Provider, { value: idle }, children);
  return createElement(Runner, null, children);
}

const pending = (store: Store) => syncTables.reduce((n, t) => n + store.listDirty(t).length, 0);

// Syncs on start, on foreground, two seconds after a local write and on request, retrying with
// backoff after a failure.
function Runner({ children }: { children: ReactNode }) {
  const store = useStore();
  const api = useApi();
  const userId = useSession().user?.id;
  const [state, setState] = useState<Omit<SyncStatus, "syncNow">>(() => ({
    status: "idle",
    pendingCount: pending(store),
    reasons: {},
  }));
  const runner = useRef(async () => {});

  useEffect(() => {
    let timer: ReturnType<typeof setTimeout> | undefined;
    let running: Promise<void> | undefined;
    let queued = false;
    let failures = 0;
    let disposed = false;
    const update = (patch: Partial<typeof state>) =>
      !disposed && setState((s) => ({ ...s, ...patch }));
    const schedule = (ms: number) => {
      clearTimeout(timer);
      timer = setTimeout(() => void run(), ms);
    };
    async function sync() {
      update({ status: "syncing" });
      try {
        const { reasons, pushed } = await syncOnce(api, store, { userId });
        failures = 0;
        update({ status: "idle", lastSyncedAt: new Date().toISOString(), reasons });
        // Rows still waiting already have a debounced run that will do the same.
        if (pushed > 0 && pending(store) === 0) schedule(SERVER_PULL_LAG_MS);
      } catch (e) {
        failures += 1;
        update({ status: isOffline(e) ? "offline" : "error" });
        schedule(Math.min(RETRY_MS * 2 ** (failures - 1), MAX_RETRY_MS));
      }
      update({ pendingCount: pending(store) });
    }
    // A run asked for during a sync follows it, so a write made meanwhile is not left behind;
    // the caller's promise settles with the whole run.
    function run() {
      if (running) {
        queued = true;
        return running;
      }
      running = (async () => {
        do {
          queued = false;
          await sync();
        } while (queued && !disposed);
        running = undefined;
      })();
      return running;
    }
    runner.current = run;
    void run();
    const unsubscribe = store.onChange(() => {
      update({ pendingCount: pending(store) });
      schedule(DEBOUNCE_MS);
    });
    const appState = AppState.addEventListener("change", (next) => {
      if (next === "active") void run();
    });
    return () => {
      disposed = true;
      clearTimeout(timer);
      unsubscribe();
      appState.remove();
      runner.current = async () => {};
    };
  }, [api, store, userId]);

  const syncNow = useCallback(() => runner.current(), []);
  const value = useMemo(() => ({ ...state, syncNow }), [state, syncNow]);
  // eslint-disable-next-line react-hooks/refs -- syncNow reads the ref when called, not in render
  return createElement(SyncContext.Provider, { value }, children);
}
