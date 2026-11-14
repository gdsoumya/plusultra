import { useMemo } from "react";
import { View } from "react-native";

import type { Store } from "../../data/store";
import { addDays, mondayOf, shortDate } from "../../domain/dates";
import { Card, stroke, Text, typography, useTheme } from "../../ui";
import { StatTiles } from "../../ui/charts/StatTiles";
import type { SectionProps } from "./shared";

type Props = Pick<SectionProps, "store" | "range" | "today" | "width">;

const gap = 2;
// Room for the weekday initials.
const labelColumn = 16;
const weekdays = ["M", "T", "W", "T", "F", "S", "S"];
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

export function TrainingSection({ store, range, today, width }: Props) {
  const { colors } = useTheme();
  const { from, to } = range;
  const trained = useMemo(() => new Set(finishedDates(store, from, to)), [store, from, to]);
  const streak = useMemo(() => weekStreak(store, today), [store, today]);
  // Finished workouts in the range over its length in weeks; days counts the span, so add one.
  const perWeek = trained.size / ((range.days + 1) / 7);
  const weeks: string[] = [];
  for (let w = mondayOf(from); w <= to; w = addDays(w, 7)) weeks.push(w);
  // Squares shrink so the weeks always fit one row; a year on a phone is about 4 px.
  const size = Math.max(
    4,
    Math.min(16, Math.floor((width - labelColumn - (weeks.length - 1) * gap) / weeks.length)),
  );
  const outside = (date: string) => date < from || date > to;
  const square = (date: string) => ({
    width: size,
    height: size,
    borderRadius: 2,
    ...(outside(date)
      ? { borderWidth: stroke.control, borderColor: colors.border, opacity: 0.3 }
      : { backgroundColor: trained.has(date) ? colors.accent : colors.border }),
  });

  return (
    <Card testID="progress-training">
      <Text variant="heading" accessibilityRole="header">
        Training days
      </Text>
      <StatTiles
        items={[
          { label: "Workouts per week", value: String(Math.round(perWeek * 10) / 10) },
          { label: "Week streak", value: streak },
        ]}
      />
      <View
        accessible
        accessibilityLabel={`${trained.size} training days from ${shortDate(from)} to ${shortDate(to)}.`}
        style={{ flexDirection: "row", gap }}
      >
        <View style={{ width: labelColumn, gap }}>
          {weekdays.map((name, i) => (
            <Text
              key={i}
              variant="caption"
              style={{
                height: size,
                lineHeight: size,
                fontSize: Math.min(typography.caption.fontSize, size),
              }}
            >
              {name}
            </Text>
          ))}
        </View>
        {weeks.map((monday) => (
          <View key={monday} style={{ gap }}>
            {Array.from({ length: 7 }, (_, d) => addDays(monday, d)).map((date) => (
              <View
                key={date}
                testID={outside(date) ? "outside" : trained.has(date) ? "trained" : undefined}
                style={square(date)}
              />
            ))}
          </View>
        ))}
      </View>
    </Card>
  );
}
