import { timestampDate } from "@bufbuild/protobuf/wkt";
import { useCallback, useState } from "react";

import { useApi } from "../../api/useApi";
import { localDate } from "../../domain/dates";
import { ApiKeyScope, type ApiKey } from "../../gen/plusultra/v1/apikey_pb";
import {
  Button,
  ConfirmSheet,
  EmptyState,
  ErrorLine,
  ListCard,
  ListRow,
  Loading,
  Text,
} from "../../ui";
import { NewKeySheet, type NewKeyRequest } from "./NewKeySheet";
import { SubScreen } from "./parts";
import { useRemote } from "./useRemote";

const scopeNames: Partial<Record<ApiKeyScope, string>> = {
  [ApiKeyScope.READ]: "read",
  [ApiKeyScope.WRITE]: "write",
  [ApiKeyScope.ADMIN]: "admin",
};

export default function ApiKeysScreen() {
  const api = useApi();
  const load = useCallback(() => api.apiKeys.listApiKeys({}).then((r) => r.apiKeys), [api]);
  const { data: keys = [], setData, loading, busy, error, call } = useRemote(load);
  const [creating, setCreating] = useState(false);
  const [opened, setOpened] = useState(0);
  const [secret, setSecret] = useState<string>();
  const [revoking, setRevoking] = useState<ApiKey>();
  const [confirmOpen, setConfirmOpen] = useState(false);

  const describe = (k: ApiKey) =>
    [
      `lsk_${k.prefix}`,
      k.scopes.map((s) => scopeNames[s] ?? "?").join(", "),
      k.revokedAt ? "revoked" : k.expiresAt && `expires ${localDate(timestampDate(k.expiresAt))}`,
    ]
      .filter(Boolean)
      .join(" · ");

  const create = (request: NewKeyRequest) =>
    call(async () => {
      const r = await api.apiKeys.createApiKey(request);
      setData([r.apiKey!, ...keys]);
      setSecret(r.key);
    });

  const revoke = () =>
    call(async () => {
      const r = await api.apiKeys.revokeApiKey({ id: revoking!.id });
      setData(keys.map((k) => (k.id === r.apiKey?.id ? r.apiKey : k)));
      setConfirmOpen(false);
    });

  const sheetOpen = creating || confirmOpen;

  return (
    <SubScreen title="API keys" testID="api-keys-screen">
      <Text color="textMuted">
        Keys let scripts and LLM clients use your data over the REST and MCP APIs. Tap a key to
        revoke it.
      </Text>
      {!sheetOpen && <ErrorLine message={error} />}
      {loading ? (
        <Loading />
      ) : keys.length ? (
        <ListCard>
          {keys.map((k) => (
            <ListRow
              key={k.id}
              title={k.name}
              subtitle={describe(k)}
              onPress={
                k.revokedAt
                  ? undefined
                  : () => {
                      setRevoking(k);
                      setConfirmOpen(true);
                    }
              }
            />
          ))}
        </ListCard>
      ) : (
        !error && <EmptyState icon="key-outline" title="No keys yet" />
      )}
      <Button
        title="New key"
        icon="add"
        variant="secondary"
        onPress={() => {
          setOpened((n) => n + 1);
          setSecret(undefined);
          setCreating(true);
        }}
      />
      <NewKeySheet
        key={opened}
        open={creating}
        secret={secret}
        busy={busy}
        error={error}
        onCreate={create}
        onClose={() => setCreating(false)}
      />
      {revoking && (
        <ConfirmSheet
          open={confirmOpen}
          title={`Revoke ${revoking.name}?`}
          message="Anything using this key stops working at once. This cannot be undone."
          action={`Revoke ${revoking.name}`}
          busy={busy}
          error={error}
          onConfirm={revoke}
          onClose={() => setConfirmOpen(false)}
        />
      )}
    </SubScreen>
  );
}
