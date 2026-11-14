import { useCallback } from "react";

import type { Api } from "../../api/clients";
import type { Store } from "../../data/store";
import type { WeeklyTrendResponse } from "../../gen/plusultra/v1/insight_pb";
import type { Range } from "./shared";
import { useCached, type Cached } from "./useCached";

export type Trend = Cached<WeeklyTrendResponse>;

// One fetch of the range's weeks, shared by the sections that chart them.
export function useWeeklyTrend(store: Store, insights: Api["insights"], { from, to }: Range) {
  const fetch = useCallback(
    () => insights.weeklyTrend({ dates: { from, to } }),
    [insights, from, to],
  );
  return useCached(store, `progress.trend.${from}.${to}`, fetch);
}
