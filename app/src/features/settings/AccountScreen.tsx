import { useState } from "react";

import { useApi } from "../../api/useApi";
import { useSession } from "../../data/session";
import { useStore } from "../../data/StoreProvider";
import { formatWeight } from "../../domain/units";
import { Button, Card, Chip, ChipStrip, ErrorLine, Field, Text } from "../../ui";
import { DetailLine, SubScreen } from "./parts";
import { useCall } from "./useRemote";
import { signOut } from "./useServer";
import { syncUserSettings } from "./syncUserSettings";
import { useSettings, type MeasurementUnit } from "../../data/settings";

const weightUnits = ["kg", "lb"] as const;
const lengthUnits: MeasurementUnit[] = ["cm", "in"];

export default function AccountScreen() {
  const api = useApi();
  const session = useSession();
  const store = useStore();
  const [settings, save] = useSettings();
  const update = (patch: Partial<typeof settings>) => {
    save(patch);
    void syncUserSettings(api, { ...settings, ...patch });
  };
  const [displayName, setDisplayName] = useState(settings.displayName);
  const [current, setCurrent] = useState("");
  const [next, setNext] = useState("");
  const [changed, setChanged] = useState(false);
  const { call, busy, error } = useCall();

  const changePassword = () =>
    call(async () => {
      await api.auth.changePassword({ currentPassword: current, newPassword: next });
      setCurrent("");
      setNext("");
      setChanged(true);
    });

  return (
    <SubScreen title="Account" testID="account-screen">
      <Card>
        <DetailLine label="Signed in as" value={session.user?.username} />
        <Field
          label="Display name"
          value={displayName}
          onChangeText={setDisplayName}
          onBlur={() => update({ displayName: displayName.trim() })}
        />
      </Card>
      <Card>
        <Text variant="label" color="textMuted">
          Weights
        </Text>
        <ChipStrip>
          {weightUnits.map((unit) => (
            <Chip
              key={unit}
              label={unit}
              selected={settings.units === unit}
              onPress={() => update({ units: unit })}
            />
          ))}
        </ChipStrip>
        <Text variant="caption">{`Weights show as ${formatWeight({ value: 100, unit: "kg" }, settings.units)}`}</Text>
        <Text variant="label" color="textMuted">
          Measurements
        </Text>
        <ChipStrip>
          {lengthUnits.map((unit) => (
            <Chip
              key={unit}
              label={unit}
              selected={settings.measurementUnit === unit}
              onPress={() => update({ measurementUnit: unit })}
            />
          ))}
        </ChipStrip>
      </Card>
      <Card>
        <Text variant="heading">Password</Text>
        <Field label="Current password" secureTextEntry value={current} onChangeText={setCurrent} />
        <Field
          label="New password"
          helper="At least 10 characters"
          secureTextEntry
          value={next}
          onChangeText={(text) => {
            setNext(text);
            setChanged(false);
          }}
        />
        {changed && <Text color="success">Password changed</Text>}
        <ErrorLine message={error} />
        <Button
          title="Change password"
          variant="secondary"
          onPress={changePassword}
          loading={busy}
          disabled={!current || !next}
        />
      </Card>
      <Button
        title="Log out"
        variant="danger"
        icon="log-out-outline"
        onPress={() => void signOut(session, store)}
      />
    </SubScreen>
  );
}
