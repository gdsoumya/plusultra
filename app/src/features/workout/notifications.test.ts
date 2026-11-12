import * as Notifications from "expo-notifications";

import config from "../../../app.json";
import { setTimerOnScreen } from "./notifications";

jest.mock("expo-notifications", () => ({ setNotificationHandler: jest.fn() }));

describe("timer notifications", () => {
  it("may use exact alarms on Android, so a rest's notification is not minutes late", () => {
    expect(config.expo.android.permissions).toEqual(
      expect.arrayContaining([
        "android.permission.SCHEDULE_EXACT_ALARM",
        "android.permission.USE_EXACT_ALARM",
      ]),
    );
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
