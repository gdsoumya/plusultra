import { Linking, Platform } from "react-native";

import { useSettings } from "../../data/settings";
import { Chip, ChipStrip, ListCard, ListRow, Text } from "../../ui";
import { SubScreen } from "./parts";

const volumes = [
  { label: "Quiet", value: 0.3 },
  { label: "Normal", value: 0.6 },
  { label: "Loud", value: 1 },
];

// Android only lets a notification through Do Not Disturb once the user has allowed the app in the
// system's DND access list; the setting opens that page. Other platforms have no such switch.
const openDndAccess = () =>
  Platform.OS === "android" &&
  Linking.sendIntent("android.settings.NOTIFICATION_POLICY_ACCESS_SETTINGS").catch(() => {});

export default function TimersScreen() {
  const [settings, update] = useSettings();
  const onOff = (on: boolean) => (on ? "On" : "Off");
  return (
    <SubScreen title="Timers and screen" testID="timers-screen">
      <ListCard>
        <ListRow
          icon="sunny-outline"
          title="Keep screen on"
          subtitle="While the app is in front; never in the background"
          value={onOff(settings.keepAwake)}
          onPress={() => update({ keepAwake: !settings.keepAwake })}
          testID="keep-awake"
        />
        <ListRow
          icon="phone-portrait-outline"
          title="Vibration"
          subtitle="With the timer beeps"
          value={onOff(settings.vibrate)}
          onPress={() => update({ vibrate: !settings.vibrate })}
          testID="vibrate"
        />
        <ListRow
          icon="notifications-outline"
          title="Alert in Do Not Disturb"
          subtitle="Timer notifications get through DND once the app has DND access"
          value={onOff(settings.alertInDnd)}
          onPress={() => {
            update({ alertInDnd: !settings.alertInDnd });
            if (!settings.alertInDnd) void openDndAccess();
          }}
          testID="alert-in-dnd"
        />
      </ListCard>
      <Text variant="label" color="textMuted">
        Timer volume
      </Text>
      <ChipStrip selected={volumes.findIndex((v) => v.value === settings.volume)}>
        {volumes.map((v) => (
          <Chip
            key={v.label}
            label={v.label}
            selected={v.value === settings.volume}
            onPress={() => update({ volume: v.value })}
          />
        ))}
      </ChipStrip>
      <Text variant="caption">
        Beeps and vibration play while the workout is on screen. With the screen off, the end of a
        timer comes as a notification with the same sound, at the notification volume of the phone.
      </Text>
    </SubScreen>
  );
}
