import * as Notifications from "expo-notifications";

import config from "../../../app.json";
import {
  askTimerPermission,
  scheduleTimerEnd,
  setAlertInDnd,
  setTimerOnScreen,
  TIMER_SOUND,
} from "./notifications";

jest.mock("expo-notifications", () => ({
  setNotificationHandler: jest.fn(),
  setNotificationChannelAsync: jest.fn().mockResolvedValue(null),
  deleteNotificationChannelAsync: jest.fn().mockResolvedValue(undefined),
  requestPermissionsAsync: jest.fn().mockResolvedValue({ granted: true }),
  getPermissionsAsync: jest.fn().mockResolvedValue({ granted: true }),
  scheduleNotificationAsync: jest.fn().mockResolvedValue("n1"),
  AndroidImportance: { MAX: 5 },
  SchedulableTriggerInputTypes: { DATE: "date" },
}));

describe("timer notifications", () => {
  it("may use exact alarms on Android, so a rest's notification is not minutes late", () => {
    expect(config.expo.android.permissions).toEqual(
      expect.arrayContaining([
        "android.permission.SCHEDULE_EXACT_ALARM",
        "android.permission.USE_EXACT_ALARM",
      ]),
    );
  });

  it("asks for permission up front, never while a timer is being scheduled", async () => {
    const ask = jest.mocked(Notifications.requestPermissionsAsync);
    askTimerPermission();
    expect(ask).toHaveBeenCalledTimes(1);
    jest
      .mocked(Notifications.getPermissionsAsync)
      .mockResolvedValueOnce({ granted: false } as never);
    expect(await scheduleTimerEnd(Date.now() + 60_000, { title: "t", body: "b" })).toBeUndefined();
    expect(ask).toHaveBeenCalledTimes(1);
  });

  it("ships the end beep as the notification sound and keeps it through shrinking", () => {
    expect(config.expo.plugins).toContainEqual([
      "expo-notifications",
      expect.objectContaining({ sounds: ["./assets/sounds/end.wav"] }),
    ]);
    expect(config.expo.plugins).toContain("./plugins/withKeepNotificationSound");
  });

  it("schedules on a loud channel with the beep, and a DND-bypassing one when asked", async () => {
    const channel = jest.mocked(Notifications.setNotificationChannelAsync);
    const schedule = jest.mocked(Notifications.scheduleNotificationAsync);
    const notice = { title: "Rest over", body: "Next set" };
    expect(await scheduleTimerEnd(Date.now() + 60_000, notice)).toBe("n1");
    // The channels of builds that shipped without the sound resource go, since Android would
    // keep their system sound.
    for (const old of ["timers", "timers-priority"]) {
      expect(Notifications.deleteNotificationChannelAsync).toHaveBeenCalledWith(old);
    }
    expect(channel).toHaveBeenCalledWith(
      "timers-2",
      expect.objectContaining({ sound: TIMER_SOUND, bypassDnd: false }),
    );
    expect(schedule).toHaveBeenCalledWith({
      content: { ...notice, sound: TIMER_SOUND },
      trigger: expect.objectContaining({ channelId: "timers-2" }),
    });
    // The channel is made once, not per timer.
    await scheduleTimerEnd(Date.now() + 60_000, notice);
    expect(channel).toHaveBeenCalledTimes(1);

    setAlertInDnd(true);
    await scheduleTimerEnd(Date.now() + 60_000, notice);
    expect(channel).toHaveBeenLastCalledWith(
      "timers-priority-2",
      expect.objectContaining({ bypassDnd: true }),
    );
    expect(schedule).toHaveBeenLastCalledWith(
      expect.objectContaining({
        trigger: expect.objectContaining({ channelId: "timers-priority-2" }),
      }),
    );
    setAlertInDnd(false);
  });

  it("stay quiet while the workout screen shows the timer, and speak anywhere else", async () => {
    const [[handler]] = jest.mocked(Notifications.setNotificationHandler).mock.calls;
    const behavior = () => handler!.handleNotification({} as Notifications.Notification);
    const quiet = { shouldShowBanner: false, shouldShowList: false, shouldPlaySound: false };
    setTimerOnScreen(true);
    expect(await behavior()).toMatchObject(quiet);
    setTimerOnScreen(false);
    expect(await behavior()).toMatchObject({
      shouldShowBanner: true,
      shouldShowList: true,
      shouldPlaySound: true,
    });
  });
});
