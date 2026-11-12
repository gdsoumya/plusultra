import { Tabs } from "expo-router";

import { TabBar, type TabSpec } from "../../src/features/shell/TabBar";
import { useTheme, useWide } from "../../src/ui";

// Each tab is a route group with its own stack, so pushed screens keep the bar in view.
const tabs: readonly TabSpec[] = [
  { name: "(today)", title: "Today", icon: "today" },
  { name: "(calendar)", title: "Calendar", icon: "calendar" },
  { name: "(programs)", title: "Programs", icon: "barbell" },
  { name: "(progress)", title: "Progress", icon: "stats-chart" },
  { name: "(settings)", title: "Settings", icon: "settings" },
];

export default function TabsLayout() {
  const { colors } = useTheme();
  const wide = useWide();
  return (
    <Tabs
      tabBar={(props) => <TabBar {...props} tabs={tabs} />}
      screenOptions={{
        headerShown: false,
        tabBarPosition: wide ? "top" : "bottom",
        sceneStyle: { backgroundColor: colors.bg },
      }}
    >
      {tabs.map(({ name, title }) => (
        <Tabs.Screen key={name} name={name} options={{ title }} />
      ))}
    </Tabs>
  );
}
