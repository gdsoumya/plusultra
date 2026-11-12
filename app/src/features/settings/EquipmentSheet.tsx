import { create } from "@bufbuild/protobuf";
import { useState } from "react";

import type { Unit } from "../../domain/types";
import { EquipmentSchema, type Equipment } from "../../gen/plusultra/v1/exercise_pb";
import { Button, ErrorLine, Field, Sheet, Text } from "../../ui";
import { draftOf, parseRounding } from "./rounding";
import { RoundingFields } from "./RoundingFields";

export function EquipmentSheet({
  open,
  item,
  unit,
  taken,
  busy,
  error,
  onSave,
  onRemove,
  onClose,
}: {
  open: boolean;
  /** The equipment being edited; unset when adding. */
  item?: Equipment;
  unit: Unit;
  /** Names already in use, so a new piece cannot shadow one. */
  taken: string[];
  busy: boolean;
  error?: string;
  onSave: (equipment: Equipment) => void;
  onRemove: () => void;
  onClose: () => void;
}) {
  const [name, setName] = useState(item?.name ?? "");
  const [draft, setDraft] = useState(() => draftOf(item?.rounding));
  const [problem, setProblem] = useState<string>();

  const save = () => {
    const trimmed = name.trim().toLowerCase();
    if (!trimmed) return setProblem("Give it a name");
    if (!item && taken.includes(trimmed)) return setProblem(`You already have ${trimmed}`);
    try {
      onSave(create(EquipmentSchema, { name: trimmed, rounding: parseRounding(draft) }));
    } catch (e) {
      setProblem(e instanceof Error ? e.message : String(e));
    }
  };

  return (
    <Sheet open={open} onClose={onClose} title={item?.name ?? "New equipment"} snapPoints={[0.75]}>
      {!item && (
        <Field
          label="Name"
          helper="As exercises name it, for example barbell or cable"
          autoCapitalize="none"
          value={name}
          onChangeText={setName}
        />
      )}
      <RoundingFields value={draft} unit={unit} onChange={setDraft} />
      <Text variant="caption">{`Rounding values are in ${unit}. Changing units does not convert them.`}</Text>
      <ErrorLine message={problem ?? error} />
      <Button title="Save" onPress={save} loading={busy} />
      {item && <Button title="Remove" variant="danger" onPress={onRemove} disabled={busy} />}
    </Sheet>
  );
}
