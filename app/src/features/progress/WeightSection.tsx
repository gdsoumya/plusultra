import { useCallback } from "react";

import { daysBetween, shortDate } from "../../domain/dates";
import { Card, EmptyState, Text } from "../../ui";
import { LineChart } from "../../ui/charts/LineChart";
import { StatTiles } from "../../ui/charts/StatTiles";
import { CachedView } from "./CachedView";
import { inUnit, showWeight, signed, type SectionProps } from "./shared";
import { useCached } from "./useCached";

export function WeightSection({ insights, store, range, unit, width }: SectionProps) {
  const { from, to } = range;
  const fetch = useCallback(
    () => insights.weightTrend({ dates: { from, to } }),
    [insights, from, to],
  );
  const state = useCached(store, `progress.weight.${range.days}`, fetch);
  const fmt = (v: number) => v.toFixed(1);

  return (
    <Card testID="progress-weight">
      <Text variant="heading" accessibilityRole="header">
        Weight ({unit})
      </Text>
      <CachedView state={state}>
        {({ points, weeklyRate }) => {
          if (!points.length) {
            return (
              <EmptyState
                icon="scale-outline"
                title="No weigh-ins yet"
                message="Log your weight on Today to see the trend."
              />
            );
          }
          const last = points[points.length - 1];
          const average = fmt(inUnit(last.movingAverage, unit));
          const rate = signed(inUnit(weeklyRate, unit));
          const at = (date: string) => daysBetween(from, date);
          return (
            <>
              <StatTiles
                items={[
                  { label: "7-day average", value: average },
                  { label: `${unit} per week`, value: rate },
                ]}
              />
              <LineChart
                width={width}
                points={points.map((p) => ({ x: at(p.date), y: inUnit(p.weight, unit) }))}
                trend={points.map((p) => ({ x: at(p.date), y: inUnit(p.movingAverage, unit) }))}
                xDomain={[0, range.days]}
                xLabels={[shortDate(from), shortDate(to)]}
                format={fmt}
                legend={["Weigh-ins", "7-day average"]}
                summary={`${points.length} weigh-ins from ${shortDate(points[0].date)} to ${shortDate(last.date)}. Latest ${showWeight(last.weight, unit)}, 7-day average ${showWeight(last.movingAverage, unit)}, ${rate} ${unit} a week.`}
                table={{
                  columns: ["Date", unit, "7-day avg"],
                  rows: points.map((p) => [
                    shortDate(p.date),
                    fmt(inUnit(p.weight, unit)),
                    fmt(inUnit(p.movingAverage, unit)),
                  ]),
                }}
              />
            </>
          );
        }}
      </CachedView>
    </Card>
  );
}
