import { Modal } from "react-native";

import { EmptyState } from "../../ui";
import type { BarcodeScannerProps } from "./BarcodeScanner";

// Keeps expo-camera out of the web bundle; scanning is a phone feature.
export function BarcodeScanner({ onClose }: BarcodeScannerProps) {
  return (
    <Modal visible animationType="slide" onRequestClose={onClose}>
      <EmptyState
        icon="camera-outline"
        title="Scanning needs the phone app"
        message="Search for the food by name instead."
        action={{ title: "Close", onPress: onClose }}
      />
    </Modal>
  );
}
