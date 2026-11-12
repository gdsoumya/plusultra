import { useState } from "react";
import { Pressable, View } from "react-native";

import { useExerciseInfo } from "../../data/exerciseInfo";
import { clock } from "../../domain/dates";
import { exerciseTarget } from "../../domain/sets";
import type { ExerciseEntry, Unit, WorkoutRecord } from "../../domain/types";
import {
  Badge,
  Button,
  Card,
  EmptyState,
  IconButton,
  Sheet,
  Text,
  Thumbnail,
  control,
  space,
} from "../../ui";
import { ExerciseDetail } from "../exercises/ExerciseDetail";
import { ExerciseSheet } from "./ExerciseSheet";
import { LastTimeRow } from "./LastTimeRow";
import type { Values } from "./SetEditor";
import { SetList } from "./SetList";
import { exerciseModified } from "./workoutEdits";

type Props = {
  entry: ExerciseEntry;
  /** The record the entry belongs to; "Last time" looks for earlier ones. */
  record: Pick<WorkoutRecord, "id" | "date">;
  index: number;
  count: number;
  unit: Unit;
  active?: number;
  running?: number;
  editing?: number;
  /** A finished workout: sets are shown as logged, nothing is edited, no Finish button. */
  finished: boolean;
  onDone: (set: number) => void;
  onEdit: (set: number) => void;
  onRemoveSet: (set: number) => void;
  onSetValues: (set: number, values: Values) => void;
  onChange: (edit: (e: ExerciseEntry) => ExerciseEntry) => void;
  onSwitch: (name: string) => void;
  onMove: (to: number) => void;
  onRemove: () => void;
  /** Next exercise, or Finish on the last one. */
  onNext: () => void;
};

// "15 / 15 reps · 1500 / 1500 kg": done against planned over every set, tonnage in whole units
// so the line fits a phone; an ad hoc exercise has no plan to compare with, so just "10 reps ·
// 200 kg". A bodyweight exercise moves no logged weight, so it gets no tonnage. A timed exercise
// counts sets and time instead: "1 / 3 sets · 0:50 / 3:00". Meeting the plan is a badge.
function summary(entry: ExerciseEntry, unit: Unit) {
  const t = exerciseTarget(entry.sets, entry.unilateral, unit);
  if (t.plannedWork) {
    const done = entry.sets.filter((s) => s.completed).length;
    return `${done} / ${entry.sets.length} sets · ${clock(t.work)} / ${clock(t.plannedWork)}`;
  }
  const [load, planned] = [Math.round(t.load), Math.round(t.plannedLoad)];
  const weighed = load > 0 || planned > 0;
  if (!t.plannedReps) return `${t.reps} reps${weighed ? ` · ${load} ${unit}` : ""}`;
  const tonnage = weighed ? ` · ${load} / ${planned} ${unit}` : "";
  return `${t.reps} / ${t.plannedReps} reps${tonnage}`;
}

// shortcut: a note is "long" by its length, not its laid-out lines, since the web has no
// onTextLayout; measure once React Native Web supports it.
const noteLines = 3;
const longNote = (desc: string) => desc.length > 200 || desc.split("\n").length > noteLines;

// The shown exercise: picture, name and plan line, a gear for its settings, the full notes, every
// set, and the way on to the next exercise.
export function ExercisePane(p: Props) {
  const { entry, index, count, unit } = p;
  const { info } = useExerciseInfo({ slug: entry.slug, name: entry.name });
  const [menu, setMenu] = useState(false);
  const [details, setDetails] = useState(false);
  const [noteOpen, setNoteOpen] = useState(false);
  const folded = longNote(entry.desc) && !noteOpen;
  const last = index === count - 1;
  return (
    <View style={{ gap: space[4] }} testID={`exercise-${index}`}>
      <View style={{ flexDirection: "row", alignItems: "flex-start", gap: space[3] }}>
        <Pressable
          accessibilityRole="button"
          accessibilityLabel={`${entry.name} details`}
          onPress={() => setDetails(true)}
          style={{ flex: 1, flexDirection: "row", alignItems: "flex-start", gap: space[3] }}
        >
          <Thumbnail uri={info?.images[0]} size={control.lg} />
          <View style={{ flex: 1, gap: space[1] }}>
            <Text variant="heading" numberOfLines={2}>
              {entry.name}
            </Text>
            {entry.equipment || entry.label ? (
              <Text variant="caption" numberOfLines={1} testID={`exercise-${index}-equipment`}>
                {[entry.equipment, entry.label].filter(Boolean).join(" · ")}
              </Text>
            ) : null}
            <Text variant="caption" numberOfLines={2} testID={`exercise-${index}-summary`}>
              {summary(entry, unit)}
            </Text>
            <View style={{ flexDirection: "row", gap: space[2] }}>
              {exerciseTarget(entry.sets, entry.unilateral, unit).met && (
                <Badge label="On target" icon="checkmark" testID={`exercise-${index}-on-target`} />
              )}
              {entry.superset ? (
                <Badge label={`Superset ${entry.superset}`} icon="link-outline" />
              ) : null}
              {exerciseModified(entry) && (
                <Badge label="Modified" icon="pencil" testID={`exercise-${index}-modified`} />
              )}
            </View>
          </View>
        </Pressable>
        <IconButton
          icon="settings-outline"
          label={`${entry.name} options`}
          onPress={() => setMenu(true)}
          testID={`exercise-${index}-menu`}
        />
      </View>
      <LastTimeRow entry={entry} record={p.record} unit={unit} index={index} />
      {entry.desc ? (
        <Card style={{ padding: space[3], gap: space[2] }}>
          <Text color="textMuted" selectable numberOfLines={folded ? noteLines : undefined}>
            {entry.desc}
          </Text>
          {longNote(entry.desc) && (
            <Button
              title={noteOpen ? "Less" : "More"}
              variant="ghost"
              compact
              onPress={() => setNoteOpen(!noteOpen)}
              style={{ alignSelf: "flex-start" }}
            />
          )}
        </Card>
      ) : null}
      <SetList
        entry={entry}
        index={index}
        unit={unit}
        active={p.active}
        running={p.running}
        editing={p.editing}
        onDone={p.onDone}
        onEdit={p.onEdit}
        onRemoveSet={p.onRemoveSet}
        onSetValues={p.onSetValues}
        onChange={p.onChange}
      />
      {last && p.finished ? null : (
        <Button
          title={last ? "Finish" : "Next exercise"}
          variant="secondary"
          icon={last ? "flag" : "arrow-forward"}
          onPress={p.onNext}
          testID="next-exercise"
        />
      )}
      <Sheet
        open={details}
        onClose={() => setDetails(false)}
        title={entry.name}
        snapPoints={[0.6, 0.9]}
        testID={`exercise-${index}-details`}
      >
        {info ? (
          <ExerciseDetail exercise={info} />
        ) : (
          <EmptyState
            icon="information-circle-outline"
            title="No details"
            message="This exercise is not in the catalog."
          />
        )}
      </Sheet>
      <ExerciseSheet
        open={menu}
        entry={entry}
        index={index}
        count={count}
        onSwitch={p.onSwitch}
        onMove={p.onMove}
        onRemove={() => {
          setMenu(false);
          p.onRemove();
        }}
        onUnilateral={(unilateral) => p.onChange((e) => ({ ...e, unilateral }))}
        onClose={() => setMenu(false)}
      />
    </View>
  );
}
