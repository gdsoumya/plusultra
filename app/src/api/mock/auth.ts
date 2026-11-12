import { create } from "@bufbuild/protobuf";
import { timestampMs } from "@bufbuild/protobuf/wkt";
import { Code, ConnectError, type ServiceImpl } from "@connectrpc/connect";

import { AuthService, InviteSchema, UserSchema, type User } from "../../gen/plusultra/v1/auth_pb";
import { Unit } from "../../gen/plusultra/v1/common_pb";
import { exists, invalid, later, newId, nowStamp, type MockDb } from "./db";

function checkPassword(password: string) {
  if (password.length < 10) throw invalid("password must be at least 10 characters");
}

export function auth(db: MockDb): ServiceImpl<typeof AuthService> {
  const signIn = (user: User) => ({ user, token: newId(db, "mock-token"), expiresAt: later(90) });
  return {
    register({ username, password, displayName, inviteCode, units, timezone }) {
      if (!/^[a-z0-9_]{3,32}$/.test(username)) {
        throw invalid("username must be 3 to 32 lowercase letters, digits or underscores");
      }
      checkPassword(password);
      if (username === db.user.username) throw exists("username is taken");
      // shortcut: registration is always invite-only (the server can run REGISTRATION=open);
      // upgrade when the register screen offers open sign-up.
      const invite = db.invites.find(
        (i) =>
          i.code === inviteCode &&
          !i.usedAt &&
          i.expiresAt &&
          timestampMs(i.expiresAt) > Date.now(),
      );
      if (!invite) throw new ConnectError("a valid invite code is required", Code.PermissionDenied);
      Object.assign(invite, { usedAt: nowStamp(), usedBy: username });
      return signIn(
        create(UserSchema, {
          ...db.user,
          id: newId(db, "user"),
          username,
          displayName: displayName || username,
          units: units || Unit.KG,
          timezone: timezone || "UTC",
          isAdmin: false,
          createdAt: nowStamp(),
        }),
      );
    },
    login({ username, password }) {
      if (username !== db.user.username || password !== db.password) {
        throw new ConnectError("wrong username or password", Code.Unauthenticated);
      }
      return signIn(db.user);
    },
    logout: () => ({}),
    changePassword({ currentPassword, newPassword }) {
      if (currentPassword !== db.password) {
        throw new ConnectError("current password is wrong", Code.PermissionDenied);
      }
      checkPassword(newPassword);
      db.password = newPassword;
      return {};
    },
    getMe: () => ({ user: db.user }),
    updateMe({ units, timezone, displayName, measurementUnit }) {
      if (timezone !== undefined && !timezone) throw invalid("timezone must not be empty");
      if (displayName !== undefined && !displayName)
        throw invalid("display name must not be empty");
      db.user = create(UserSchema, {
        ...db.user,
        units: units ?? db.user.units,
        timezone: timezone ?? db.user.timezone,
        displayName: displayName ?? db.user.displayName,
        measurementUnit: measurementUnit ?? db.user.measurementUnit,
      });
      return { user: db.user };
    },
    createInvite({ expiresAt = later(7) }) {
      const invite = create(InviteSchema, {
        code: newId(db, "invite"),
        expiresAt,
        createdAt: nowStamp(),
      });
      db.invites.unshift(invite);
      return { invite };
    },
    listInvites: () => ({ invites: db.invites }),
  };
}
