import { useRouter } from "expo-router";

import { useExerciseInfo, type ExerciseRef } from "../../data/exerciseInfo";
import type { Exercise } from "../../gen/plusultra/v1/exercise_pb";
import { ListRow, Thumbnail } from "../../ui";

// "chest · barbell": what an exercise works and needs, under its name.
export const infoLine = (e?: Exercise) =>
  e ? [e.primaryMuscles.join(", "), e.equipment].filter(Boolean).join(" · ") : undefined;

export const exerciseHref = (slug: string) => `/exercise/${slug}` as const;

export function ExerciseThumb({ exercise, size }: { exercise: ExerciseRef; size?: number }) {
  const { info } = useExerciseInfo(exercise);
  return <Thumbnail uri={info?.images[0]} size={size} />;
}

type Props = {
  exercise: ExerciseRef;
  /** Defaults to the muscles and equipment line. */
  subtitle?: string;
  value?: string;
  /** Defaults to opening the exercise detail once the catalog entry is known. */
  onPress?: () => void;
};

// A list row for an exercise: thumbnail, name and what it works; tapping opens the detail.
export function ExerciseRow({ exercise, subtitle, value, onPress }: Props) {
  const router = useRouter();
  const { info } = useExerciseInfo(exercise);
  const open = info && (() => router.push(exerciseHref(info.slug)));
  return (
    <ListRow
      leading={<Thumbnail uri={info?.images[0]} />}
      title={exercise.name ?? info?.name ?? ""}
      subtitle={subtitle ?? infoLine(info)}
      value={value}
      onPress={onPress ?? open}
    />
  );
}
