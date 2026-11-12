import { createClient, type Client, type Transport } from "@connectrpc/connect";

import { ApiKeyService } from "../gen/plusultra/v1/apikey_pb";
import { AuthService } from "../gen/plusultra/v1/auth_pb";
import { CalendarService } from "../gen/plusultra/v1/calendar_pb";
import { CheckinService } from "../gen/plusultra/v1/checkin_pb";
import { ExerciseService } from "../gen/plusultra/v1/exercise_pb";
import { ExportService } from "../gen/plusultra/v1/export_pb";
import { FoodService } from "../gen/plusultra/v1/food_pb";
import { InsightService } from "../gen/plusultra/v1/insight_pb";
import { MealService } from "../gen/plusultra/v1/meal_pb";
import { ProgramService } from "../gen/plusultra/v1/program_pb";
import { ReferenceService } from "../gen/plusultra/v1/reference_pb";
import { SyncService } from "../gen/plusultra/v1/sync_pb";
import { WorkoutService } from "../gen/plusultra/v1/workout_pb";

export const services = {
  auth: AuthService,
  apiKeys: ApiKeyService,
  programs: ProgramService,
  calendar: CalendarService,
  workouts: WorkoutService,
  meals: MealService,
  checkins: CheckinService,
  exercises: ExerciseService,
  foods: FoodService,
  insights: InsightService,
  export: ExportService,
  reference: ReferenceService,
  sync: SyncService,
};

// The server's API: one generated Connect client per service.
export type Api = { [K in keyof typeof services]: Client<(typeof services)[K]> };

// "service.method", for example "sync.push".
export type ApiMethod = {
  [K in keyof Api]: `${K}.${Extract<keyof Api[K], string>}`;
}[keyof Api];

export const createClients = (transport: Transport) =>
  Object.fromEntries(
    Object.entries(services).map(([key, service]) => [key, createClient(service, transport)]),
  ) as Api;
