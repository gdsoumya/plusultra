import { entries } from "../../domain/quickfill";
import type { DayKind, DayStatus, Item } from "../../domain/types";
import type { ServerRows } from "../convert";
import type { PlannedDay, PlannedMealDay } from "../mock/db";
import { kg } from "./build";
import { addDays, at, datesIn, daysBetween, weekday } from "./dates";
import { scale } from "./macros";
import { item, totals } from "./meals";
import { prng } from "./prng";

export interface Plans {
  // First date of the anchored program; before it the user ran the floating program.
  start: string;
  anchored(date: string): PlannedDay | undefined;
  floating: PlannedDay[];
  meal(date: string, workoutKind?: DayKind): PlannedMealDay | undefined;
}

const notes = [
  "Slept badly, kept it light.",
  "Felt strong today.",
  "Knee a bit sore on the lunges.",
  "Busy day, trained late.",
  "Gym was crowded, swapped a few exercises.",
];

const extras = (): Item[] => [
  item("Peanuts, raw", 30, "g", false),
  item("Apple, raw", 1, "apple", false),
  item("Bread, whole wheat", 2, "slice", false),
];

const round1 = (v: number) => Math.round(v * 10) / 10;

// The 30 days before today, plus today's started workout and breakfast.
export function history(today: string, userId: string, plans: Plans): ServerRows {
  const rand = prng(3);
  const pick = <T>(all: T[]) => all[Math.floor(rand() * all.length)];
  const rows: ServerRows = { dayLogs: [], workoutRecords: [], mealLogs: [], checkins: [] };
  const first = addDays(today, -30);
  let floating = 0;

  for (const date of datesIn(first, today)) {
    const isToday = date === today;
    const meta = (prefix: string) => ({
      id: `${prefix}-${date}`,
      userId,
      version: 1,
      deviceId: "phone-1",
      updatedAt: at(date, isToday ? "08:00" : "21:00"),
    });
    // Before the anchored start the floating program ran on Mondays, Wednesdays and Fridays.
    const plan =
      date >= plans.start
        ? plans.anchored(date)
        : [1, 3, 5].includes(weekday(date))
          ? plans.floating[floating++]
          : undefined;
    const roll = rand();
    const status: DayStatus = !plan?.exercises.length
      ? "rest"
      : roll < 0.1
        ? "skipped"
        : roll < 0.22
          ? "partial"
          : "done";

    if (!isToday) {
      rows.dayLogs.push({
        ...meta("day"),
        date,
        status,
        notes: rand() < 0.2 ? pick(notes) : "",
        readiness: {
          sleep: 2 + Math.floor(rand() * 4),
          soreness: 2 + Math.floor(rand() * 4),
          stress: 2 + Math.floor(rand() * 4),
        },
      });
    }

    if (plan && (isToday || status === "done" || status === "partial")) {
      const half = Math.ceil(plan.exercises.length / 2);
      const done = (i: number) => (isToday ? i === 0 : status === "done" || i < half);
      // AMRAP sets go past the planned reps.
      const workout = entries(plan, done).map((e, i) => ({
        ...e,
        sets: e.sets.map((s, j) =>
          s.completed && plan.exercises[i].sets[j].amrap
            ? { ...s, reps: s.reps + Math.floor(rand() * 4) }
            : s,
        ),
      }));
      rows.workoutRecords.push({
        ...meta("workout"),
        date,
        programId: plan.programId,
        week: plan.week,
        day: plan.day,
        dayName: plan.name,
        dayDesc: plan.desc,
        kind: plan.kind,
        entries: workout,
        startedAt: at(date, "07:00"),
        finishedAt: isToday ? undefined : at(date, "08:10"),
        finalized: !isToday,
        finalizeRequested: false,
      });
    }

    const mealPlan = plans.meal(date, plan?.kind);
    if (mealPlan) {
      const meals = mealPlan.meals
        .filter((_, i) => !isToday || i === 0)
        .map((m) => ({
          ...m,
          items: [
            ...m.items
              .filter(() => rand() > 0.08)
              .map((it) => {
                if (rand() > 0.2) return it;
                const qty = Math.max(1, Math.round(it.qty * (0.8 + rand() * 0.4)));
                return { ...it, qty, macros: scale(it.macros, qty / it.qty) };
              }),
            ...(rand() < 0.12 ? [pick(extras())] : []),
          ],
        }));
      rows.mealLogs.push({
        ...meta("meal"),
        date,
        programId: mealPlan.programId,
        target: mealPlan.target,
        meals,
        totals: totals(meals),
      });
    }

    // Weighed most mornings, trending down about 0.4 kg a week; waist and body fat on Mondays.
    if (!isToday && rand() < 0.85) {
      const n = daysBetween(first, date);
      const monday = weekday(date) === 1;
      rows.checkins.push({
        ...meta("checkin"),
        date,
        weight: kg(round1(84.6 - 0.06 * n + (rand() - 0.5) * 0.6)),
        bodyFat: monday ? round1(21 - 0.04 * n) : undefined,
        measurements: monday ? { waist: round1(88 - 0.05 * n) } : {},
      });
    }
  }
  return rows;
}
