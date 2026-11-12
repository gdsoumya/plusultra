import { useEffect, useState } from "react";

import { errorMessage } from "../../api/errors";
import type { Store } from "../../data/store";

export type Cached<T> = {
  data?: T;
  // Set when `data` is the stored copy rather than this session's response.
  at?: string;
  error?: string;
  loading: boolean;
  retry(): void;
};

type Entry<T> = { at: string; data: T };
type Result<T> = { key: string; attempt: number; data?: T; error?: string };

// Fetches on every key change and keeps the last good response in the store's kv table, so the
// screen renders offline. `fetch` must be stable for a key (useCallback).
export function useCached<T>(store: Store, key: string, fetch: () => Promise<T>): Cached<T> {
  const [result, setResult] = useState<Result<T>>();
  const [attempt, setAttempt] = useState(0);

  useEffect(() => {
    let live = true;
    fetch().then(
      (data) => {
        store.setKv(key, { at: new Date().toISOString(), data } satisfies Entry<T>);
        if (live) setResult({ key, attempt, data });
      },
      (e: unknown) => {
        if (live) setResult({ key, attempt, error: errorMessage(e) });
      },
    );
    return () => {
      live = false;
    };
  }, [store, key, fetch, attempt]);

  const fresh = result?.key === key && result.attempt === attempt ? result : undefined;
  const cached = store.getKv<Entry<T>>(key);
  return {
    data: fresh?.data ?? cached?.data,
    at: fresh?.data ? undefined : cached?.at,
    error: fresh?.error,
    loading: !fresh,
    retry: () => setAttempt((n) => n + 1),
  };
}
