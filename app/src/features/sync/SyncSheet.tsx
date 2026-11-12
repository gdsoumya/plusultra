import { useSync } from "../../data/useSync";
import { formatDateTime } from "../../domain/dates";
import { Button, Sheet } from "../../ui";
import { DetailLine } from "../settings/parts";
import { describeSync } from "./status";

// What the sync is doing and why rows are waiting, with a way to run it now.
export function SyncSheet({ open, onClose }: { open: boolean; onClose: () => void }) {
  const sync = useSync();
  const { status, lastSyncedAt, pendingCount, reasons, syncNow } = sync;
  const refused = [...new Set(Object.values(reasons))].join(", ").replaceAll("_", " ");
  return (
    <Sheet open={open} onClose={onClose} title="Sync" testID="sync-sheet">
      <DetailLine label="Status" value={describeSync(sync).label} />
      <DetailLine
        label="Last synced"
        value={lastSyncedAt ? formatDateTime(new Date(lastSyncedAt)) : "Never"}
      />
      <DetailLine
        label="Waiting to upload"
        value={`${pendingCount} ${pendingCount === 1 ? "change" : "changes"}`}
      />
      {refused && <DetailLine label="Refused by the server" value={refused} />}
      <Button
        title="Sync now"
        icon="sync-outline"
        variant="secondary"
        loading={status === "syncing"}
        onPress={() => void syncNow()}
      />
    </Sheet>
  );
}
