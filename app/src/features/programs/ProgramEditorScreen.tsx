import { router } from "expo-router";
import { useCallback, useEffect, useRef, useState, type ComponentRef } from "react";
import { ScrollView, View, type TextInput, type TextInputSelectionChangeEvent } from "react-native";

import { fromProto } from "../../api/convert";
import { errorMessage } from "../../api/errors";
import { useApi } from "../../api/useApi";
import { useStore } from "../../data/StoreProvider";
import type { ResolvedDay, Unit } from "../../domain/types";
import type { Exercise } from "../../gen/plusultra/v1/exercise_pb";
import {
  ProgramKind,
  type ProgramStructure,
  type ScriptError,
} from "../../gen/plusultra/v1/program_pb";
import {
  BackButton,
  Button,
  Chip,
  ChipStrip,
  ConfirmSheet,
  ErrorLine,
  IconButton,
  ListCard,
  ListRow,
  Screen,
  space,
  Text,
  useBackToClose,
} from "../../ui";
import { ExerciseSearchSheet } from "../exercises/ExerciseSearchSheet";
import { scriptErrorsOf } from "./errors";
import { PreviewSheet } from "./PreviewSheet";
import { remember } from "./programStore";
import { insertExercise, kindOf, offsetOf, replaceName, type Cursor } from "./script";
import { ScriptEditor } from "./ScriptEditor";

// An unknown name carries the server's closest matches, offered as replacements.
type LineError = Pick<ScriptError, "line" | "col" | "message"> & {
  name?: string;
  suggestions?: string[];
};
type Preview = { days: ResolvedDay[]; output: string[] };

const DEBOUNCE_MS = 800;

