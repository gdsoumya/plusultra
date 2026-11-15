import { useCallback } from "react";

import type { Api } from "../../api/clients";
import type { Store } from "../../data/store";
import type { WeeklyTrendResponse } from "../../gen/plusultra/v1/insight_pb";
import { DAILY_UP_TO, type Range } from "./shared";
import { useCached, type Cached } from "./useCached";

export type Trend = Cached<WeeklyTrendResponse>;

// One fetch of the range's buckets (days up to four weeks, else weeks), shared by the sections
// that chart them.
export function useWeeklyTrend(store: Store, insights: Api["insights"], { from, to, days }: Range) {
  const daily = days <= DAILY_UP_TO;
  const fetch = useCallback(
    () => insights.weeklyTrend({ dates: { from, to }, daily }),
    [insights, from, to, daily],
  );
  return useCached(store, `progress.trend.${from}.${to}`, fetch);
}
