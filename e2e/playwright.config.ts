import { defineConfig, devices } from "@playwright/test";

// Runs inside the Playwright container that e2e/compose.e2e.yaml starts beside the app.
export default defineConfig({
  testDir: "./tests",
  globalSetup: "./global-setup.ts",
  forbidOnly: !!process.env.CI,
  retries: process.env.CI ? 1 : 0,
  reporter: process.env.CI ? [["list"], ["html", { open: "never" }]] : "list",
  use: {
    baseURL: process.env.BASE_URL ?? "http://localhost:8080",
    trace: "retain-on-failure",
    video: "retain-on-failure",
  },
  projects: [
    { name: "webkit-iphone", use: { ...devices["iPhone 15"] } },
    { name: "chromium-desktop", use: { ...devices["Desktop Chrome"] } },
  ],
});
