import { Fragment, useMemo, useReducer, useState } from "react";
import { useWindowDimensions, View } from "react-native";

import type { Api } from "../../api/clients";
import type { Store } from "../../data/store";
import { useRefresh } from "../../data/useRefresh";
import { addDays, mondayOf } from "../../domain/dates";
import type { Unit } from "../../domain/types";
import { Chip, maxWidth, Screen, space } from "../../ui";
import { SyncBadge } from "../sync/SyncBadge";
import { AdherenceSection } from "./AdherenceSection";
import { LiftSection } from "./LiftSection";
import type { Range } from "./shared";
import { VolumeSection } from "./VolumeSection";
import { WeightSection } from "./WeightSection";

export type ProgressScreenProps = {
  insights: Api["insights"];
  store: Store;
  today: string;
  unit: Unit;
};

const ranges = [
  { label: "4 weeks", days: 28 },
  { label: "3 months", days: 91 },
  { label: "1 year", days: 365 },
];

export default function ProgressScreen({ insights, store, today, unit }: ProgressScreenProps) {
  const [days, setDays] = useState(ranges[0].days);
  const range = useMemo<Range>(
    () => ({ from: addDays(today, -days), to: today, days }),
    [today, days],
  );
  // The chart fills a card in the screen column, which stops growing at maxWidth.wide.
  const width = Math.min(useWindowDimensions().width - 2 * space[4], maxWidth.wide) - 2 * space[4];
  const section = { insights, store, range, unit, width };
  // The insights are the server's; remounting the sections fetches them again after the sync.
  const [generation, reload] = useReducer((n: number) => n + 1, 0);
  const refresh = useRefresh(reload);

  return (
    <Screen title="Progress" testID="progress-screen" right={<SyncBadge />} refresh={refresh} wide>
      <View style={{ flexDirection: "row", gap: space[2] }}>
        {ranges.map((r) => (
          <Chip
            key={r.days}
            label={r.label}
            selected={r.days === days}
            onPress={() => setDays(r.days)}
          />
        ))}
      </View>
      <Fragment key={generation}>
        <WeightSection {...section} />
        <LiftSection {...section} />
        <VolumeSection insights={insights} store={store} weekStart={mondayOf(today)} />
        <AdherenceSection {...section} />
      </Fragment>
    </Screen>
  );
}
