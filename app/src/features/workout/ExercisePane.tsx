import { useState } from "react";
import { View } from "react-native";

import { useExerciseInfo } from "../../data/exerciseInfo";
import { exerciseTarget } from "../../domain/sets";
import type { ExerciseEntry, Unit } from "../../domain/types";
import { round1 } from "../../domain/units";
import { Badge, Button, Card, IconButton, Text, Thumbnail, control, space } from "../../ui";
import { ExerciseSheet } from "./ExerciseSheet";
import type { Values } from "./SetEditor";
import { SetList } from "./SetList";
import { exerciseModified } from "./workoutEdits";

type Props = {
  entry: ExerciseEntry;
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

// "On target · 15 / 15 reps · 1500 / 1500 kg": done against planned over every set, tonnage in the
// user's unit; an ad hoc exercise has no plan to compare with, so just "10 reps · 200 kg". A
// bodyweight exercise moves no logged weight, so it gets no tonnage.
function summary(entry: ExerciseEntry, unit: Unit) {
  const t = exerciseTarget(entry.sets, entry.unilateral, unit);
  const [load, planned] = [round1(t.load), round1(t.plannedLoad)];
  const weighed = load > 0 || planned > 0;
  if (!t.plannedReps) return `${t.reps} reps${weighed ? ` · ${load} ${unit}` : ""}`;
  const tonnage = weighed ? ` · ${load} / ${planned} ${unit}` : "";
  return `${t.met ? "On target · " : ""}${t.reps} / ${t.plannedReps} reps${tonnage}`;
}

// The shown exercise: picture, name and plan line, a gear for its settings, the full notes, every
// set, and the way on to the next exercise.
export function ExercisePane(p: Props) {
  const { entry, index, count, unit } = p;
  const { info } = useExerciseInfo({ slug: entry.slug, name: entry.name });
  const [menu, setMenu] = useState(false);
  const last = index === count - 1;
  return (
    <View style={{ gap: space[4] }} testID={`exercise-${index}`}>
      <View style={{ flexDirection: "row", alignItems: "flex-start", gap: space[3] }}>
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
          <Text variant="caption" numberOfLines={1} testID={`exercise-${index}-summary`}>
            {summary(entry, unit)}
          </Text>
          <View style={{ flexDirection: "row", gap: space[2] }}>
            {entry.superset ? (
              <Badge label={`Superset ${entry.superset}`} icon="link-outline" />
            ) : null}
            {exerciseModified(entry) && (
              <Badge label="Modified" icon="pencil" testID={`exercise-${index}-modified`} />
            )}
          </View>
        </View>
        <IconButton
          icon="settings-outline"
          label={`${entry.name} options`}
          onPress={() => setMenu(true)}
          testID={`exercise-${index}-menu`}
        />
      </View>
      {entry.desc ? (
        <Card style={{ padding: space[3] }}>
          <Text color="textMuted" selectable>
            {entry.desc}
          </Text>
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
