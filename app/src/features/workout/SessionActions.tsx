import { View } from "react-native";

import { Button, space } from "../../ui";

type Props = {
  onAdd: () => void;
  onReorder: () => void;
  onFinish?: () => void;
};

// Always in view under the exercise carousel. Finish takes its own width and the two secondary
// buttons split the rest equally, so the three fit one line at 390 px. With a large system font
// only "Add exercise" gives way, wrapping between its two words.
export function SessionActions({ onAdd, onReorder, onFinish }: Props) {
  const half = { flex: 1, flexBasis: 0, paddingHorizontal: space[2] };
  return (
    <View style={{ flexDirection: "row", gap: space[2] }} testID="session-actions">
      <Button
        title="Add exercise"
        icon="add"
        variant="secondary"
        style={half}
        onPress={onAdd}
        testID="add-exercise-button"
      />
      <Button
        title="Reorder"
        icon="reorder-three-outline"
        variant="secondary"
        style={half}
        onPress={onReorder}
        testID="reorder-button"
      />
      {onFinish && (
        <Button
          title="Finish"
          icon="flag"
          style={{ flexShrink: 0, paddingHorizontal: space[2] }}
          onPress={onFinish}
          testID="finish-workout"
        />
      )}
    </View>
  );
}
