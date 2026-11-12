import { useCallback, useState } from "react";

import { useApi } from "../../api/useApi";
import type { Exercise } from "../../gen/plusultra/v1/exercise_pb";
import type { Food } from "../../gen/plusultra/v1/food_pb";
import {
  Button,
  Chip,
  ChipStrip,
  ErrorLine,
  Field,
  ListCard,
  ListRow,
  Sheet,
  Text,
  useDebouncedSearch,
} from "../../ui";
import { CatalogDetail, type Entry } from "./CatalogDetail";
import { CustomExerciseSheet, CustomFoodSheet } from "./CustomEntrySheet";
import { SubScreen } from "./parts";
import { useCall } from "./useRemote";

type Kind = Entry["kind"];

const kinds: { kind: Kind; label: string; single: string }[] = [
  { kind: "exercise", label: "Exercises", single: "exercise" },
  { kind: "food", label: "Foods", single: "food" },
];

export default function CatalogScreen() {
  const api = useApi();
  const { call, busy, error } = useCall();
  const [kind, setKind] = useState<Kind>("exercise");
  const [query, setQuery] = useState("");
  const [results, setResults] = useState<Entry[]>([]);
  const [detail, setDetail] = useState<Entry>();
  const [detailOpen, setDetailOpen] = useState(false);
  const [confirming, setConfirming] = useState(false);
  const [creating, setCreating] = useState(false);
  const [opened, setOpened] = useState(0);

  const search = useCallback(
    (q: string) =>
      call(async () => {
        if (!q.trim()) return setResults([]);
        if (kind === "exercise") {
          const r = await api.exercises.searchExercises({ query: q, limit: 50 });
          setResults(r.exercises.map((item) => ({ kind, item })));
        } else {
          const r = await api.foods.searchFoods({ query: q, limit: 50 });
          setResults(r.foods.map((item) => ({ kind, item })));
        }
      }),
    [api, call, kind],
  );
  useDebouncedSearch(query, search);

  const show = (entry: Entry) => {
    setDetail(entry);
    setConfirming(false);
    setDetailOpen(true);
  };

  const created = (entry: Entry) => {
    setQuery(entry.item.name);
    setResults([entry]);
    setCreating(false);
  };

  const createExercise = (exercise: Exercise) =>
    call(async () => {
      const r = await api.exercises.createCustomExercise({ exercise });
      created({ kind: "exercise", item: r.exercise! });
    });

  const createFood = (food: Food) =>
    call(async () => {
      const r = await api.foods.createCustomFood({ food });
      created({ kind: "food", item: r.food! });
    });

  const remove = (entry: Entry) =>
    call(async () => {
      if (entry.kind === "exercise") {
        await api.exercises.deleteCustomExercise({ slug: entry.item.slug });
      } else {
        await api.foods.deleteCustomFood({ id: entry.item.id });
      }
      setResults((all) => all.filter((e) => e.item !== entry.item));
      setDetailOpen(false);
    });

  const subtitle = (entry: Entry) =>
    entry.kind === "exercise"
      ? [entry.item.equipment, entry.item.custom && "custom"].filter(Boolean).join(" · ")
      : `${Math.round(entry.item.per100g?.kcal ?? 0)} kcal · ${entry.item.source}`;

  const current = kinds.find((k) => k.kind === kind)!;
  const sheetOpen = detailOpen || creating;

  return (
    <SubScreen title="Catalog" testID="catalog-screen">
      <ChipStrip selected={kinds.indexOf(current)}>
        {kinds.map((k) => (
          <Chip
            key={k.kind}
            label={k.label}
            selected={kind === k.kind}
            onPress={() => {
              setKind(k.kind);
              setResults([]);
            }}
          />
        ))}
      </ChipStrip>
      <Field
        label="Search"
        placeholder={kind === "exercise" ? "Bench press" : "Chicken breast"}
        returnKeyType="search"
        value={query}
        onChangeText={setQuery}
        onSubmitEditing={() => search(query)}
      />
      {!sheetOpen && <ErrorLine message={error} />}
      {results.length > 0 && (
        <ListCard>
          {results.map((entry) => (
            <ListRow
              key={entry.item.id}
              title={entry.item.name}
              subtitle={subtitle(entry)}
              onPress={() => show(entry)}
            />
          ))}
        </ListCard>
      )}
      <Button
        title={`New ${current.single}`}
        icon="add"
        variant="secondary"
        onPress={() => {
          setOpened((n) => n + 1);
          setCreating(true);
        }}
      />
      {detail && (
        <Sheet
          open={detailOpen}
          onClose={() => setDetailOpen(false)}
          title={confirming ? `Delete ${detail.item.name}?` : detail.item.name}
          snapPoints={[0.6]}
        >
          {confirming ? (
            <>
              <Text>Scripts and logs that name it keep working; it only leaves the search.</Text>
              <ErrorLine message={error} />
              <Button
                title={`Delete ${detail.item.name}`}
                variant="danger"
                onPress={() => remove(detail)}
                loading={busy}
              />
              <Button title="Cancel" variant="secondary" onPress={() => setConfirming(false)} />
            </>
          ) : (
            <>
              <CatalogDetail entry={detail} />
              {detail.item.custom && (
                <Button title="Delete" variant="danger" onPress={() => setConfirming(true)} />
              )}
            </>
          )}
        </Sheet>
      )}
      {kind === "exercise" ? (
        <CustomExerciseSheet
          key={opened}
          open={creating}
          busy={busy}
          error={error}
          onCreate={createExercise}
          onClose={() => setCreating(false)}
        />
      ) : (
        <CustomFoodSheet
          key={opened}
          open={creating}
          busy={busy}
          error={error}
          onCreate={createFood}
          onClose={() => setCreating(false)}
        />
      )}
    </SubScreen>
  );
}
