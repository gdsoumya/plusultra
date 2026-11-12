import { useState } from "react";
import { TextInput, View, type TextInputProps } from "react-native";

import { Text } from "./Text";
import { useTheme } from "./theme";
import { control, radius, space, stroke, tabular, typography } from "./tokens";

export type FieldProps = Omit<TextInputProps, "style"> & {
  label: string;
  /** Hint under the input, hidden while `error` is set. */
  helper?: string;
  error?: string;
  /** Decimal keypad and tabular figures, for weights and reps. */
  numeric?: boolean;
};

export function Field({
  label,
  helper,
  error,
  numeric = false,
  onFocus,
  onBlur,
  ...input
}: FieldProps) {
  const { colors, fontScale } = useTheme();
  const [focused, setFocused] = useState(false);
  const { fontSize, lineHeight } = typography.body;
  // The focus ring is thicker; the padding gives back the difference so the text does not move.
  const ring = focused || error ? stroke.focus : stroke.control;
  const border = error ? colors.danger : focused ? colors.accent : colors.border;
  return (
    <View style={{ gap: space[1] }}>
      <Text variant="label" color="textMuted">
        {label}
      </Text>
      <TextInput
        accessibilityLabel={label}
        accessibilityHint={error ?? helper}
        placeholderTextColor={colors.textMuted}
        selectionColor={colors.accent}
        keyboardType={numeric ? "decimal-pad" : "default"}
        inputMode={numeric ? "decimal" : undefined}
        onFocus={(e) => {
          setFocused(true);
          onFocus?.(e);
        }}
        onBlur={(e) => {
          setFocused(false);
          onBlur?.(e);
        }}
        style={[
          {
            minHeight: control.md,
            paddingHorizontal: space[4] - (ring - stroke.control),
            paddingVertical: space[3] - (ring - stroke.control),
            borderRadius: radius.control,
            borderWidth: ring,
            borderColor: border,
            backgroundColor: colors.surfaceRaised,
            color: colors.text,
            fontSize: fontSize * fontScale,
            lineHeight: lineHeight * fontScale,
          },
          numeric && tabular,
        ]}
        {...input}
      />
      {error ? (
        <Text variant="caption" color="danger" accessibilityLiveRegion="polite">
          {error}
        </Text>
      ) : (
        helper && <Text variant="caption">{helper}</Text>
      )}
    </View>
  );
}
