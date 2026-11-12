import { useMemo, type ComponentRef, type ReactNode, type RefObject } from "react";
import { Platform, ScrollView, TextInput, View, type TextInputProps } from "react-native";

import { monoFont, radius, space, stroke, Text, typography, useTheme } from "../../ui";
import { lineCount } from "./script";

type Props = Pick<TextInputProps, "value" | "onChangeText" | "selection" | "onSelectionChange"> & {
  value: string;
  errorLines: number[];
  inputRef: RefObject<ComponentRef<typeof TextInput> | null>;
};

// Line numbers as one string, with error lines as their own node, so a long script costs a few
// nodes rather than one per line.
function gutter(lines: number, errorLines: number[]): ReactNode[] {
  const parts: ReactNode[] = [];
  let plain = "";
  for (let i = 1; i <= lines; i++) {
    const text = `${i > 1 ? "\n" : ""}${i}`;
    if (!errorLines.includes(i)) {
      plain += text;
      continue;
    }
    if (plain) parts.push(plain);
    plain = "";
    parts.push(
      <Text key={i} color="danger" style={{ fontFamily: monoFont }}>
        {text}
      </Text>,
    );
  }
  if (plain) parts.push(plain);
  return parts;
}

export function ScriptEditor({ value, errorLines, inputRef, ...input }: Props) {
  const { colors, fontScale } = useTheme();
  const lines = lineCount(value);
  const errorKey = errorLines.join(",");
  const numbers = useMemo(() => gutter(lines, errorLines), [lines, errorKey]); // eslint-disable-line react-hooks/exhaustive-deps
  const font = {
    fontFamily: monoFont,
    fontSize: typography.mono.fontSize * fontScale,
    lineHeight: typography.mono.lineHeight * fontScale,
    includeFontPadding: false,
    paddingVertical: space[3],
  } as const;
  // Lines scroll sideways instead of wrapping, so every gutter number sits beside its line.
  return (
    <View
      style={{
        flexDirection: "row",
        borderRadius: radius.control,
        backgroundColor: colors.surfaceRaised,
        borderWidth: stroke.control,
        borderColor: colors.border,
      }}
    >
      <Text
        testID="script-gutter"
        aria-hidden
        selectable={false}
        style={[font, { textAlign: "right", paddingHorizontal: space[2], color: colors.textMuted }]}
      >
        {numbers}
      </Text>
      <ScrollView
        horizontal
        keyboardShouldPersistTaps="handled"
        contentContainerStyle={{ flexGrow: 1 }}
      >
        <TextInput
          ref={inputRef}
          accessibilityLabel="Script"
          multiline
          scrollEnabled={false}
          autoCapitalize="none"
          autoCorrect={false}
          spellCheck={false}
          textAlignVertical="top"
          placeholder={'program("My plan", weeks=4)'}
          placeholderTextColor={colors.textMuted}
          selectionColor={colors.accent}
          value={value}
          style={[
            font,
            { flexGrow: 1, padding: 0, paddingRight: space[3], color: colors.text },
            // The browser wraps a textarea unless told not to.
            Platform.OS === "web" && ({ whiteSpace: "pre" } as object),
          ]}
          {...input}
        />
      </ScrollView>
    </View>
  );
}
