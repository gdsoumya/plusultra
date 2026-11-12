import { fromProto } from "../../api/convert";
import { programsRepo } from "../../data/repositories/programs";
import type { Store } from "../../data/store";
import type { MealDay, ResolvedDay } from "../../domain/types";
import type { Program } from "../../gen/plusultra/v1/program_pb";

// Keeps the store's summary in step with a program the server just returned.
export const remember = (store: Store, program: Program) =>
  programsRepo(store).upsertFromServer([fromProto.program(program)]);

const daysKey = (programId: string) => `programDays:${programId}`;

// The detail screen's resolved or simulated days, kept so the plan view and an offline reopen have
// every week; the sync cache only holds an active program's days from today on.
export const rememberDays = (store: Store, programId: string, days: ResolvedDay[]) =>
  store.setKv(daysKey(programId), days);

export const cachedDays = (store: Store, programId: string): ResolvedDay[] =>
  store.getKv<ResolvedDay[]>(daysKey(programId)) ??
  store.list("resolvedDays", { programId }).map((r) => r.payload);

const mealDaysKey = (programId: string) => `programMealDays:${programId}`;

export const rememberMealDays = (store: Store, programId: string, days: MealDay[]) =>
  store.setKv(mealDaysKey(programId), days);

export const cachedMealDays = (store: Store, programId: string): MealDay[] =>
  store.getKv<MealDay[]>(mealDaysKey(programId)) ??
  store.list("mealDays", { programId }).map((r) => r.payload);

// Kind days share day 0, so the route names them by kind instead.
export const mealDaySegment = (d: Pick<MealDay, "day" | "kind">) =>
  d.day ? String(d.day) : d.kind;
