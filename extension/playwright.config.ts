import { defineConfig } from "@playwright/test";

/**
 * Extension tests need a real Chrome with a persistent profile, so they run
 * serially in one worker and drive the browser themselves.
 */
export default defineConfig({
  testDir: "./tests/e2e",
  timeout: 90_000,
  expect: { timeout: 15_000 },
  fullyParallel: false,
  workers: 1,
  reporter: [["list"]],
  use: { trace: "off", video: "off" },
});
