import { Platform } from "react-native";

// Feature flags for the meals screen. Mutable so tests and a dev menu can flip them.
export const mealFlags = {
  // Shows the barcode scanner (expo-camera) in the add-item sheet; the web has no camera module.
  scanBarcodes: Platform.OS !== "web",
};
