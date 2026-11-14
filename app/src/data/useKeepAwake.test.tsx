import { renderHook } from "@testing-library/react-native";
import * as KeepAwake from "expo-keep-awake";
import type { ReactNode } from "react";

import { createMemoryStore } from "./memoryStore";
import { SETTINGS_KEY, SettingsProvider } from "./settings";
import { StoreProvider } from "./StoreProvider";
import { useKeepAwake } from "./useKeepAwake";

jest.mock("./session", () => ({
  ...jest.requireActual("./session"),
  useSession: () => ({ user: undefined }),
}));
jest.mock("expo-keep-awake", () => ({
  activateKeepAwakeAsync: jest.fn().mockResolvedValue(undefined),
  deactivateKeepAwake: jest.fn().mockResolvedValue(undefined),
}));

function wrapperFor(keepAwake: boolean) {
  const store = createMemoryStore();
  store.setKv(SETTINGS_KEY, { keepAwake });
  return function Wrapper({ children }: { children: ReactNode }) {
    return (
      <StoreProvider store={store}>
        <SettingsProvider>{children}</SettingsProvider>
      </StoreProvider>
    );
  };
}

describe("useKeepAwake", () => {
  it("leaves the screen alone while the setting is off", async () => {
    const { unmount } = await renderHook(() => useKeepAwake(), { wrapper: wrapperFor(false) });
    await unmount();
    expect(KeepAwake.activateKeepAwakeAsync).not.toHaveBeenCalled();
    expect(KeepAwake.deactivateKeepAwake).not.toHaveBeenCalled();
  });

  it("holds the screen on while the setting is on, and lets go on unmount", async () => {
    const { unmount } = await renderHook(() => useKeepAwake(), { wrapper: wrapperFor(true) });
    expect(KeepAwake.activateKeepAwakeAsync).toHaveBeenCalledTimes(1);
    await unmount();
    expect(KeepAwake.deactivateKeepAwake).toHaveBeenCalledTimes(1);
  });
});
