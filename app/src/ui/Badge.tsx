import { Pressable, View } from "react-native";

import { Icon, type IconName } from "./Icon";
import { Text } from "./Text";
import { useTheme } from "./theme";
import { control, iconSize, pressedOpacity, radius, space, stroke, type ColorRole } from "./tokens";

export type BadgeProps = {
  label: string;
  /** The text and icon color; the pill itself stays a raised neutral. */
  tone?: ColorRole;
  icon?: IconName;
  /** Makes the pill a button, e.g. to open details. */
  onPress?: () => void;
  testID?: string;
};

// A short status word, e.g. "Offline" in a header. It leaves its alignment to the row it sits in,
// so a header centres it with its other controls.
export function Badge({ label, tone = "textMuted", icon, onPress, testID }: BadgeProps) {
  const { colors } = useTheme();
  const pill = {
    minHeight: control.sm - space[2],
    paddingHorizontal: space[3],
    borderRadius: radius.pill,
    borderWidth: stroke.control,
    borderColor: colors.border,
    backgroundColor: colors.surfaceRaised,
    flexDirection: "row",
    alignItems: "center",
    gap: space[1],
    maxWidth: "100%",
  } as const;
  const content = (
    <>
      {icon && <Icon name={icon} size={iconSize.sm} color={tone} />}
      <Text variant="label" color={tone} numberOfLines={1} style={{ flexShrink: 1 }}>
        {label}
      </Text>
    </>
  );
  return onPress ? (
    <Pressable
      testID={testID}
      accessibilityRole="button"
      accessibilityLabel={label}
      onPress={onPress}
      style={({ pressed }) => [pill, { opacity: pressed ? pressedOpacity : 1 }]}
    >
      {content}
    </Pressable>
  ) : (
    <View testID={testID} accessible accessibilityLabel={label} style={pill}>
      {content}
    </View>
  );
}
