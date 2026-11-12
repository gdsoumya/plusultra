import { useRouter } from "expo-router";

import { setsSummary } from "../../domain/sets";
import type { ResolvedDay, Unit } from "../../domain/types";
import type { StructureDay } from "../../gen/plusultra/v1/program_pb";
import { Chip, ChipStrip, ListCard, ListRow } from "../../ui";
import { ExerciseRow } from "../exercises/ExerciseRow";

type Props = {
  programId: string;
  unit: Unit;
  weeks: number;
  /** The declared days; offline, the cached resolved days stand in. */
  days: Pick<StructureDay, "week" | "day" | "name" | "kind">[];
  /** Resolved days, when the server or the cache has them; a day without one lists no exercises. */
  plans: ResolvedDay[];
  week: number;
  onWeek: (week: number) => void;
};

// One week at a time: a chip per week, then a card per day with its exercises and their sets;
// a row opens the exercise's plan.
export function WeekPlan({ programId, unit, weeks, days, plans, week, onWeek }: Props) {
  const router = useRouter();
  const numbers = Array.from({ length: weeks }, (_, i) => i + 1);
  return (
    <>
      {numbers.length > 1 && (
        <ChipStrip selected={week - 1}>
          {numbers.map((w) => (
            <Chip key={w} label={`Week ${w}`} selected={w === week} onPress={() => onWeek(w)} />
          ))}
        </ChipStrip>
      )}
      {days
        .filter((d) => d.week === week)
        .map((d, i) => (
          <ListCard key={i}>
            <ListRow
              title={d.day ? `${d.day}. ${d.name}` : d.name}
              subtitle={
                d.kind !== "training" && !d.name.toLowerCase().includes(d.kind) ? d.kind : undefined
              }
            />
            {plans
              .find((r) => r.week === d.week && r.day === d.day)
              ?.exercises.map((e, n) => (
                <ExerciseRow
                  key={n}
                  exercise={{ slug: e.slug, name: e.name }}
                  subtitle={setsSummary(e.sets, unit) + (e.unilateral ? " · per side" : "")}
                  onPress={() =>
                    router.push(`/program/${programId}/plan/${d.week}/${d.day}/${n + 1}`)
                  }
                />
              ))}
          </ListCard>
        ))}
    </>
  );
}
