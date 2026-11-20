import type { Api } from "../../api/clients";
import type { Store } from "../../data/store";
import { setsSummary } from "../../domain/sets";
import type { MealDay, ProgramSummary, ResolvedDay, Unit } from "../../domain/types";
import { cachedDays, cachedMealDays } from "../programs/programStore";
import { kcal } from "./format";
import { mealDays, workoutDays } from "./programDays";
import type { Planned } from "./ProgramDayPicker";

type Row = { programId: string; payload: Omit<Planned, "programId"> };
const asPlanned = (row?: Row): Planned | undefined =>
  row && { programId: row.programId, ...row.payload };

// Draft and active programs; finished ones belong to history.
const offered = (programs: ProgramSummary[], kind: ProgramSummary["kind"]) =>
  programs.filter((p) => p.kind === kind && (p.status === "draft" || p.status === "active"));
const finishedOnly = (programs: ProgramSummary[], kind: ProgramSummary["kind"]) =>
  offered(programs, kind).length === 0 && programs.some((p) => p.kind === kind);

// What the day picker offers for workouts, each day summarised by its exercises; create builds
// the record from the chosen day.
export const workoutPicker = (
  api: Api,
  store: Store,
  unit: Unit,
  programs: ProgramSummary[],
  planned: Row | undefined,
  create: (programId: string, day: ResolvedDay) => void,
) => {
  const pick = (p: ProgramSummary) => (d: ResolvedDay) => ({
    ...d,
    summary: d.exercises.map((e) => `${e.name} ${setsSummary(e.sets, unit)}`).join(" · "),
    create: () => create(p.id, d),
  });
  return {
    title: "Workout from a program",
    programs: offered(programs, "workout"),
    finishedOnly: finishedOnly(programs, "workout"),
    planned: asPlanned(planned),
    cached: (p: ProgramSummary) => cachedDays(store, p.id).map(pick(p)),
    loadDays: (p: ProgramSummary) => workoutDays(api, store, p).then((days) => days.map(pick(p))),
  };
};

export const mealPicker = (
  api: Api,
  store: Store,
  programs: ProgramSummary[],
  planned: Row | undefined,
  create: (programId: string, day: MealDay) => void,
) => {
  const pick = (p: ProgramSummary) => (d: MealDay) => ({
    ...d,
    summary: [d.target && `Target ${kcal(d.target.kcal)}`, `${d.meals.length} meals`]
      .filter(Boolean)
      .join(" · "),
    create: () => create(p.id, d),
  });
  return {
    title: "Meals from a program",
    programs: offered(programs, "meal"),
    finishedOnly: finishedOnly(programs, "meal"),
    planned: asPlanned(planned),
    cached: (p: ProgramSummary) => cachedMealDays(store, p.id).map(pick(p)),
    loadDays: (p: ProgramSummary) => mealDays(api, store, p).then((days) => days.map(pick(p))),
  };
};
