import { useCallback, useState } from "react";
import { View } from "react-native";

import { errorMessage } from "../../api/errors";
import { useApi } from "../../api/useApi";
import type { Exercise } from "../../gen/plusultra/v1/exercise_pb";
import type { WorkoutRecord } from "../../domain/types";
import {
  Chip,
  ErrorLine,
  Field,
  ListCard,
  ListRow,
  Loading,
  ChipStrip,
  Sheet,
  space,
  Text,
  useDebouncedSearch,
} from "../../ui";
import type { Picked } from "./workoutEdits";

// Exercises from past workouts, latest first, skipping the names already in today's.
export function recentExercises(records: WorkoutRecord[], exclude: string[], limit = 8): Picked[] {
  const seen = new Set(exclude);
  const out: Picked[] = [];
  for (const r of [...records].sort((a, b) => b.date.localeCompare(a.date))) {
    for (const { name, slug, equipment } of r.entries) {
      if (seen.has(name) || out.length >= limit) continue;
      seen.add(name);
      out.push({ name, slug, equipment });
    }
  }
  return out;
}

type Props = {
  open: boolean;
  recents: Picked[];
  onPick: (exercise: Picked) => void;
  onClose: () => void;
};

export function AddExerciseSheet({ open, recents, onPick, onClose }: Props) {
  const api = useApi();
  const [query, setQuery] = useState("");
  const [results, setResults] = useState<Exercise[]>([]);
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState<string>();

  const search = useCallback(
    async (q: string) => {
      setBusy(true);
      setError(undefined);
      try {
        setResults((await api.exercises.searchExercises({ query: q, limit: 10 })).exercises);
      } catch (e) {
        setError(errorMessage(e));
      } finally {
        setBusy(false);
      }
    },
    [api],
  );
  useDebouncedSearch(query, search);

  return (
    <Sheet
      open={open}
      onClose={onClose}
      title="Add exercise"
      snapPoints={[0.7]}
      testID="add-exercise"
    >
      <Field
        label="Find an exercise"
        placeholder="Bench press"
        returnKeyType="search"
        value={query}
        onChangeText={setQuery}
        onSubmitEditing={() => search(query)}
        autoFocus
      />
      <ErrorLine message={error} />
      {busy && <Loading />}
      {results.length > 0 && (
        <ListCard>
          {results.map((e) => (
            <ListRow key={e.slug} title={e.name} subtitle={e.equipment} onPress={() => onPick(e)} />
          ))}
        </ListCard>
      )}
      {recents.length > 0 && (
        <View style={{ gap: space[2] }}>
          <Text variant="label" color="textMuted">
            Recent
          </Text>
          <ChipStrip>
            {recents.map((e) => (
              <Chip key={e.name} label={e.name} onPress={() => onPick(e)} />
            ))}
          </ChipStrip>
        </View>
      )}
    </Sheet>
  );
}
