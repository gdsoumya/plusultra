import { useState } from "react";

import { useApi } from "../../api/useApi";
import { Button, Card, ErrorLine, Text } from "../../ui";
import { SubScreen } from "./parts";
import { shareZip } from "./share";
import { useCall } from "./useRemote";

const size = (bytes: number) =>
  bytes < 1024 ? `${bytes} B` : `${(bytes / 1024).toFixed(bytes < 102_400 ? 1 : 0)} KB`;

export default function ExportScreen() {
  const api = useApi();
  const { call, busy, error } = useCall();
  const [done, setDone] = useState<string>();

  const run = () =>
    call(async () => {
      const { zip, filename } = await api.export.exportAll({});
      await shareZip(zip, filename);
      setDone(`${filename} (${size(zip.length)}) is ready.`);
    });

  return (
    <SubScreen title="Export" testID="export-screen">
      <Card>
        <Text variant="heading">Everything as one zip</Text>
        <Text color="textMuted">
          Every program as a script, each day of workouts, meals and check-ins as text, and your
          custom exercises and foods. Programs import back with their progression state.
        </Text>
        <Button title="Export everything" icon="download-outline" onPress={run} loading={busy} />
        {done && <Text color="success">{done}</Text>}
        <ErrorLine message={error} />
      </Card>
    </SubScreen>
  );
}
