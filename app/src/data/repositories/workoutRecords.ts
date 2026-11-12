import type { Store } from "../store";
import { syncRepo } from "./syncRepo";

export const workoutRecordsRepo = (store: Store) => syncRepo(store, "workoutRecords");
