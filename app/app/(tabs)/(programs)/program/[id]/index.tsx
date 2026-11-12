import { useLocalSearchParams } from "expo-router";

import { useUnit } from "../../../../../src/data/settings";
import ProgramDetailScreen from "../../../../../src/features/programs/ProgramDetailScreen";

export default function ProgramRoute() {
  const { id } = useLocalSearchParams<{ id: string }>();
  return <ProgramDetailScreen id={id} unit={useUnit()} />;
}
