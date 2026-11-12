import type { ProgramSummary } from "../../domain/types";
import type { Store } from "../store";

export function programsRepo(store: Store) {
  return {
    get: (id: string) => store.get("programs", id),
    all: () => store.list("programs"),
    remove: (id: string) => store.delete("programs", id),
    upsertFromServer: (rows: ProgramSummary[]) =>
      store.transaction(() => rows.forEach((row) => store.put("programs", row))),
  };
}
