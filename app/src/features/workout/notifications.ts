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

// The end beep ships as a notification sound (app.json, expo-notifications `sounds`), so a timer
// that ends with the screen off is as loud as one on screen. Android fixes a channel's sound and
// DND behaviour at creation, so the DND-bypassing variant is a channel of its own.
export const TIMER_SOUND = "end.wav";
let alertInDnd = false;
export const setAlertInDnd = (on: boolean) => {
  alertInDnd = on;
};
const channels = new Map<string, Promise<unknown>>();
function timerChannel() {
  const id = alertInDnd ? "timers-priority" : "timers";
  if (!channels.has(id))
    channels.set(
      id,
      Promise.resolve()
        .then(() =>
          Notifications.setNotificationChannelAsync(id, {
            name: alertInDnd ? "Timers (in Do Not Disturb)" : "Timers",
            // eslint-disable-next-line import/namespace -- re-exported from a .types module
            importance: Notifications.AndroidImportance.MAX,
            sound: TIMER_SOUND,
            vibrationPattern: [0, 300, 150, 300],
            bypassDnd: alertInDnd,
          }),
        )
        .catch(() => {}),
    );
  return channels.get(id)!.then(() => id);
}

// Asked when a workout opens: the system prompt pauses the activity, and with it the JS timers,
// so it must not come up as the first set starts.
export const askTimerPermission = () => {
  if (Platform.OS !== "web") Notifications.requestPermissionsAsync().catch(() => {});
};

// Web has no local notifications; the bar on screen is the only timer there. A reminder that
// cannot be scheduled (permission denied, module missing) is dropped the same way.
export async function scheduleTimerEnd(
  endsAt: number,
  content: Notice,
): Promise<string | undefined> {
  if (Platform.OS === "web") return undefined;
  try {
    const { granted } = await Notifications.getPermissionsAsync();
    if (!granted) return undefined;
    const channelId = await timerChannel();
    return await Notifications.scheduleNotificationAsync({
      content: { ...content, sound: TIMER_SOUND },
      trigger: { type: Notifications.SchedulableTriggerInputTypes.DATE, date: endsAt, channelId },
    });
  } catch {
    return undefined;
  }
}

export const cancelTimerEnd = (id: string) =>
  Notifications.cancelScheduledNotificationAsync(id).catch(() => {});
