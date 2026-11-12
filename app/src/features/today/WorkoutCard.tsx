import { View } from "react-native";

import { setsSummary } from "../../domain/sets";
import type { DayLog, ProgramSummary, ResolvedDay, Unit, WorkoutRecord } from "../../domain/types";
import { Button, Card, Text, space, tabular } from "../../ui";
import { CardMenu, type MenuItem } from "../calendar/CardMenu";
import { ExerciseThumb } from "../exercises/ExerciseRow";

type Props = {
  program?: ProgramSummary;
  plan?: ResolvedDay;
  record?: WorkoutRecord;
  dayLog?: DayLog;
  unit: Unit;
  onOpen: () => void;
  onDone: () => void;
  menu: MenuItem[];
};

function statusOf(record?: WorkoutRecord, dayLog?: DayLog) {
  if (!record) return dayLog?.status === "skipped" ? "Skipped" : undefined;
  if (record.finalized) return "Finished";
  if (record.finalizeRequested) return "Finished, waiting to sync";
  const sets = record.entries.flatMap((e) => e.sets);
  return `In progress · ${sets.filter((s) => s.completed).length} of ${sets.length} sets`;
}

// The planned day with its exercises, while today's record (if any) is that day; any other record
// (ad hoc, another program's day, or a floating day the cursor has moved past) lists its own; a
// rest day with neither says so.
export function WorkoutCard({ program, plan, record, dayLog, unit, onOpen, onDone, menu }: Props) {
  const ofPlan =
    !record ||
    (record.programId === program?.id && record.week === plan?.week && record.day === plan?.day);
  const planned = !!plan && plan.kind !== "rest" && plan.exercises.length > 0 && ofPlan;
  const ended = record?.finalized || record?.finalizeRequested;
  const status = statusOf(record, dayLog);
  const lines =
    planned && plan
      ? plan.exercises.map((e) => ({
          slug: e.slug,
          name: e.name,
          summary: setsSummary(e.sets, unit) + (e.unilateral ? " · per side" : ""),
        }))
      : (record?.entries ?? []).map((e) => ({
          slug: e.slug,
          name: e.name,
          summary: `${e.sets.length} set${e.sets.length === 1 ? "" : "s"}`,
        }));
  const caption =
    !plan || !ofPlan
      ? record?.week
        ? `Week ${record.week}, day ${record.day}`
        : "Ad hoc"
      : program?.mode === "floating"
        ? `Next: week ${plan.week} day ${plan.day}`
        : `Week ${plan.week}, day ${plan.day}`;
  return (
    <Card>
      <View style={{ flexDirection: "row", alignItems: "flex-start", gap: space[3] }}>
        <View style={{ flex: 1 }}>
          <Text variant="caption">{caption}</Text>
          <Text variant="heading">
            {planned && plan ? plan.name : record ? record.dayName || "Workout" : "Rest day"}
          </Text>
        </View>
        <CardMenu title="Workout" items={menu} />
      </View>
      {lines.length > 0 ? (
        <View style={{ gap: space[2] }}>
          {lines.map((e, i) => (
            <View key={i} style={{ flexDirection: "row", alignItems: "center", gap: space[3] }}>
              <ExerciseThumb exercise={{ slug: e.slug, name: e.name }} />
              {/* Name and summary each take one truncated line, so nothing leaves the card. */}
              <View style={{ flex: 1 }} testID={`exercise-line-${i}`}>
                <Text numberOfLines={1}>{e.name}</Text>
                <Text color="textMuted" numberOfLines={1} style={tabular}>
                  {e.summary}
                </Text>
              </View>
            </View>
          ))}
        </View>
      ) : (
        !record && <Text color="textMuted">Nothing planned. Recover well.</Text>
      )}
      {status && <Text variant="label">{status}</Text>}
      {(planned || record) && (
        <Button
          title={!record ? "Start workout" : ended ? "Edit workout" : "Continue workout"}
          icon={record ? undefined : "play"}
          onPress={onOpen}
        />
      )}
      {planned && !ended && (
        <Button
          title="Done as planned"
          variant="secondary"
          icon="checkmark-done"
          onPress={onDone}
        />
      )}
    </Card>
  );
}
