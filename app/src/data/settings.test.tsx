import { act, renderHook } from "@testing-library/react-native";
import type { ReactNode } from "react";

import { user as fixtureUser } from "../api/fixtures/user";
import type { User } from "../gen/plusultra/v1/auth_pb";
import { Unit } from "../gen/plusultra/v1/common_pb";
import { createMemoryStore } from "./memoryStore";
import { SETTINGS_KEY, SettingsProvider, useSettings, useUnit } from "./settings";
import { StoreProvider } from "./StoreProvider";

let mockUser: User | undefined;
jest.mock("./session", () => ({ useSession: () => ({ user: mockUser }) }));

const store = createMemoryStore();
const wrapper = ({ children }: { children: ReactNode }) => (
  <StoreProvider store={store}>
    <SettingsProvider>{children}</SettingsProvider>
  </StoreProvider>
);

beforeEach(() => {
  store.clearUserData();
  mockUser = { ...fixtureUser("2026-11-11"), units: Unit.LB };
});

describe("SettingsProvider", () => {
  it("starts from the signed-in user and keeps a change in the store for every reader", async () => {
    const { result } = await renderHook(() => ({ unit: useUnit(), settings: useSettings() }), {
      wrapper,
    });
    expect(result.current.unit).toBe("lb");
    await act(() => result.current.settings[1]({ units: "kg" }));
    expect(result.current.unit).toBe("kg");
    expect(store.getKv(SETTINGS_KEY)).toMatchObject({ units: "kg", displayName: "Demo" });
  });

  it("reads the next user's settings after the store was cleared on sign-out", async () => {
    store.setKv(SETTINGS_KEY, { displayName: "Old", units: "kg", measurementUnit: "in" });
    const { result, rerender } = await renderHook(() => useSettings()[0], { wrapper });
    expect(result.current.displayName).toBe("Old");
    store.clearUserData();
    mockUser = { ...mockUser!, id: "user-2", displayName: "Next" };
    await rerender({});
    expect(result.current).toEqual({ displayName: "Next", units: "lb", measurementUnit: "cm" });
  });
});
