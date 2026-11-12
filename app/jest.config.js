module.exports = {
  preset: "jest-expo",
  resolver: "react-native-reanimated/jest/resolver",
  setupFiles: ["react-native-gesture-handler/jestSetup.js"],
  setupFilesAfterEnv: ["./jest.setup.js"],
  testPathIgnorePatterns: ["/node_modules/", "/e2e/", "/android/"],
};
