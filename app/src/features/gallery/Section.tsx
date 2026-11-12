import type { ReactNode } from "react";
import { View } from "react-native";

import { letterSpacing, space, Text } from "../../ui";

export const row = {
  flexDirection: "row",
  flexWrap: "wrap",
  alignItems: "center",
  gap: space[2],
} as const;

export function Section({ title, children }: { title: string; children: ReactNode }) {
  return (
    <View style={{ gap: space[3], marginTop: space[2] }}>
      <Text
        variant="label"
        color="textMuted"
        accessibilityRole="header"
        style={{ textTransform: "uppercase", letterSpacing: letterSpacing.label }}
      >
        {title}
      </Text>
      {children}
    </View>
  );
}
