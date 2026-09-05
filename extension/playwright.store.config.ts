import { defineConfig } from "@playwright/test";

/**
 * The store screenshot capture. Kept out of the test run: it produces assets,
 * takes longer, and writes into the repository.
 */
export default defineConfig({
  testDir: "./tests/store",
  timeout: 180_000,
  expect: { timeout: 20_000 },
  fullyParallel: false,
  workers: 1,
  reporter: [["list"]],
  use: { trace: "off", video: "off" },
});
