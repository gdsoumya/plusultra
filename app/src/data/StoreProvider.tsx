import { createContext, useContext, useState, type ReactNode } from "react";

import { createStore } from "./createStore";
import type { Store } from "./store";

const StoreContext = createContext<Store | null>(null);

let device: Store | undefined;
// One device store for the app, opened on first use; tests pass a memory store to the provider.
const deviceStore = () => (device ??= createStore());

export function StoreProvider({ store, children }: { store?: Store; children: ReactNode }) {
  const [value] = useState(() => store ?? deviceStore());
  return <StoreContext.Provider value={value}>{children}</StoreContext.Provider>;
}

// Without a provider (tests, the gallery) callers share the device store.
export const useStore = (): Store => useContext(StoreContext) ?? deviceStore();
