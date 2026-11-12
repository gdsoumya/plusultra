import { createClients } from "../api/clients";
import { fromProto } from "../api/convert";
import { createMockTransport } from "../api/mockTransport";
import { ProgramStatus } from "../gen/plusultra/v1/program_pb";
import { createMemoryStore } from "./memoryStore";
import { checkinsRepo } from "./repositories/checkins";
import { dayLogsRepo } from "./repositories/dayLogs";
import { mealLogsRepo } from "./repositories/mealLogs";
import { programsRepo } from "./repositories/programs";
import { mealDaysRepo, resolvedDaysRepo } from "./repositories/resolvedDays";
import { workoutRecordsRepo } from "./repositories/workoutRecords";
import type { Store } from "./store";

// Test seed: a memory store filled the way a first sync pull fills it from the mock server's
// fixtures, plus every dated day of the active programs so past dates have their plan too.
export async function seedStore(today: string): Promise<Store> {
  const api = createClients(createMockTransport({ today }).transport);
  const store = createMemoryStore();
  const pulled = await api.sync.pull({});
  const rows = fromProto.syncRows(pulled.rows, "user-1");
  dayLogsRepo(store).upsertFromServer(rows.dayLogs);
  workoutRecordsRepo(store).upsertFromServer(rows.workoutRecords);
  mealLogsRepo(store).upsertFromServer(rows.mealLogs);
  checkinsRepo(store).upsertFromServer(rows.checkins);
  mealDaysRepo(store).upsertFromServer(pulled.resolvedMealDays.map(fromProto.mealDay));
  const { programs } = await api.programs.listPrograms({});
  programsRepo(store).upsertFromServer(programs.map(fromProto.program));
  for (const { id } of programs.filter((p) => p.status === ProgramStatus.ACTIVE)) {
    const { days } = await api.programs.getResolvedDays({ programId: id });
    resolvedDaysRepo(store).upsertFromServer(days.map(fromProto.resolvedDay));
  }
  return store;
}

// Swaps the active workout program for a floating one whose cursor is at week 2 day 1.
export function makeFloating(store: Store) {
  const programs = programsRepo(store);
  const [anchored] = programs.all().filter((p) => p.kind === "workout" && p.status === "active");
  const floating = programs.get("program-gzclp")!;
  programs.upsertFromServer([
    { ...anchored, status: "finished" },
    { ...floating, status: "active", mode: "floating", nextWeek: 2, nextDay: 1 },
  ]);
  resolvedDaysRepo(store).upsertFromServer([
    {
      programId: floating.id,
      week: 2,
      day: 1,
      payload: { week: 2, day: 1, name: "GZCLP A1", desc: "", kind: "training", exercises: [] },
    },
  ]);
}
