import { uuid } from "./uuid";

import type { Store } from "./store";

export function deviceId(store: Store): string {
  let id = store.getKv<string>("deviceId");
  if (!id) {
    id = uuid();
    store.setKv("deviceId", id);
  }
  return id;
}
