import { act, renderHook } from "@testing-library/react-native";

import { useRefresh } from "./useRefresh";

const mockSync = { syncNow: jest.fn<Promise<void>, []>() };
jest.mock("./useSync", () => ({ useSync: () => mockSync }));

let finish: () => void;
beforeEach(() => {
  mockSync.syncNow.mockReset().mockImplementation(() => new Promise((r) => (finish = r)));
});

it("is refreshing until the sync settles, then reloads", async () => {
  const reload = jest.fn();
  const { result } = await renderHook(() => useRefresh(reload));
  expect(result.current.refreshing).toBe(false);
  let done: Promise<void>;
  await act(() => {
    done = result.current.refresh();
  });
  expect(result.current.refreshing).toBe(true);
  expect(reload).not.toHaveBeenCalled();
  await act(async () => {
    finish();
    await done;
  });
  expect(reload).toHaveBeenCalledTimes(1);
  expect(result.current.refreshing).toBe(false);
});

it("clears after a failed sync", async () => {
  mockSync.syncNow.mockRejectedValue(new Error("offline"));
  const { result } = await renderHook(() => useRefresh());
  await act(() => result.current.refresh());
  expect(mockSync.syncNow).toHaveBeenCalledTimes(1);
  expect(result.current.refreshing).toBe(false);
});
