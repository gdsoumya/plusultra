import { Pressable, View } from "react-native";

import type { ExerciseEntry } from "../../domain/types";
import {
  Icon,
  type ColorRole,
  type IconName,
  Text,
  control,
  iconSize,
  pressedOpacity,
  radius,
  space,
  stroke,
  tabular,
  useTheme,
} from "../../ui";
import { ExerciseThumb } from "../exercises/ExerciseRow";

const width = 72;

const tile = (pressed: boolean) => ({
  width,
  alignItems: "center" as const,
  gap: space[1],
  opacity: pressed ? pressedOpacity : 1,
});

type Props = {
  entry: ExerciseEntry;
  /** The exercise the pane shows; drawn with an accent ring. */
  selected: boolean;
  onPress: () => void;
  onLongPress?: () => void;
  testID: string;
};

// One exercise in the session carousel: picture, short name and done/total sets; a check once
// every set is done.
export function ExerciseTile({ entry, selected, onPress, onLongPress, testID }: Props) {
  const { colors } = useTheme();
  const total = entry.sets.length;
  const done = entry.sets.filter((s) => s.completed).length;
  const complete = total > 0 && done === total;
  return (
    <Pressable
      accessibilityRole="button"
      accessibilityLabel={`${entry.name}, ${done} of ${total} sets`}
      accessibilityState={{ selected }}
      onPress={onPress}
      onLongPress={onLongPress}
      testID={testID}
      style={({ pressed }) => tile(pressed)}
    >
      <View
        style={{
          padding: stroke.focus,
          borderRadius: radius.control + 2 * stroke.focus,
          borderWidth: stroke.focus,
          borderColor: selected ? colors.accent : "transparent",
        }}
      >
        <ExerciseThumb exercise={{ slug: entry.slug, name: entry.name }} size={control.md} />
        {complete && (
          <View
            testID={`${testID}-check`}
            style={{
              position: "absolute",
              right: -space[1],
              bottom: -space[1],
              borderRadius: radius.pill,
              backgroundColor: colors.bg,
            }}
          >
            <Icon name="checkmark-circle" size={iconSize.md} color="success" />
          </View>
        )}
      </View>
      <Text variant="caption" color={selected ? "text" : "textMuted"} numberOfLines={1}>
        {entry.name}
      </Text>
      <Text variant="caption" color={complete ? "success" : "textMuted"} style={tabular}>
        {`${done}/${total}`}
      </Text>
    </Pressable>
  );
}

type ActionProps = {
  icon: IconName;
  label: string;
  tone?: ColorRole;
  onPress: () => void;
  testID: string;
};

// Add, Exercises and Finish: the same footprint as an exercise tile, a glyph instead of a picture.
export function ActionTile({ icon, label, tone = "text", onPress, testID }: ActionProps) {
  const { colors } = useTheme();
  return (
    <Pressable
      accessibilityRole="button"
      accessibilityLabel={label}
      onPress={onPress}
      testID={testID}
      style={({ pressed }) => tile(pressed)}
    >
      <View
        style={{
          width: control.md,
          height: control.md,
          margin: 2 * stroke.focus,
          borderRadius: radius.control,
          borderWidth: stroke.control,
          borderColor: colors.border,
          backgroundColor: colors.surface,
          alignItems: "center",
          justifyContent: "center",
        }}
      >
        <Icon name={icon} color={tone} />
      </View>
      <Text variant="caption" color={tone} numberOfLines={1}>
        {label}
      </Text>
    </Pressable>
  );
}
