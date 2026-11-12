import { View } from "react-native";

import { Numeral } from "../Numeral";
import { Text } from "../Text";
import { space } from "../tokens";

export type Stat = { label: string; value: string | number };

export function StatTiles({ items, testID }: { items: Stat[]; testID?: string }) {
  return (
    <View style={{ flexDirection: "row", flexWrap: "wrap", gap: space[3] }} testID={testID}>
      {items.map((s) => (
        <View
          key={s.label}
          accessible
          accessibilityLabel={`${s.label}: ${s.value}`}
          style={{ flexGrow: 1, flexBasis: 96 }}
        >
          <Numeral value={s.value} />
          <Text variant="caption">{s.label}</Text>
        </View>
      ))}
    </View>
  );
}
