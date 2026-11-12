import type { ReactNode } from "react";
import { Pressable, View, type ViewStyle } from "react-native";

import { Icon, type IconName } from "./Icon";
import { Text } from "./Text";
import { useTheme } from "./theme";
import { control, iconSize, radius, space, tabular, type ColorRole } from "./tokens";

export type ListRowProps = {
  title: string;
  /** Lines the title may take before it truncates. */
  titleLines?: 1 | 2;
  subtitle?: string;
  icon?: IconName;
  /** Leading tile in place of the icon, e.g. a Thumbnail. */
  leading?: ReactNode;
  /** Trailing text, e.g. the current setting. */
  value?: string;
  /** Shows a chevron and makes the row a button. */
  onPress?: () => void;
  /** Title and icon color, e.g. danger for a destructive action. */
  tone?: ColorRole;
  testID?: string;
};

const row: ViewStyle = {
  minHeight: control.lg,
  paddingVertical: space[2],
  paddingHorizontal: space[4],
  flexDirection: "row",
  alignItems: "center",
  gap: space[3],
};

// The title truncates and the subtitle clamps to two lines, so the value never leaves the screen.
export function ListRow({
  title,
  titleLines = 1,
  subtitle,
  icon,
  leading,
  value,
  onPress,
  tone = "text",
  testID,
}: ListRowProps) {
  const { colors } = useTheme();
  const label = [title, subtitle, value].filter(Boolean).join(", ");
  const content = (
    <>
      {leading}
      {icon && !leading && (
        <View
          style={{
            width: control.sm,
            height: control.sm,
            borderRadius: radius.control,
            backgroundColor: colors.surfaceRaised,
            alignItems: "center",
            justifyContent: "center",
          }}
        >
          <Icon name={icon} size={iconSize.md} color={tone} />
        </View>
      )}
      <View style={{ flex: 1 }}>
        <Text numberOfLines={titleLines} color={tone}>
          {title}
        </Text>
        {subtitle && (
          <Text variant="caption" numberOfLines={2}>
            {subtitle}
          </Text>
        )}
      </View>
      {value && (
        <Text color="textMuted" numberOfLines={1} style={[tabular, { maxWidth: "45%" }]}>
          {value}
        </Text>
      )}
      {onPress && <Icon name="chevron-forward" size={iconSize.md} color="textMuted" />}
    </>
  );

  if (!onPress) {
    return (
      <View accessible accessibilityLabel={label} testID={testID} style={row}>
        {content}
      </View>
    );
  }
  return (
    <Pressable
      accessibilityRole="button"
      accessibilityLabel={label}
      onPress={onPress}
      testID={testID}
      style={({ pressed }) => [row, pressed && { backgroundColor: colors.surfaceRaised }]}
    >
      {content}
    </Pressable>
  );
}
