import { ActivityIndicator, Pressable } from "react-native";

import { Icon, type IconName } from "./Icon";
import { useTheme } from "./theme";
import { disabledOpacity, minTouch, radius } from "./tokens";

export type IconButtonProps = {
  icon: IconName;
  /** The spoken name; the glyph has none. */
  label: string;
  onPress: () => void;
  /** Shows a spinner and blocks presses. */
  loading?: boolean;
  disabled?: boolean;
  testID?: string;
};

// A square touch target with a glyph, for header and toolbar actions.
export function IconButton({
  icon,
  label,
  onPress,
  loading = false,
  disabled = false,
  testID,
}: IconButtonProps) {
  const { colors } = useTheme();
  return (
    <Pressable
      accessibilityRole="button"
      accessibilityLabel={label}
      aria-busy={loading}
      aria-disabled={disabled}
      disabled={loading || disabled}
      onPress={onPress}
      testID={testID}
      style={({ pressed }) => ({
        width: minTouch,
        height: minTouch,
        borderRadius: radius.control,
        alignItems: "center",
        justifyContent: "center",
        backgroundColor: pressed ? colors.surfaceRaised : "transparent",
        opacity: disabled ? disabledOpacity : 1,
      })}
    >
      {loading ? <ActivityIndicator color={colors.text} /> : <Icon name={icon} />}
    </Pressable>
  );
}
