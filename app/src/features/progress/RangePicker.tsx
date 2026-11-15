import { useState } from "react";
import { View } from "react-native";

import { addDays, monthOf, shortDate } from "../../domain/dates";
import { Button, Chip, Sheet, space } from "../../ui";
import { DateRangeCalendar, tooLong, type Span } from "./DateRangeCalendar";
import { rangeOf, type Range } from "./shared";

// A preset is the last N dates, today included; a custom range is fixed dates.
export type RangeChoice = { days: number } | { from: string; to: string };

const presets = [
  { label: "7 days", days: 7 },
  { label: "4 weeks", days: 28 },
  { label: "3 months", days: 91 },
  { label: "6 months", days: 182 },
  { label: "1 year", days: 365 },
];

export const validChoice = (choice: RangeChoice) =>
  "days" in choice || !tooLong(choice.from, choice.to);

export const rangeFor = (choice: RangeChoice, today: string): Range =>
  "days" in choice
    ? rangeOf(addDays(today, 1 - choice.days), today)
    : rangeOf(choice.from, choice.to);

const label = (choice: RangeChoice) =>
  "days" in choice
    ? (presets.find((p) => p.days === choice.days)?.label ?? `${choice.days} days`)
    : `${shortDate(choice.from)} – ${shortDate(choice.to)}`;

type Props = { range: Range; choice: RangeChoice; onChange: (choice: RangeChoice) => void };

export function RangePicker({ range, choice, onChange }: Props) {
  const [open, setOpen] = useState(false);
  const [custom, setCustom] = useState(!("days" in choice));
  const [span, setSpan] = useState<Span>(() => ("days" in choice ? {} : choice));
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
      {/* Short for the chips alone; the calendar needs most of the window. */}
      <Sheet
        open={open}
        onClose={() => setOpen(false)}
        title="Range"
        snapPoints={[custom ? 0.85 : 0.3]}
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
            <DateRangeCalendar
              span={span}
              initialMonth={monthOf(span.to ?? range.to)}
              onChange={setSpan}
            />
            <Button
              title="Apply"
              disabled={!span.from || !span.to || tooLong(span.from, span.to)}
              onPress={() => pick({ from: span.from!, to: span.to! })}
            />
          </>
        )}
      </Sheet>
    </>
  );
}
