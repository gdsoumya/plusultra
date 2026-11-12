import type { Store } from "../store";
import { syncRepo } from "./syncRepo";

export const mealLogsRepo = (store: Store) => syncRepo(store, "mealLogs");
