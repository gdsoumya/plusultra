import * as Notifications from "expo-notifications";
import { Platform } from "react-native";

export type Notice = { title: string; body: string };

// The open workout screen shows its timers and beeps when they end, so a notification that comes
// while it is in front stays out of the way; on any other screen it shows as usual.
let timerOnScreen = false;
export const setTimerOnScreen = (shown: boolean) => {
  timerOnScreen = shown;
};
if (Platform.OS !== "web") {
  Notifications.setNotificationHandler({
    handleNotification: async () => ({
      shouldShowBanner: !timerOnScreen,
      shouldShowList: !timerOnScreen,
      shouldPlaySound: !timerOnScreen,
      shouldSetBadge: false,
    }),
  });
}

// Web has no local notifications; the bar on screen is the only timer there. A reminder that
// cannot be scheduled (permission denied, module missing) is dropped the same way.
export async function scheduleTimerEnd(
  endsAt: number,
  content: Notice,
): Promise<string | undefined> {
  if (Platform.OS === "web") return undefined;
  try {
    const { granted } = await Notifications.requestPermissionsAsync();
    if (!granted) return undefined;
    return await Notifications.scheduleNotificationAsync({
      content,
      trigger: { type: Notifications.SchedulableTriggerInputTypes.DATE, date: endsAt },
    });
  } catch {
    return undefined;
  }
}

export const cancelTimerEnd = (id: string) =>
  Notifications.cancelScheduledNotificationAsync(id).catch(() => {});
