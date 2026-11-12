import { useRouter, useSegments } from "expo-router";
import { useState } from "react";
import { Pressable, View } from "react-native";

import { weekdayDate } from "../../domain/dates";
import { doneSummary, setDelta, setLine } from "../../domain/lastSession";
import type { ExerciseEntry, Unit, WorkoutRecord } from "../../domain/types";
import { formatWeight } from "../../domain/units";
import {
  Button,
  Icon,
  iconSize,
  pressedOpacity,
  radius,
  space,
  stroke,
  tabular,
  Text,
  useBackToClose,
  useTheme,
} from "../../ui";
import { useLastSession } from "./useLastSession";

type Props = {
  entry: ExerciseEntry;
  record: Pick<WorkoutRecord, "id" | "date">;
  unit: Unit;
  index: number;
};

// What the exercise looked like last time (P2-R53): date and set summary on one quiet row that
// expands to every set with its delta against today's plan, the best set and a way to the session.
export function LastTimeRow({ entry, record, unit, index }: Props) {
  const last = useLastSession(entry, record);
  const [open, setOpen] = useState(false);
  const router = useRouter();
  // The Calendar tab has the workout screen at its own path; the plain one would switch to Today.
  const calendar = useSegments().includes("(calendar)");
  const { colors } = useTheme();
  useBackToClose(() => setOpen(false), open);
  if (!last) return null;

  const unilateral = !!entry.unilateral;
  const best = `${formatWeight(last.best.weight, unit)} x ${last.best.reps}`;
  const summary = last.sets.length ? doneSummary(last.sets, unilateral, unit) : `Best ${best}`;
  const planned = entry.sets.filter((s) => !s.planned.warmup);
  const id = `exercise-${index}-last`;
  return (
    <Pressable
      accessibilityRole="button"
      accessibilityLabel={`Last time ${weekdayDate(last.date)}, ${summary}`}
      accessibilityState={{ expanded: open }}
      onPress={() => setOpen((o) => !o)}
      testID={id}
      style={({ pressed }) => ({
        backgroundColor: colors.surfaceRaised,
        borderColor: colors.border,
        borderWidth: stroke.control,
        borderRadius: radius.control,
        padding: space[3],
        gap: space[2],
        opacity: pressed ? pressedOpacity : 1,
      })}
    >
      <View style={{ flexDirection: "row", alignItems: "center", gap: space[2] }}>
        <Icon name="time-outline" size={iconSize.md} color="textMuted" />
        <View style={{ flex: 1, gap: space[1] }}>
          <Text variant="caption" numberOfLines={1}>
            {`Last time · ${weekdayDate(last.date)}`}
          </Text>
          <Text variant="label" numberOfLines={2} style={tabular}>
            {summary}
          </Text>
        </View>
        <Icon name={open ? "chevron-up" : "chevron-down"} size={iconSize.md} color="textMuted" />
      </View>
      {open && (
        <View style={{ gap: space[1] }} testID={`${id}-sets`}>
          {last.sets.map((s, i) => {
            const delta = planned[i] && setDelta(planned[i].planned, s, unit);
            return (
              <View
                key={i}
                style={{ flexDirection: "row", alignItems: "center", gap: space[3] }}
                testID={`${id}-set-${i}`}
              >
                <Text variant="caption" style={{ width: space[10] }}>
                  {`Set ${i + 1}`}
                </Text>
                <Text variant="label" style={[tabular, { flex: 1 }]}>
                  {setLine(s, unilateral, unit)}
                </Text>
                {delta && (
                  <Text variant="caption" color={delta.tone} style={tabular}>
                    {delta.text}
                  </Text>
                )}
              </View>
            );
          })}
          <Text variant="caption" style={tabular} testID={`${id}-best`}>
            {`Best ${best}${last.e1rm ? ` · e1RM ${formatWeight(last.e1rm, unit)}` : ""}`}
          </Text>
          {last.recordId && (
            <Button
              title="Open session"
              variant="ghost"
              icon="open-outline"
              style={{ alignSelf: "flex-start" }}
              onPress={() =>
                router.push(
                  calendar
                    ? `/day/${last.date}/workout/${last.recordId}`
                    : `/workout/${last.recordId}`,
                )
              }
            />
          )}
        </View>
      )}
    </Pressable>
  );
}
