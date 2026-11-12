import { useRouter } from "expo-router";
import { useState } from "react";

import { credentials } from "../../api/fixtures/user";
import { useApi } from "../../api/useApi";
import { MOCK_SERVER, useSession } from "../../data/session";
import { Button, ErrorLine, Field, Screen, Text } from "../../ui";
import { signIn } from "../settings/useServer";
import { useSubmit } from "./useSubmit";

export default function LoginScreen() {
  const session = useSession();
  const api = useApi();
  const router = useRouter();
  const demo = session.serverUrl === MOCK_SERVER;
  const [username, setUsername] = useState(demo ? credentials.username : "");
  const [password, setPassword] = useState(demo ? credentials.password : "");
  const { busy, error, submit } = useSubmit();
  const login = () =>
    submit(async () => signIn(session, await api.auth.login({ username, password })));
  return (
    <Screen title="Sign in" testID="login-screen">
      <Text variant="caption" numberOfLines={1} ellipsizeMode="middle">
        {`Server: ${session.serverUrl}`}
      </Text>
      {session.expired && <Text color="warning">Session expired. Sign in again.</Text>}
      <Field
        label="Username"
        value={username}
        onChangeText={setUsername}
        autoCapitalize="none"
        autoCorrect={false}
        autoComplete="username"
        textContentType="username"
      />
      <Field
        label="Password"
        value={password}
        onChangeText={setPassword}
        secureTextEntry
        autoComplete="current-password"
        textContentType="password"
        returnKeyType="go"
        onSubmitEditing={login}
      />
      <ErrorLine message={error} />
      <Button title="Sign in" onPress={login} loading={busy} />
      <Button
        title="Create an account"
        variant="ghost"
        onPress={() => router.navigate("/register")}
      />
      <Button title="Change server" variant="ghost" onPress={() => router.navigate("/server")} />
    </Screen>
  );
}
