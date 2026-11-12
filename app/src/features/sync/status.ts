import type { SyncStatus } from "../../data/useSync";
import type { ColorRole, IconName } from "../../ui";

// The badge's word for the sync state, most urgent first.
export function describeSync({ status, pendingCount }: SyncStatus): {
  label: string;
  icon: IconName;
  tone: ColorRole;
} {
  if (status === "syncing") return { label: "Syncing", icon: "sync-outline", tone: "textMuted" };
  if (status === "offline") {
    return { label: "Offline", icon: "cloud-offline-outline", tone: "warning" };
  }
  if (status === "error") {
    return { label: "Sync failed", icon: "alert-circle-outline", tone: "danger" };
  }
  if (pendingCount > 0) {
    return { label: `${pendingCount} unsynced`, icon: "cloud-upload-outline", tone: "textMuted" };
  }
  return { label: "Synced", icon: "checkmark-circle-outline", tone: "textMuted" };
}
