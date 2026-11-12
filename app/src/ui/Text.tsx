import { Text as RNText, type TextProps as RNTextProps } from "react-native";

import { useTheme } from "./theme";
import { typography, type ColorRole, type TextVariant } from "./tokens";

export type TextProps = RNTextProps & {
  /** Type scale step. Defaults to `body`. */
  variant?: TextVariant;
  /** Color role. Defaults to `textMuted` for captions, `text` otherwise. */
  color?: ColorRole;
};

export function Text({ variant = "body", color, style, ...rest }: TextProps) {
  const { colors, fontScale } = useTheme();
  const { fontSize, lineHeight, ...face } = typography[variant];
  const role = color ?? (variant === "caption" ? "textMuted" : "text");
  return (
    <RNText
      style={[
        { ...face, fontSize: fontSize * fontScale, lineHeight: lineHeight * fontScale },
        { color: colors[role] },
        style,
      ]}
      {...rest}
    />
  );
}
