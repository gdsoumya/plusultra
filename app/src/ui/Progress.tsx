import type { ReactNode } from "react";
import { View } from "react-native";
import Animated, { cubicBezier } from "react-native-reanimated";
import Svg, { Circle } from "react-native-svg";

import { useTheme } from "./theme";
import { motion, stroke } from "./tokens";

const AnimatedCircle = Animated.createAnimatedComponent(Circle);
const transition = {
  transitionDuration: motion.normal,
  transitionTimingFunction: cubicBezier(...motion.easing),
};

export type ProgressProps = {
  /** Fraction done; clamped to 0..1. */
  value: number;
  variant?: "bar" | "ring";
  /** Ring diameter. */
  size?: number;
  /** Ring center content. */
  children?: ReactNode;
  testID?: string;
};

const clamp01 = (value: number) => (Number.isNaN(value) ? 0 : Math.min(1, Math.max(0, value)));

export function Progress({ value, variant = "bar", size = 96, children, testID }: ProgressProps) {
  const { colors } = useTheme();
  const p = clamp01(value);
  const a11y = {
    accessible: true,
    accessibilityRole: "progressbar" as const,
    accessibilityValue: { min: 0, max: 100, now: Math.round(p * 100) },
    testID,
  };

  if (variant === "bar") {
    return (
      <View
        {...a11y}
        style={{
          height: stroke.progress,
          borderRadius: stroke.progress / 2,
          backgroundColor: colors.border,
          overflow: "hidden",
        }}
      >
        <Animated.View
          style={{
            width: `${p * 100}%`,
            height: "100%",
            borderRadius: stroke.progress / 2,
            backgroundColor: colors.accent,
            transitionProperty: "width",
            ...transition,
          }}
        />
      </View>
    );
  }

  const r = (size - stroke.progress) / 2;
  const circumference = 2 * Math.PI * r;
  const circle = { cx: size / 2, cy: size / 2, r, strokeWidth: stroke.progress, fill: "none" };
  return (
    <View
      {...a11y}
      style={{ width: size, height: size, alignItems: "center", justifyContent: "center" }}
    >
      <Svg
        width={size}
        height={size}
        style={{ position: "absolute", transform: [{ rotate: "-90deg" }] }}
      >
        <Circle {...circle} stroke={colors.border} />
        {p > 0 && (
          <AnimatedCircle
            {...circle}
            stroke={colors.accent}
            strokeLinecap="round"
            strokeDasharray={circumference}
            strokeDashoffset={circumference * (1 - p)}
            animatedProps={{ transitionProperty: "strokeDashoffset", ...transition }}
          />
        )}
      </Svg>
      {children}
    </View>
  );
}
