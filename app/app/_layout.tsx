import { Stack } from "expo-router";
import { StatusBar } from "expo-status-bar";
import { GestureHandlerRootView } from "react-native-gesture-handler";

import { SessionProvider, useSession } from "../src/data/session";
import { SettingsProvider } from "../src/data/settings";
import { StoreProvider } from "../src/data/StoreProvider";
import { SyncProvider } from "../src/data/useSync";
import { ThemeProvider, useTheme } from "../src/ui";

export default function RootLayout() {
  return (
    <GestureHandlerRootView style={{ flex: 1 }}>
      <ThemeProvider>
        <SessionProvider>
          <StoreProvider>
            <SettingsProvider>
              <SyncProvider>
                <ThemedStack />
              </SyncProvider>
            </SettingsProvider>
          </StoreProvider>
        </SessionProvider>
      </ThemeProvider>
    </GestureHandlerRootView>
  );
}

function ThemedStack() {
  const { colors } = useTheme();
  const { status } = useSession();
  if (status === "loading") return null;
  return (
    <>
      <StatusBar style="light" />
      <Stack screenOptions={{ headerShown: false, contentStyle: { backgroundColor: colors.bg } }}>
        <Stack.Protected guard={status === "signedIn"}>
          <Stack.Screen name="(tabs)" />
        </Stack.Protected>
        <Stack.Protected guard={status === "signedOut"}>
          <Stack.Screen name="(auth)" />
        </Stack.Protected>
      </Stack>
    </>
  );
}
