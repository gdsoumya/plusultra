import { useFocusEffect, useRouter } from "expo-router";
import { useCallback, useRef, useState } from "react";
import { Pressable, View } from "react-native";
import { GestureDetector, usePanGesture } from "react-native-gesture-handler";

import { useApi } from "../../api/useApi";
import { fetchUnilateral } from "../../data/exerciseData";
import { programsRepo } from "../../data/repositories/programs";
import { useStore } from "../../data/StoreProvider";
import { useRefresh } from "../../data/useRefresh";
import { clock, formatDateTime, today } from "../../domain/dates";
import { movedIndex, moveItem } from "../../domain/order";
import { exerciseTarget } from "../../domain/sets";
import type { Unit, WorkoutRecord } from "../../domain/types";
import {
  BackButton,
  Badge,
  ConfirmSheet,
  EmptyState,
  Screen,
  space,
  Text,
  useBackToClose,
} from "../../ui";
import { SyncBadge } from "../sync/SyncBadge";
import { AddExerciseSheet, recentExercises } from "./AddExerciseSheet";
import { ExerciseCarousel } from "./ExerciseCarousel";
import { ExercisePane } from "./ExercisePane";
import { ExercisesSheet } from "./ExercisesSheet";
import { SessionActions } from "./SessionActions";
import { setTimerOnScreen } from "./notifications";
import { beep, type Beep } from "./sounds";
import { TimerBar } from "./TimerBar";
import { useTimer } from "./useTimer";
import { useWorkoutRecord } from "./useWorkoutRecord";
import {
  addExercise,
  completeSet,
  finish,
  mapExercise,
  markModified,
  nextPending,
  removeExercise,
  removeSet,
  setValues,
  switchAlternate,
  tapDone,
} from "./workoutEdits";

type Target = { exercise: number; set: number };
// The rest timer remembers the set it follows, so auto can chain into the next one.
type Timer = Target & { kind: "rest" | "work" };

const notices = {
  rest: { title: "Rest is over", body: "Time for the next set." },
  work: { title: "Set is over", body: "Stop and rest." },
};

// A sideways pan this far moves to the previous or next exercise; a vertical start scrolls instead.
const swipe = { activate: 24, fail: 16, distance: 60 };

