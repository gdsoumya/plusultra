import { createContext, useContext, useMemo, useState, type ReactNode } from "react";

import type { Unit } from "../domain/types";
import type { User } from "../gen/plusultra/v1/auth_pb";
import { LengthUnit, Unit as ProtoUnit } from "../gen/plusultra/v1/common_pb";
import { useSession } from "./session";
import { useStore } from "./StoreProvider";

export type MeasurementUnit = "cm" | "in";

export interface UserSettings {
  displayName: string;
  units: Unit;
  measurementUnit: MeasurementUnit;
}

export const SETTINGS_KEY = "userSettings";

const fromUser = (user?: User): UserSettings => ({
  displayName: user?.displayName ?? "",
  units: user?.units === ProtoUnit.LB ? "lb" : "kg",
  measurementUnit: user?.measurementUnit === LengthUnit.IN ? "in" : "cm",
});

type Settings = [UserSettings, (patch: Partial<UserSettings>) => void];

const SettingsContext = createContext<Settings | null>(null);

// The user's settings live in the store's kv table, seeded from the signed-in user. One provider
// holds them so a unit changed in Account shows on every screen at once.
export function SettingsProvider({ children }: { children: ReactNode }) {
  const store = useStore();
  const { user } = useSession();
  const read = () => ({ userId: user?.id, stored: store.getKv<UserSettings>(SETTINGS_KEY) });
  const [state, setState] = useState(read);
  // Signing out clears the store; the next user starts from their own profile.
  if (state.userId !== user?.id) setState(read());

  const value = useMemo<Settings>(() => {
    const settings = state.stored ?? fromUser(user);
    return [
      settings,
      (patch) => {
        const next = { ...settings, ...patch };
        store.setKv(SETTINGS_KEY, next);
        setState({ userId: user?.id, stored: next });
      },
    ];
  }, [state, user, store]);

  return <SettingsContext.Provider value={value}>{children}</SettingsContext.Provider>;
}

export function useSettings(): Settings {
  const settings = useContext(SettingsContext);
  if (!settings) throw new Error("useSettings must be used inside a SettingsProvider");
  return settings;
}

// The unit weights are shown in.
export const useUnit = () => useSettings()[0].units;
