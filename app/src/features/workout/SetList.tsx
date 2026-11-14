import { useState } from "react";
import { View } from "react-native";

import type { ExerciseEntry, Unit } from "../../domain/types";
import { Button, Card, Text, space } from "../../ui";
import { NoteSheet } from "./NoteSheet";
import type { Values } from "./SetEditor";
import { SetRow } from "./SetRow";
import { addSet, setNotes } from "./workoutEdits";

type Props = {
  entry: ExerciseEntry;
  index: number;
  unit: Unit;
  /** Index of the next set to do, when it is in this exercise. */
  active?: number;
  /** Index of the set whose work timer is running. */
  running?: number;
  /** Index of the set whose inline editor is open. */
  editing?: number;
  onDone: (set: number) => void;
  onEdit: (set: number) => void;
  onRemoveSet: (set: number) => void;
  onSetValues: (set: number, values: Values) => void;
  onChange: (edit: (e: ExerciseEntry) => ExerciseEntry) => void;
};

// Every set of the exercise, warmups first, numbered within their group, then the lifter's note
// with Add set and Add note.
export function SetList(p: Props) {
  const { entry, index, unit } = p;
  const [noting, setNoting] = useState(false);
  const notes = entry.notes ?? "";
  let warmups = 0;
  let work = 0;
  return (
    <View style={{ gap: space[2] }}>
      {entry.sets.map((set, i) => (
        <SetRow
          key={i}
          set={set}
          number={set.planned.warmup ? ++warmups : ++work}
          unit={unit}
          unilateral={!!entry.unilateral}
          active={p.active === i}
          running={p.running === i}
          editing={p.editing === i}
          onDone={() => p.onDone(i)}
          onEdit={() => p.onEdit(i)}
          onRemove={() => p.onRemoveSet(i)}
          onChange={(values) => p.onSetValues(i, values)}
          testID={`set-${index}-${i}`}
        />
      ))}
      {notes ? (
        <Card style={{ padding: space[3] }}>
          <Text color="textMuted" selectable testID={`exercise-${index}-note`}>
            {notes}
          </Text>
        </Card>
      ) : null}
      <View style={{ flexDirection: "row", flexWrap: "wrap", gap: space[2] }}>
        <Button title="Add set" variant="ghost" icon="add" onPress={() => p.onChange(addSet)} />
        <Button
          title={notes ? "Edit note" : "Add note"}
          variant="ghost"
          icon="create-outline"
          onPress={() => setNoting(true)}
        />
      </View>
      {noting && (
        <NoteSheet
          notes={notes}
          onSave={(text) => {
            p.onChange((e) => setNotes(e, text));
            setNoting(false);
          }}
          onClose={() => setNoting(false)}
        />
      )}
    </View>
  );
}