export default function WorkoutScreen({ id, unit }: { id: string; unit: Unit }) {
  const router = useRouter();
  const api = useApi();
  const store = useStore();
  const { record, update } = useWorkoutRecord(id);
  // Beeps belong to the open screen; a sub-route on top (exercise details) leaves the notification
  // to speak for the timer.
  const focused = useRef(true);
  useFocusEffect(
    useCallback(() => {
      focused.current = true;
      setTimerOnScreen(true);
      return () => {
        focused.current = false;
        setTimerOnScreen(false);
      };
    }, []),
  );
  const sound = (kind: Beep) => focused.current && beep(kind);
  const timer = useTimer<Timer>((t, endedAt) => {
    sound("end");
    // The set may have been removed while its timer ran.
    const s = record?.entries[t.exercise]?.sets[t.set];
    if (!s) return;
    if (t.kind === "work") complete(t.exercise, t.set, s.planned.work, endedAt);
    else if (s.planned.auto) chain(t.exercise, endedAt);
  });
  // The record follows lastSyncedAt in useWorkoutRecord, so the sync's copy shows when it settles.
  const refresh = useRefresh();
  // Opens on the exercise with the next set to do.
  const [shownIndex, setShown] = useState(
    () => nextPending(record ?? { entries: [] })?.exercise ?? 0,
  );
  const [editing, setEditing] = useState<Target>();
  const [removing, setRemoving] = useState<number>();
  const [adding, setAdding] = useState(false);
  const [listing, setListing] = useState(false);
  useBackToClose(() => setEditing(undefined), editing !== undefined);
  const count = record?.entries.length ?? 0;
  const show = (i: number) => setShown(Math.max(0, Math.min(count - 1, i)));
  const pan = usePanGesture({
    testID: "exercise-pane-pan",
    runOnJS: true,
    activeOffsetX: [-swipe.activate, swipe.activate],
    failOffsetY: [-swipe.fail, swipe.fail],
    onDeactivate: (e) => {
      if (Math.abs(e.translationX) >= swipe.distance)
        show(shownIndex + (e.translationX < 0 ? 1 : -1));
    },
  });

  if (!record) {
    return (
      <Screen title="Workout" testID="workout-screen" left={<BackButton />}>
        <EmptyState icon="barbell-outline" title="Workout not found" />
      </Screen>
    );
  }

  const shown = Math.min(shownIndex, count - 1);
  const entry = record.entries[shown];
  const apply = (edit: Parameters<typeof update>[0]) => update((r) => markModified(edit(r)));
  // A finished workout stays editable (P2-R51); the server replays its progression on an edit.
  const finished = !!record.finishedAt || record.finalized;
  const active = nextPending(record);
  const onTarget = record.entries.filter((e) => exerciseTarget(e.sets, e.unilateral).met).length;
  // The server runs the program's progression on a past workout as if it were just finished.
  const countsForProgram =
    !record.finishedAt &&
    record.date < today() &&
    programsRepo(store).get(record.programId ?? "")?.status === "active";
  // Order is not a deviation: the server matches entries by exercise, not position.
  // A running timer addresses its set by position, so reordering or removing cancels it.
  const restructure = (edit: Parameters<typeof update>[0]) => {
    timer.stop();
    update(edit);
  };
  // Entries have no id, so the pane follows its exercise by moving its index along.
  const move = (from: number, to: number) => {
    const target = Math.max(0, Math.min(count - 1, to));
    restructure((r) => ({ ...r, entries: moveItem(r.entries, from, target) }));
    setShown(movedIndex(shown, from, target));
  };

  // `from` is when the timer before it ended, which is in the past if the app was away.
  const startSet = (exercise: number, set: number, from?: number) => {
    const { work, countUp, rest } = record.entries[exercise].sets[set].planned;
    const next = rest ? { seconds: rest, notice: notices.rest } : undefined;
    timer.start(
      { kind: "work", exercise, set },
      countUp ? undefined : work,
      notices.work,
      from,
      next,
    );
    sound("start");
  };

  // Hoisted so the timer callback above may call them; hoisting loses the record narrowing.
  // auto: the next pending timed set of the exercise starts by itself.
  function chain(exercise: number, from?: number) {
    const sets = record!.entries[exercise].sets;
    const next = sets.findIndex((s) => !s.completed);
    if (next >= 0 && sets[next].planned.work) startSet(exercise, next, from);
  }

  // Completing moves the active set on, so an open editor closes with it.
  function complete(exercise: number, set: number, workTime?: number, from?: number) {
    const { planned } = record!.entries[exercise].sets[set];
    setEditing(undefined);
    apply((r) => mapExercise(r, exercise, (e) => completeSet(e, set, workTime)));
    if (planned.rest)
      timer.start({ kind: "rest", exercise, set }, planned.rest, notices.rest, from);
    else if (workTime && planned.auto) chain(exercise, from);
  }

  const isRunning = (exercise: number, set: number) =>
    timer.running?.kind === "work" &&
    timer.running.exercise === exercise &&
    timer.running.set === set;

  const stopSet = (exercise: number, set: number) => complete(exercise, set, timer.stop());

  // The new name's flag arrives later and lands only if the entry still sits where it was.
  const switchExercise = (i: number, name: string) => {
    apply((r) => mapExercise(r, i, (e) => switchAlternate(e, name)));
    fetchUnilateral(api, store, { name }).then(
      (unilateral) =>
        apply((r) => mapExercise(r, i, (e) => (e.name === name ? { ...e, unilateral } : e))),
      () => {},
    );
  };

  const done = (exercise: number, set: number) => {
    const s = record.entries[exercise].sets[set];
    if (s.completed) {
      setEditing(undefined);
      return apply((r) => mapExercise(r, exercise, (e) => tapDone(e, set)));
    }
    // A set that asks for its weight or RPE is filled in before it can be ticked.
    if (!s.edited && (s.planned.ask || s.planned.logRpe)) return setEditing({ exercise, set });
    if (isRunning(exercise, set)) return stopSet(exercise, set);
    if (s.planned.work) return startSet(exercise, set);
    complete(exercise, set);
  };

  const finishWorkout = () => {
    timer.stop();
    update((r) => finish(r, new Date().toISOString()));
    // Back to the screen that opened it (Today, or a past day in the Calendar tab); a deep link
    // with nothing beneath it lands on Today.
    if (router.canGoBack()) router.back();
    else router.replace("/");
  };

  const running = timer.running;
  const runningSet = running && record.entries[running.exercise].sets[running.set];
  // Numbered like the rows: within warmups or within work sets.
  const setNumber = ({ exercise, set }: Target) => {
    const sets = record.entries[exercise].sets;
    const warmup = sets[set].planned.warmup;
    return sets.slice(0, set).filter((s) => s.planned.warmup === warmup).length + 1;
  };

  const toggleEditing = (exercise: number, set: number) =>
    setEditing((t) => (t?.exercise === exercise && t.set === set ? undefined : { exercise, set }));

  return (
    <View style={{ flex: 1 }}>
      <Screen
        title={record.dayName}
        testID="workout-screen"
        left={<BackButton />}
        refresh={refresh}
        right={<SyncBadge />}
      >
        <ExerciseCarousel
          entries={record.entries}
          shown={shown}
          onShow={show}
          onList={() => setListing(true)}
        />
        <SessionActions
          onAdd={() => setAdding(true)}
          onReorder={() => setListing(true)}
          onFinish={finished ? undefined : finishWorkout}
        />
        <View style={{ flexDirection: "row", alignItems: "center", gap: space[2] }}>
          <Text variant="caption" numberOfLines={1} style={{ flex: 1 }}>
            {`${onTarget} of ${count} exercises on target`}
          </Text>
          {record.modified && <Badge label="Modified" icon="pencil" testID="workout-modified" />}
        </View>
        {finished && (
          <Text variant="caption" color="textMuted">
            {`Finished${record.finishedAt ? ` ${formatDateTime(new Date(record.finishedAt))}` : ""}. Edits update the program's next sessions.`}
          </Text>
        )}
        {countsForProgram && (
          <Text variant="caption" color="textMuted">
            Finishing this past workout counts for the program.
          </Text>
        )}
        {entry && (
          <GestureDetector gesture={pan}>
            {/* A tap on the pane outside any control closes the inline editor. */}
            <Pressable
              accessible={false}
              testID="exercise-pane"
              onPress={() => setEditing(undefined)}
            >
              <ExercisePane
                key={shown}
                entry={entry}
                index={shown}
                count={count}
                unit={unit}
                finished={finished}
                active={active?.exercise === shown ? active.set : undefined}
                running={
                  running?.kind === "work" && running.exercise === shown ? running.set : undefined
                }
                editing={editing?.exercise === shown ? editing.set : undefined}
                onDone={(set) => done(shown, set)}
                onEdit={(set) => toggleEditing(shown, set)}
                onRemoveSet={(set) => {
                  setEditing(undefined);
                  restructure((r) => markModified(removeSet(r, shown, set)));
                }}
                onSetValues={(set, values) =>
                  apply((r) => mapExercise(r, shown, (e) => setValues(e, set, values)))
                }
                onChange={(edit) => apply((r) => mapExercise(r, shown, edit))}
                onSwitch={(name) => switchExercise(shown, name)}
                onMove={(to) => move(shown, to)}
                onRemove={() => setRemoving(shown)}
                onNext={shown === count - 1 ? finishWorkout : () => show(shown + 1)}
              />
            </Pressable>
          </GestureDetector>
        )}
      </Screen>
      {running?.kind === "rest" && (
        <TimerBar
          icon="timer-outline"
          title="Rest"
          value={clock(timer.remaining)}
          action="Skip"
          onAction={timer.stop}
          onAdjust={timer.adjust}
          testID="rest-timer"
        />
      )}
      {running?.kind === "work" && runningSet && (
        <TimerBar
          icon="stopwatch-outline"
          title={record.entries[running.exercise].name}
          detail={`${runningSet.planned.warmup ? "Warmup" : "Set"} ${setNumber(running)}`}
          value={
            runningSet.planned.countUp
              ? `${clock(timer.elapsed)} / ${clock(runningSet.planned.work)}`
              : clock(timer.remaining)
          }
          action="Stop"
          onAction={() => stopSet(running.exercise, running.set)}
          testID="set-timer"
        />
      )}
      <ExercisesSheet
        open={listing}
        entries={record.entries}
        onMove={move}
        onRemove={(i) => {
          setListing(false);
          setRemoving(i);
        }}
        onClose={() => setListing(false)}
      />
      <ConfirmSheet
        open={removing !== undefined}
        title="Remove from today"
        message={`${removing !== undefined ? record.entries[removing]?.name : ""} and its sets leave today's workout. The program keeps it.`}
        action="Remove exercise"
        onConfirm={() => {
          if (removing !== undefined) {
            restructure((r) => markModified(removeExercise(r, removing)));
            if (removing < shown) setShown(shown - 1);
          }
          setEditing(undefined);
          setRemoving(undefined);
        }}
        onClose={() => setRemoving(undefined)}
      />
      <AddExerciseSheet
        open={adding}
        recents={recentExercises(
          store.list("workoutRecords", {}),
          record.entries.map((e) => e.name),
        )}
        onPick={async (picked) => {
          setAdding(false);
          const unilateral = await fetchUnilateral(api, store, picked).catch(() => false);
          // Right after the shown exercise. A timer on a later exercise addresses it by position,
          // which the insert moves, so that one stops.
          const at = shown + 1;
          const edit = (r: WorkoutRecord) => addExercise(r, { ...picked, unilateral }, at);
          if (timer.running && timer.running.exercise >= at) restructure(edit);
          else update(edit);
          setShown(at);
        }}
        onClose={() => setAdding(false)}
      />
    </View>
  );
}
