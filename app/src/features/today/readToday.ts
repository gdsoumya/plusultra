import { activeProgram, plannedMealDay, plannedWorkout } from "../../data/plans";
import { dayLogsRepo } from "../../data/repositories/dayLogs";
import { mealLogsRepo } from "../../data/repositories/mealLogs";
import { workoutRecordsRepo } from "../../data/repositories/workoutRecords";
import type { Store } from "../../data/store";

// Everything Today shows for a date, read from the store: the active programs' days for it and
// what was logged.
export function readToday(store: Store, today: string) {
  const mealDay = plannedMealDay(store, today);
  return {
    program: activeProgram(store, "workout"),
    plan: plannedWorkout(store, today)?.day,
    mealProgramId: mealDay?.programId,
    mealPlan: mealDay?.payload,
    dayLog: dayLogsRepo(store).byDate(today)[0],
    record: workoutRecordsRepo(store).byDate(today)[0],
    mealLog: mealLogsRepo(store).byDate(today)[0],
    checkins: store.list("checkins", { dateTo: today }),
  };
}
