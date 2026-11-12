import { uuid } from "../../data/uuid";
import { useFocusEffect, useRouter } from "expo-router";
import { useEffect, useReducer, useState } from "react";

import { useApi } from "../../api/useApi";
import { fetchPastPlan, plannedDay, plannedMealDay } from "../../data/plans";
import { checkinsRepo } from "../../data/repositories/checkins";
import { dayLogsRepo } from "../../data/repositories/dayLogs";
import { mealLogsRepo } from "../../data/repositories/mealLogs";
import { programsRepo } from "../../data/repositories/programs";
import { workoutRecordsRepo } from "../../data/repositories/workoutRecords";
import { useStore } from "../../data/StoreProvider";
import { useRefresh } from "../../data/useRefresh";
import { dateLabel } from "../../domain/dates";
import { adHocWorkout, workoutFromDay } from "../../domain/quickfill";
import { newMealLog } from "../../domain/totals";
import type { DayStatus, MealDay, ResolvedDay, Unit, Weight } from "../../domain/types";
import { BackButton, EmptyState, Screen } from "../../ui";
import { SyncBadge } from "../sync/SyncBadge";
import { CheckinCard as WeightCard } from "../today/CheckinCard";
import { AddSheet } from "./AddSheet";
import { resetItem } from "./CardMenu";
import {
  AddWorkout,
  CheckinCard,
  MealsCard,
  PlanCard,
  StatusEditor,
  WorkoutCard,
} from "./DayCards";
import { mealPicker, workoutPicker } from "./pickers";
import { ProgramDayPicker } from "./ProgramDayPicker";
import { ResetSheet, type Content } from "./ResetSheet";

type Props = { date: string; today: string; unit: Unit; userId: string };

export default function DayScreen({ date, today, unit, userId }: Props) {
  const store = useStore();
  const router = useRouter();
  // Local writes re-read the store; so does regaining focus after the workout or meals screen.
  const [, changed] = useReducer((n: number) => n + 1, 0);
  useFocusEffect(changed);
  const api = useApi();
  useEffect(() => {
    // Offline the cached plan, if any, stands.
    if (date < today) fetchPastPlan(api, store, date).then(changed, () => {});
  }, [api, store, date, today]);
  const logs = dayLogsRepo(store);
  const log = logs.byDate(date)[0];
  const refresh = useRefresh(changed);
  const record = workoutRecordsRepo(store).byDate(date)[0];
  const meal = mealLogsRepo(store).byDate(date)[0];
  const checkin = checkinsRepo(store).byDate(date)[0];
  const planRow = plannedDay(store, date);
  const plan = planRow?.payload;
  const mealPlanRow = plannedMealDay(store, date);
  const mealPlan = mealPlanRow?.payload;
  const editable = date <= today;
  const [adding, setAdding] = useState<Content>();
  const [picking, setPicking] = useState<Content>();
  const [resetting, setResetting] = useState<Content>();
  const empty = !editable && !record && !meal && !checkin && !plan && !mealPlan;
  const newRow = () => ({ id: uuid(), date, userId });
  const newLog = { ...newRow(), version: 0, status: "planned" as DayStatus, notes: "" };

  const setStatus = (status: DayStatus) => {
    logs.save({ ...(log ?? newLog), status });
    changed();
  };
  // Adding content to a skipped day un-skips it; the pushed record or log then says what happened.
  const unskip = () => log?.status === "skipped" && logs.save({ ...log, status: "planned" });

  function addWorkout(from?: { programId: string; day: ResolvedDay }) {
    const row = newRow();
    const now = new Date().toISOString();
    store.transaction(() => {
      workoutRecordsRepo(store).save(
        from ? workoutFromDay(from.day, from.programId, row, now) : adHocWorkout(row, now),
      );
      unskip();
    });
    changed();
    router.push(`/day/${date}/workout/${row.id}`);
  }

  function addMeals(from?: { programId: string; day: MealDay }) {
    store.transaction(() => {
      mealLogsRepo(store).save(newMealLog({ ...newRow(), programId: from?.programId }, from?.day));
      unskip();
    });
    changed();
    router.push(`/day/${date}/meals`);
  }

  const programs = programsRepo(store).all();
  const pickers = {
    workout: workoutPicker(api, store, unit, programs, planRow, (programId, day) =>
      addWorkout({ programId, day }),
    ),
    meals: mealPicker(api, store, programs, mealPlanRow, (programId, day) =>
      addMeals({ programId, day }),
    ),
  };

  // Removes the date's record or log as a tombstone the sync pushes.
  function reset() {
    if (resetting === "workout" && record) workoutRecordsRepo(store).remove(record.id);
    if (resetting === "meals" && meal) mealLogsRepo(store).remove(meal.id);
    setResetting(undefined);
    changed();
  }

  function saveCheckin(weight: Weight) {
    checkinsRepo(store).save({
      ...(checkin ?? { ...newRow(), version: 0, measurements: {} }),
      weight,
    });
    changed();
  }

  return (
    <Screen
      title={dateLabel(date)}
      left={<BackButton fallback="/calendar" />}
      right={<SyncBadge />}
      testID="day-screen"
      refresh={refresh}
    >
      {record ? (
        <WorkoutCard
          record={record}
          onPress={() => router.push(`/day/${date}/workout/${record.id}`)}
          menu={[resetItem(() => setResetting("workout"))]}
        />
      ) : (
        <>
          {plan && <PlanCard plan={plan} unit={unit} />}
          {editable && <AddWorkout onAdd={() => setAdding("workout")} />}
        </>
      )}
      <MealsCard
        log={meal}
        plan={mealPlan}
        onPress={() => router.push(`/day/${date}/meals`)}
        onAdd={editable && !meal ? () => setAdding("meals") : undefined}
        menu={[resetItem(() => setResetting("meals"))]}
      />
      {editable ? (
        <WeightCard
          checkins={store.list("checkins", { dateTo: date })}
          today={today}
          unit={unit}
          onSave={saveCheckin}
        />
      ) : (
        checkin && <CheckinCard checkin={checkin} unit={unit} />
      )}
      {empty && <EmptyState icon="calendar-outline" title="Nothing on this day" />}
      {!record && editable && <StatusEditor status={log?.status} onChange={setStatus} />}
      <AddSheet
        open={!!adding}
        onClose={() => setAdding(undefined)}
        title={adding === "meals" ? "Add meals" : "Add workout"}
        adHoc={adding === "meals" ? "Ad hoc meals" : "Ad hoc workout"}
        onAdHoc={() => {
          setAdding(undefined);
          if (adding === "meals") addMeals();
          else addWorkout();
        }}
        onProgram={() => {
          setPicking(adding);
          setAdding(undefined);
        }}
      />
      <ProgramDayPicker
        open={!!picking}
        onClose={() => setPicking(undefined)}
        {...pickers[picking ?? "workout"]}
      />
      <ResetSheet
        what={resetting}
        finalized={record?.finalized}
        onConfirm={reset}
        onClose={() => setResetting(undefined)}
      />
    </Screen>
  );
}
