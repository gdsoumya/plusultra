import { useState } from "react";

import { useSync } from "../../data/useSync";
import { Badge } from "../../ui";
import { describeSync } from "./status";
import { SyncSheet } from "./SyncSheet";

// The header's word on sync; tapping it opens the details and a Sync now button.
export function SyncBadge() {
  const sync = useSync();
  const [open, setOpen] = useState(false);
  return (
    <>
      <Badge {...describeSync(sync)} onPress={() => setOpen(true)} />
      <SyncSheet open={open} onClose={() => setOpen(false)} />
    </>
  );
}
