import { useLocalSearchParams } from "expo-router";

import MealPlanScreen from "../../../../../../../../src/features/programs/MealPlanScreen";

export default function MealPlanRoute() {
  const p = useLocalSearchParams<{ id: string; week: string; day: string; n: string }>();
  return <MealPlanScreen programId={p.id} week={Number(p.week)} day={p.day} n={Number(p.n)} />;
}
