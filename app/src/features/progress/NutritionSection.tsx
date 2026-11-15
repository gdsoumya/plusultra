import { useMemo } from "react";

import type { Store } from "../../data/store";
import { addDays, mondayOf, shortDate } from "../../domain/dates";
import type { Macros } from "../../domain/types";
import { fromKg, inUnit } from "../../domain/units";
import { Card, EmptyState, Text } from "../../ui";
import { ComboChart } from "../../ui/charts/ComboChart";
import { StatTiles } from "../../ui/charts/StatTiles";
import { DAILY_UP_TO, signed, type SectionProps } from "./shared";

type Props = Pick<SectionProps, "store" | "range" | "unit" | "width">;

const avg = (values: number[]) =>
  values.length ? values.reduce((t, v) => t + v, 0) / values.length : undefined;

// Logged days and weigh-ins in the range, from the local store, in the buckets the range calls for.
export function nutritionBuckets(store: Store, from: string, to: string, days: number) {
  const daily = days <= DAILY_UP_TO;
  const bucketOf = (date: string) => (daily ? date : mondayOf(date));
  const starts: string[] = [];
  for (let d = daily ? from : mondayOf(from); d <= to; d = addDays(d, daily ? 1 : 7))
    starts.push(d);
  const meals = store
    .list("mealLogs", { dateFrom: from, dateTo: to })
    .filter((l) => l.totals.kcal > 0);
  const weighIns = store
    .list("checkins", { dateFrom: from, dateTo: to })
    .filter((c) => c.weight?.value)
    .sort((a, b) => a.date.localeCompare(b.date));
  const macrosIn = (start: string) =>
    meals.filter((l) => bucketOf(l.date) === start).map((l) => l.totals);
  const kgIn = (start: string) =>
    weighIns.filter((c) => bucketOf(c.date) === start).map((c) => inUnit(c.weight!, "kg"));
  return {
    daily,
    starts,
    kcal: starts.map((s) => avg(macrosIn(s).map((m) => m.kcal))),
    kg: starts.map((s) => avg(kgIn(s))),
    logged: meals.map((l) => l.totals),
    weighed: weighIns.map((c) => inUnit(c.weight!, "kg")),
  };
}

export function NutritionSection({ store, range, unit, width }: Props) {
  const { from, to, days } = range;
  const b = useMemo(() => nutritionBuckets(store, from, to, days), [store, from, to, days]);
  const toUnit = (kg: number | undefined) => (kg === undefined ? undefined : fromKg(kg, unit));
  const weight = b.kg.map(toUnit);
  const macro = (of: (m: Macros) => number) => {
    const v = avg(b.logged.map(of));
    return v === undefined ? undefined : Math.round(v);
  };
  const [kcalAvg, proteinAvg] = [macro((m) => m.kcal), macro((m) => m.protein)];
  const change = b.weighed.length ? fromKg(b.weighed.at(-1)! - b.weighed[0], unit) : undefined;
  const labels = b.starts.map(shortDate);
  const show = (v: number | undefined, f: (v: number) => string = String) =>
    v === undefined ? "–" : f(v);
  const round = (v: number) => String(Math.round(v));

  return (
    <Card testID="progress-nutrition">
      <Text variant="heading" accessibilityRole="header">
        Calories and weight
      </Text>
      {!b.logged.length && !b.weighed.length ? (
        <EmptyState
          icon="restaurant-outline"
          title="No meals or weigh-ins yet"
          message="Log meals and weigh-ins to see how intake moves your weight."
        />
      ) : (
        <>
          <StatTiles
            items={[
              { label: "kcal a day", value: show(kcalAvg) },
              { label: "Protein a day", value: show(proteinAvg, (v) => `${v} g`) },
              { label: "Weight change", value: show(change, (v) => `${signed(v)} ${unit}`) },
            ]}
          />
          <ComboChart
            width={width}
            labels={labels}
            bars={b.kcal}
            line={weight}
            format={round}
            lineFormat={(v) => v.toFixed(1)}
            lineMinSpan={fromKg(2, unit)}
            legend={[b.daily ? "kcal" : "kcal a day", unit]}
            summary={`${b.logged.length} days with meals averaging ${show(kcalAvg)} kcal and ${show(proteinAvg)} g protein a day; weight ${show(change, signed)} ${unit} across ${b.weighed.length} weighings.`}
            table={{
              columns: [b.daily ? "Day" : "Week", "kcal", unit],
              rows: b.starts.map((_, i) => [
                labels[i],
                show(b.kcal[i], round),
                show(weight[i], (v) => v.toFixed(1)),
              ]),
            }}
          />
        </>
      )}
    </Card>
  );
}
