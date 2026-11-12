import { act, renderHook } from "@testing-library/react-native";
import * as Notifications from "expo-notifications";

import { useTimer } from "./useTimer";

jest.mock("expo-notifications", () => ({
  SchedulableTriggerInputTypes: { DATE: "date" },
  setNotificationHandler: jest.fn(),
  requestPermissionsAsync: jest.fn(async () => ({ granted: true })),
  scheduleNotificationAsync: jest.fn(async () => "n1"),
  cancelScheduledNotificationAsync: jest.fn(async () => {}),
}));

const notice = { title: "Set is over", body: "Stop and rest." };
const restNotice = { title: "Rest is over", body: "Time for the next set." };
const tick = (ms: number) => act(() => jest.advanceTimersByTime(ms));

beforeEach(() => {
  jest.useFakeTimers();
  jest.clearAllMocks();
});
afterEach(() => jest.useRealTimers());

describe("useTimer", () => {
  it("counts a set down, then ends it once with its meta", async () => {
    const onEnd = jest.fn();
    const { result } = await renderHook(() => useTimer<string>(onEnd));
    expect(result.current.running).toBeUndefined();
    const startedAt = Date.now();
    await act(() => result.current.start("set", 3, notice));
    expect(result.current).toMatchObject({ running: "set", remaining: 3, elapsed: 0 });
    await tick(1000);
    expect(result.current).toMatchObject({ remaining: 2, elapsed: 1 });
    expect(onEnd).not.toHaveBeenCalled();
    await tick(2000);
    expect(onEnd).toHaveBeenCalledTimes(1);
    expect(onEnd).toHaveBeenCalledWith("set", startedAt + 3000);
    expect(result.current.running).toBeUndefined();
    await tick(5000);
    expect(onEnd).toHaveBeenCalledTimes(1);
    expect(Notifications.scheduleNotificationAsync).toHaveBeenCalledWith({
      content: notice,
      trigger: { type: "date", date: startedAt + 3000 },
    });
    expect(Notifications.cancelScheduledNotificationAsync).not.toHaveBeenCalled();
  });

  it("stops early with the elapsed seconds, at least one, and cancels the notification", async () => {
    const onEnd = jest.fn();
    const { result } = await renderHook(() => useTimer<string>(onEnd));
    await act(() => result.current.start("set", 60, notice));
    let elapsed = 0;
    await act(async () => {
      elapsed = result.current.stop();
    });
    expect(elapsed).toBe(1);
    expect(result.current.running).toBeUndefined();
    expect(onEnd).not.toHaveBeenCalled();
    expect(Notifications.cancelScheduledNotificationAsync).toHaveBeenCalledWith("n1");

    await act(() => result.current.start("set", 60, notice));
    await tick(2400);
    await act(async () => {
      elapsed = result.current.stop();
    });
    expect(elapsed).toBe(2);
  });

  it("counts up without an end or a notification", async () => {
    const onEnd = jest.fn();
    const { result } = await renderHook(() => useTimer<string>(onEnd));
    await act(() => result.current.start("hang"));
    await tick(45_000);
    expect(result.current).toMatchObject({ running: "hang", elapsed: 45, remaining: 0 });
    expect(onEnd).not.toHaveBeenCalled();
    expect(Notifications.scheduleNotificationAsync).not.toHaveBeenCalled();
  });

  it("adjusts the end by seconds, never below one second left, and reschedules the notice", async () => {
    const onEnd = jest.fn();
    const { result } = await renderHook(() => useTimer<string>(onEnd));
    const startedAt = Date.now();
    await act(() => result.current.start("rest", 60, notice));
    await act(() => result.current.adjust(10));
    expect(result.current.remaining).toBe(70);
    await act(async () => {});
    expect(Notifications.cancelScheduledNotificationAsync).toHaveBeenCalledWith("n1");
    expect(Notifications.scheduleNotificationAsync).toHaveBeenLastCalledWith({
      content: notice,
      trigger: { type: "date", date: startedAt + 70_000 },
    });
    await act(() => result.current.adjust(-10));
    expect(result.current.remaining).toBe(60);
    await tick(5000);
    await act(() => result.current.adjust(-100));
    expect(result.current.remaining).toBe(1);
    await tick(1000);
    expect(onEnd).toHaveBeenCalledWith("rest", expect.any(Number));
    expect(result.current.running).toBeUndefined();
  });

  it("runs one timer at a time", async () => {
    const onEnd = jest.fn();
    const { result } = await renderHook(() => useTimer<string>(onEnd));
    await act(() => result.current.start("rest", 30, notice));
    await act(() => result.current.start("set", 10, notice));
    expect(result.current).toMatchObject({ running: "set", remaining: 10 });
    expect(Notifications.cancelScheduledNotificationAsync).toHaveBeenCalledWith("n1");
    await tick(10_000);
    expect(onEnd).toHaveBeenCalledTimes(1);
    expect(onEnd).toHaveBeenCalledWith("set", expect.any(Number));
  });

  it("reports when a timer ended while the app was away, and the next one counts from then", async () => {
    const onEnd = jest.fn();
    const { result } = await renderHook(() => useTimer<string>(onEnd));
    const startedAt = Date.now();
    await act(() => result.current.start("set", 10, notice));
    // In the background no interval runs; the first tick comes when the app is back, 26 s on.
    jest.setSystemTime(startedAt + 25_000);
    await tick(1000);
    expect(onEnd).toHaveBeenCalledWith("set", startedAt + 10_000);

    jest.clearAllMocks();
    await act(() => result.current.start("rest", 30, notice, startedAt + 10_000));
    expect(result.current).toMatchObject({ running: "rest", remaining: 14, elapsed: 16 });
    expect(Notifications.scheduleNotificationAsync).toHaveBeenCalledWith({
      content: notice,
      trigger: { type: "date", date: startedAt + 40_000 },
    });

    // A timer whose end has already passed schedules no notification and ends on the next tick.
    jest.clearAllMocks();
    await act(() => result.current.start("short", 5, notice, startedAt + 10_000));
    expect(Notifications.scheduleNotificationAsync).not.toHaveBeenCalled();
    await tick(1000);
    expect(onEnd).toHaveBeenCalledWith("short", startedAt + 15_000);
  });

  it("schedules the rest after a set with it, keeps it for that rest and cancels it on an early stop", async () => {
    const scheduled = jest.mocked(Notifications.scheduleNotificationAsync);
    const cancelled = jest.mocked(Notifications.cancelScheduledNotificationAsync);
    const onEnd = jest.fn();
    const { result } = await renderHook(() => useTimer<string>(onEnd));
    const startedAt = Date.now();
    const rest = { seconds: 30, notice: restNotice };
    scheduled.mockResolvedValueOnce("work").mockResolvedValueOnce("rest");
    await act(() => result.current.start("set", 10, notice, undefined, rest));
    expect(scheduled.mock.calls).toEqual([
      [{ content: notice, trigger: { type: "date", date: startedAt + 10_000 } }],
      [{ content: restNotice, trigger: { type: "date", date: startedAt + 40_000 } }],
    ]);
    await tick(10_000);
    expect(onEnd).toHaveBeenCalledWith("set", startedAt + 10_000);
    await act(() => result.current.start("rest", 30, restNotice, startedAt + 10_000));
    expect(scheduled).toHaveBeenCalledTimes(2);
    await act(async () => void result.current.stop());
    expect(cancelled.mock.calls).toEqual([["rest"]]);

    jest.clearAllMocks();
    scheduled.mockResolvedValueOnce("work 2").mockResolvedValueOnce("rest 2");
    await act(() => result.current.start("set", 10, notice, undefined, rest));
    await act(async () => void result.current.stop());
    expect(cancelled.mock.calls).toEqual([["work 2"], ["rest 2"]]);
  });
});
