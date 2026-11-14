import { useMemo } from "react";
import { View } from "react-native";

import type { Store } from "../../data/store";
import { addDays, mondayOf, shortDate } from "../../domain/dates";
import { Card, space, Text, useTheme } from "../../ui";
import { StatTiles } from "../../ui/charts/StatTiles";
import { CachedView } from "./CachedView";
import type { SectionProps } from "./shared";
import type { Trend } from "./useWeeklyTrend";

type Props = Pick<SectionProps, "store" | "range" | "today" | "width"> & { trend: Trend };

const gap = space[1];
const finishedDates = (store: Store, from?: string, to?: string) =>
  store
    .list("workoutRecords", { dateFrom: from, dateTo: to })
    .filter((r) => r.finishedAt)
    .map((r) => r.date);

// Consecutive weeks with a finished workout, counted back from this week; a week with none yet
// does not break it.
function weekStreak(store: Store, today: string) {
  const weeks = new Set(finishedDates(store).map(mondayOf));
  let week = mondayOf(today);
  if (!weeks.has(week)) week = addDays(week, -7);
  let n = 0;
  for (; weeks.has(week); week = addDays(week, -7)) n++;
  return n;
}

export function TrainingSection({ store, range, today, width, trend }: Props) {
  const { colors } = useTheme();
  const { from, to } = range;
  const trained = useMemo(() => new Set(finishedDates(store, from, to)), [store, from, to]);
  const streak = useMemo(() => weekStreak(store, today), [store, today]);
  const weeks: string[] = [];
  for (let w = mondayOf(from); w <= to; w = addDays(w, 7)) weeks.push(w);
  // Squares shrink to fit the weeks in one row down to 8 px, then the row wraps.
  const size = Math.max(8, Math.min(16, Math.floor(width / weeks.length) - gap));
  const fill = (date: string) =>
    date < from || date > to ? "transparent" : trained.has(date) ? colors.accent : colors.border;

  return (
    <Card testID="progress-training">
      <Text variant="heading" accessibilityRole="header">
        Training days
      </Text>
      <CachedView state={trend}>
        {({ weeks: summaries }) => {
          const active = summaries.filter(
            (w) => w.adherence && (w.adherence.planned || w.adherence.done),
          );
          const done = active.reduce((t, w) => t + w.adherence!.done, 0);
          return (
            <StatTiles
              items={[
                {
                  label: "Workouts per week",
                  value: active.length ? (done / active.length).toFixed(1) : "–",
                },
                { label: "Week streak", value: streak },
              ]}
            />
          );
        }}
      </CachedView>
      <View
        accessible
        accessibilityLabel={`${trained.size} training days from ${shortDate(from)} to ${shortDate(to)}.`}
        style={{ flexDirection: "row", flexWrap: "wrap", gap }}
      >
        {weeks.map((monday) => (
          <View key={monday} style={{ gap }}>
            {Array.from({ length: 7 }, (_, d) => addDays(monday, d)).map((date) => (
              <View
                key={date}
                testID={trained.has(date) ? "trained" : undefined}
                style={{ width: size, height: size, borderRadius: 2, backgroundColor: fill(date) }}
              />
            ))}
          </View>
        ))}
      </View>
    </Card>
  );
}
