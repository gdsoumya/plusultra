import { useCallback } from "react";

import { muscleGroups, muscleRegions } from "../../domain/muscles";
import { Card, EmptyState, Text } from "../../ui";
import { BarChart } from "../../ui/charts/BarChart";
import { CachedView } from "./CachedView";
import type { SectionProps } from "./shared";
import { useCached } from "./useCached";

const capitalize = (s: string) => s.charAt(0).toUpperCase() + s.slice(1);

export function VolumeSection({
  insights,
  store,
  weekStart,
}: Pick<SectionProps, "insights" | "store"> & { weekStart: string }) {
  const fetch = useCallback(() => insights.weeklySummary({ weekStart }), [insights, weekStart]);
  const state = useCached(store, `progress.week.${weekStart}`, fetch);

  return (
    <Card testID="progress-volume">
      <Text variant="heading" accessibilityRole="header">
        Sets this week
      </Text>
      <CachedView state={state}>
        {({ setsPerMuscle, setsPerRegion }) => {
          const bars = Object.entries(setsPerMuscle)
            .filter(([, sets]) => sets > 0)
            .map(([muscle, sets]) => ({ label: capitalize(muscle), value: sets }))
            .sort((a, b) => b.value - a.value);
          if (!bars.length) {
            return (
              <EmptyState
                icon="barbell-outline"
                title="No sets this week"
                message="Working sets you complete show up here by muscle."
              />
            );
          }
          const total = bars.reduce((t, b) => t + b.value, 0);
          const regionLines = muscleGroups.flatMap((g) => {
            const parts = muscleRegions[g]
              .filter((r) => setsPerRegion[r] > 0)
              .map((r) => `${r} ${setsPerRegion[r]}`);
            return parts.length ? [`${capitalize(g)}: ${parts.join(" · ")}`] : [];
          });
          return (
            <>
              <BarChart
                bars={bars}
                columns={["Muscle", "Sets"]}
                summary={`${total} working sets this week, most for ${bars[0].label} (${bars[0].value}).`}
              />
              {regionLines.map((line) => (
                <Text key={line} variant="caption">
                  {line}
                </Text>
              ))}
            </>
          );
        }}
      </CachedView>
    </Card>
  );
}
