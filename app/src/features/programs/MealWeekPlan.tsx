import { useRouter } from "expo-router";

import { View } from "react-native";

import { dayTotals, mealTotals } from "../../domain/totals";
import type { MealDay } from "../../domain/types";
import { Chip, ChipStrip, ListCard, ListRow, space, Text } from "../../ui";
import { itemsSummary, kcalProtein } from "../meals/format";
import { MacroStats } from "../meals/MacroStats";
import { mealDaySegment } from "./programStore";

type Props = {
  programId: string;
  weeks: number;
  days: MealDay[];
  week: number;
  onWeek: (week: number) => void;
};

// One week at a time: the kind days and numbered days as cards, each meal a row opening its plan.
export function MealWeekPlan({ programId, weeks, days, week, onWeek }: Props) {
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
        .map((d, i) => {
          const about = [d.kind !== "training" ? d.kind : "", d.desc].filter(Boolean).join(" · ");
          return (
            <ListCard key={i}>
              {/* The day's name and its planned macros against the target lead the card; the
                card's divider sets them off from the meals. */}
              <View style={{ padding: space[4], gap: space[3] }}>
                <View style={{ gap: space[1] }}>
                  <Text variant="heading" accessibilityRole="header" numberOfLines={2}>
                    {d.day ? `${d.day}. ${d.name}` : d.name}
                  </Text>
                  {about ? (
                    <Text variant="caption" color="textMuted">
                      {about}
                    </Text>
                  ) : null}
                </View>
                <MacroStats totals={dayTotals(d.meals)} target={d.target} />
              </View>
              {d.meals.map((m, n) => (
                <ListRow
                  key={n}
                  title={m.name}
                  subtitle={[m.time, itemsSummary(m.items)].filter(Boolean).join(" · ")}
                  value={kcalProtein(mealTotals(m))}
                  onPress={() =>
                    router.push(
                      `/program/${programId}/meal/${d.week}/${mealDaySegment(d)}/${n + 1}`,
                    )
                  }
                />
              ))}
            </ListCard>
          );
        })}
    </>
  );
}
