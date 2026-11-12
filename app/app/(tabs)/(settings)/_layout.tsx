import { Stack } from "expo-router";

import { useTheme } from "../../../src/ui";

export const unstable_settings = { anchor: "settings" };

export default function Layout() {
  const { colors } = useTheme();
  return (
    <Stack screenOptions={{ headerShown: false, contentStyle: { backgroundColor: colors.bg } }} />
  );
}