export default function ProgramEditorScreen({ id, unit = "kg" }: { id?: string; unit?: Unit }) {
  const api = useApi();
  const store = useStore();
  const [script, setScript] = useState("");
  const [loaded, setLoaded] = useState(!id);
  const [errors, setErrors] = useState<LineError[]>([]);
  const [structure, setStructure] = useState<ProgramStructure>();
  const [checked, setChecked] = useState(false);
  const [message, setMessage] = useState<string>();
  const [saving, setSaving] = useState(false);
  const [week, setWeek] = useState(1);
  const [preview, setPreview] = useState<Preview>();
  const [previewing, setPreviewing] = useState(false);
  const [finding, setFinding] = useState(false);
  const [leaving, setLeaving] = useState(false);
  // What was loaded or last saved, so Back knows whether there is anything to lose.
  const [saved, setSaved] = useState("");
  // Set only to move the cursor; cleared on the next selection event so typing stays uncontrolled.
  const [selection, setSelection] = useState<{ start: number; end: number }>();
  const inputRef = useRef<ComponentRef<typeof TextInput>>(null);
  // Where the cursor is, so a found exercise lands there.
  const cursor = useRef<Cursor>({ start: 0, end: 0 });
  // What the input holds now, so a slow validation of older text is ignored.
  const latest = useRef("");

  const change = (text: string) => {
    latest.current = text;
    setScript(text);
    setChecked(false);
  };

  useEffect(() => {
    if (!id) return;
    api.programs.getProgram({ id }).then(
      ({ program }) => {
        setSaved(program?.script ?? "");
        change(program?.script ?? "");
        setLoaded(true);
      },
      (e) => setMessage(errorMessage(e)),
    );
  }, [api, id]);

  const validate = useCallback(
    async (text: string) => {
      try {
        const r = await api.programs.validateProgram({ kind: kindOf(text), script: text });
        if (text !== latest.current) return;
        // One row per unknown name, at its first line; fixing it there re-validates the rest.
        const seen = new Set<string>();
        const names = [...r.nameErrors]
          .sort((a, b) => a.line - b.line)
          .filter((n) => {
            const key = `${n.name}\n${n.suggestions.join("\n")}`;
            if (seen.has(key)) return false;
            seen.add(key);
            return true;
          })
          .map((n) => ({
            line: n.line,
            col: 0,
            message: `unknown name "${n.name}"`,
            name: n.name,
            suggestions: n.suggestions.slice(0, 3),
          }));
        setErrors([...r.errors, ...names].sort((a, b) => a.line - b.line));
        setStructure(r.structure);
        setChecked(true);
      } catch (e) {
        setMessage(errorMessage(e));
      }
    },
    [api],
  );

  useEffect(() => {
    if (!script.trim()) return;
    const timer = setTimeout(() => validate(script), DEBOUNCE_MS);
    return () => clearTimeout(timer);
  }, [script, validate]);

  const save = async () => {
    setSaving(true);
    setMessage(undefined);
    try {
      const { program } = id
        ? await api.programs.updateProgram({ id, script })
        : await api.programs.createProgram({ kind: kindOf(script), script });
      remember(store, program!);
      setSaved(script);
      if (id) router.back();
      else router.replace(`/program/${program!.id}`);
    } catch (e) {
      const lineErrors = scriptErrorsOf(e);
      if (lineErrors) {
        setErrors(lineErrors);
        setChecked(true);
      } else {
        setMessage(errorMessage(e));
      }
    } finally {
      setSaving(false);
    }
  };

  const runPreview = async () => {
    setPreviewing(true);
    try {
      const r = await api.programs.simulateProgram({
        script,
        weeks: week,
        assumptions: { completeAll: true, reps: {} },
      });
      if (r.errors.length) {
        setErrors(r.errors);
        setChecked(true);
      } else {
        setPreview({
          days: r.days.filter((d) => d.week === week).map((d) => fromProto.resolvedDay(d).payload),
          output: r.output,
        });
      }
    } catch (e) {
      setMessage(errorMessage(e));
    } finally {
      setPreviewing(false);
    }
  };

  const jumpTo = (e: LineError) => moveCursor(offsetOf(script, e.line, e.col));

  const moveCursor = (offset: number) => {
    cursor.current = { start: offset, end: offset };
    setSelection(cursor.current);
    inputRef.current?.focus();
  };

  const insert = (exercise: Exercise) => {
    const r = insertExercise(script, cursor.current, exercise.name);
    change(r.script);
    setFinding(false);
    moveCursor(r.cursor);
  };

  const fix = (e: LineError, to: string) => {
    change(replaceName(script, e.line, e.name!, to));
    setErrors((all) => all.filter((other) => other !== e));
  };

  const selectionChange = (e: TextInputSelectionChangeEvent) => {
    cursor.current = e.nativeEvent.selection;
    if (selection) setSelection(undefined);
  };

  const weeks = structure?.weeks ?? 1;
  const isWorkout = kindOf(script) === ProgramKind.WORKOUT;
  const dirty = script !== saved;
  const leave = () => (router.canGoBack() ? router.back() : router.replace("/programs"));
  useBackToClose(() => setLeaving(true), dirty && !leaving);

  return (
    <Screen
      title={id ? "Edit program" : "New program"}
      scroll={false}
      testID="program-editor"
      left={
        <BackButton fallback="/programs" onPress={dirty ? () => setLeaving(true) : undefined} />
      }
      right={
        <>
          {isWorkout && (
            <IconButton
              icon="search-outline"
              label="Find exercise"
              onPress={() => setFinding(true)}
            />
          )}
          <Button
            title="Save"
            loading={saving}
            disabled={!loaded || !script.trim()}
            onPress={save}
          />
        </>
      }
    >
      <ScrollView keyboardShouldPersistTaps="handled" contentContainerStyle={{ gap: space[3] }}>
        <ErrorLine message={message} />
        <View style={{ flexDirection: "row", gap: space[2] }}>
          <Button
            title="Check now"
            variant="secondary"
            style={{ flex: 1 }}
            onPress={() => validate(script)}
          />
          {isWorkout && (
            <Button
              title={`Preview week ${week}`}
              variant="secondary"
              style={{ flex: 1 }}
              loading={previewing}
              disabled={!script.trim()}
              onPress={runPreview}
            />
          )}
        </View>
        {isWorkout && weeks > 1 && (
          <ChipStrip selected={week - 1}>
            {Array.from({ length: weeks }, (_, i) => i + 1).map((w) => (
              <Chip key={w} label={`${w}`} selected={w === week} onPress={() => setWeek(w)} />
            ))}
          </ChipStrip>
        )}
        {checked && errors.length === 0 && structure && (
          <Text color="success">
            {`No errors. ${structure.weeks} week${structure.weeks === 1 ? "" : "s"}, ${structure.days.length} day${structure.days.length === 1 ? "" : "s"}.`}
          </Text>
        )}
        {errors.length > 0 && (
          <ListCard>
            {errors.flatMap((e, i) => [
              <ListRow
                key={i}
                icon="alert-circle-outline"
                title={`Line ${e.line}`}
                subtitle={e.message}
                onPress={() => jumpTo(e)}
              />,
              ...(e.suggestions ?? []).map((to) => (
                <ListRow
                  key={`${i}-${to}`}
                  icon="swap-horizontal-outline"
                  title={to}
                  subtitle={`Did you mean this? Replaces "${e.name}" on line ${e.line}`}
                  onPress={() => fix(e, to)}
                />
              )),
            ])}
          </ListCard>
        )}
        <ScriptEditor
          value={script}
          onChangeText={change}
          errorLines={errors.map((e) => e.line)}
          selection={selection}
          onSelectionChange={selectionChange}
          inputRef={inputRef}
        />
      </ScrollView>
      <ExerciseSearchSheet open={finding} onPick={insert} onClose={() => setFinding(false)} />
      <ConfirmSheet
        open={leaving}
        title="Discard changes?"
        message="The program is not saved. Leaving loses what you typed."
        action="Discard"
        cancel="Keep editing"
        onConfirm={leave}
        onClose={() => setLeaving(false)}
      />
      <PreviewSheet
        week={week}
        unit={unit}
        preview={preview}
        onClose={() => setPreview(undefined)}
      />
    </Screen>
  );
}
