import { useState } from "react";

import { errorMessage } from "../../api/errors";
import { useApi } from "../../api/useApi";
import { cachedExercise, rememberExercise } from "../../data/exerciseInfo";
import type { Store } from "../../data/store";
import { useStore } from "../../data/StoreProvider";
import type { Exercise } from "../../gen/plusultra/v1/exercise_pb";
import { ErrorLine, Field, ListCard, Loading, Sheet, Text } from "../../ui";
import { ExerciseRow } from "./ExerciseRow";

export const RECENTS_KEY = "recentExercisePicks";
const RECENTS = 8;

// The slugs picked here before, latest first; each is in the exercise cache, so recents show offline.
export const recentPicks = (store: Store) => store.getKv<string[]>(RECENTS_KEY) ?? [];

function pick(store: Store, exercise: Exercise) {
  rememberExercise(store, exercise);
  const rest = recentPicks(store).filter((slug) => slug !== exercise.slug);
  store.setKv(RECENTS_KEY, [exercise.slug, ...rest].slice(0, RECENTS));
}

type Props = {
  open: boolean;
  onPick: (exercise: Exercise) => void;
  onClose: () => void;
};

// Search the catalog by name; picking remembers the exercise for the Recent list.
export function ExerciseSearchSheet({ open, onPick, onClose }: Props) {
  const api = useApi();
  const store = useStore();
  const [query, setQuery] = useState("");
  const [results, setResults] = useState<Exercise[]>([]);
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState<string>();
  const recents = recentPicks(store);

  const search = async () => {
    setBusy(true);
    setError(undefined);
    try {
      setResults((await api.exercises.searchExercises({ query, limit: 10 })).exercises);
    } catch (e) {
      setError(errorMessage(e));
    } finally {
      setBusy(false);
    }
  };

  // The next opening starts fresh, with the pick at the top of Recent.
  const choose = (exercise: Exercise) => {
    pick(store, exercise);
    setQuery("");
    setResults([]);
    onPick(exercise);
  };

  return (
    <Sheet
      open={open}
      onClose={onClose}
      title="Find exercise"
      snapPoints={[0.7]}
      testID="exercise-search"
    >
      <Field
        label="Search"
        placeholder="Bench press"
        returnKeyType="search"
        value={query}
        onChangeText={setQuery}
        onSubmitEditing={search}
        autoFocus
      />
      <ErrorLine message={error} />
      {busy && <Loading />}
      {results.length > 0 && (
        <ListCard>
          {results.map((e) => (
            <ExerciseRow key={e.slug} exercise={e} onPress={() => choose(e)} />
          ))}
        </ListCard>
      )}
      {results.length === 0 && recents.length > 0 && (
        <>
          <Text variant="label" color="textMuted">
            Recent
          </Text>
          <ListCard>
            {recents.map((slug) => (
              <RecentRow key={slug} slug={slug} onPick={choose} />
            ))}
          </ListCard>
        </>
      )}
    </Sheet>
  );
}

function RecentRow({ slug, onPick }: { slug: string; onPick: (e: Exercise) => void }) {
  const store = useStore();
  const cached = cachedExercise(store, { slug })?.exercise;
  return cached ? <ExerciseRow exercise={cached} onPress={() => onPick(cached)} /> : null;
}
