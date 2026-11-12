import { create } from "@bufbuild/protobuf";

import { ApiKeyScope, ApiKeySchema, type ApiKey } from "../../gen/plusultra/v1/apikey_pb";
import { InviteSchema, UserSchema, type Invite, type User } from "../../gen/plusultra/v1/auth_pb";
import { LengthUnit, Unit } from "../../gen/plusultra/v1/common_pb";
import { addDays, stampAt } from "./dates";

export const credentials = { username: "demo", password: "demo-password" };
export const inviteCode = "invite-demo";
export const timezone = "Asia/Kolkata";

export const user = (today: string): User =>
  create(UserSchema, {
    id: "user-1",
    username: credentials.username,
    displayName: "Demo",
    units: Unit.KG,
    measurementUnit: LengthUnit.CM,
    timezone,
    isAdmin: true,
    createdAt: stampAt(addDays(today, -90), "09:00"),
  });

export const invites = (today: string): Invite[] => [
  create(InviteSchema, {
    code: inviteCode,
    expiresAt: stampAt(addDays(today, 7), "09:00"),
    createdAt: stampAt(today, "06:30"),
  }),
  create(InviteSchema, {
    code: "invite-used",
    expiresAt: stampAt(addDays(today, -40), "09:00"),
    usedAt: stampAt(addDays(today, -45), "18:00"),
    usedBy: "friend",
    createdAt: stampAt(addDays(today, -47), "09:00"),
  }),
];

const { READ, WRITE } = ApiKeyScope;

export const apiKeys = (today: string): ApiKey[] => [
  create(ApiKeySchema, {
    id: "key-3",
    name: "claude desktop",
    prefix: "c1a2u3d4",
    scopes: [READ, WRITE],
    expiresAt: stampAt(addDays(today, 300), "09:00"),
    lastUsedAt: stampAt(addDays(today, -1), "21:10"),
    createdAt: stampAt(addDays(today, -65), "09:00"),
  }),
  create(ApiKeySchema, {
    id: "key-2",
    name: "backup script",
    prefix: "b4c5k6u7",
    scopes: [READ],
    expiresAt: stampAt(addDays(today, 30), "09:00"),
    lastUsedAt: stampAt(today, "03:00"),
    createdAt: stampAt(addDays(today, -80), "09:00"),
  }),
  create(ApiKeySchema, {
    id: "key-1",
    name: "old laptop",
    prefix: "o1d2l3p4",
    scopes: [READ, WRITE],
    expiresAt: stampAt(addDays(today, 100), "09:00"),
    revokedAt: stampAt(addDays(today, -20), "12:00"),
    createdAt: stampAt(addDays(today, -88), "09:00"),
  }),
];
