import { useState } from "react";
import { View } from "react-native";

import { Button, Field, Sheet, space } from "../../ui";

type Props = {
  notes: string;
  /** The trimmed note; "" removes it. */
  onSave(notes: string): void;
  onClose(): void;
};

export function NoteSheet({ notes, onSave, onClose }: Props) {
  const [text, setText] = useState(notes);
  const trimmed = text.trim();
  return (
    <Sheet
      open
      title={notes ? "Edit note" : "Add note"}
      onClose={onClose}
      snapPoints={[0.6]}
      testID="note-sheet"
      footer={
        <View style={{ gap: space[2] }}>
          <Button title="Save" disabled={!trimmed && !notes} onPress={() => onSave(trimmed)} />
          {notes ? <Button title="Remove note" variant="ghost" onPress={() => onSave("")} /> : null}
        </View>
      }
    >
      <Field
        label="Note"
        value={text}
        onChangeText={setText}
        placeholder="Felt heavy, left knee twinge on set 3"
        multiline
        autoFocus
      />
    </Sheet>
  );
}
