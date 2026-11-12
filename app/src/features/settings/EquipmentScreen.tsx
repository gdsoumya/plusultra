import { useCallback, useState } from "react";

import { useApi } from "../../api/useApi";
import { useUnit } from "../../data/settings";
import type { Equipment } from "../../gen/plusultra/v1/exercise_pb";
import { Button, EmptyState, ErrorLine, ListCard, ListRow, Loading } from "../../ui";
import { EquipmentSheet } from "./EquipmentSheet";
import { SubScreen } from "./parts";
import { describeRounding } from "./rounding";
import { useRemote } from "./useRemote";

export default function EquipmentScreen() {
  const api = useApi();
  const unit = useUnit();
  const load = useCallback(() => api.exercises.getEquipment({}).then((r) => r.equipment), [api]);
  const { data: equipment = [], setData, loading, busy, error, call } = useRemote(load);
  const [editing, setEditing] = useState<Equipment>();
  const [open, setOpen] = useState(false);
  // Bumped on every open so the sheet starts from the chosen item, not the last edit.
  const [opened, setOpened] = useState(0);

  const edit = (item?: Equipment) => {
    setEditing(item);
    setOpened((n) => n + 1);
    setOpen(true);
  };

  // The server replaces the whole list, so every change sends all of it.
  const save = (list: Equipment[]) =>
    call(async () => {
      const r = await api.exercises.setEquipment({ equipment: list });
      setData(r.equipment);
      setOpen(false);
    });

  return (
    <SubScreen title="Equipment" testID="equipment-screen">
      {!open && <ErrorLine message={error} />}
      {loading ? (
        <Loading />
      ) : equipment.length ? (
        <ListCard>
          {equipment.map((item) => (
            <ListRow
              key={item.name}
              title={item.name}
              subtitle={describeRounding(item.rounding)}
              onPress={() => edit(item)}
            />
          ))}
        </ListCard>
      ) : (
        !error && (
          <EmptyState
            icon="barbell-outline"
            title="No equipment yet"
            message="Add what you train with so weights round to what you can load."
          />
        )
      )}
      <Button title="Add equipment" icon="add" variant="secondary" onPress={() => edit()} />
      <EquipmentSheet
        key={opened}
        open={open}
        item={editing}
        unit={unit}
        taken={equipment.map((e) => e.name)}
        busy={busy}
        error={error}
        onClose={() => setOpen(false)}
        onSave={(saved) =>
          save(
            editing
              ? equipment.map((e) => (e.name === editing.name ? saved : e))
              : [...equipment, saved],
          )
        }
        onRemove={() => save(equipment.filter((e) => e.name !== editing?.name))}
      />
    </SubScreen>
  );
}
