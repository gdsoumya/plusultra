import { router } from "expo-router";
import { useCallback, useEffect, useState } from "react";
import { View } from "react-native";

import { errorMessage } from "../../api/errors";
import { fromProto } from "../../api/convert";
import { useApi } from "../../api/useApi";
import { rememberExerciseData } from "../../data/exerciseData";
import { useStore } from "../../data/StoreProvider";
import type { ResolvedDay, Unit as UnitName } from "../../domain/types";
import { formatWeight } from "../../domain/units";
import type { ExerciseData } from "../../gen/plusultra/v1/exercise_pb";
import { Unit } from "../../gen/plusultra/v1/common_pb";
import { ProgramKind, type CatalogProgram } from "../../gen/plusultra/v1/program_pb";
import {
  BackButton,
  Button,
  Card,
  Chip,
  ConfirmSheet,
  ErrorLine,
  Screen,
  Text,
  space,
} from "../../ui";
import { MaxSheet } from "../settings/MaxSheet";
import { remember } from "./programStore";
import { andList, levelLabel, spanLabel, withMaxesHint } from "./summary";
import { WeekPlan } from "./WeekPlan";

// shortcut: a lift's 1RM is looked up by slug prefix ("Bench Press" -> "bench-press-barbell"), as
// exercise data carries no names; upgrade if the server ever returns the slug of each max.
const hasMax = (name: string, data: ExerciseData[]) => {
  const slug = name.toLowerCase().replace(/[^a-z0-9]+/g, "-");
  return data.some((d) => d.rm1 && (d.slug === slug || d.slug.startsWith(`${slug}-`)));
};

// Stands in for a 1RM the user has not set, so the preview still shows weights.
const placeholderKg: Record<string, number> = {
  Squat: 100,
  "Bench Press": 70,
  Deadlift: 120,
  "Overhead Press": 45,
};
const placeholder = (name: string) => ({ value: placeholderKg[name] ?? 60, unit: Unit.KG });

