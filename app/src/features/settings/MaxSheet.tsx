import { create } from "@bufbuild/protobuf";
import { useState } from "react";

import { fromProto } from "../../api/convert";
import type { Unit as UnitName } from "../../domain/types";
import { inUnit } from "../../domain/units";
import { Unit, type Weight } from "../../gen/plusultra/v1/common_pb";
import { ExerciseDataSchema, type ExerciseData } from "../../gen/plusultra/v1/exercise_pb";
import { Button, Chip, ChipStrip, ErrorLine, Field, Sheet } from "../../ui";
import { draftOf, parseRounding } from "./rounding";
import { RoundingFields } from "./RoundingFields";

export function MaxSheet({
  open,
  slug,
  name,
  data,
  unit,
  busy,
  error,
  onSave,
  onClose,
}: {
  open: boolean;
  slug: string;
  name: string;
  /** What the server holds for the exercise; unset when nothing is stored. */
  data?: ExerciseData;
  unit: UnitName;
  busy: boolean;
  error?: string;
  onSave: (data: ExerciseData) => void;
  onClose: () => void;
}) {
  const [text, setText] = useState(
    data?.rm1 ? String(inUnit(fromProto.weight(data.rm1), unit)) : "",
  );
  const [custom, setCustom] = useState(!!data?.rounding);
  const [unilateral, setUnilateral] = useState(!!data?.unilateral);
  const [draft, setDraft] = useState(() => draftOf(data?.rounding));
  const [problem, setProblem] = useState<string>();

  const store = (rm1?: Weight) => {
    try {
      onSave(
        create(ExerciseDataSchema, {
          slug,
          rm1,
          rounding: custom ? parseRounding(draft) : undefined,
          notes: data?.notes ?? "",
          unilateral,
        }),
      );
    } catch (e) {
      setProblem(e instanceof Error ? e.message : String(e));
    }
  };

  // An empty field keeps no 1RM, so the other settings save for exercises without one.
  const save = () => {
    if (!text.trim()) return store();
    const value = Number(text.trim());
    if (!Number.isFinite(value) || value <= 0) {
      return setProblem("1RM must be a weight above 0");
    }
    store({ $typeName: "plusultra.v1.Weight", value, unit: unit === "lb" ? Unit.LB : Unit.KG });
  };

  return (
    <Sheet open={open} onClose={onClose} title={name} snapPoints={[0.75]}>
      <Field
        label={`1RM (${unit})`}
        helper="Percentage weights in programs are taken from this"
        numeric
        value={text}
        onChangeText={setText}
      />
      <ChipStrip>
        <Chip label="Custom rounding" selected={custom} onPress={() => setCustom((c) => !c)} />
        <Chip
          label="Unilateral (log left and right reps)"
          selected={unilateral}
          onPress={() => setUnilateral((u) => !u)}
        />
      </ChipStrip>
      {custom && <RoundingFields value={draft} unit={unit} onChange={setDraft} />}
      <ErrorLine message={problem ?? error} />
      <Button title="Save" onPress={save} loading={busy} />
      {data?.rm1 && (
        <Button title="Clear 1RM" variant="secondary" onPress={() => store()} disabled={busy} />
      )}
    </Sheet>
  );
}
