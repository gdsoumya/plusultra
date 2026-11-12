import { useCallback, useState } from "react";

import { fromProto } from "../../api/convert";
import { useApi } from "../../api/useApi";
import { rememberExerciseData } from "../../data/exerciseData";
import { useStore } from "../../data/StoreProvider";
import { formatWeight } from "../../domain/units";
import type { Exercise, ExerciseData } from "../../gen/plusultra/v1/exercise_pb";
import {
  EmptyState,
  ErrorLine,
  Field,
  ListCard,
  ListRow,
  Loading,
  Text,
  useDebouncedSearch,
} from "../../ui";
import { MaxSheet } from "./MaxSheet";
import { SubScreen } from "./parts";
import { describeRounding } from "./rounding";
import { useRemote } from "./useRemote";
import { useUnit } from "../../data/settings";

// "squat-barbell" as "Squat Barbell"; the list has only slugs.
const nameOf = (slug: string) =>
  slug
    .split("-")
    .map((w) => w.charAt(0).toUpperCase() + w.slice(1))
    .join(" ");

export default function MaxesScreen() {
  const api = useApi();
  const store = useStore();
  const units = useUnit();
  const load = useCallback(() => api.exercises.listExerciseData({}).then((r) => r.items), [api]);
  const { data: items = [], setData, loading, busy, error, call } = useRemote(load);
  const [query, setQuery] = useState("");
  const [results, setResults] = useState<Exercise[]>([]);
  const [target, setTarget] = useState<{ slug: string; name: string }>();
  const [open, setOpen] = useState(false);
  const [opened, setOpened] = useState(0);

  const search = useCallback(
    (q: string) =>
      call(async () => {
        const r = await api.exercises.searchExercises({ query: q, limit: 10 });
        setResults(r.exercises);
      }),
    [api, call],
  );
  useDebouncedSearch(query, search);

  const edit = (slug: string, name: string) => {
    setTarget({ slug, name });
    setOpened((n) => n + 1);
    setOpen(true);
  };

  const save = (data: ExerciseData) =>
    call(async () => {
      const r = await api.exercises.setExerciseData({ data });
      rememberExerciseData(store, r.data!);
      setData([...items.filter((d) => d.slug !== data.slug), r.data!]);
      setOpen(false);
    });

  return (
    <SubScreen title="1RMs" testID="maxes-screen">
      <Field
        label="Find an exercise"
        placeholder="Bench press"
        returnKeyType="search"
        value={query}
        onChangeText={setQuery}
        onSubmitEditing={() => search(query)}
      />
      {results.length > 0 && (
        <ListCard>
          {results.map((e) => (
            <ListRow
              key={e.slug}
              title={e.name}
              subtitle={e.equipment}
              onPress={() => edit(e.slug, e.name)}
            />
          ))}
        </ListCard>
      )}
      {!open && <ErrorLine message={error} />}
      <Text variant="heading">Your lifts</Text>
      {loading ? (
        <Loading />
      ) : items.length ? (
        <ListCard>
          {items.map((d) => (
            <ListRow
              key={d.slug}
              title={nameOf(d.slug)}
              subtitle={describeRounding(d.rounding)}
              value={d.rm1 && formatWeight(fromProto.weight(d.rm1), units)}
              onPress={() => edit(d.slug, nameOf(d.slug))}
            />
          ))}
        </ListCard>
      ) : (
        !error && (
          <EmptyState
            icon="trophy-outline"
            title="No 1RMs yet"
            message="Find an exercise above to set one."
          />
        )
      )}
      {target && (
        <MaxSheet
          key={opened}
          open={open}
          slug={target.slug}
          name={target.name}
          data={items.find((d) => d.slug === target.slug)}
          unit={units}
          busy={busy}
          error={error}
          onSave={save}
          onClose={() => setOpen(false)}
        />
      )}
    </SubScreen>
  );
}
