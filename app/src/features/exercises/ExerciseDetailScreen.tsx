import type { Href } from "expo-router";

import { useExerciseInfo } from "../../data/exerciseInfo";
import { BackButton, Card, EmptyState, Loading, Screen } from "../../ui";
import { ExerciseDetail } from "./ExerciseDetail";

// One shared screen; each tab's stack mounts it so the detail opens inside the current tab.
export default function ExerciseDetailScreen({ slug, fallback }: { slug: string; fallback: Href }) {
  const { info, loading } = useExerciseInfo({ slug });
  return (
    <Screen
      title={info?.name ?? "Exercise"}
      left={<BackButton fallback={fallback} />}
      testID="exercise-screen"
    >
      {info ? (
        <Card>
          <ExerciseDetail exercise={info} />
        </Card>
      ) : loading ? (
        <Loading />
      ) : (
        <EmptyState
          icon="cloud-offline-outline"
          title="Exercise not available"
          message="Connect to your server to load it."
        />
      )}
    </Screen>
  );
}
