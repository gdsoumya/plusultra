import { useCallback, useEffect, useState } from "react";

import { errorMessage } from "../../api/errors";

// Runs server calls one at a time and keeps the last failure as the screen's error.
export function useCall() {
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState<string>();
  const call = useCallback(async (fn: () => Promise<void>) => {
    setBusy(true);
    setError(undefined);
    try {
      await fn();
      return true;
    } catch (e) {
      setError(errorMessage(e));
      return false;
    } finally {
      setBusy(false);
    }
  }, []);
  return { call, busy, error, setError };
}

// Loads `load` when it changes, plus the call helper for what the screen does with the data.
export function useRemote<T>(load: () => Promise<T>) {
  const [data, setData] = useState<T>();
  const [loading, setLoading] = useState(true);
  const { call, busy, error, setError } = useCall();

  useEffect(() => {
    let live = true;
    load().then(
      (result) => {
        if (!live) return;
        setData(result);
        setLoading(false);
      },
      (e: unknown) => {
        if (!live) return;
        setError(errorMessage(e));
        setLoading(false);
      },
    );
    return () => {
      live = false;
    };
  }, [load, setError]);

  return { data, setData, loading, busy, error, call };
}
