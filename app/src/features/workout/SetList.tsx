import { View } from "react-native";

import type { ExerciseEntry, Unit } from "../../domain/types";
import { Button, space } from "../../ui";
import type { Values } from "./SetEditor";
import { SetRow } from "./SetRow";
import { addSet } from "./workoutEdits";

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

// Every set of the exercise, warmups first, numbered within their group, then Add set.
export function SetList(p: Props) {
  const { entry, index, unit } = p;
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
      <Button
        title="Add set"
        variant="ghost"
        icon="add"
        style={{ alignSelf: "flex-start" }}
        onPress={() => p.onChange(addSet)}
      />
    </View>
  );
}
