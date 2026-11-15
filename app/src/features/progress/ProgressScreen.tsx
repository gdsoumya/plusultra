import { Fragment, useMemo, useReducer, useState } from "react";
import { useWindowDimensions } from "react-native";

import type { Api } from "../../api/clients";
import type { Store } from "../../data/store";
import { useRefresh } from "../../data/useRefresh";
import { mondayOf } from "../../domain/dates";
import type { Unit } from "../../domain/types";
import { maxWidth, Screen, space, stroke } from "../../ui";
import { SyncBadge } from "../sync/SyncBadge";
import { AdherenceSection } from "./AdherenceSection";
import { LiftSection } from "./LiftSection";
import { MuscleTrendSection } from "./MuscleTrendSection";
import { NutritionSection } from "./NutritionSection";
import { rangeFor, RangePicker, validChoice, type RangeChoice } from "./RangePicker";
import { TrainingSection } from "./TrainingSection";
import { useWeeklyTrend } from "./useWeeklyTrend";
import { VolumeSection } from "./VolumeSection";
import { WeightSection } from "./WeightSection";

export type ProgressScreenProps = {
  insights: Api["insights"];
  store: Store;
  today: string;
  unit: Unit;
};

// The last range picked, kept across sessions.
const RANGE_KEY = "progress.range";

export default function ProgressScreen({ insights, store, today, unit }: ProgressScreenProps) {
  const [choice, setChoice] = useState<RangeChoice>(() => {
    const saved = store.getKv<RangeChoice>(RANGE_KEY);
    return saved && validChoice(saved) ? saved : { days: 28 };
  });
  const pickRange = (next: RangeChoice) => {
    store.setKv(RANGE_KEY, next);
    setChoice(next);
  };
  const range = useMemo(() => rangeFor(choice, today), [choice, today]);
  // The chart fills a card in the screen column, which stops growing at maxWidth.wide.
  const width =
    Math.min(useWindowDimensions().width - 2 * space[4], maxWidth.wide) -
    2 * (space[4] + stroke.control);
  const section = { insights, store, range, unit, today, width };
  const trend = useWeeklyTrend(store, insights, range);
  // The insights are the server's; remounting the sections fetches them again after the sync.
  const [generation, reload] = useReducer((n: number) => n + 1, 0);
  const refresh = useRefresh(() => {
    reload();
    trend.retry();
  });

  return (
    <Screen title="Progress" testID="progress-screen" right={<SyncBadge />} refresh={refresh} wide>
      <RangePicker range={range} choice={choice} onChange={pickRange} />
      <Fragment key={generation}>
        <WeightSection {...section} />
        <NutritionSection {...section} />
        <LiftSection {...section} />
        <VolumeSection insights={insights} store={store} weekStart={mondayOf(today)} />
        <MuscleTrendSection {...section} trend={trend} />
        <TrainingSection {...section} />
        <AdherenceSection {...section} />
      </Fragment>
    </Screen>
  );
}
