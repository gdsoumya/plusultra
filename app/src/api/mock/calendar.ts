import type { ServiceImpl } from "@connectrpc/connect";

import { fill } from "../../domain/quickfill";
import type { DayStatus as Status } from "../../domain/types";
import { CalendarService } from "../../gen/plusultra/v1/calendar_pb";
import { fromProto, toProto } from "../convert";
import { datesIn } from "../fixtures/dates";
import { totals } from "../fixtures/meals";
import {
  newId,
  onDate,
  precondition,
  required,
  save,
  type MockDb,
  type PlannedDay,
  type PlannedMealDay,
} from "./db";
import { finalize, mealPlan, plannedOrDone, recordDay, startRecord, workoutPlan } from "./plans";

// Photos never travel with a day.
export function dayView(db: MockDb, date: string) {
  const workout = onDate(db, "workoutRecords", date);
  const plan = workoutPlan(db, date) ?? (workout && recordDay(db, workout));
  const mealDay = mealPlan(db, date);
  const log = onDate(db, "dayLogs", date);
  const meal = onDate(db, "mealLogs", date);
  const checkin = onDate(db, "checkins", date);
  return {
    date,
    log: log && toProto.dayLog(log),
    plan: plan && toProto.resolvedDay(plan, plan.programId),
    mealPlan: mealDay && toProto.mealDay(mealDay, mealDay.programId, date),
    workout: workout && toProto.workoutRecord(workout),
    meal: meal && toProto.mealLog(meal),
    checkin: checkin && toProto.checkin(checkin),
  };
}

function setStatus(db: MockDb, date: string, status: Status) {
  const log = onDate(db, "dayLogs", date);
  save(db, "dayLogs", { notes: "", ...log, id: log?.id ?? newId(db, "day"), date, status });
}

function fillWorkout(db: MockDb, date: string, plan: PlannedDay, andFinalize: boolean) {
  const started =
    onDate(db, "workoutRecords", date) ?? startRecord(newId(db, "workout"), plan, date);
  const filled = { ...started, entries: fill(started.entries) };
  save(db, "workoutRecords", andFinalize && !filled.finalized ? finalize(db, filled) : filled);
}

function fillMeals(db: MockDb, date: string, plan: PlannedMealDay) {
  const log = onDate(db, "mealLogs", date);
  // Planned items replace what was logged from the plan; items added by hand stay.
  const meals = plan.meals.map((m) => ({
    ...m,
    items: [
      ...m.items,
      ...(log?.meals.find((l) => l.name === m.name)?.items.filter((i) => !i.planned) ?? []),
    ],
  }));
  const id = log?.id ?? newId(db, "meal");
  save(db, "mealLogs", {
    id,
    date,
    programId: plan.programId,
    target: plan.target,
    meals,
    totals: totals(meals),
  });
}

export function calendar(db: MockDb): ServiceImpl<typeof CalendarService> {
  return {
    getCalendar({ dates }) {
      const { from, to } = required(dates, "dates");
      return {
        days: [...datesIn(from, to)].map((date) => {
          const day = plannedOrDone(db, date);
          const log = onDate(db, "dayLogs", date);
          return {
            date,
            kind: day?.kind ?? "rest",
            workoutWeek: day?.week,
            workoutDay: day?.day,
            workoutName: day?.name,
            mealKind: mealPlan(db, date)?.kind,
            logStatus: log && toProto.dayStatus(log.status),
          };
        }),
      };
    },
    getDay: ({ date }) => ({ day: dayView(db, date) }),
    updateDay({ date, status, notes, readiness }) {
      const log = onDate(db, "dayLogs", date);
      const next = status ? fromProto.dayStatus(status) : (log?.status ?? "planned");
      const saved = save(db, "dayLogs", {
        id: log?.id ?? newId(db, "day"),
        date,
        status: next,
        notes,
        readiness: readiness && fromProto.readiness(readiness),
      });
      return { log: toProto.dayLog(saved) };
    },
    quickFillDay({ date, workout, meals, finalize: andFinalize }) {
      const plan = plannedOrDone(db, date);
      const menu = mealPlan(db, date);
      if (workout && !plan?.exercises.length)
        throw precondition(`no workout is planned on ${date}`);
      if (meals && !menu) throw precondition(`no meals are planned on ${date}`);
      if (workout && plan) {
        fillWorkout(db, date, plan, andFinalize);
        setStatus(db, date, "done");
      }
      if (meals && menu) fillMeals(db, date, menu);
      return { day: dayView(db, date) };
    },
  };
}
