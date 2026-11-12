import { View, type ViewProps } from "react-native";

import { useTheme } from "./theme";
import { stroke } from "./tokens";

export function Divider({ style, ...rest }: ViewProps) {
  const { colors } = useTheme();
  return (
    <View style={[{ height: stroke.control, backgroundColor: colors.border }, style]} {...rest} />
  );
}
