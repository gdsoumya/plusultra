import { defineConfig } from "@playwright/test";

export default defineConfig({
  testDir: "e2e/web",
  webServer: {
    command: "npx expo export --platform web && node e2e/web/serve.mjs 8082",
    url: "http://localhost:8082",
    reuseExistingServer: false,
    timeout: 300_000,
  },
  use: { baseURL: "http://localhost:8082" },
  // Every spec at a phone and a desktop width (P2-R14).
  projects: [
    { name: "phone", use: { viewport: { width: 390, height: 844 } } },
    { name: "desktop", use: { viewport: { width: 1280, height: 800 } } },
  ],
});
