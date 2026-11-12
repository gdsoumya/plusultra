/* global jest */
require("react-native-reanimated").setUpTests();

jest.mock("react-native-worklets", () => require("react-native-worklets/lib/module/mock"));

jest.mock(
  "react-native-safe-area-context",
  () => require("react-native-safe-area-context/jest/mock").default,
);

// Screens rendered without a StoreProvider get a memory store, as the web build does.
jest.mock("./src/data/createStore", () => require("./src/data/createStore.web"));

// No native audio in jest; screens that import the beeps still mount.
jest.mock("expo-audio", () => ({
  setAudioModeAsync: async () => {},
  createAudioPlayer: () => ({ seekTo() {}, play() {} }),
}));

// jest-expo's native notifications mock hands out listeners without remove(), so a second
// handler registration (route modules loaded twice by the router's test renderer) would throw.
jest.mock("expo-notifications", () => ({
  ...jest.requireActual("expo-notifications"),
  setNotificationHandler: jest.fn(),
}));
