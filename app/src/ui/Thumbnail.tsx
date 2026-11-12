import { Image, View } from "react-native";

import { Icon } from "./Icon";
import { useTheme } from "./theme";
import { iconSize, radius } from "./tokens";

// A square picture tile, e.g. an exercise image beside its name; a neutral tile when there is none.
// Decorative: the row's text names what it shows.
export function Thumbnail({ uri, size = iconSize.xl }: { uri?: string; size?: number }) {
  const { colors } = useTheme();
  const box = {
    width: size,
    height: size,
    borderRadius: radius.control,
    backgroundColor: colors.surfaceRaised,
  };
  if (!uri) {
    return (
      <View
        testID="thumbnail-placeholder"
        style={[box, { alignItems: "center", justifyContent: "center" }]}
      >
        <Icon name="barbell-outline" size={iconSize.md} color="textMuted" />
      </View>
    );
  }
  return <Image source={{ uri }} style={box} testID="thumbnail" accessible={false} />;
}
