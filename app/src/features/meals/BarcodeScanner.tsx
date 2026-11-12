import { CameraView, useCameraPermissions } from "expo-camera";
import { useEffect, useRef } from "react";
import { Modal, View } from "react-native";

import { Button, EmptyState, space, useTheme } from "../../ui";

export type BarcodeScannerProps = {
  // Called once per scan with an 8 to 14 digit EAN or UPC.
  onScanned(barcode: string): void;
  onClose(): void;
};

export function BarcodeScanner({ onScanned, onClose }: BarcodeScannerProps) {
  const { colors } = useTheme();
  const [permission, request] = useCameraPermissions();
  // The camera keeps reporting the same code while it is in view; only the first one counts.
  const seen = useRef<string>(undefined);

  useEffect(() => {
    if (permission && !permission.granted && permission.canAskAgain) request();
  }, [permission, request]);

  return (
    <Modal visible animationType="slide" onRequestClose={onClose}>
      <View style={{ flex: 1, backgroundColor: colors.bg }}>
        {permission?.granted ? (
          <CameraView
            style={{ flex: 1 }}
            barcodeScannerSettings={{ barcodeTypes: ["ean13", "ean8", "upc_a", "upc_e"] }}
            onBarcodeScanned={({ data }) => {
              if (seen.current === data || !/^\d{8,14}$/.test(data)) return;
              seen.current = data;
              onScanned(data);
            }}
          />
        ) : (
          <EmptyState
            icon="camera-outline"
            title="Camera access needed"
            message="Allow the camera to scan food barcodes."
            action={{ title: "Allow camera", onPress: () => void request() }}
          />
        )}
        <Button title="Cancel" variant="secondary" onPress={onClose} style={{ margin: space[4] }} />
      </View>
    </Modal>
  );
}
