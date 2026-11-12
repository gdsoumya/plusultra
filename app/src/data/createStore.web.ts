// The web app is online only, so its store lives in memory and keeps expo-sqlite out of the web bundle.
export { createMemoryStore as createStore } from "./memoryStore";
