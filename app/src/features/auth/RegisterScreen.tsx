import { useRouter } from "expo-router";
import { useState } from "react";

import { useApi } from "../../api/useApi";
import { useSession } from "../../data/session";
import { Button, ErrorLine, Field, Screen } from "../../ui";
import { signIn } from "../settings/useServer";
import { useSubmit } from "./useSubmit";

const empty = { displayName: "", username: "", password: "", inviteCode: "" };

export default function RegisterScreen() {
  const session = useSession();
  const api = useApi();
  const router = useRouter();
  const [form, setForm] = useState(empty);
  const { busy, error, submit } = useSubmit();
  const set = (key: keyof typeof empty) => (value: string) => setForm({ ...form, [key]: value });
  const register = () =>
    submit(async () => {
      const timezone = Intl.DateTimeFormat().resolvedOptions().timeZone;
      signIn(session, await api.auth.register({ ...form, timezone }));
    });
  return (
    <Screen title="Create account" testID="register-screen">
      <Field
        label="Display name"
        value={form.displayName}
        onChangeText={set("displayName")}
        autoComplete="name"
      />
      <Field
        label="Username"
        value={form.username}
        onChangeText={set("username")}
        helper="3 to 32 lowercase letters, digits or underscores"
        autoCapitalize="none"
        autoCorrect={false}
        autoComplete="username-new"
      />
      <Field
        label="Password"
        value={form.password}
        onChangeText={set("password")}
        helper="At least 10 characters"
        secureTextEntry
        autoComplete="new-password"
        textContentType="newPassword"
      />
      <Field
        label="Invite code"
        value={form.inviteCode}
        onChangeText={set("inviteCode")}
        helper="The first account on a new server needs none"
        autoCapitalize="none"
        autoCorrect={false}
        returnKeyType="go"
        onSubmitEditing={register}
      />
      <ErrorLine message={error} />
      <Button title="Create account" onPress={register} loading={busy} />
      <Button title="Sign in instead" variant="ghost" onPress={() => router.navigate("/login")} />
    </Screen>
  );
}
