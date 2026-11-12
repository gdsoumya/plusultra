import { ScrollView, View } from "react-native";

import type { ExerciseEntry } from "../../domain/types";
import { DraggableList, IconButton, Sheet, space, Text } from "../../ui";
import { ExerciseThumb } from "../exercises/ExerciseRow";

type Props = {
  open: boolean;
  entries: ExerciseEntry[];
  onMove: (from: number, to: number) => void;
  onRemove: (index: number) => void;
  onClose: () => void;
};

// Today's exercises in order: hold and drag to reorder, X to remove one.
export function ExercisesSheet({ open, entries, onMove, onRemove, onClose }: Props) {
  return (
    <Sheet
      open={open}
      onClose={onClose}
      title="Reorder"
      snapPoints={[0.7]}
      scroll={false}
      testID="exercises-sheet"
    >
      <Text variant="caption">Hold a row and drag to reorder.</Text>
      <ScrollView keyboardShouldPersistTaps="handled" testID="exercises-scroll">
        <DraggableList
          items={entries}
          keyOf={(_, i) => String(i)}
          testID="exercises"
          onMove={onMove}
          renderItem={(entry, i) => (
            <View style={{ flexDirection: "row", alignItems: "center", gap: space[3] }}>
              <ExerciseThumb exercise={{ slug: entry.slug, name: entry.name }} />
              <Text numberOfLines={1} style={{ flex: 1 }}>
                {entry.name}
              </Text>
              <IconButton
                icon="close"
                label={`Remove ${entry.name}`}
                onPress={() => onRemove(i)}
                testID={`exercise-${i}-remove`}
              />
            </View>
          )}
        />
      </ScrollView>
    </Sheet>
  );
}
