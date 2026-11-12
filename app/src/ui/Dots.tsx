import { View } from "react-native";

import { useTheme } from "./theme";
import { space } from "./tokens";

const dot = 6;

// Which page of a paged or scrolling row is in view; the row says "Page 2 of 3" to a reader.
export function Dots({ count, current }: { count: number; current: number }) {
  const { colors } = useTheme();
  if (count < 2) return null;
  return (
    <View
      accessible
      accessibilityLabel={`Page ${current + 1} of ${count}`}
      style={{ flexDirection: "row", justifyContent: "center", gap: space[2] }}
    >
      {Array.from({ length: count }, (_, i) => (
        <View
          key={i}
          testID={i === current ? "dot-current" : "dot"}
          style={{
            width: dot,
            height: dot,
            borderRadius: dot / 2,
            backgroundColor: i === current ? colors.accent : colors.border,
          }}
        />
      ))}
    </View>
  );
}
