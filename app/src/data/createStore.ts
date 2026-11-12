import { openDatabaseSync } from "expo-sqlite";

import { createSqliteStore } from "./sqliteStore";

export const createStore = () => createSqliteStore(openDatabaseSync("plusultra.db"));
