import Ionicons from "@expo/vector-icons/Ionicons";
import type { ComponentProps } from "react";

import { useTheme } from "./theme";
import { iconSize, type ColorRole } from "./tokens";

export type IconName = ComponentProps<typeof Ionicons>["name"];

export type IconProps = {
  name: IconName;
  size?: number;
  color?: ColorRole;
  testID?: string;
};

export function Icon({ name, size = iconSize.lg, color = "text", testID }: IconProps) {
  const { colors } = useTheme();
  // Decorative: the glyph is a private-use character that screen readers would read aloud.
  return <Ionicons aria-hidden name={name} size={size} color={colors[color]} testID={testID} />;
}
