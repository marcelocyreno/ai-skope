/**
 * Starts Brave with AI Skope loaded and paired, then leaves it running.
 *
 * It uses a dedicated profile rather than your everyday one: launching Brave
 * with --load-extension only works at startup, and quitting your running Brave
 * would lose whatever tabs you have open. This profile persists, so the
 * pairing and any chats survive between runs.
 */
import { createHash } from "node:crypto";
import { spawn } from "node:child_process";
import { existsSync, mkdirSync, readFileSync, realpathSync } from "node:fs";
import { homedir } from "node:os";
import { join, resolve } from "node:path";
import { chromium } from "@playwright/test";

const BRAVE = "/Applications/Brave Browser.app/Contents/MacOS/Brave Browser";
const DIST = realpathSync(resolve("dist"));
const PROFILE = join(homedir(), ".local", "share", "ai-skope", "brave-test-profile");
const PORT = 9333;
const CODE = readFileSync("/tmp/skope-pair-code", "utf8").trim();

const extensionId = [...createHash("sha256").update(DIST, "utf8").digest("hex").slice(0, 32)]
  .map((c) => String.fromCharCode(97 + parseInt(c, 16))).join("");

if (!existsSync(BRAVE)) throw new Error("Brave is not installed at " + BRAVE);
mkdirSync(PROFILE, { recursive: true });

const brave = spawn(BRAVE, [
  `--user-data-dir=${PROFILE}`,
  `--load-extension=${DIST}`,
  `--remote-debugging-port=${PORT}`,
  "--no-first-run",
  "--no-default-browser-check",
  "--restore-last-session",
], { detached: true, stdio: "ignore" });
brave.unref(); // Brave outlives this script

const deadline = Date.now() + 30000;
let browser;
while (Date.now() < deadline) {
  try {
    browser = await chromium.connectOverCDP(`http://127.0.0.1:${PORT}`);
    break;
  } catch {
    await new Promise((r) => setTimeout(r, 400));
  }
}
if (!browser) throw new Error("Brave did not expose its debugging port");

const ctx = browser.contexts()[0];
const panel = await ctx.newPage();
await panel.goto(`chrome-extension://${extensionId}/sidepanel.html`);

// Pair, unless this profile is already paired from a previous run.
const needsPairing = await panel
  .getByRole("heading", { name: "Pair this browser" })
  .isVisible({ timeout: 5000 })
  .catch(() => false);

if (needsPairing) {
  await panel.getByLabel("Pairing code").fill(CODE);
  await panel.getByRole("button", { name: "Pair", exact: true }).click();
  await panel.getByLabel("Message").waitFor({ timeout: 20000 });
  console.log("paired");
} else {
  console.log("already paired from a previous run");
}

// A page worth aiming at, left in front so it is ready to test.
const page = await ctx.newPage();
await page.goto("https://developer.mozilla.org/en-US/docs/Web/CSS/position");
await page.bringToFront();

console.log("extension id:", extensionId);
console.log("profile:", PROFILE);
console.log("Brave is running and left open.");
// No browser.close(): disconnecting leaves Brave up.
