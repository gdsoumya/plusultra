import { Pressable } from "react-native";

import { Icon, type IconName } from "./Icon";
import { Text } from "./Text";
import { useTheme } from "./theme";
import { control, iconSize, minTouch, pressedOpacity, radius, space, stroke } from "./tokens";

export type ChipProps = {
  label: string;
  selected?: boolean;
  onPress?: () => void;
  icon?: IconName;
  testID?: string;
};

// Selected fills with the accent; unselected is a raised outline. The label never wraps: a chip
// that does not fit moves to the next line of its row instead.
export function Chip({ label, selected = false, onPress, icon, testID }: ChipProps) {
  const { colors } = useTheme();
  const fg = selected ? "onAccent" : "text";
  return (
    <Pressable
      role="checkbox"
      accessibilityLabel={label}
      aria-checked={selected}
      onPress={onPress}
      testID={testID}
      hitSlop={(minTouch - control.sm) / 2}
      style={({ pressed }) => ({
        minHeight: control.sm,
        maxWidth: "100%",
        paddingHorizontal: space[3],
        borderRadius: radius.control,
        borderWidth: stroke.control,
        borderColor: selected ? colors.accent : colors.border,
        backgroundColor: selected ? colors.accent : colors.surfaceRaised,
        flexDirection: "row",
        alignItems: "center",
        alignSelf: "flex-start",
        gap: space[1],
        opacity: pressed ? pressedOpacity : 1,
      })}
    >
      {icon && <Icon name={icon} size={iconSize.sm} color={fg} />}
      <Text variant="label" color={fg} numberOfLines={1} style={{ flexShrink: 1 }}>
        {label}
      </Text>
    </Pressable>
  );
}
