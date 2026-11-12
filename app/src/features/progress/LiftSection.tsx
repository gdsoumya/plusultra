import { useCallback, useMemo, useState } from "react";
import { ScrollView } from "react-native";

import { daysBetween, shortDate } from "../../domain/dates";
import { Card, Chip, EmptyState, space, Text } from "../../ui";
import { LineChart } from "../../ui/charts/LineChart";
import { CachedView } from "./CachedView";
import { loadedLifts } from "./lifts";
import { inUnit, showWeight, type SectionProps } from "./shared";
import { useCached } from "./useCached";

export function LiftSection(props: SectionProps) {
  const { store, range, unit } = props;
  const lifts = useMemo(() => loadedLifts(store, range), [store, range]);
  const [picked, setPicked] = useState<string>();
  const slug = lifts.some((l) => l.slug === picked) ? picked : lifts[0]?.slug;

  return (
    <Card testID="progress-lifts">
      <Text variant="heading" accessibilityRole="header">
        Estimated 1RM ({unit})
      </Text>
      {slug ? (
        <>
          <ScrollView
            horizontal
            showsHorizontalScrollIndicator={false}
            contentContainerStyle={{ gap: space[2] }}
          >
            {lifts.map((l) => (
              <Chip
                key={l.slug}
                label={l.name}
                selected={l.slug === slug}
                onPress={() => setPicked(l.slug)}
              />
            ))}
          </ScrollView>
          <LiftChart {...props} slug={slug} />
        </>
      ) : (
        <EmptyState
          icon="barbell-outline"
          title="No lifts logged yet"
          message="Finish a workout with loaded sets to track your lifts."
        />
      )}
    </Card>
  );
}

function LiftChart({ insights, store, range, unit, width, slug }: SectionProps & { slug: string }) {
  const { from, to } = range;
  const fetch = useCallback(
    () => insights.liftHistory({ slug, dates: { from, to } }),
    [insights, slug, from, to],
  );
  const state = useCached(store, `progress.lift.${slug}.${range.days}`, fetch);
  const fmt = (v: number) => v.toFixed(1);

  return (
    <CachedView state={state}>
      {({ name, points }) => {
        if (!points.length) {
          return <EmptyState icon="barbell-outline" title={`No loaded sets for ${name} yet`} />;
        }
        const e1rm = points.map((p) => inUnit(p.e1rm, unit));
        const best = points[e1rm.indexOf(Math.max(...e1rm))];
        return (
          <LineChart
            width={width}
            points={points.map((p, i) => ({ x: daysBetween(from, p.date), y: e1rm[i] }))}
            xDomain={[0, range.days]}
            xLabels={[shortDate(from), shortDate(to)]}
            format={fmt}
            summary={`${name}: ${points.length} sessions, estimated 1RM ${showWeight(points[0].e1rm, unit)} to ${showWeight(points[points.length - 1].e1rm, unit)}, best ${showWeight(best.e1rm, unit)} on ${shortDate(best.date)}.`}
            table={{
              columns: ["Date", `e1RM ${unit}`, "Best set"],
              rows: points.map((p, i) => [
                shortDate(p.date),
                fmt(e1rm[i]),
                `${fmt(inUnit(p.bestWeight, unit))} × ${p.bestReps}`,
              ]),
            }}
          />
        );
      }}
    </CachedView>
  );
}
