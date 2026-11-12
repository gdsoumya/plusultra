import type { ServiceImpl } from "@connectrpc/connect";

import { groupOf } from "../../domain/muscles";
import type { ExerciseEntry } from "../../domain/types";
import { InsightService } from "../../gen/plusultra/v1/insight_pb";
import { toProto } from "../convert";
import { kg } from "../fixtures/build";
import { addDays, datesIn, daysBetween } from "../fixtures/dates";
import { average } from "../fixtures/macros";
import { inDates, live, notFound, onDate, required, type MockDb, type Range } from "./db";
import { workoutPlan } from "./plans";

const round = (v: number, places = 1) => Math.round(v * 10 ** places) / 10 ** places;
const weight = (v: number) => toProto.weight(kg(round(v)));

// Epley; a single is its own 1RM.
const e1rm = (w: number, reps: number) => (reps <= 1 ? w : w * (1 + reps / 30));

function bestSet(entry: ExerciseEntry) {
  return entry.sets
    .filter((s) => s.completed && s.reps > 0 && s.weight.value > 0)
    .map((s) => ({ weight: s.weight.value, reps: s.reps, e1rm: e1rm(s.weight.value, s.reps) }))
    .sort((a, b) => b.e1rm - a.e1rm)[0];
}

export function insights(db: MockDb): ServiceImpl<typeof InsightService> {
  const weighIns = () => live(db, "checkins").filter((c) => c.weight);
  // Mean of the weigh-ins in the 7 days ending on date.
  const movingAverage = (date: string) => {
    const week = weighIns().filter((c) => c.date > addDays(date, -7) && c.date <= date);
    return week.reduce((t, c) => t + c.weight!.value, 0) / week.length;
  };
  const slugOf = (name: string) => db.exercises.find((e) => e.name === name)?.slug ?? "";

  function adherence(dates?: Range) {
    const { from, to } = required(dates, "dates");
    const counts = { planned: 0, done: 0, partial: 0, skipped: 0 };
    for (const date of datesIn(from, to)) {
      const status = onDate(db, "dayLogs", date)?.status;
      const planned = workoutPlan(db, date)?.exercises.length || (status && status !== "rest");
      if (!planned) continue;
      counts.planned++;
      if (status === "done" || status === "partial" || status === "skipped") counts[status]++;
    }
    return counts;
  }

  return {
    weeklySummary({ weekStart }) {
      const dates = { from: weekStart, to: addDays(weekStart, 6) };
      const setsPerMuscle: Record<string, number> = {};
      const setsPerRegion: Record<string, number> = {};
      const best = new Map<string, number>();
      for (const record of inDates(db, "workoutRecords", dates)) {
        for (const entry of record.entries) {
          const muscles = db.exercises.find((e) => e.name === entry.name)?.primaryMuscles ?? [];
          const working = entry.sets.filter((s) => s.completed && !s.planned.warmup).length;
          for (const g of new Set(muscles.map(groupOf))) {
            setsPerMuscle[g] = (setsPerMuscle[g] ?? 0) + working;
          }
          for (const m of muscles.filter((m) => groupOf(m) !== m)) {
            setsPerRegion[m] = (setsPerRegion[m] ?? 0) + working;
          }
          const top = bestSet(entry);
          if (top && top.e1rm > (best.get(entry.name) ?? 0)) best.set(entry.name, top.e1rm);
        }
      }
      const meals = inDates(db, "mealLogs", dates);
      const weights = inDates(db, "checkins", dates).filter((c) => c.weight);
      const last = weights.at(-1);
      return {
        weekStart,
        adherence: adherence(dates),
        setsPerMuscle,
        setsPerRegion,
        e1rm: [...best].map(([name, v]) => ({ slug: slugOf(name), name, e1rm: weight(v) })),
        macros: {
          target: average(meals.flatMap((m) => (m.target ? [m.target] : []))),
          actual: average(meals.map((m) => m.totals)),
        },
        weight: last && {
          start: toProto.weight(weights[0].weight!),
          end: toProto.weight(last.weight!),
          average: weight(movingAverage(last.date)),
        },
      };
    },
    liftHistory({ slug, dates }) {
      const exercise = db.exercises.find((e) => e.slug === slug);
      if (!exercise) throw notFound(`exercise ${slug}`);
      const records = dates ? inDates(db, "workoutRecords", dates) : live(db, "workoutRecords");
      const byDate = new Map<string, NonNullable<ReturnType<typeof bestSet>>>();
      for (const r of records) {
        for (const entry of r.entries.filter((e) => e.name === exercise.name)) {
          const top = bestSet(entry);
          if (top && top.e1rm > (byDate.get(r.date)?.e1rm ?? 0)) byDate.set(r.date, top);
        }
      }
      return {
        slug,
        name: exercise.name,
        points: [...byDate].map(([date, top]) => ({
          date,
          e1rm: weight(top.e1rm),
          bestWeight: weight(top.weight),
          bestReps: top.reps,
        })),
      };
    },
    weightTrend({ dates }) {
      const { from, to } = required(dates, "dates");
      const points = weighIns()
        .filter((c) => c.date >= from && c.date <= to)
        .map((c) => ({ date: c.date, weight: c.weight!.value, average: movingAverage(c.date) }));
      const [first, last] = [points[0], points.at(-1)];
      const span = first && last ? daysBetween(first.date, last.date) : 0;
      const rate = span ? ((last!.average - first!.average) / span) * 7 : 0;
      return {
        points: points.map((p) => ({
          date: p.date,
          weight: weight(p.weight),
          movingAverage: weight(p.average),
        })),
        weeklyRate: toProto.weight(kg(round(rate, 2))),
      };
    },
    adherence: ({ dates }) => ({ counts: adherence(dates) }),
  };
}
