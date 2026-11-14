import { activateKeepAwakeAsync, deactivateKeepAwake } from "expo-keep-awake";
import { useEffect } from "react";

import { useSettings } from "./settings";

// Keeps the screen on while the app is in front, never in the background: the flag lives on the
// app's own window. A platform without it (some web browsers) is left alone.
export function useKeepAwake() {
  const [{ keepAwake }] = useSettings();
  useEffect(() => {
    if (!keepAwake) return;
    activateKeepAwakeAsync().catch(() => {});
    return () => {
      deactivateKeepAwake().catch(() => {});
    };
  }, [keepAwake]);
}
