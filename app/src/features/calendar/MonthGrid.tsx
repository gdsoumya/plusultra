import { Pressable, View } from "react-native";

import { dateLabel, monthOf } from "../../domain/dates";
import type { DayStatus } from "../../domain/types";
import { minTouch, radius, space, stroke, tabular, Text, useTheme, type ColorRole } from "../../ui";
import type { MonthDay } from "./month";

export const markerColor: Record<DayStatus, ColorRole> = {
  done: "success",
  partial: "warning",
  skipped: "danger",
  rest: "textMuted",
  planned: "accent",
};

const weekdays = ["Mon", "Tue", "Wed", "Thu", "Fri", "Sat", "Sun"];

// A filled dot for how a day went; planned days are a ring, so they read apart from done ones.
export function Marker({ status }: { status: DayStatus }) {
  const { colors } = useTheme();
  const color = colors[markerColor[status]];
  return (
    <View
      style={{
        width: space[2],
        height: space[2],
        borderRadius: space[1],
        borderWidth: stroke.control,
        borderColor: color,
        backgroundColor: status === "planned" ? "transparent" : color,
      }}
    />
  );
}

// A check-in is a fact, not a verdict on the day, so it gets its own small blue dot.
export function CheckinMarker() {
  const { colors } = useTheme();
  return (
    <View
      testID="checkin-marker"
      style={{
        width: space[1] + 2,
        height: space[1] + 2,
        borderRadius: space[1],
        backgroundColor: colors.info,
      }}
    />
  );
}

type GridProps = {
  month: string;
  weeks: string[][];
  days: Record<string, MonthDay>;
  today: string;
  onPress: (date: string) => void;
};

export function MonthGrid({ month, weeks, days, today, onPress }: GridProps) {
  const { colors } = useTheme();
  return (
    <View style={{ gap: space[1] }}>
      <View style={{ flexDirection: "row" }}>
        {weekdays.map((name) => (
          <Text key={name} variant="caption" style={{ flex: 1, textAlign: "center" }}>
            {name}
          </Text>
        ))}
      </View>
      {weeks.map((week) => (
        <View key={week[0]} style={{ flexDirection: "row" }}>
          {week.map((date) => {
            const day = days[date];
            const isToday = date === today;
            const label = [
              dateLabel(date),
              day?.status,
              day?.checkin && "check-in",
              isToday && "today",
            ]
              .filter(Boolean)
              .join(", ");
            return (
              <Pressable
                key={date}
                accessibilityRole="button"
                accessibilityLabel={label}
                onPress={() => onPress(date)}
                style={({ pressed }) => ({
                  flex: 1,
                  minHeight: minTouch + space[2],
                  alignItems: "center",
                  justifyContent: "center",
                  gap: space[1],
                  borderRadius: radius.control,
                  borderWidth: stroke.focus,
                  borderColor: isToday ? colors.accent : "transparent",
                  backgroundColor: pressed
                    ? colors.surfaceRaised
                    : isToday
                      ? colors.accentSoft
                      : "transparent",
                })}
              >
                <Text
                  variant="label"
                  color={monthOf(date) === month ? "text" : "textMuted"}
                  style={tabular}
                >
                  {Number(date.slice(8))}
                </Text>
                <View
                  style={{
                    flexDirection: "row",
                    alignItems: "center",
                    gap: space[1],
                    minHeight: space[2],
                  }}
                >
                  {day?.status && <Marker status={day.status} />}
                  {day?.checkin && <CheckinMarker />}
                </View>
              </Pressable>
            );
          })}
        </View>
      ))}
    </View>
  );
}
