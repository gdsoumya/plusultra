import { useEffect } from "react";
import { BackHandler } from "react-native";

// While something is open (a sheet, an inline editor), the Android back button closes it instead
// of leaving the screen. Nothing else may claim the button (P2-R18).
export function useBackToClose(onClose: () => void, visible: boolean) {
  useEffect(() => {
    if (!visible) return;
    const sub = BackHandler.addEventListener("hardwareBackPress", () => {
      onClose();
      return true;
    });
    return () => sub.remove();
  }, [visible, onClose]);
}
