import { renderHook } from "@testing-library/react-native";
import type { ReactNode } from "react";

import { createMemoryStore } from "./memoryStore";
import { StoreProvider, useStore } from "./StoreProvider";

describe("StoreProvider", () => {
  it("hands the given store to useStore", async () => {
    const store = createMemoryStore();
    const wrapper = ({ children }: { children: ReactNode }) => (
      <StoreProvider store={store}>{children}</StoreProvider>
    );
    const { result } = await renderHook(() => useStore(), { wrapper });
    expect(result.current).toBe(store);
  });

  it("falls back to one shared device store without a provider", async () => {
    const first = await renderHook(() => useStore());
    const second = await renderHook(() => useStore());
    expect(first.result.current).toBe(second.result.current);
    first.result.current.setKv("probe", 1);
    expect(second.result.current.getKv("probe")).toBe(1);
  });
});
