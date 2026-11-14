import { View } from "react-native";

import { Button, space } from "../../ui";

type Props = {
  onAdd: () => void;
  onReorder: () => void;
  onFinish?: () => void;
};

// Always in view under the exercise carousel. With Finish, each button takes its own width and
// shares what is left, so the three fit one line at 390 px; with a large system font only "Add
// exercise" gives way, wrapping between its two words. Without Finish (a finished workout) the two
// split the row equally.
export function SessionActions({ onAdd, onReorder, onFinish }: Props) {
  const fill = onFinish
    ? { flexGrow: 1, flexShrink: 0, paddingHorizontal: space[2] }
    : { flex: 1, flexBasis: 0, paddingHorizontal: space[2] };
  return (
    <View style={{ flexDirection: "row", gap: space[2] }} testID="session-actions">
      <Button
        title="Add exercise"
        icon="add"
        variant="secondary"
        style={[fill, { flexShrink: 1 }]}
        onPress={onAdd}
        testID="add-exercise-button"
      />
      <Button
        title="Reorder"
        icon="reorder-three-outline"
        variant="secondary"
        style={fill}
        onPress={onReorder}
        testID="reorder-button"
      />
      {onFinish && (
        <Button
          title="Finish"
          icon="flag"
          style={fill}
          onPress={onFinish}
          testID="finish-workout"
        />
      )}
    </View>
  );
}
