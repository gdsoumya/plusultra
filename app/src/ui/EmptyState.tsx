import { View } from "react-native";

import { Button } from "./Button";
import { Icon, type IconName } from "./Icon";
import { Text } from "./Text";
import { useTheme } from "./theme";
import { iconSize, radius, space } from "./tokens";

export type EmptyStateProps = {
  icon: IconName;
  title: string;
  message?: string;
  action?: { title: string; onPress: () => void };
};

const tile = iconSize.xl + 2 * space[3];

export function EmptyState({ icon, title, message, action }: EmptyStateProps) {
  const { colors } = useTheme();
  return (
    <View style={{ alignItems: "center", gap: space[3], paddingVertical: space[6] }}>
      <View
        style={{
          width: tile,
          height: tile,
          borderRadius: radius.pill,
          backgroundColor: colors.surfaceRaised,
          alignItems: "center",
          justifyContent: "center",
        }}
      >
        <Icon name={icon} size={iconSize.xl} color="textMuted" />
      </View>
      <Text variant="heading" style={{ textAlign: "center" }}>
        {title}
      </Text>
      {message && (
        <Text color="textMuted" style={{ textAlign: "center", maxWidth: 320 }}>
          {message}
        </Text>
      )}
      {action && (
        <Button title={action.title} onPress={action.onPress} style={{ marginTop: space[2] }} />
      )}
    </View>
  );
}
