import { View, type ViewProps } from "react-native";

import { useTheme } from "./theme";
import { radius, space, stroke } from "./tokens";

// The first surface level above the background; a full border keeps the edge visible on OLED.
export function Card({ style, ...rest }: ViewProps) {
  const { colors } = useTheme();
  return (
    <View
      style={[
        {
          backgroundColor: colors.surface,
          borderColor: colors.border,
          borderWidth: stroke.control,
          borderRadius: radius.card,
          padding: space[4],
          gap: space[3],
        },
        style,
      ]}
      {...rest}
    />
  );
}
