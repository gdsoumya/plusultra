import { uuid } from "../../data/uuid";
import { useRouter } from "expo-router";
import { useReducer, useState } from "react";

import { useApi } from "../../api/useApi";
import { checkinsRepo } from "../../data/repositories/checkins";
import { dayLogsRepo } from "../../data/repositories/dayLogs";
import { mealLogsRepo } from "../../data/repositories/mealLogs";
import { programsRepo } from "../../data/repositories/programs";
import { workoutRecordsRepo } from "../../data/repositories/workoutRecords";
import { useStore } from "../../data/StoreProvider";
import { useRefresh } from "../../data/useRefresh";
import { useSync } from "../../data/useSync";
import { today as todayLocal } from "../../domain/dates";
import { adHocWorkout, completeAsPlanned, workoutFromDay } from "../../domain/quickfill";
import { dayTotals, enableAll, mealPlanOf, mealQuickFill, newMealLog } from "../../domain/totals";
import type { MealDay, ResolvedDay, Unit, Weight } from "../../domain/types";
import { Card, EmptyState, Screen } from "../../ui";
import { resetItem, type MenuItem } from "../calendar/CardMenu";
import { mealPicker, workoutPicker } from "../calendar/pickers";
import { ProgramDayPicker } from "../calendar/ProgramDayPicker";
import { ResetSheet, type Content } from "../calendar/ResetSheet";
import { SyncBadge } from "../sync/SyncBadge";
import { CheckinCard } from "./CheckinCard";
import { MealsCard } from "./MealsCard";
import { readToday } from "./readToday";
import { WorkoutCard } from "./WorkoutCard";

export type TodayScreenProps = {
  today?: string;
  unit?: Unit;
  userId?: string;
};

export default function TodayScreen({
  today = todayLocal(),
  unit = "kg",
  userId = "",
}: TodayScreenProps) {
  const store = useStore();
  const api = useApi();
  const router = useRouter();
  // Local writes re-read the store; a finished sync changes the sync context, which re-renders.
  const [, changed] = useReducer((n: number) => n + 1, 0);
  useSync();
  const refresh = useRefresh();
  const data = readToday(store, today);
  const { program, plan, mealProgramId, mealPlan } = data;
  const planned = plan && plan.kind !== "rest" && plan.exercises.length > 0;
  const [resetting, setResetting] = useState<Content>();
  const [picking, setPicking] = useState<Content>();

  const newRow = () => ({ id: uuid(), date: today, userId });
  const now = () => new Date().toISOString();

  function openWorkout() {
    let record = data.record;
    if (!record && planned && program) {
      record = workoutRecordsRepo(store).save(workoutFromDay(plan, program.id, newRow(), now()));
      changed();
    }
    if (record) router.push(`/workout/${record.id}`);
  }

  function doneAsPlanned() {
    if (!planned || !program) return;
    const at = now();
    store.transaction(() => {
      const record = data.record ?? workoutFromDay(plan, program.id, newRow(), at);
      workoutRecordsRepo(store).save(completeAsPlanned(record, at));
      dayLogsRepo(store).save({
        ...(data.dayLog ?? { ...newRow(), version: 0, notes: "" }),
        status: "done",
      });
    });
    changed();
  }

  function logMeals() {
    if (!mealPlan) return;
    const log = data.mealLog;
    const meals = log && enableAll(log.meals, mealPlan);
    mealLogsRepo(store).save(
      log && meals
        ? { ...log, meals, totals: dayTotals(meals) }
        : mealQuickFill(mealPlan, { ...newRow(), programId: mealProgramId }),
    );
    changed();
  }

  // The overrides (P2-R37): any program's day or an ad hoc record in place of the planned one.
  function addWorkout(from?: { programId: string; day: ResolvedDay }) {
    const row = newRow();
    workoutRecordsRepo(store).save(
      from ? workoutFromDay(from.day, from.programId, row, now()) : adHocWorkout(row, now()),
    );
    changed();
    router.push(`/workout/${row.id}`);
  }

  function addMeals(from?: { programId: string; day: MealDay }) {
    mealLogsRepo(store).save(newMealLog({ ...newRow(), programId: from?.programId }, from?.day));
    changed();
    router.push(`/meals/${today}`);
  }

  const programs = programsRepo(store).all();
  const pickers = {
    workout: workoutPicker(
      api,
      store,
      unit,
      programs,
      plan && program && { programId: program.id, payload: plan },
      (programId, day) => addWorkout({ programId, day }),
    ),
    meals: mealPicker(
      api,
      store,
      programs,
      mealPlan && mealProgramId ? { programId: mealProgramId, payload: mealPlan } : undefined,
      (programId, day) => addMeals({ programId, day }),
    ),
  };
  const overrides = (what: Content, adHoc: string, onAdHoc: () => void): MenuItem[] => [
    { title: adHoc, icon: "add", onPress: onAdHoc },
    { title: "From a program", icon: "list-outline", onPress: () => setPicking(what) },
  ];

  // Removes today's record or log as a tombstone the sync pushes.
  function reset() {
    if (resetting === "workout" && data.record) workoutRecordsRepo(store).remove(data.record.id);
    if (resetting === "meals" && data.mealLog) mealLogsRepo(store).remove(data.mealLog.id);
    setResetting(undefined);
    changed();
  }

  function saveCheckin(weight: Weight) {
    const todays = data.checkins.find((c) => c.date === today);
    checkinsRepo(store).save({
      ...(todays ?? { ...newRow(), version: 0, measurements: {} }),
      weight,
    });
    changed();
  }

  return (
    <Screen title="Today" testID="today-screen" right={<SyncBadge />} refresh={refresh}>
      {!program && !data.record ? (
        <Card>
          <EmptyState
            icon="barbell-outline"
            title="No program"
            message="Pick a program to see your plan here."
            action={{ title: "Browse programs", onPress: () => router.push("/programs") }}
          />
        </Card>
      ) : (
        <WorkoutCard
          program={program}
          plan={plan}
          record={data.record}
          dayLog={data.dayLog}
          unit={unit}
          onOpen={openWorkout}
          onDone={doneAsPlanned}
          menu={
            data.record
              ? [resetItem(() => setResetting("workout"))]
              : overrides("workout", "Ad hoc workout", () => addWorkout())
          }
        />
      )}
      <MealsCard
        plan={mealPlanOf(data.mealLog, mealProgramId, mealPlan)}
        log={data.mealLog}
        onLog={logMeals}
        onOpen={() => router.push(`/meals/${today}`)}
        menu={
          data.mealLog
            ? [resetItem(() => setResetting("meals"))]
            : overrides("meals", "Ad hoc meals", () => addMeals())
        }
      />
      <CheckinCard checkins={data.checkins} today={today} unit={unit} onSave={saveCheckin} />
      <ResetSheet
        what={resetting}
        finalized={data.record?.finalized}
        onConfirm={reset}
        onClose={() => setResetting(undefined)}
      />
      <ProgramDayPicker
        open={!!picking}
        onClose={() => setPicking(undefined)}
        {...pickers[picking ?? "workout"]}
      />
    </Screen>
  );
}
