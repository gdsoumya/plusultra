import Constants from "expo-constants";
import { useCallback } from "react";

import { MOCK_SERVER, useSession } from "../../data/session";
import { Card, ErrorLine } from "../../ui";
import { DetailLine, SubScreen } from "./parts";
import { useRemote } from "./useRemote";

export default function AboutScreen() {
  const { serverUrl } = useSession();
  // healthz answers "ok <version>".
  const load = useCallback(async () => {
    if (serverUrl === MOCK_SERVER) return "demo";
    const res = await fetch(`${serverUrl}/healthz`);
    if (!res.ok) throw new Error(`The server answered ${res.status}`);
    return (await res.text()).trim().split(/\s+/)[1] ?? "unknown";
  }, [serverUrl]);
  const { data: version, loading, error } = useRemote(load);

  return (
    <SubScreen title="About" testID="about-screen">
      <Card>
        <DetailLine label="Server" value={serverUrl ?? "none"} />
        <DetailLine label="Server version" value={loading ? "..." : version} />
        <ErrorLine message={error} />
        <DetailLine label="App version" value={Constants.expoConfig?.version ?? "dev"} />
      </Card>
    </SubScreen>
  );
}
