import { useState } from "react";
import { View } from "react-native";

import type { Checkin, Unit, Weight } from "../../domain/types";
import { weekdayDate } from "../../domain/dates";
import { formatWeight, inUnit, toKg } from "../../domain/units";
import { Button, Card, Field, Icon, Numeral, Text, iconSize, space } from "../../ui";

type Props = {
  // Oldest first, up to today.
  checkins: Checkin[];
  today: string;
  unit: Unit;
  onSave: (weight: Weight) => void;
};

function Trend({ delta, unit }: { delta: number; unit: Unit }) {
  const abs = Math.abs(delta);
  const label =
    delta === 0
      ? "No change since the last check-in"
      : `${delta > 0 ? "Up" : "Down"} ${abs} ${unit} since the last check-in`;
  return (
    <View
      accessible
      accessibilityLabel={label}
      style={{ flexDirection: "row", alignItems: "center", gap: space[1] }}
    >
      <Icon
        name={delta === 0 ? "remove" : delta > 0 ? "arrow-up" : "arrow-down"}
        size={iconSize.sm}
        color="textMuted"
      />
      <Text variant="caption">{delta === 0 ? "no change" : `${abs} ${unit}`}</Text>
    </View>
  );
}

export function CheckinCard({ checkins, today, unit, onSave }: Props) {
  const [text, setText] = useState("");
  const [error, setError] = useState<string>();
  const weighed = checkins.filter((c) => c.weight);
  const latest = weighed.at(-1);
  const previous = weighed.at(-2);
  const delta =
    previous && latest
      ? Math.round((inUnit(latest.weight!, unit) - inUnit(previous.weight!, unit)) * 10) / 10
      : undefined;

  const save = () => {
    const value = Number(text);
    if (!(value > 0)) return setError("Enter a weight");
    setError(undefined);
    setText("");
    onSave({ value: toKg(value, unit), unit: "kg" });
  };

  return (
    <Card>
      <Text variant="caption">
        {latest
          ? latest.date === today
            ? "Weight today"
            : `Weight on ${weekdayDate(latest.date)}`
          : "Weight"}
      </Text>
      {latest && (
        <View style={{ flexDirection: "row", alignItems: "baseline", gap: space[3] }}>
          <Numeral value={formatWeight(latest.weight!, unit)} />
          {delta !== undefined && <Trend delta={delta} unit={unit} />}
        </View>
      )}
      <Field
        label={`Weight (${unit})`}
        numeric
        value={text}
        onChangeText={setText}
        error={error}
        placeholder={latest ? String(inUnit(latest.weight!, unit)) : undefined}
        onSubmitEditing={save}
      />
      <Button title="Save weight" variant="secondary" onPress={save} />
    </Card>
  );
}
