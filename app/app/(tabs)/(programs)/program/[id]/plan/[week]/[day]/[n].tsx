import { useLocalSearchParams } from "expo-router";

import { useUnit } from "../../../../../../../../src/data/settings";
import PlanScreen from "../../../../../../../../src/features/programs/PlanScreen";

export default function PlanRoute() {
  const p = useLocalSearchParams<{ id: string; week: string; day: string; n: string }>();
  return (
    <PlanScreen
      programId={p.id}
      week={Number(p.week)}
      day={Number(p.day)}
      n={Number(p.n)}
      unit={useUnit()}
    />
  );
}
