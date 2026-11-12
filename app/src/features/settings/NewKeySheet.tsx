import { timestampFromDate } from "@bufbuild/protobuf/wkt";
import { useState } from "react";

import { ApiKeyScope, type CreateApiKeyRequest } from "../../gen/plusultra/v1/apikey_pb";
import { Button, Card, Chip, ChipStrip, ErrorLine, Field, Sheet, Text } from "../../ui";
import { copyText } from "./share";

const scopes = [
  { scope: ApiKeyScope.READ, label: "Read" },
  { scope: ApiKeyScope.WRITE, label: "Write" },
];

// The server allows at most 365 days.
const expiries = [
  { days: 30, label: "30 days" },
  { days: 90, label: "90 days" },
  { days: 365, label: "1 year" },
];

export type NewKeyRequest = Pick<CreateApiKeyRequest, "name" | "scopes" | "expiresAt">;

export function NewKeySheet({
  open,
  secret,
  busy,
  error,
  onCreate,
  onClose,
}: {
  open: boolean;
  /** The plaintext key once created; the sheet then shows it instead of the form. */
  secret?: string;
  busy: boolean;
  error?: string;
  onCreate: (request: NewKeyRequest) => void;
  onClose: () => void;
}) {
  const [name, setName] = useState("");
  const [copyFailed, setCopyFailed] = useState(false);
  const [granted, setGranted] = useState([ApiKeyScope.READ]);
  const [days, setDays] = useState(90);
  const [problem, setProblem] = useState<string>();

  const toggle = (scope: ApiKeyScope) =>
    setGranted((all) => (all.includes(scope) ? all.filter((s) => s !== scope) : [...all, scope]));

  const submit = () => {
    if (!name.trim()) return setProblem("Give the key a name");
    if (!granted.length) return setProblem("Pick at least one scope");
    setProblem(undefined);
    onCreate({
      name: name.trim(),
      scopes: scopes.map((s) => s.scope).filter((s) => granted.includes(s)),
      expiresAt: timestampFromDate(new Date(Date.now() + days * 86_400_000)),
    });
  };

  return (
    <Sheet
      open={open}
      onClose={onClose}
      title={secret ? name : "New key"}
      snapPoints={[0.75]}
      footer={
        secret ? (
          <Button title="Done" variant="secondary" onPress={onClose} />
        ) : (
          <Button title="Create key" onPress={submit} loading={busy} />
        )
      }
    >
      {secret ? (
        <>
          <Card>
            <Text variant="label">Your key</Text>
            <Text selectable variant="mono">
              {secret}
            </Text>
            <Text variant="caption">
              It is shown only once. Copy it now and send it as Authorization: Bearer.
            </Text>
          </Card>
          {copyFailed && <ErrorLine message="Copy is not available here; select the key above." />}
          <Button
            title="Copy"
            icon="copy-outline"
            onPress={() =>
              void copyText(secret)
                .then((ok) => setCopyFailed(!ok))
                .catch(() => setCopyFailed(true))
            }
          />
        </>
      ) : (
        <>
          <Field
            label="Name"
            helper="Where the key is used, for example claude desktop"
            value={name}
            onChangeText={setName}
          />
          <Text variant="label" color="textMuted">
            Scope
          </Text>
          <ChipStrip>
            {scopes.map(({ scope, label }) => (
              <Chip
                key={scope}
                label={label}
                selected={granted.includes(scope)}
                onPress={() => toggle(scope)}
              />
            ))}
          </ChipStrip>
          <Text variant="label" color="textMuted">
            Expires in
          </Text>
          <ChipStrip selected={expiries.findIndex((e) => e.days === days)}>
            {expiries.map((e) => (
              <Chip
                key={e.days}
                label={e.label}
                selected={days === e.days}
                onPress={() => setDays(e.days)}
              />
            ))}
          </ChipStrip>
          <ErrorLine message={problem ?? error} />
        </>
      )}
    </Sheet>
  );
}
