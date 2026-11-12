import { useRouter } from "expo-router";
import { useState } from "react";
import { View } from "react-native";

import { useRefresh } from "../../data/useRefresh";
import { addMonths, monthLabel, monthOf } from "../../domain/dates";
import type { DayStatus } from "../../domain/types";
import { Card, IconButton, Screen, space, Text } from "../../ui";
import { CheckinMarker, Marker, markerColor, MonthGrid } from "./MonthGrid";
import { SyncBadge } from "../sync/SyncBadge";
import { useMonth } from "./useMonth";

export default function CalendarScreen({ today }: { today: string }) {
  const [month, setMonth] = useState(monthOf(today));
  const { weeks, days, next } = useMonth(month, today);
  const router = useRouter();
  // The month is read from the store on every render, so settling the refresh shows the sync.
  const refresh = useRefresh();
  return (
    <Screen title="Calendar" testID="calendar-screen" right={<SyncBadge />} refresh={refresh} wide>
      <Card>
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
        <MonthGrid
          month={month}
          weeks={weeks}
          days={days}
          today={today}
          onPress={(date) => router.push(`/day/${date}`)}
        />
        <View style={{ flexDirection: "row", flexWrap: "wrap", gap: space[3] }}>
          {(Object.keys(markerColor) as DayStatus[]).map((status) => (
            <View
              key={status}
              style={{ flexDirection: "row", alignItems: "center", gap: space[1] }}
            >
              <Marker status={status} />
              <Text variant="caption">{status}</Text>
            </View>
          ))}
          <View style={{ flexDirection: "row", alignItems: "center", gap: space[1] }}>
            <CheckinMarker />
            <Text variant="caption">check-in</Text>
          </View>
        </View>
      </Card>
      {next && (
        <Text color="textMuted">
          Next up: {next.name} (week {next.week}, day {next.day}). A floating program is not placed
          on dates; finish a workout to move on.
        </Text>
      )}
    </Screen>
  );
}
