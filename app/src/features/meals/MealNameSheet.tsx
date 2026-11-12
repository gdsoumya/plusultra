import { useState } from "react";

import { Button, Field, Sheet } from "../../ui";

type Props = {
  onAdd(name: string): void;
  onClose(): void;
};

export function MealNameSheet({ onAdd, onClose }: Props) {
  const [name, setName] = useState("");
  const trimmed = name.trim();
  return (
    <Sheet open title="Add meal" onClose={onClose} snapPoints={[0.4]} testID="meal-sheet">
      <Field
        label="Meal name"
        value={name}
        onChangeText={setName}
        placeholder="Second dinner"
        autoFocus
        onSubmitEditing={() => trimmed && onAdd(trimmed)}
      />
      <Button title="Add" disabled={!trimmed} onPress={() => onAdd(trimmed)} />
    </Sheet>
  );
}
