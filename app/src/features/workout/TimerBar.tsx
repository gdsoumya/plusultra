import { StyleSheet, View } from "react-native";

import { Button, Icon, type IconName, Numeral, Text, maxWidth, space, useTheme } from "../../ui";

type Props = {
  icon: IconName;
  title: string;
  detail?: string;
  value: string;
  action: string;
  onAction: () => void;
  /** Offers -10 s and +10 s on a countdown that may be nudged, e.g. rest. */
  onAdjust?: (seconds: number) => void;
  testID: string;
};

const step = 10;

// With the nudge buttons the countdown sits under the title, so everything fits a 390 px phone.
// It sits on the tab bar, which already clears the home indicator.
export function TimerBar({
  icon,
  title,
  detail,
  value,
  action,
  onAction,
  onAdjust,
  testID,
}: Props) {
  const { colors } = useTheme();
  const numeral = <Numeral value={value} />;
  return (
    <View
      testID={testID}
      accessibilityLiveRegion="polite"
      style={{
        backgroundColor: colors.surface,
        borderTopWidth: StyleSheet.hairlineWidth,
        borderTopColor: colors.border,
      }}
    >
      {/* The screen's centered column, so on a wide window the bar's content sits under it. */}
      <View
        testID={`${testID}-row`}
        style={{
          width: "100%",
          maxWidth: maxWidth.content + 2 * space[4],
          alignSelf: "center",
          flexDirection: "row",
          alignItems: "center",
          gap: space[2],
          paddingHorizontal: space[4],
          paddingTop: space[2],
          paddingBottom: space[2],
        }}
      >
        {!onAdjust && <Icon name={icon} color="accent" />}
        <View style={{ flex: 1 }}>
          <Text variant="label" numberOfLines={1}>
            {title}
          </Text>
          {detail && <Text variant="caption">{detail}</Text>}
          {onAdjust && numeral}
        </View>
        {onAdjust ? (
          <>
            <Button
              title={`-${step} s`}
              variant="secondary"
              style={{ paddingHorizontal: space[3] }}
              onPress={() => onAdjust(-step)}
              testID={`${testID}-minus`}
            />
            <Button
              title={`+${step} s`}
              variant="secondary"
              style={{ paddingHorizontal: space[3] }}
              onPress={() => onAdjust(step)}
              testID={`${testID}-plus`}
            />
          </>
        ) : (
          numeral
        )}
        <Button title={action} variant="ghost" onPress={onAction} testID={`${testID}-action`} />
      </View>
    </View>
  );
}
