import * as Haptics from "expo-haptics";
import { ActivityIndicator, Pressable, type StyleProp, type ViewStyle } from "react-native";

import { Icon, type IconName } from "./Icon";
import { Text } from "./Text";
import { useTheme } from "./theme";
import {
  control,
  disabledOpacity,
  iconSize,
  pressedOpacity,
  pressedScale,
  radius,
  space,
  stroke,
  type ColorRole,
} from "./tokens";

export type ButtonVariant = "primary" | "secondary" | "ghost" | "danger";

export type ButtonProps = {
  title: string;
  onPress?: () => void;
  variant?: ButtonVariant;
  icon?: IconName;
  /** Icon only, for a narrow header; the title stays the accessible name. */
  compact?: boolean;
  /** Shows a spinner and blocks presses. */
  loading?: boolean;
  disabled?: boolean;
  style?: StyleProp<ViewStyle>;
  testID?: string;
};

// Secondary is the quiet default inside cards: raised, with a border so it reads on any surface.
const roles: Record<ButtonVariant, { bg: ColorRole | null; fg: ColorRole; outline?: boolean }> = {
  primary: { bg: "accent", fg: "onAccent" },
  secondary: { bg: "surfaceRaised", fg: "text", outline: true },
  ghost: { bg: null, fg: "accent" },
  danger: { bg: "danger", fg: "onAccent" },
};

export function Button({
  title,
  onPress,
  variant = "primary",
  icon,
  compact = false,
  loading = false,
  disabled = false,
  style,
  testID,
}: ButtonProps) {
  const { colors } = useTheme();
  const { bg, fg, outline } = roles[variant];
  const inactive = disabled || loading;
  return (
    <Pressable
      accessibilityRole="button"
      // Explicit, so the icon glyph never becomes part of the spoken name.
      accessibilityLabel={title}
      aria-disabled={inactive}
      aria-busy={loading}
      disabled={inactive}
      testID={testID}
      onPress={() => {
        // Haptics are unavailable on web and some devices; the press must still go through.
        Haptics.impactAsync(Haptics.ImpactFeedbackStyle.Light).catch(() => {});
        onPress?.();
      }}
      style={({ pressed }) => [
        {
          minHeight: control.md,
          paddingHorizontal: compact ? space[3] : space[5],
          paddingVertical: space[3],
          borderRadius: radius.control,
          backgroundColor: bg ? colors[bg] : "transparent",
          borderWidth: outline ? stroke.control : 0,
          borderColor: colors.border,
          flexDirection: "row",
          alignItems: "center",
          justifyContent: "center",
          gap: space[2],
          opacity: disabled ? disabledOpacity : pressed ? pressedOpacity : 1,
          transform: [{ scale: pressed ? pressedScale : 1 }],
        },
        style,
      ]}
    >
      {loading ? (
        <ActivityIndicator color={colors[fg]} />
      ) : (
        icon && <Icon name={icon} size={iconSize.md} color={fg} />
      )}
      {!compact && (
        <Text variant="label" color={fg} style={{ flexShrink: 1, textAlign: "center" }}>
          {title}
        </Text>
      )}
    </Pressable>
  );
}
