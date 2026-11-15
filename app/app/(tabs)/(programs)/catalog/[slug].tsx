import { useLocalSearchParams } from "expo-router";

import { useUnit } from "../../../../src/data/settings";
import CatalogProgramScreen from "../../../../src/features/programs/CatalogProgramScreen";

export default function CatalogProgramRoute() {
  const { slug } = useLocalSearchParams<{ slug: string }>();
  return <CatalogProgramScreen slug={slug} unit={useUnit()} />;
}
