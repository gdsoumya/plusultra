import { useFocusEffect } from "expo-router";
import { useCallback, useEffect, useReducer, useState } from "react";

import { useApi } from "../../api/useApi";
import { useStore } from "../../data/StoreProvider";
import { buildMonth, fetchMonth, type MonthData } from "./month";

export function useMonth(month: string, today: string): MonthData {
  const api = useApi();
  const store = useStore();
  const [, refresh] = useReducer((n: number) => n + 1, 0);
  // Months asked of the server while the screen is mounted; a failed ask is retried on the next visit.
  const [fetched] = useState(() => new Set<string>());
  // A status set on the day screen must show once the tab is focused again.
  useFocusEffect(useCallback(() => refresh(), []));
  useEffect(() => {
    if (fetched.has(month)) return;
    fetched.add(month);
    fetchMonth(api, store, month).then(refresh, () => fetched.delete(month));
  }, [api, store, month, fetched]);
  return buildMonth(store, month, today);
}
