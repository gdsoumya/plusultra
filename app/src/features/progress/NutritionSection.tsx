import { shortDate } from "../../domain/dates";
import { fromKg } from "../../domain/units";
import type { Macros } from "../../gen/plusultra/v1/common_pb";
import { Card, EmptyState, Text } from "../../ui";
import { ComboChart } from "../../ui/charts/ComboChart";
import { StatTiles } from "../../ui/charts/StatTiles";
import { CachedView } from "./CachedView";
import { inUnit, signed, type SectionProps } from "./shared";
import type { Trend } from "./useWeeklyTrend";

type Props = Pick<SectionProps, "unit" | "width"> & { trend: Trend };

export function NutritionSection({ unit, width, trend }: Props) {
  return (
    <Card testID="progress-nutrition">
      <Text variant="heading" accessibilityRole="header">
        Calories and weight
      </Text>
      <CachedView state={trend}>
        {({ weeks }) => {
          // A week without logged meals averages to nothing; one without weigh-ins has no weight.
          const logged = weeks.flatMap((w) => (w.macros?.actual?.kcal ? [w.macros.actual] : []));
          const kcal = weeks.map((w) => w.macros?.actual?.kcal || undefined);
          const weight = weeks.map(
            (w) => w.weight && inUnit(w.weight.average ?? w.weight.end, unit),
          );
          const weighed = weight.filter((v) => v !== undefined);
          if (!logged.length && !weighed.length) {
            return (
              <EmptyState
                icon="restaurant-outline"
                title="No meals or weigh-ins yet"
                message="Log meals and weigh-ins to see how intake moves your weight."
              />
            );
          }
          const avg = (of: (m: Macros) => number) =>
            logged.length
              ? Math.round(logged.reduce((t, m) => t + of(m), 0) / logged.length)
              : undefined;
          const [kcalAvg, proteinAvg] = [avg((m) => m.kcal), avg((m) => m.protein)];
          const change = weighed.length ? weighed[weighed.length - 1] - weighed[0] : undefined;
          const labels = weeks.map((w) => shortDate(w.weekStart));
          const show = (v: number | undefined, f: (v: number) => string = String) =>
            v === undefined ? "–" : f(v);
          const round = (v: number) => String(Math.round(v));
          return (
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
                bars={kcal}
                line={weight}
                format={round}
                lineFormat={(v) => v.toFixed(1)}
                lineMinSpan={fromKg(2, unit)}
                legend={["kcal a day", unit]}
                summary={`${logged.length} weeks with meals averaging ${show(kcalAvg)} kcal and ${show(proteinAvg)} g protein a day; weight ${show(change, signed)} ${unit} over ${weighed.length} weeks with weigh-ins.`}
                table={{
                  columns: ["Week", "kcal", "Protein", unit],
                  rows: weeks.map((w, i) => [
                    labels[i],
                    show(kcal[i], round),
                    show(kcal[i] && w.macros?.actual?.protein, round),
                    show(weight[i], (v) => v.toFixed(1)),
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
