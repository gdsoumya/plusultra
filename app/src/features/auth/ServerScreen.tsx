import { useEffect, useState } from "react";

import { Button, ErrorLine, Field, Screen, Text } from "../../ui";
import { defaultServerUrl, useServer } from "../settings/useServer";
import { useSubmit } from "./useSubmit";

export default function ServerScreen() {
  const { serverUrl, connect, tryDemo } = useServer();
  const [url, setUrl] = useState(serverUrl ?? defaultServerUrl() ?? "");
  const { busy, error, submit } = useSubmit();
  const go = () => submit(() => connect(url));
  // Hosted web: the page's origin is tried once, quietly; the field stays for another address.
  useEffect(() => {
    const origin = defaultServerUrl();
    if (!serverUrl && origin) connect(origin).catch(() => {});
  }, []); // eslint-disable-line react-hooks/exhaustive-deps
  return (
    <Screen title="Your server" testID="server-screen">
      <Text>PlusUltra runs on a server you host. Enter its address to get started.</Text>
      <Field
        label="Server address"
        value={url}
        onChangeText={setUrl}
        placeholder="https://lift.example.com"
        autoCapitalize="none"
        autoCorrect={false}
        keyboardType="url"
        autoComplete="url"
        returnKeyType="go"
        onSubmitEditing={go}
      />
      <ErrorLine message={error} />
      <Button title={error ? "Retry" : "Connect"} onPress={go} loading={busy} />
      {__DEV__ && <Button title="Try the demo" variant="ghost" onPress={tryDemo} />}
    </Screen>
  );
}
