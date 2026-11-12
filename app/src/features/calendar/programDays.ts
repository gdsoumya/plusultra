import type { Api } from "../../api/clients";
import { fromProto } from "../../api/convert";
import { withKnownFlags } from "../../data/exerciseData";
import type { Store } from "../../data/store";
import type { MealDay, ProgramSummary, ResolvedDay } from "../../domain/types";
import {
  cachedDays,
  cachedMealDays,
  rememberDays,
  rememberMealDays,
} from "../programs/programStore";

// Every day of a program for the picker: resolved for an active one, simulated for any other.
// Offline the detail screen's cache stands in.
async function load(api: Api, p: ProgramSummary) {
  if (p.status === "active") return api.programs.getResolvedDays({ programId: p.id });
  const { program } = await api.programs.getProgram({ id: p.id });
  return api.programs.simulateProgram({ script: program?.script ?? "", weeks: p.weeks || 1 });
}

export async function workoutDays(
  api: Api,
  store: Store,
  p: ProgramSummary,
): Promise<ResolvedDay[]> {
  try {
    const days = (await load(api, p)).days.map((d) =>
      withKnownFlags(store, fromProto.resolvedDay(d).payload),
    );
    rememberDays(store, p.id, days);
    return days;
  } catch {
    return cachedDays(store, p.id);
  }
}

export async function mealDays(api: Api, store: Store, p: ProgramSummary): Promise<MealDay[]> {
  try {
    const days = (await load(api, p)).mealDays.map((d) => fromProto.mealDay(d).payload);
    rememberMealDays(store, p.id, days);
    return days;
  } catch {
    return cachedMealDays(store, p.id);
  }
}
