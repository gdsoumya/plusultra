import { create } from "@bufbuild/protobuf";
import { useState } from "react";

import { errorMessage } from "../../api/errors";
import { useApi } from "../../api/useApi";
import { rememberExerciseData } from "../../data/exerciseData";
import { useStore } from "../../data/StoreProvider";
import { ExerciseDataSchema } from "../../gen/plusultra/v1/exercise_pb";
import { ErrorLine, ListCard, ListRow } from "../../ui";

type Props = {
  /** Unknown until the catalog entry is cached; the row is then read-only. */
  slug?: string;
  unilateral: boolean;
  onChange: (unilateral: boolean) => void;
};

// SetExerciseData replaces everything, so the 1RM, rounding and notes are read first and sent back
// with the flipped flag.
export function UnilateralRow({ slug, unilateral, onChange }: Props) {
  const api = useApi();
  const store = useStore();
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState<string>();

  const toggle = async () => {
    if (!slug || busy) return;
    setBusy(true);
    setError(undefined);
    try {
      const { data } = await api.exercises.getExerciseData({ slug });
      const next = { ...(data ?? create(ExerciseDataSchema, { slug })), unilateral: !unilateral };
      const saved = (await api.exercises.setExerciseData({ data: next })).data ?? next;
      rememberExerciseData(store, saved);
      onChange(saved.unilateral);
    } catch (e) {
      setError(errorMessage(e));
    } finally {
      setBusy(false);
    }
  };

  return (
    <>
      <ListCard>
        <ListRow
          icon="swap-horizontal-outline"
          title="Unilateral"
          subtitle="Log left and right reps"
          value={busy ? "Saving" : unilateral ? "On" : "Off"}
          onPress={slug ? toggle : undefined}
          testID="unilateral-toggle"
        />
      </ListCard>
      <ErrorLine message={error} />
    </>
  );
}
