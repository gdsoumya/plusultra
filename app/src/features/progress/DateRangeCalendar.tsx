import { useState } from "react";
import { Pressable, View } from "react-native";

import {
  addMonths,
  dateLabel,
  monthGrid,
  monthLabel,
  monthOf,
  shortDate,
} from "../../domain/dates";
import { IconButton, minTouch, radius, space, tabular, Text, useTheme } from "../../ui";

export type Span = { from?: string; to?: string };

type Props = { span: Span; initialMonth: string; onChange: (span: Span) => void };

const weekdays = ["Mon", "Tue", "Wed", "Thu", "Fri", "Sat", "Sun"];

// A month at a time, as on Calendar; the first tap starts the span, the second ends it, and a tap
// before the start begins again.
export function DateRangeCalendar({ span: { from, to }, initialMonth, onChange }: Props) {
  const { colors } = useTheme();
  const [month, setMonth] = useState(initialMonth);
  const pick = (date: string) =>
    onChange(!from || to || date < from ? { from: date } : { from, to: date });
  const within = (date: string) => !!from && date >= from && date <= (to ?? from);
  const isEnd = (date: string) => date === from || date === to;

  return (
    <View style={{ gap: space[1] }}>
      <View style={{ flexDirection: "row", alignItems: "center" }}>
        <IconButton
          icon="chevron-back"
          label="Previous month"
          onPress={() => setMonth(addMonths(month, -1))}
        />
        <Text variant="heading" style={{ flex: 1, textAlign: "center" }}>
          {monthLabel(month)}
        </Text>
        <IconButton
          icon="chevron-forward"
          label="Next month"
          onPress={() => setMonth(addMonths(month, 1))}
        />
      </View>
      <View style={{ flexDirection: "row" }}>
        {weekdays.map((name) => (
          <Text key={name} variant="caption" style={{ flex: 1, textAlign: "center" }}>
            {name}
          </Text>
        ))}
      </View>
      {monthGrid(month).map((week) => (
        <View key={week[0]} style={{ flexDirection: "row" }}>
          {week.map((date) => (
            <Pressable
              key={date}
              accessibilityRole="button"
              accessibilityLabel={dateLabel(date)}
              accessibilityState={{ selected: within(date) }}
              onPress={() => pick(date)}
              style={{
                flex: 1,
                minHeight: minTouch,
                alignItems: "center",
                justifyContent: "center",
                borderRadius: radius.control,
                backgroundColor: isEnd(date)
                  ? colors.accent
                  : within(date)
                    ? colors.accentSoft
                    : "transparent",
              }}
            >
              <Text
                variant="label"
                color={isEnd(date) ? "onAccent" : monthOf(date) === month ? "text" : "textMuted"}
                style={tabular}
              >
                {Number(date.slice(8))}
              </Text>
            </Pressable>
          ))}
        </View>
      ))}
      <Text variant="caption">
        {`From ${from ? shortDate(from) : "…"} – To ${to ? shortDate(to) : "…"}`}
      </Text>
    </View>
  );
}
