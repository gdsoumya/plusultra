import type { ReactNode } from "react";
import { View } from "react-native";

import { BackButton, Screen, Text } from "../../ui";

// A settings page below the Settings tab, with a way back.
export function SubScreen({
  title,
  children,
  testID,
}: {
  title: string;
  children: ReactNode;
  testID?: string;
}) {
  return (
    <Screen title={title} testID={testID} left={<BackButton fallback="/settings" />}>
      {children}
    </Screen>
  );
}

export function DetailLine({ label, value }: { label: string; value?: string }) {
  return value ? (
    <View>
      <Text variant="caption">{label}</Text>
      <Text selectable>{value}</Text>
    </View>
  ) : null;
}
