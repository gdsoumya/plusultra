import type { ReactNode } from "react";
import {
  KeyboardAvoidingView,
  Platform,
  RefreshControl,
  ScrollView,
  View,
  type TextStyle,
} from "react-native";
import { useSafeAreaInsets } from "react-native-safe-area-context";

import { IconButton } from "./IconButton";
import { Text } from "./Text";
import { useTheme } from "./theme";
import { maxWidth, minTouch, space } from "./tokens";
import { useWide } from "./useWide";

export type ScreenProps = {
  /** Large title at the top of the content. */
  title?: string;
  /** Leading header action, e.g. a Back button. */
  left?: ReactNode;
  /** Header action, e.g. a ghost Button. */
  right?: ReactNode;
  /** Wraps content in a ScrollView. Defaults to true. */
  scroll?: boolean;
  /** Pull to refresh; on web, where there is no pull gesture, a header button instead. */
  refresh?: { refreshing: boolean; refresh(): void };
  /** A wider content column on large windows, for grids and charts. */
  wide?: boolean;
  children: ReactNode;
  testID?: string;
};

export function Screen({
  title,
  left,
  right,
  scroll = true,
  refresh,
  wide = false,
  children,
  testID,
}: ScreenProps) {
  const { colors } = useTheme();
  const insets = useSafeAreaInsets();
  const wideWindow = useWide();
  // A centered column with side gutters on wide windows; the full width on a phone.
  const content = {
    width: "100%" as const,
    maxWidth: maxWidth[wide ? "wide" : "content"] + 2 * space[4],
    alignSelf: "center" as const,
    paddingHorizontal: space[4],
    paddingBottom: insets.bottom + space[6],
    gap: space[4],
  };
  const row = {
    flexDirection: "row",
    alignItems: "center",
    gap: space[3],
    minHeight: minTouch,
    // Actions keep their width; a long title truncates instead (P2-R49).
    flexShrink: 0,
  } as const;
  const heading = (style?: TextStyle) => (
    <Text variant="title" accessibilityRole="header" numberOfLines={1} style={style}>
      {title}
    </Text>
  );
  // At phone widths the header has no room for it next to the sync badge and a primary action;
  // the badge's sheet syncs there.
  const actions =
    refresh && Platform.OS === "web" && wideWindow ? (
      <>
        <IconButton
          icon="refresh"
          label="Refresh"
          loading={refresh.refreshing}
          onPress={refresh.refresh}
        />
        {right}
      </>
    ) : (
      right
    );
  // With a back action the title gets a row of its own, so more of a long one fits on a phone.
  const header = left ? (
    <>
      <View
        testID={testID && `${testID}-actions`}
        style={[row, { justifyContent: "space-between" }]}
      >
        {left}
        <View style={row}>{actions}</View>
      </View>
      {title ? heading() : null}
    </>
  ) : title || actions ? (
    <View style={row}>
      {heading({ flexGrow: 1, flexShrink: 1 })}
      <View testID={testID && `${testID}-actions`} style={row}>
        {actions}
      </View>
    </View>
  ) : null;
  const body = (
    <>
      {header}
      {children}
    </>
  );
  return (
    <KeyboardAvoidingView
      behavior="padding"
      testID={testID}
      style={{ flex: 1, backgroundColor: colors.bg, paddingTop: insets.top + space[2] }}
    >
      {scroll ? (
        <ScrollView
          contentContainerStyle={content}
          keyboardShouldPersistTaps="handled"
          refreshControl={
            refresh && (
              <RefreshControl
                refreshing={refresh.refreshing}
                onRefresh={refresh.refresh}
                tintColor={colors.accent}
                colors={[colors.accent]}
              />
            )
          }
        >
          {body}
        </ScrollView>
      ) : (
        <View style={[{ flex: 1 }, content]}>{body}</View>
      )}
    </KeyboardAvoidingView>
  );
}
