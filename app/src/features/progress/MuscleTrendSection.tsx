import { useState } from "react";

import { shortDate } from "../../domain/dates";
import { muscleGroups } from "../../domain/muscles";
import { Card, Chip, ChipStrip, EmptyState, Text } from "../../ui";
import { ComboChart } from "../../ui/charts/ComboChart";
import { CachedView } from "./CachedView";
import { capitalize, DAILY_UP_TO, type SectionProps } from "./shared";
import type { Trend } from "./useWeeklyTrend";

type Props = Pick<SectionProps, "width" | "range"> & { trend: Trend };

export function MuscleTrendSection({ width, range, trend }: Props) {
  const [picked, setPicked] = useState("All");
  const bucket = range.days <= DAILY_UP_TO ? "day" : "week";

  return (
    <Card testID="progress-muscles">
      <Text variant="heading" accessibilityRole="header">
        Sets per muscle
      </Text>
      <CachedView state={trend}>
        {({ weeks }) => {
          const groups = muscleGroups.filter((g) => weeks.some((w) => w.setsPerMuscle[g] > 0));
          if (!groups.length) {
            return (
              <EmptyState
                icon="barbell-outline"
                title="No working sets in this range"
                message="Finish workouts with working sets to see the trend."
              />
            );
          }
          const choices = ["All", ...groups];
          const group = choices.includes(picked) ? picked : "All";
          const sets = weeks.map((w) =>
            group === "All"
              ? Object.values(w.setsPerMuscle).reduce((t, n) => t + n, 0)
              : (w.setsPerMuscle[group] ?? 0),
          );
          const total = sets.reduce((t, n) => t + n, 0);
          const labels = weeks.map((w) => shortDate(w.weekStart));
          return (
            <>
              <ChipStrip selected={choices.indexOf(group)}>
                {choices.map((g) => (
                  <Chip
                    key={g}
                    label={capitalize(g)}
                    selected={g === group}
                    onPress={() => setPicked(g)}
                  />
                ))}
              </ChipStrip>
              <ComboChart
                width={width}
                labels={labels}
                bars={sets}
                format={String}
                summary={`${capitalize(group)}: ${total} working sets over ${weeks.length} ${bucket}s, ${Math.round(total / weeks.length)} a ${bucket}.`}
                table={{
                  columns: [capitalize(bucket), "Sets"],
                  rows: weeks.map((_, i) => [labels[i], String(sets[i])]),
                }}
              />
            </>
          );
        }}
      </CachedView>
    </Card>
  );
}
