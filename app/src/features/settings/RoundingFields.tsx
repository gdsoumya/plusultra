import type { Unit } from "../../domain/types";
import { Chip, ChipStrip, Field } from "../../ui";
import { roundingKinds, type RoundingDraft } from "./rounding";

// Rounding numbers are in the user's unit and are not converted when it changes (P2-R30).
export function RoundingFields({
  value,
  unit,
  onChange,
}: {
  value: RoundingDraft;
  unit: Unit;
  onChange: (draft: RoundingDraft) => void;
}) {
  const set = (patch: Partial<RoundingDraft>) => onChange({ ...value, ...patch });
  return (
    <>
      <ChipStrip selected={roundingKinds.findIndex((k) => k === value.kind)}>
        {roundingKinds.map((kind) => (
          <Chip
            key={kind}
            label={kind}
            selected={value.kind === kind}
            onPress={() => set({ kind })}
          />
        ))}
      </ChipStrip>
      {value.kind === "plates" && (
        <Field
          label={`Bar (${unit})`}
          helper="Empty bar weight"
          numeric
          value={value.bar}
          onChangeText={(bar) => set({ bar })}
        />
      )}
      {(value.kind === "plates" || value.kind === "fixed") && (
        <Field
          label={`${value.kind === "plates" ? "Plates per side" : "Weights"} (${unit})`}
          helper="Separate with commas"
          value={value.plates}
          onChangeText={(plates) => set({ plates })}
        />
      )}
      {value.kind === "stack" && (
        <Field
          label={`Increment (${unit})`}
          helper="Step between loadable weights"
          numeric
          value={value.increment}
          onChangeText={(increment) => set({ increment })}
        />
      )}
    </>
  );
}
