import { create } from "@bufbuild/protobuf";
import { timestampMs } from "@bufbuild/protobuf/wkt";
import type { ServiceImpl } from "@connectrpc/connect";

import { ApiKeySchema, ApiKeyService } from "../../gen/plusultra/v1/apikey_pb";
import { invalid, newId, notFound, nowStamp, type MockDb } from "./db";

const year = 365 * 86_400_000;

export function apiKeys(db: MockDb): ServiceImpl<typeof ApiKeyService> {
  return {
    createApiKey({ name, scopes, expiresAt }) {
      const ahead = expiresAt ? timestampMs(expiresAt) - Date.now() : 0;
      if (!name || name.length > 64) throw invalid("name must be 1 to 64 characters");
      if (!scopes.length) throw invalid("at least one scope is required");
      if (!(ahead > 0 && ahead <= year)) throw invalid("expiry must be within the next 365 days");
      const id = newId(db, "key");
      const prefix = id.replace(/\W/g, "").padStart(8, "0").slice(-8);
      const apiKey = create(ApiKeySchema, {
        id,
        name,
        prefix,
        scopes,
        expiresAt,
        createdAt: nowStamp(),
      });
      db.apiKeys.unshift(apiKey);
      return { key: `lsk_${prefix}_mocksecret${db.ids}`, apiKey };
    },
    listApiKeys: () => ({ apiKeys: db.apiKeys }),
    revokeApiKey({ id }) {
      const apiKey = db.apiKeys.find((k) => k.id === id);
      if (!apiKey) throw notFound(`api key ${id}`);
      apiKey.revokedAt ??= nowStamp();
      return { apiKey };
    },
  };
}