// A built-in program: what it is, a simulated preview, and a button that copies it into a draft.
export default function CatalogProgramScreen({
  slug,
  unit = "kg",
}: {
  slug: string;
  unit?: UnitName;
}) {
  const api = useApi();
  const store = useStore();
  const [program, setProgram] = useState<CatalogProgram>();
  const [days, setDays] = useState<ResolvedDay[]>([]);
  const [previewError, setPreviewError] = useState<string>();
  const [week, setWeek] = useState(1);
  const [message, setMessage] = useState<string>();
  const [missing, setMissing] = useState<string[]>([]);
  const [adding, setAdding] = useState(false);
  const [busy, setBusy] = useState(false);
  const [addError, setAddError] = useState<string>();
  const [lift, setLift] = useState<{ slug: string; name: string }>();
  const [savingMax, setSavingMax] = useState(false);
  const [maxError, setMaxError] = useState<string>();

  const preview = useCallback(
    async (p: CatalogProgram, missing: string[]) => {
      const rm1 = Object.fromEntries(missing.map((n) => [n, placeholder(n)]));
      const r = await api.programs.simulateProgram({
        script: p.script,
        weeks: p.weeks,
        overrides: missing.length ? { rm1 } : undefined,
      });
      setPreviewError(r.errors[0] && withMaxesHint(r.errors[0].message));
      setDays(r.days.map((d) => fromProto.resolvedDay(d).payload));
    },
    [api],
  );

  useEffect(() => {
    api.programs
      .getCatalogProgram({ slug })
      .then(async ({ program: p }) => {
        if (!p) return;
        setProgram(p);
        let names: string[] = [];
        if (p.maxes.length) {
          const { items } = await api.exercises.listExerciseData({});
          names = p.maxes.filter((m) => !hasMax(m, items));
          setMissing(names);
        }
        await preview(p, names);
      })
      .catch((e) => setMessage(errorMessage(e)));
  }, [api, slug, preview]);

  // The program names lifts ("Squat"); the sheet needs the exercise's slug.
  const openMax = async (name: string) => {
    try {
      const { exercises } = await api.exercises.searchExercises({ query: name, limit: 5 });
      const found = exercises.find((e) => e.name === name) ?? exercises[0];
      setMaxError(undefined);
      if (found) setLift({ slug: found.slug, name });
    } catch (e) {
      setMessage(errorMessage(e));
    }
  };

  const saveMax = async (data: ExerciseData) => {
    setSavingMax(true);
    try {
      const r = await api.exercises.setExerciseData({ data });
      rememberExerciseData(store, r.data ?? data);
      setLift(undefined);
      if (data.rm1) {
        const left = missing.filter((n) => n !== lift!.name);
        setMissing(left);
        await preview(program!, left);
      }
    } catch (e) {
      setMaxError(errorMessage(e));
    }
    setSavingMax(false);
  };

  const add = async () => {
    setBusy(true);
    try {
      const { program: p } = await api.programs.createProgram({
        kind: ProgramKind.WORKOUT,
        script: program!.script,
      });
      remember(store, p!);
      router.replace(`/program/${p!.id}`);
    } catch (e) {
      setAddError(errorMessage(e));
    }
    setBusy(false);
  };

  return (
    <Screen title={program?.name ?? "Catalog"} left={<BackButton fallback="/programs" />}>
      <ErrorLine message={message} />
      {program && (
        <>
          <Card>
            <Text variant="label">
              {`${levelLabel[program.level] ?? program.level}, ${program.daysPerWeek} days a week`}
            </Text>
            <Text>{`${spanLabel(program.weeks, program.loop)}${program.loop ? ", repeats" : ""}`}</Text>
            <Text color="textMuted">{`After ${program.author}`}</Text>
            <Text>{program.summary}</Text>
          </Card>
          {missing.length > 0 && (
            <>
              <Text color="textMuted">
                {`Preview uses ${missing.length === 1 ? "a placeholder 1RM" : "placeholder 1RMs"}: ${missing
                  .map((n) => `${n} ${formatWeight({ ...placeholder(n), unit: "kg" }, unit)}`)
                  .join(", ")}. Set yours here or under Settings › 1RMs.`}
              </Text>
              <View style={{ flexDirection: "row", flexWrap: "wrap", gap: space[2] }}>
                {missing.map((name) => (
                  <Chip key={name} label={`Set ${name} 1RM`} onPress={() => openMax(name)} />
                ))}
              </View>
            </>
          )}
          {lift && (
            <MaxSheet
              key={lift.slug}
              open
              slug={lift.slug}
              name={lift.name}
              unit={unit}
              busy={savingMax}
              error={maxError}
              onSave={saveMax}
              onClose={() => setLift(undefined)}
            />
          )}
          {(previewError || days.length > 0) && (
            <>
              <Text variant="heading">Preview</Text>
              {previewError ? (
                <ErrorLine message={previewError} />
              ) : (
                <WeekPlan
                  unit={unit}
                  weeks={program.weeks}
                  days={days}
                  plans={days}
                  week={week}
                  onWeek={setWeek}
                />
              )}
            </>
          )}
          <Button title="Add to my programs" icon="add" onPress={() => setAdding(true)} />
          <ConfirmSheet
            open={adding}
            title={`Add ${program.name}?`}
            message={`Saved as a draft under Mine, with no dates. Review or edit it there, then Activate to pick a start date or run it floating (no dates).${
              missing.length
                ? ` Set your ${andList(missing)} 1RM${missing.length === 1 ? "" : "s"} under Settings › 1RMs before activating.`
                : ""
            }`}
            action="Add as draft"
            variant="primary"
            busy={busy}
            error={addError}
            onConfirm={add}
            onClose={() => setAdding(false)}
          />
        </>
      )}
    </Screen>
  );
}
