import type { Store } from "../store";
import { syncRepo } from "./syncRepo";

export const checkinsRepo = (store: Store) => syncRepo(store, "checkins");
