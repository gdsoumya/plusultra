import { act, renderHook, waitFor } from "@testing-library/react-native";
import * as SecureStore from "expo-secure-store";
import { Platform } from "react-native";

import { createMemoryStore } from "../../data/memoryStore";
import { MOCK_SERVER, SessionProvider, useSession } from "../../data/session";
import { dayLog } from "../../data/test-utils";
import { UserSchema } from "../../gen/plusultra/v1/auth_pb";
import { create } from "@bufbuild/protobuf";
import { OFFLINE } from "../../api/errors";
import { defaultServerUrl, probe, signOut } from "./useServer";

jest.mock("expo-secure-store", () => ({ getItemAsync: jest.fn(), setItemAsync: jest.fn() }));

const fetch = jest.spyOn(globalThis, "fetch");

afterEach(() => {
  fetch.mockReset();
  jest.useRealTimers();
});

describe("defaultServerUrl", () => {
  it("is the page's origin on web and nothing on native", () => {
    Object.defineProperty(globalThis, "location", {
      value: { origin: "https://lift.example.com" },
      configurable: true,
    });
    expect(defaultServerUrl()).toBeUndefined();
    const os = jest.replaceProperty(Platform, "OS", "web");
    expect(defaultServerUrl()).toBe("https://lift.example.com");
    os.restore();
  });
});

describe("probe", () => {
  it("returns the version of a healthy PlusUltra server", async () => {
    fetch.mockResolvedValue(new Response("ok 1.4.0\n"));
    await expect(probe("https://lift.example.com")).resolves.toBe("1.4.0");
    expect(fetch).toHaveBeenCalledWith("https://lift.example.com/healthz", expect.anything());
  });

  it("rejects a server that is down or something else entirely", async () => {
    fetch.mockResolvedValueOnce(new Response("database unavailable", { status: 503 }));
    await expect(probe("https://lift.example.com")).rejects.toThrow("503");
    fetch.mockResolvedValueOnce(new Response("<html>hi</html>"));
    await expect(probe("https://www.example.com")).rejects.toThrow("not a PlusUltra server");
  });

  it("shows the offline sentence when fetch itself fails", async () => {
    fetch.mockRejectedValueOnce(new Error("fetch failed: java.net.ConnectException: Failed"));
    await expect(probe("https://down.example.com")).rejects.toThrow(OFFLINE);
  });

  it("gives up after the timeout", async () => {
    jest.useFakeTimers();
    fetch.mockImplementation(
      (_url, init) =>
        new Promise((_resolve, reject) =>
          init?.signal?.addEventListener("abort", () => reject(new Error("aborted"))),
        ),
    );
    const pending = probe("https://slow.example.com", 5000);
    const result = expect(pending).rejects.toThrow("did not answer within 5 seconds");
    await jest.advanceTimersByTimeAsync(5000);
    await result;
  });
});

describe("signOut", () => {
  it("clears the session and every local row, keeping the server address", async () => {
    jest.mocked(SecureStore.getItemAsync).mockResolvedValue(null);
    jest.mocked(SecureStore.setItemAsync).mockResolvedValue();
    const store = createMemoryStore();
    store.put("dayLogs", dayLog("a", "2026-11-10"));
    store.setKv("syncCursor", "7");
    const { result } = await renderHook(() => useSession(), { wrapper: SessionProvider });
    await waitFor(() => expect(result.current.status).toBe("signedOut"));
    await act(() => result.current.setServer(MOCK_SERVER));
    await act(() => result.current.login("token-1", create(UserSchema, { username: "demo" })));
    expect(result.current.status).toBe("signedIn");

    await act(() => signOut(result.current, store));

    expect(result.current).toMatchObject({ status: "signedOut", serverUrl: MOCK_SERVER });
    expect(store.list("dayLogs")).toEqual([]);
    expect(store.getKv("syncCursor")).toBeUndefined();
    expect(fetch).not.toHaveBeenCalled();
  });
});
