import { useLocalSearchParams } from "expo-router";

import { useUnit } from "../../../../src/data/settings";
import WorkoutScreen from "../../../../src/features/workout/WorkoutScreen";

export default function WorkoutRoute() {
  const { id } = useLocalSearchParams<{ id: string }>();
  return <WorkoutScreen id={id} unit={useUnit()} />;
}
