import { useState } from "react";
import { View } from "react-native";

import { addDays, isDate, shortDate } from "../../domain/dates";
import { Button, Chip, Field, Sheet, space } from "../../ui";
import { rangeOf, type Range } from "./shared";

// A preset counts back from today; a custom range is fixed dates.
export type RangeChoice = { days: number } | { from: string; to: string };

const presets = [
  { label: "7 days", days: 7 },
  { label: "4 weeks", days: 28 },
  { label: "3 months", days: 91 },
  { label: "6 months", days: 182 },
  { label: "1 year", days: 365 },
];

export const rangeFor = (choice: RangeChoice, today: string): Range =>
  "days" in choice ? rangeOf(addDays(today, -choice.days), today) : rangeOf(choice.from, choice.to);

const label = (choice: RangeChoice) =>
  "days" in choice
    ? (presets.find((p) => p.days === choice.days)?.label ?? `${choice.days} days`)
    : `${shortDate(choice.from)} – ${shortDate(choice.to)}`;

type Props = { range: Range; choice: RangeChoice; onChange: (choice: RangeChoice) => void };

export function RangePicker({ range, choice, onChange }: Props) {
  const [open, setOpen] = useState(false);
  const [custom, setCustom] = useState(!("days" in choice));
  const [from, setFrom] = useState(range.from);
  const [to, setTo] = useState(range.to);
  const badDate = "Enter a date as YYYY-MM-DD";
  const fromError = isDate(from) ? undefined : badDate;
  const toError = !isDate(to) ? badDate : to < from ? "Must be on or after From" : undefined;
  const pick = (next: RangeChoice) => {
    onChange(next);
    setOpen(false);
  };

  return (
    <>
      <Chip
        label={label(choice)}
        icon="chevron-down"
        onPress={() => setOpen(true)}
        testID="progress-range"
      />
      <Sheet
        open={open}
        onClose={() => setOpen(false)}
        title="Range"
        snapPoints={[0.6]}
        testID="progress-range-sheet"
      >
        <View style={{ flexDirection: "row", flexWrap: "wrap", gap: space[2] }}>
          {presets.map((p) => (
            <Chip
              key={p.days}
              label={p.label}
              selected={!custom && "days" in choice && choice.days === p.days}
              onPress={() => {
                setCustom(false);
                pick({ days: p.days });
              }}
            />
          ))}
          <Chip label="Custom" selected={custom} onPress={() => setCustom(true)} />
        </View>
        {custom && (
          <>
            <Field
              label="From"
              value={from}
              onChangeText={setFrom}
              placeholder="YYYY-MM-DD"
              autoCapitalize="none"
              autoCorrect={false}
              error={fromError}
            />
            <Field
              label="To"
              value={to}
              onChangeText={setTo}
              placeholder="YYYY-MM-DD"
              autoCapitalize="none"
              autoCorrect={false}
              error={toError}
            />
            <Button
              title="Apply"
              disabled={!!(fromError || toError)}
              onPress={() => pick({ from, to })}
            />
          </>
        )}
      </Sheet>
    </>
  );
}
