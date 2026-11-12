import { ActivityIndicator } from "react-native";

import { useTheme } from "./theme";
import { space } from "./tokens";

export function Loading() {
  const { colors } = useTheme();
  return (
    <ActivityIndicator
      accessibilityLabel="Loading"
      color={colors.accent}
      style={{ padding: space[4] }}
    />
  );
}
