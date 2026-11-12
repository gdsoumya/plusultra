import { fireEvent, screen } from "@testing-library/react-native";

import type { SyncStatus } from "../../data/useSync";
import { formatDateTime } from "../../domain/dates";
import { renderThemed } from "../../ui/test-utils";
import { SyncBadge } from "./SyncBadge";

let mockSync: SyncStatus;
jest.mock("../../data/useSync", () => ({ useSync: () => mockSync }));

beforeEach(() => {
  mockSync = { status: "idle", pendingCount: 0, reasons: {}, syncNow: jest.fn(async () => {}) };
});

describe("SyncBadge", () => {
  it.each([
    ["idle", "Synced"],
    ["syncing", "Syncing"],
    ["offline", "Offline"],
    ["error", "Sync failed"],
  ] as const)("shows %s as %s", async (status, text) => {
    mockSync.status = status;
    await renderThemed(<SyncBadge />);
    expect(screen.getByRole("button", { name: text })).toBeOnTheScreen();
  });

  it("counts the rows waiting for the server", async () => {
    mockSync.pendingCount = 3;
    await renderThemed(<SyncBadge />);
    expect(screen.getByRole("button", { name: "3 unsynced" })).toBeOnTheScreen();
  });

  it("opens a sheet with the last sync, the pending rows and the server's reasons", async () => {
    mockSync.lastSyncedAt = "2026-11-11T09:30:00Z";
    mockSync.pendingCount = 3;
    mockSync.reasons = { a: "finalized", b: "unknown_program", c: "finalized" };
    await renderThemed(<SyncBadge />);
    await fireEvent.press(screen.getByRole("button", { name: "3 unsynced" }));
    expect(screen.getByText(formatDateTime(new Date(mockSync.lastSyncedAt)))).toBeOnTheScreen();
    expect(screen.getByText("3 changes")).toBeOnTheScreen();
    expect(screen.getByText("finalized, unknown program")).toBeOnTheScreen();
    await fireEvent.press(screen.getByRole("button", { name: "Sync now" }));
    expect(mockSync.syncNow).toHaveBeenCalledTimes(1);
  });

  it("says Never before the first sync and blocks Sync now while syncing", async () => {
    mockSync.status = "syncing";
    await renderThemed(<SyncBadge />);
    await fireEvent.press(screen.getByRole("button", { name: "Syncing" }));
    expect(screen.getByText("Never")).toBeOnTheScreen();
    expect(screen.getByRole("button", { name: "Sync now" })).toBeDisabled();
  });
});
