import { useRouter } from "expo-router";
import { useCallback, useMemo, useState } from "react";
import { ScrollView } from "react-native";

import { addDays, daysBetween, shortDate } from "../../domain/dates";
import type { Unit } from "../../domain/types";
import type { LiftPoint } from "../../gen/plusultra/v1/insight_pb";
import { Card, Chip, EmptyState, space, Text } from "../../ui";
import { LineChart } from "../../ui/charts/LineChart";
import { StatTiles } from "../../ui/charts/StatTiles";
import { CachedView } from "./CachedView";
import { loadedLifts } from "./lifts";
import { inUnit, type SectionProps } from "./shared";
import { useCached } from "./useCached";

type Mode = {
  label: string;
  of: (p: LiftPoint, unit: Unit) => number;
  reps?: boolean;
  integer?: boolean;
};

const modes: Mode[] = [
  { label: "e1RM", of: (p, unit) => inUnit(p.e1rm, unit) },
  { label: "Max weight", of: (p, unit) => inUnit(p.bestWeight, unit) },
  { label: "Volume", of: (p, unit) => inUnit(p.volume, unit), integer: true },
  { label: "Max reps", of: (p) => p.bestReps, reps: true, integer: true },
];

export function LiftSection(props: SectionProps) {
  const { store, range } = props;
  const lifts = useMemo(() => loadedLifts(store, range), [store, range]);
  const [picked, setPicked] = useState<string>();
  const [mode, setMode] = useState(modes[0]);
  const slug = lifts.some((l) => l.slug === picked) ? picked : lifts[0]?.slug;

  return (
    <Card testID="progress-lifts">
      <Text variant="heading" accessibilityRole="header">
        Lifts
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
          <ScrollView
            horizontal
            showsHorizontalScrollIndicator={false}
            contentContainerStyle={{ gap: space[2] }}
          >
            {modes.map((m) => (
              <Chip
                key={m.label}
                label={m.label}
                selected={m === mode}
                onPress={() => setMode(m)}
              />
            ))}
          </ScrollView>
          <Records {...props} slug={slug} />
          <LiftChart {...props} slug={slug} mode={mode} />
        </>
      ) : loadedLifts(store).length ? (
        <EmptyState
          icon="barbell-outline"
          title="No lifts in this range"
          message="Pick a longer range, or finish a workout with loaded sets."
        />
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

const fmt = (v: number) => v.toFixed(1);

// All-time bests; the range chart below reports a failed fetch, so this one just stays out.
function Records({ insights, store, unit, slug }: SectionProps & { slug: string }) {
  const fetch = useCallback(() => insights.liftHistory({ slug }), [insights, slug]);
  const { data } = useCached(store, `progress.lift.${slug}.all`, fetch);
  if (!data?.points.length) return null;
  const top = (of: (p: LiftPoint) => number) =>
    data.points.reduce((best, p) => (of(p) > of(best) ? p : best));
  const e1rm = top((p) => inUnit(p.e1rm, unit));
  const heavy = top((p) => inUnit(p.bestWeight, unit));
  return (
    <StatTiles
      items={[
        {
          label: `All-time e1RM (${unit}) · ${shortDate(e1rm.date)}`,
          value: fmt(inUnit(e1rm.e1rm, unit)),
        },
        {
          label: `All-time max (${unit}) · ${shortDate(heavy.date)}`,
          value: `${fmt(inUnit(heavy.bestWeight, unit))} × ${heavy.bestReps}`,
        },
      ]}
    />
  );
}

function LiftChart({
  insights,
  store,
  range,
  unit,
  width,
  slug,
  mode,
}: SectionProps & { slug: string; mode: Mode }) {
  const router = useRouter();
  const { from, to } = range;
  const fetch = useCallback(
    () => insights.liftHistory({ slug, dates: { from, to } }),
    [insights, slug, from, to],
  );
  const state = useCached(store, `progress.lift.${slug}.${from}.${to}`, fetch);
  const format = mode.integer ? (v: number) => String(Math.round(v)) : fmt;
  const show = (v: number) => `${format(v)} ${mode.reps ? "reps" : unit}`;

  return (
    <CachedView state={state}>
      {({ name, points }) => {
        if (!points.length) {
          return <EmptyState icon="barbell-outline" title={`No loaded sets for ${name} yet`} />;
        }
        const ys = points.map((p) => mode.of(p, unit));
        const best = ys.indexOf(Math.max(...ys));
        return (
          <LineChart
            key={slug}
            testID="lift-chart"
            width={width}
            points={points.map((p, i) => ({ x: daysBetween(from, p.date), y: ys[i] }))}
            xDomain={[0, range.days]}
            xLabels={[shortDate(from), shortDate(to)]}
            xLabel={(x) => shortDate(addDays(from, x))}
            onOpen={
              points[0].recordId ? (i) => router.push(`/session/${points[i].recordId}`) : undefined
            }
            format={format}
            minSpan={mode.integer ? 4 : 0}
            summary={`${name}: ${points.length} sessions, ${mode.label} ${show(ys[0])} to ${show(ys[ys.length - 1])}, best ${show(ys[best])} on ${shortDate(points[best].date)}.`}
            table={{
              columns: ["Date", mode.reps ? mode.label : `${mode.label} ${unit}`, "Best set"],
              rows: points.map((p, i) => [
                shortDate(p.date),
                format(ys[i]),
                `${fmt(inUnit(p.bestWeight, unit))} × ${p.bestReps}`,
              ]),
            }}
          />
        );
      }}
    </CachedView>
  );
}
