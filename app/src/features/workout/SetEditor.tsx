import { useState } from "react";
import { Pressable, View } from "react-native";

import type { SetEntry, Unit } from "../../domain/types";
import { inUnit, toKg } from "../../domain/units";
import { Field, Icon, minTouch, pressedOpacity, space } from "../../ui";
import { shown } from "./workoutEdits";

export type Values = Partial<Pick<SetEntry, "reps" | "repsLeft" | "weight" | "rpe">>;

// shortcut: one plate step for every exercise, swap in the exercise's rounding once exercise data
// is cached in the store.
const weightStep: Record<Unit, number> = { kg: 2.5, lb: 5 };

const decimal = (s: string) => /^\d+(\.\d+)?$/.test(s.trim());

function Stepper({
  label,
  name,
  value,
  step,
  onChange,
}: {
  label: string;
  name: string;
  value: number;
  step: number;
  onChange: (value: number) => void;
}) {
  const [text, setText] = useState(`${value}`);
  const [seen, setSeen] = useState(value);
  // A stepper press or an outside change shows the new value; typing keeps the user's text.
  if (value !== seen) {
    setSeen(value);
    if (Number(text) !== value) setText(`${value}`);
  }
  const nudge = (dir: 1 | -1) => () =>
    onChange(Math.max(0, Math.round((value + dir * step) * 100) / 100));
  const icon = (which: "minus" | "plus", iconName: "remove" | "add", onPress: () => void) => (
    <Pressable
      accessibilityRole="button"
      accessibilityLabel={`${name} ${which}`}
      onPress={onPress}
      style={({ pressed }) => ({
        width: minTouch,
        height: minTouch,
        alignItems: "center",
        justifyContent: "center",
        opacity: pressed ? pressedOpacity : 1,
      })}
    >
      <Icon name={iconName} color="accent" />
    </Pressable>
  );
  return (
    <View style={{ flexDirection: "row", alignItems: "flex-end", gap: space[1] }}>
      {icon("minus", "remove", nudge(-1))}
      <View style={{ flex: 1 }}>
        <Field
          label={label}
          value={text}
          numeric
          onChangeText={(t) => {
            setText(t);
            if (decimal(t)) onChange(Number(t));
          }}
        />
      </View>
      {icon("plus", "add", nudge(1))}
    </View>
  );
}

export function SetEditor({
  set,
  unit,
  unilateral,
  onChange,
}: {
  set: SetEntry;
  unit: Unit;
  unilateral: boolean;
  onChange: (values: Values) => void;
}) {
  const [rpe, setRpe] = useState(set.rpe ? `${set.rpe}` : "");
  const rpeValid = rpe === "" || (decimal(rpe) && Number(rpe) <= 10);
  const current = shown(set, unilateral);
  const reps = unilateral ? "Right reps" : "Reps";
  return (
    <View style={{ gap: space[2] }}>
      {unilateral && (
        <Stepper
          label="Left reps"
          name="Left reps"
          value={current.repsLeft ?? 0}
          step={1}
          onChange={(repsLeft) => onChange({ repsLeft })}
        />
      )}
      <Stepper
        label={reps}
        name={reps}
        value={current.reps}
        step={1}
        onChange={(reps) => onChange({ reps })}
      />
      <Stepper
        label={`Weight (${unit})`}
        name="Weight"
        value={inUnit(current.weight, unit)}
        step={weightStep[unit]}
        onChange={(value) => onChange({ weight: { value: toKg(value, unit), unit: "kg" } })}
      />
      <Field
        label="RPE"
        value={rpe}
        numeric
        helper="Optional, 1 to 10"
        error={rpeValid ? undefined : "RPE is 1 to 10"}
        onChangeText={(t) => {
          setRpe(t);
          if (t === "" || (decimal(t) && Number(t) <= 10)) onChange({ rpe: Number(t) });
        }}
      />
    </View>
  );
}
