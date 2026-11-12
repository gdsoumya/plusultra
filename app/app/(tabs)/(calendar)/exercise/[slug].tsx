import { useLocalSearchParams } from "expo-router";

import ExerciseDetailScreen from "../../../../src/features/exercises/ExerciseDetailScreen";

export default function ExerciseRoute() {
  const { slug } = useLocalSearchParams<{ slug: string }>();
  return <ExerciseDetailScreen slug={slug} fallback="/calendar" />;
}
