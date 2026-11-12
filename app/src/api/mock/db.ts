import { timestampFromDate } from "@bufbuild/protobuf/wkt";
import { Code, ConnectError } from "@connectrpc/connect";

import type { ServerRow } from "../../data/repositories/syncRepo";
import type { Rows, SyncTable } from "../../data/store";
import type { MealDay, ResolvedDay } from "../../domain/types";
import type { ApiKey } from "../../gen/plusultra/v1/apikey_pb";
import type { Invite, User } from "../../gen/plusultra/v1/auth_pb";
import type { Equipment, Exercise, ExerciseData } from "../../gen/plusultra/v1/exercise_pb";
import type { Food } from "../../gen/plusultra/v1/food_pb";
import type { Program, ProgramRevision } from "../../gen/plusultra/v1/program_pb";

export type Row<T extends SyncTable> = ServerRow<Rows[T]>;
export type PlannedDay = ResolvedDay & { programId: string };
export type PlannedMealDay = MealDay & { programId: string; date?: string };

// Synced rows and plans are kept in the store's shape and converted at the edge, so the mock
// exercises the same conversion the app uses.
export interface MockDb {
  today: string;
  // Bumped on every row change; a pull cursor is the value it had.
  seq: number;
  ids: number;
  user: User;
  password: string;
  invites: Invite[];
  apiKeys: ApiKey[];
  programs: Program[];
  revisions: Record<string, ProgramRevision[]>;
  days: Record<string, PlannedDay[]>;
  mealDays: Record<string, PlannedMealDay[]>;
  rows: { [T in SyncTable]: Map<string, { row: Row<T>; seq: number }> };
  photos: Map<string, { bytes: Uint8Array; contentType: string }>;
  exercises: Exercise[];
  exerciseData: ExerciseData[];
  equipment: Equipment[];
  foods: Food[];
}

export const now = () => new Date().toISOString();
export const nowStamp = () => timestampFromDate(new Date());
export const later = (days: number) => timestampFromDate(new Date(Date.now() + days * 86_400_000));
// shortcut: ids are never checked to be UUIDs and fixtures use readable ones, though the server
// requires UUIDs for client-chosen ids; clients must use uuid() regardless, and the mock
// needs a check if a test should catch a client that does not.
export const newId = (db: MockDb, prefix: string) => `${prefix}-${++db.ids}`;

export const notFound = (what: string) => new ConnectError(`${what} not found`, Code.NotFound);
export const precondition = (message: string) => new ConnectError(message, Code.FailedPrecondition);
export const invalid = (message: string) => new ConnectError(message, Code.InvalidArgument);
export const exists = (message: string) => new ConnectError(message, Code.AlreadyExists);

export function required<T>(value: T | undefined, what: string): T {
  if (value === undefined || value === "") throw invalid(`${what} is required`);
  return value;
}

// List pages: the cursor is the offset of the next item.
// shortcut: a limit above 500 is not rejected as on the server; upgrade if a client could send one.
export function page<T>(all: T[], cursor: string, limit: number, fallback = 100) {
  const start = Number(cursor || 0);
  if (!Number.isInteger(start) || start < 0) throw invalid(`bad cursor ${cursor}`);
  const end = start + (limit || fallback);
  const hasMore = end < all.length;
  return { items: all.slice(start, end), nextCursor: hasMore ? String(end) : "", hasMore };
}

export function put<T extends SyncTable>(db: MockDb, table: T, row: Row<T>): Row<T> {
  db.rows[table].set(row.id, { row, seq: ++db.seq });
  return row;
}

type Meta = "userId" | "version" | "updatedAt" | "deviceId";

// Stores a change the server made itself: the next version, stamped now.
export function save<T extends SyncTable>(db: MockDb, table: T, row: Omit<Row<T>, Meta>): Row<T> {
  const version = (db.rows[table].get(row.id)?.row.version ?? 0) + 1;
  const stamped = { ...row, userId: db.user.id, version, updatedAt: now(), deviceId: "server" };
  return put(db, table, stamped as Row<T>);
}

export const live = <T extends SyncTable>(db: MockDb, table: T): Row<T>[] =>
  [...db.rows[table].values()]
    .map((s) => s.row)
    .filter((r) => !r.deletedAt)
    .sort((a, b) => a.date.localeCompare(b.date));

export const onDate = <T extends SyncTable>(db: MockDb, table: T, date: string) =>
  live(db, table).find((r) => r.date === date);

export type Range = { from: string; to: string };

export function inDates<T extends SyncTable>(db: MockDb, table: T, dates?: Range) {
  const { from, to } = required(dates, "dates");
  return live(db, table).filter((r) => r.date >= from && r.date <= to);
}

export function find<T extends SyncTable>(db: MockDb, table: T, id: string): Row<T> {
  const row = db.rows[table].get(id)?.row;
  if (!row || row.deletedAt) throw notFound(`${table} row ${id}`);
  return row;
}

export function remove<T extends SyncTable>(db: MockDb, table: T, id: string) {
  save(db, table, { ...find(db, table, id), deletedAt: now() });
  return {};
}

export function program(db: MockDb, id: string): Program {
  const found = db.programs.find((p) => p.id === id);
  if (!found) throw notFound(`program ${id}`);
  return found;
}
