import { useState } from "react";

import { useSync } from "./useSync";

// Pull-to-refresh state for a Screen: a sync on request, with `reload` run once it has settled
// for screens that hold store rows in state.
export function useRefresh(reload?: () => void) {
  const { syncNow } = useSync();
  // A new object per change, so settling renders the screen even when the sync was instant.
  const [{ refreshing }, setState] = useState({ refreshing: false });
  const refresh = async () => {
    setState({ refreshing: true });
    // A failure shows in the sync badge, not here.
    await syncNow().catch(() => {});
    reload?.();
    setState({ refreshing: false });
  };
  return { refreshing, refresh };
}
