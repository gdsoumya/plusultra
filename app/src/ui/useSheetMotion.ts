import { useEffect, useState } from "react";
import { useWindowDimensions, type LayoutChangeEvent } from "react-native";
import { usePanGesture } from "react-native-gesture-handler";
import {
  Easing,
  Extrapolation,
  interpolate,
  useAnimatedStyle,
  useSharedValue,
  withTiming,
} from "react-native-reanimated";
import { useSafeAreaInsets } from "react-native-safe-area-context";
import { scheduleOnRN } from "react-native-worklets";

import { motion } from "./tokens";

const timing = { duration: motion.normal, easing: Easing.bezier(...motion.easing) };

type Options = {
  open: boolean;
  onClose: () => void;
  /** Fractions of the window height, ascending. */
  snapPoints: number[];
  panTestID?: string;
};

// Open/close animation, snap-point dragging and keyboard fitting for Sheet.
export function useSheetMotion({ open, onClose, snapPoints, panTestID }: Options) {
  const insets = useSafeAreaInsets();
  const { height: windowHeight } = useWindowDimensions();
  const [mounted, setMounted] = useState(open);
  if (open && !mounted) setMounted(true);

  const heights = snapPoints.map((point) => point * windowHeight);
  const rest = heights[0];
  const max = Math.max(...heights);
  // How far the sheet reaches up from the bottom. Below `rest` it slides away instead of shrinking.
  const shown = useSharedValue(0);
  const start = useSharedValue(0);
  // Space above the keyboard: KeyboardAvoidingView pads the anchor while the keyboard is open.
  const room = useSharedValue(windowHeight - insets.top);

  useEffect(() => {
    shown.set(
      withTiming(open ? rest : 0, timing, (finished) => {
        if (finished && !open) scheduleOnRN(setMounted, false);
      }),
    );
  }, [open, rest, shown]);

  const pan = usePanGesture({
    testID: panTestID,
    onActivate: () => {
      start.set(shown.get());
    },
    onUpdate: (e) => {
      shown.set(Math.min(max, Math.max(0, start.get() - e.translationY)));
    },
    onDeactivate: (e) => {
      const projected = shown.get() - e.velocityY * motion.fling;
      const target = [0, ...heights].reduce((best, h) =>
        Math.abs(h - projected) < Math.abs(best - projected) ? h : best,
      );
      shown.set(
        withTiming(target, timing, (finished) => {
          if (finished && target === 0) scheduleOnRN(onClose);
        }),
      );
    },
  });

  const sheetStyle = useAnimatedStyle(() => {
    const base = Math.min(rest, room.get());
    return {
      height: Math.min(Math.max(shown.get(), base), room.get()),
      transform: [{ translateY: Math.max(0, base - shown.get()) }],
    };
  });
  const backdropStyle = useAnimatedStyle(() => ({
    opacity: interpolate(shown.get(), [0, rest], [0, 1], Extrapolation.CLAMP),
  }));
  const onAnchorLayout = (e: LayoutChangeEvent) =>
    room.set(e.nativeEvent.layout.height - insets.top);

  return { mounted, pan, sheetStyle, backdropStyle, onAnchorLayout };
}
