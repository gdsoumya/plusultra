import type { Store } from "../store";
import { syncRepo } from "./syncRepo";

export const dayLogsRepo = (store: Store) => syncRepo(store, "dayLogs");
