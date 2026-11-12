import { create } from "@bufbuild/protobuf";
import { timestampFromDate } from "@bufbuild/protobuf/wkt";
import { act, renderHook, waitFor } from "@testing-library/react-native";
import * as SecureStore from "expo-secure-store";
import { Platform } from "react-native";

import { UserSchema } from "../gen/plusultra/v1/auth_pb";
import { LengthUnit, Unit } from "../gen/plusultra/v1/common_pb";
import { MOCK_SERVER, SessionProvider, useSession } from "./session";

jest.mock("expo-secure-store", () => ({ getItemAsync: jest.fn(), setItemAsync: jest.fn() }));

const user = create(UserSchema, {
  id: "user-1",
  username: "demo",
  displayName: "Demo",
  units: Unit.KG,
  measurementUnit: LengthUnit.CM,
  timezone: "Asia/Kolkata",
  isAdmin: true,
  createdAt: timestampFromDate(new Date("2026-09-01T00:00:00.000Z")),
});

const items = new Map<string, string>();

beforeEach(() => {
  jest.clearAllMocks();
  items.clear();
  jest.mocked(SecureStore.getItemAsync).mockImplementation(async (k) => items.get(k) ?? null);
  jest.mocked(SecureStore.setItemAsync).mockImplementation(async (k, v) => void items.set(k, v));
});

async function mount() {
  const hook = await renderHook(() => useSession(), { wrapper: SessionProvider });
  await waitFor(() => expect(hook.result.current.status).not.toBe("loading"));
  return hook;
}

describe("session", () => {
  it("starts signed out and restores what was saved", async () => {
    const first = await mount();
    expect(first.result.current.status).toBe("signedOut");
    expect(first.result.current.serverUrl).toBeUndefined();
    await act(() => first.result.current.setServer("  https://lift.example.com/ "));
    await act(() => first.result.current.login("token-1", user));
    expect(first.result.current).toMatchObject({ status: "signedIn", token: "token-1", user });
    await first.unmount();

    const second = await mount();
    expect(second.result.current).toMatchObject({
      status: "signedIn",
      serverUrl: "https://lift.example.com",
      token: "token-1",
      user,
    });
  });

  it("accepts the mock server and rejects other addresses", async () => {
    const { result } = await mount();
    await act(() => result.current.setServer(MOCK_SERVER));
    expect(result.current.serverUrl).toBe("mock://demo");
    for (const bad of ["ftp://lift.example.com", "lift.example.com", "https://", "mock://", ""]) {
      expect(() => result.current.setServer(bad)).toThrow("http:// or https://");
    }
    expect(result.current.serverUrl).toBe(MOCK_SERVER);
  });

  it("signs out when the server changes, so the token never reaches another host", async () => {
    const { result } = await mount();
    await act(() => result.current.setServer("https://a.example.com"));
    await act(() => result.current.login("token-1", user));
    await act(() => result.current.setServer("https://a.example.com/"));
    expect(result.current.status).toBe("signedIn");
    await act(() => result.current.setServer("https://b.example.com"));
    expect(result.current.status).toBe("signedOut");
    expect(result.current.token).toBeUndefined();
  });

  it("clears the token and user on logout and keeps the server", async () => {
    const first = await mount();
    await act(() => first.result.current.setServer("https://lift.example.com"));
    await act(() => first.result.current.login("token-1", user));
    await act(() => first.result.current.logout());
    expect(first.result.current.status).toBe("signedOut");
    expect(first.result.current.user).toBeUndefined();
    await first.unmount();

    const second = await mount();
    expect(second.result.current).toMatchObject({
      status: "signedOut",
      serverUrl: "https://lift.example.com",
    });
    expect(second.result.current.token).toBeUndefined();
  });

  it("drops a rejected token, remembers why until the next login and keeps the server", async () => {
    const first = await mount();
    await act(() => first.result.current.setServer("https://lift.example.com"));
    await act(() => first.result.current.login("token-1", user));
    expect(first.result.current.expired).toBeFalsy();
    await act(() => first.result.current.expire());
    expect(first.result.current).toMatchObject({
      status: "signedOut",
      serverUrl: "https://lift.example.com",
      expired: true,
    });
    expect(first.result.current.token).toBeUndefined();
    await first.unmount();

    const second = await mount();
    expect(second.result.current).toMatchObject({ status: "signedOut", expired: true });
    await act(() => second.result.current.login("token-2", user));
    expect(second.result.current.status).toBe("signedIn");
    expect(second.result.current.expired).toBeUndefined();
  });

  it("keeps working in memory when secure storage fails", async () => {
    jest.mocked(SecureStore.getItemAsync).mockRejectedValue(new Error("keystore locked"));
    jest.mocked(SecureStore.setItemAsync).mockRejectedValue(new Error("keystore locked"));
    const { result } = await mount();
    expect(result.current.status).toBe("signedOut");
    await act(() => result.current.setServer("https://lift.example.com"));
    await act(() => result.current.login("token-1", user));
    expect(result.current.status).toBe("signedIn");
  });

  describe("on the web", () => {
    beforeEach(() => jest.replaceProperty(Platform, "OS", "web"));
    afterEach(() => {
      delete (globalThis as { localStorage?: unknown }).localStorage;
    });

    it("saves to localStorage", async () => {
      const saved = new Map<string, string>();
      Object.defineProperty(globalThis, "localStorage", {
        configurable: true,
        value: {
          getItem: (k: string) => saved.get(k) ?? null,
          setItem: (k: string, v: string) => saved.set(k, v),
        },
      });
      const first = await mount();
      await act(() => first.result.current.setServer("https://lift.example.com"));
      await act(() => first.result.current.login("token-1", user));
      await first.unmount();
      expect(SecureStore.setItemAsync).not.toHaveBeenCalled();

      const second = await mount();
      expect(second.result.current).toMatchObject({ status: "signedIn", token: "token-1" });
    });

    it("works without localStorage", async () => {
      Object.defineProperty(globalThis, "localStorage", {
        configurable: true,
        get() {
          throw new Error("SecurityError");
        },
      });
      const { result } = await mount();
      expect(result.current.status).toBe("signedOut");
      await act(() => result.current.login("token-1", user));
      expect(result.current.status).toBe("signedIn");
    });
  });

  it("needs a provider", async () => {
    jest.spyOn(console, "error").mockImplementation(() => {});
    await expect(renderHook(() => useSession())).rejects.toThrow("SessionProvider");
  });
});
