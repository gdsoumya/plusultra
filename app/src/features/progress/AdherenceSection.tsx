import { useCallback } from "react";

import { Card, EmptyState, Progress, Text } from "../../ui";
import { StatTiles } from "../../ui/charts/StatTiles";
import { CachedView } from "./CachedView";
import type { SectionProps } from "./shared";
import { useCached } from "./useCached";

export function AdherenceSection({
  insights,
  store,
  range,
}: Pick<SectionProps, "insights" | "store" | "range">) {
  const { from, to } = range;
  const fetch = useCallback(
    () => insights.adherence({ dates: { from, to } }),
    [insights, from, to],
  );
  const state = useCached(store, `progress.adherence.${range.days}`, fetch);

  return (
    <Card testID="progress-adherence">
      <Text variant="heading" accessibilityRole="header">
        Adherence
      </Text>
      <CachedView state={state}>
        {({ counts }) =>
          counts?.planned ? (
            <>
              <Progress value={counts.done / counts.planned} />
              <StatTiles
                items={[
                  { label: "Planned", value: counts.planned },
                  { label: "Done", value: counts.done },
                  { label: "Partial", value: counts.partial },
                  { label: "Skipped", value: counts.skipped },
                ]}
              />
            </>
          ) : (
            <EmptyState
              icon="calendar-outline"
              title="Nothing planned yet"
              message="Activate a program to track how many planned days you train."
            />
          )
        }
      </CachedView>
    </Card>
  );
}
