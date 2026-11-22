import { defineConfig } from "@playwright/test";

export default defineConfig({
  testDir: "e2e",
  timeout: 30_000,
  retries: process.env.CI ? 1 : 0,
  use: {
    baseURL: "http://localhost:3100",
    trace: "retain-on-failure",
  },
  webServer: {
    command: "pnpm start -p 3100",
    url: "http://localhost:3100/workflows",
    reuseExistingServer: !process.env.CI,
    timeout: 60_000,
  },
});
