import {
  createRouterTransport,
  type ConnectError,
  type Interceptor,
  type Transport,
} from "@connectrpc/connect";

import { localDate } from "../domain/dates";
import { services, type ApiMethod } from "./clients";
import { timezone } from "./fixtures/user";
import { apiKeys } from "./mock/apiKeys";
import { auth } from "./mock/auth";
import { calendar } from "./mock/calendar";
import { checkins } from "./mock/checkins";
import { exercises } from "./mock/exercises";
import { exportData } from "./mock/export";
import { foods } from "./mock/foods";
import { insights } from "./mock/insights";
import { mealLogs } from "./mock/meals";
import { programs } from "./mock/programs";
import { reference } from "./mock/reference";
import { seed } from "./mock/seed";
import { sync } from "./mock/sync";
import { workouts } from "./mock/workouts";

export interface MockTransport {
  transport: Transport;
  // The next call of method fails with error instead of reaching the mock server.
  failNext(method: ApiMethod, error: ConnectError): void;
}

export interface MockTransportOptions {
  // Local date the fixtures are built around; defaults to today in the fixture user's zone.
  today?: string;
  latencyMs?: number;
}

const keyOf = new Map(Object.entries(services).map(([key, service]) => [service.typeName, key]));

// An in-memory server over the generated services, seeded with fixtures.
// shortcut: tokens are never checked, so nothing fails as unauthenticated on its own; inject it with
// failNext until a test needs real session expiry.
export function createMockTransport({
  today,
  latencyMs = 0,
}: MockTransportOptions = {}): MockTransport {
  const db = seed(today ?? localDate(new Date(), timezone));
  const failures = new Map<string, ConnectError>();

  const inject: Interceptor = (next) => async (req) => {
    if (latencyMs > 0) await new Promise((resolve) => setTimeout(resolve, latencyMs));
    const method = `${keyOf.get(req.service.typeName)}.${req.method.localName}`;
    const failure = failures.get(method);
    if (failure) {
      failures.delete(method);
      throw failure;
    }
    return next(req);
  };

  const transport = createRouterTransport(
    (router) =>
      router
        .service(services.auth, auth(db))
        .service(services.apiKeys, apiKeys(db))
        .service(services.programs, programs(db))
        .service(services.calendar, calendar(db))
        .service(services.workouts, workouts(db))
        .service(services.meals, mealLogs(db))
        .service(services.checkins, checkins(db))
        .service(services.exercises, exercises(db))
        .service(services.foods, foods(db))
        .service(services.insights, insights(db))
        .service(services.export, exportData(db))
        .service(services.reference, reference())
        .service(services.sync, sync(db)),
    { transport: { interceptors: [inject] } },
  );

  return {
    transport,
    failNext(method, error) {
      failures.set(method, error);
    },
  };
}
