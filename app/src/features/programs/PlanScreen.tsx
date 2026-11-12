import { View } from "react-native";

import { useExerciseInfo } from "../../data/exerciseInfo";
import { useStore } from "../../data/StoreProvider";
import { clock } from "../../domain/dates";
import { targetLabel } from "../../domain/sets";
import type { ResolvedSet, Unit } from "../../domain/types";
import { formatWeight } from "../../domain/units";
import {
  BackButton,
  Card,
  EmptyState,
  ListCard,
  ListRow,
  Screen,
  space,
  Text,
  Thumbnail,
} from "../../ui";
import { ExerciseDetail } from "../exercises/ExerciseDetail";
import { cachedDays } from "./programStore";

type Props = { programId: string; week: number; day: number; n: number; unit: Unit };

const target = (s: ResolvedSet, unit: Unit) =>
  s.ask
    ? `${targetLabel(s)} × ?`
    : s.weight.value
      ? `${targetLabel(s)} × ${formatWeight(s.weight, unit)}`
      : targetLabel(s);

const details = (s: ResolvedSet) =>
  [
    s.label,
    s.rpe ? `RPE ${s.rpe}` : "",
    s.rest ? `rest ${clock(s.rest)}` : "",
    s.work ? `work ${clock(s.work)}` : "",
  ]
    .filter(Boolean)
    .join(" · ");

// One exercise of a planned day: every set as the program resolved it, how it progresses, then
// what the catalog knows about the exercise.
export default function PlanScreen({ programId, week, day, n, unit }: Props) {
  const store = useStore();
  const planned = cachedDays(store, programId).find((d) => d.week === week && d.day === day);
  const e = planned?.exercises[n - 1];
  const { info } = useExerciseInfo({ slug: e?.slug, name: e?.name });
  const back = <BackButton fallback={`/program/${programId}`} />;
  if (!e) {
    return (
      <Screen title="Plan" left={back} testID="plan-screen">
        <EmptyState
          icon="cloud-offline-outline"
          title="Plan not available"
          message="Open the program while connected to load its days."
        />
      </Screen>
    );
  }
  const others = e.alternates.filter((a) => a !== e.name);
  let warmups = 0;
  let work = 0;
  return (
    <Screen title={e.name} left={back} testID="plan-screen">
      <Card style={{ flexDirection: "row", alignItems: "center" }}>
        <Thumbnail uri={info?.images[0]} />
        <View style={{ flex: 1, gap: space[1] }}>
          <Text variant="caption">{`${planned.name} · week ${week}, day ${day}`}</Text>
          <Text numberOfLines={1}>{[e.equipment, e.label].filter(Boolean).join(" · ")}</Text>
          {others.length > 0 && (
            <Text variant="caption" numberOfLines={2}>{`Alternates: ${others.join(", ")}`}</Text>
          )}
        </View>
      </Card>
      <ListCard>
        {e.sets.map((s, i) => (
          <ListRow
            key={i}
            title={s.warmup ? `Warmup ${++warmups}` : `Set ${++work}`}
            subtitle={details(s) || undefined}
            value={target(s, unit)}
          />
        ))}
      </ListCard>
      {e.progression && (
        <Card>
          <Text variant="label">Progression</Text>
          <Text>{e.progression}</Text>
        </Card>
      )}
      {info && (
        <Card>
          <ExerciseDetail exercise={info} />
        </Card>
      )}
    </Screen>
  );
}
