import type { ReactNode } from "react";
import { KeyboardAvoidingView, Modal, Pressable, ScrollView, StyleSheet, View } from "react-native";
import { GestureDetector, GestureHandlerRootView } from "react-native-gesture-handler";
import Animated from "react-native-reanimated";
import { useSafeAreaInsets } from "react-native-safe-area-context";

import { Text } from "./Text";
import { useTheme } from "./theme";
import { handle, maxWidth, minTouch, radius, space, stroke } from "./tokens";
import { useBackToClose } from "./useBackToClose";
import { useSheetMotion } from "./useSheetMotion";

export type SheetProps = {
  open: boolean;
  /** Backdrop press, back button or swipe down. The parent closes the sheet by setting `open` to false. */
  onClose: () => void;
  title?: string;
  /** Heights as fractions of the window, ascending. Opens at the first. */
  snapPoints?: number[];
  /** Scrolls the body within the sheet's height. Defaults to true; false for a body that is a list. */
  scroll?: boolean;
  children: ReactNode;
  /** Also derives `<id>-handle`, `<id>-anchor` and the pan gesture's `<id>-pan`. */
  testID?: string;
};

export function Sheet({
  open,
  onClose,
  title,
  snapPoints = [0.5],
  scroll = true,
  children,
  testID,
}: SheetProps) {
  const { colors } = useTheme();
  const insets = useSafeAreaInsets();
  const id = (suffix: string) => (testID ? `${testID}-${suffix}` : undefined);
  const { mounted, pan, sheetStyle, backdropStyle, onAnchorLayout } = useSheetMotion({
    open,
    onClose,
    snapPoints,
    panTestID: id("pan"),
  });
  // Modal's onRequestClose covers the native dialog; the hook covers web and the mounted-but-closing
  // state the same way as every other dismissible thing.
  useBackToClose(onClose, open);

  return (
    <Modal
      visible={mounted}
      transparent
      animationType="none"
      statusBarTranslucent
      navigationBarTranslucent
      onRequestClose={onClose}
    >
      <GestureHandlerRootView style={styles.fill}>
        <Animated.View
          style={[StyleSheet.absoluteFill, { backgroundColor: colors.scrim }, backdropStyle]}
        >
          <Pressable
            accessibilityRole="button"
            accessibilityLabel="Close"
            style={styles.fill}
            onPress={onClose}
          />
        </Animated.View>
        <KeyboardAvoidingView behavior="padding" style={styles.fill} pointerEvents="box-none">
          <View
            testID={id("anchor")}
            style={styles.anchor}
            pointerEvents="box-none"
            onLayout={onAnchorLayout}
          >
            <Animated.View
              testID={testID}
              accessibilityViewIsModal
              style={[
                {
                  backgroundColor: colors.surface,
                  borderColor: colors.border,
                  paddingBottom: insets.bottom + space[4],
                },
                styles.sheet,
                sheetStyle,
              ]}
            >
              {/* Only the header drags, so scrollable content in the body keeps its gestures. */}
              <GestureDetector gesture={pan}>
                <View testID={id("handle")} style={styles.header}>
                  <View style={[styles.handle, { backgroundColor: colors.textMuted }]} />
                  {title && (
                    <Text variant="heading" numberOfLines={2}>
                      {title}
                    </Text>
                  )}
                </View>
              </GestureDetector>
              {scroll ? (
                <ScrollView
                  style={styles.fill}
                  contentContainerStyle={styles.body}
                  keyboardShouldPersistTaps="handled"
                >
                  {children}
                </ScrollView>
              ) : (
                <View style={[styles.fill, styles.body]}>{children}</View>
              )}
            </Animated.View>
          </View>
        </KeyboardAvoidingView>
      </GestureHandlerRootView>
    </Modal>
  );
}

const styles = StyleSheet.create({
  fill: { flex: 1 },
  anchor: { flex: 1, justifyContent: "flex-end" },
  sheet: {
    width: "100%",
    maxWidth: maxWidth.content + 2 * space[4],
    alignSelf: "center",
    borderTopLeftRadius: radius.card,
    borderTopRightRadius: radius.card,
    borderWidth: stroke.control,
    borderBottomWidth: 0,
    paddingHorizontal: space[4],
    overflow: "hidden",
  },
  header: { minHeight: minTouch, paddingTop: space[2], paddingBottom: space[3], gap: space[3] },
  handle: { alignSelf: "center", ...handle, borderRadius: handle.height / 2 },
  // Grows to the sheet, so a `marginTop: "auto"` action still sits at the bottom of a short body.
  body: { flexGrow: 1, gap: space[3] },
});
