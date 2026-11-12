import type { BottomTabBarProps } from "expo-router/build/layouts/Tabs";
import { Pressable, View } from "react-native";

import {
  Icon,
  iconSize,
  maxWidth,
  minTouch,
  radius,
  space,
  stroke,
  Text,
  useTheme,
  useWide,
  type IconName,
} from "../../ui";

export type TabSpec = { name: string; title: string; icon: IconName };

type Props = BottomTabBarProps & { tabs: readonly TabSpec[] };

// The five destinations: a bottom bar on a phone, a top bar over the content column on a wide
// window. The current tab is the only accent in the bar.
export function TabBar({ state, descriptors, emitter, navigateToTab, insets, tabs }: Props) {
  const { colors } = useTheme();
  const wide = useWide();
  const items = state.routes.map((route, index) => {
    const spec = tabs.find((t) => t.name === route.name);
    const focused = index === state.index;
    const icon = spec?.icon ?? "ellipse";
    const onPress = () => {
      const event = emitter.emit({ type: "tabPress", target: route.key, canPreventDefault: true });
      if (focused || event.defaultPrevented) return;
      // A tab opens at its root (P2-R34): the framework only pops the focused tab's stack.
      const { navigation } = descriptors[route.key];
      const nested = navigation.getState().routes.find((r) => r.key === route.key)?.state;
      if (nested?.key && (nested.routes?.length ?? 0) > 1) {
        navigation.dispatch({ type: "POP_TO_TOP", target: nested.key });
      }
      navigateToTab(route.key);
    };
    return (
      <Pressable
        key={route.key}
        accessibilityRole="tab"
        accessibilityLabel={spec?.title ?? route.name}
        accessibilityState={{ selected: focused }}
        aria-selected={focused}
        onPress={onPress}
        style={[
          wide ? styles.topItem : styles.bottomItem,
          focused && wide && { backgroundColor: colors.accentSoft },
        ]}
      >
        <Icon
          name={focused ? icon : (`${icon}-outline` as IconName)}
          size={wide ? iconSize.md : iconSize.lg}
          color={focused ? "accent" : "textMuted"}
        />
        <Text
          variant={wide ? "label" : "caption"}
          color={focused ? "accent" : "textMuted"}
          numberOfLines={1}
        >
          {spec?.title ?? route.name}
        </Text>
      </Pressable>
    );
  });

  if (wide) {
    return (
      <View
        testID="tab-bar-top"
        style={{
          paddingTop: insets.top,
          backgroundColor: colors.surface,
          borderBottomWidth: stroke.control,
          borderColor: colors.border,
        }}
      >
        <View role="tablist" style={styles.topRow} testID="tab-bar-top-row">
          {items}
        </View>
      </View>
    );
  }
  return (
    <View
      testID="tab-bar-bottom"
      role="tablist"
      style={{
        flexDirection: "row",
        paddingBottom: insets.bottom,
        backgroundColor: colors.surface,
        borderTopWidth: stroke.control,
        borderColor: colors.border,
      }}
    >
      {items}
    </View>
  );
}

const styles = {
  bottomItem: {
    flex: 1,
    minHeight: minTouch + space[3],
    paddingTop: space[2],
    paddingBottom: space[1],
    alignItems: "center",
    justifyContent: "center",
    gap: space[1],
  },
  topRow: {
    width: "100%",
    maxWidth: maxWidth.wide + 2 * space[4],
    alignSelf: "center",
    flexDirection: "row",
    // The tabs centre on the window, as the content column under them does.
    justifyContent: "center",
    gap: space[1],
    paddingHorizontal: space[4],
    paddingVertical: space[2],
  },
  topItem: {
    minHeight: minTouch,
    paddingHorizontal: space[4],
    borderRadius: radius.control,
    flexDirection: "row",
    alignItems: "center",
    gap: space[2],
  },
} as const;
