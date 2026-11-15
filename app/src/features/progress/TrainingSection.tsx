import { useMemo, useState } from "react";
import { View } from "react-native";

import type { Store } from "../../data/store";
import { addDays, mondayOf, shortDate } from "../../domain/dates";
import { Card, stroke, Text, typography, useTheme } from "../../ui";
import { StatTiles } from "../../ui/charts/StatTiles";
import { DAILY_UP_TO, type SectionProps } from "./shared";

type Props = Pick<SectionProps, "store" | "range" | "today" | "width">;

// Room for the weekday initials beside the week columns.
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
  // The grid takes the card's real width once laid out; the screen's estimate serves until then.
  const [measured, setMeasured] = useState<number>();
  const avail = measured ?? width;
  const weeks: string[] = [];
  for (let w = mondayOf(from); w <= to; w = addDays(w, 7)) weeks.push(w);
  // Up to four weeks the weeks are rows under the weekday initials, like a calendar; beyond that
  // they are columns beside them, shrinking so a year still fits one row (about 4 px on a phone).
  const calendar = range.days <= DAILY_UP_TO;
  const gap = weeks.length > 30 ? 1 : 2;
  const size = calendar
    ? Math.min(28, Math.floor((avail - 6 * gap) / 7))
    : Math.max(
        4,
        Math.min(16, Math.floor((avail - labelColumn - weeks.length * gap) / weeks.length)),
      );
  const along = calendar ? "row" : "column";
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
        onLayout={(e) => setMeasured(e.nativeEvent.layout.width)}
        style={{ flexDirection: calendar ? "column" : "row", gap }}
      >
        <View style={{ flexDirection: along, gap }}>
          {weekdays.map((name, i) => (
            <Text
              key={i}
              variant="caption"
              style={{
                width: calendar ? size : labelColumn,
                height: size,
                lineHeight: size,
                fontSize: Math.min(typography.caption.fontSize, size),
                textAlign: calendar ? "center" : "left",
              }}
            >
              {name}
            </Text>
          ))}
        </View>
        {weeks.map((monday) => (
          <View key={monday} style={{ flexDirection: along, gap }}>
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
