/**
 * Captures the Chrome Web Store screenshots.
 *
 * Not a test — it asserts only enough to fail loudly rather than shoot a blank
 * frame. It runs the real extension against a real `aiss`, so what the store
 * shows is what the product does; only the agent is scripted.
 *
 * Chrome's side panel is browser UI and cannot be photographed by Playwright,
 * so each frame is composed: the page at 880x756 and the panel document at
 * 400x756, laid side by side under a drawn window bar. That is the split the
 * user actually sees.
 *
 * Run with:  task store:shots
 */
import { expect } from "@playwright/test";
import { test, pair } from "../e2e/harness";
import type { BrowserContext, Page } from "@playwright/test";
import { execFileSync } from "node:child_process";
import { mkdirSync, rmSync, writeFileSync } from "node:fs";
import { join, resolve } from "node:path";
import { fileURLToPath } from "node:url";

const here = fileURLToPath(new URL(".", import.meta.url));
const outDir = resolve(here, "../../../store/screenshots");

const PAGE_W = 880;
const PANE_W = 400;
const BODY_H = 756;
const BAR_H = 44;

/** Draws the window bar the composite needs, so the frame reads as a browser. */
function frame(content: string, pane: string, url: string): string {
  return `<!doctype html><html><head><meta charset="utf-8"><style>
    * { margin: 0; padding: 0; box-sizing: border-box; }
    body { width: ${PAGE_W + PANE_W}px; height: ${BAR_H + BODY_H}px; overflow: hidden;
           font: 12px -apple-system, "Segoe UI", system-ui, sans-serif; background: #E7E9EC; }
    .bar { height: ${BAR_H}px; display: flex; align-items: center; gap: 10px; padding: 0 14px;
           background: #E7E9EC; border-bottom: 1px solid #CFD4DA; }
    .dots { display: flex; gap: 6px; }
    .dots i { width: 11px; height: 11px; border-radius: 50%; display: block; }
    .url { flex: 1; height: 26px; border-radius: 13px; background: #FFF; border: 1px solid #D5DAE0;
           display: flex; align-items: center; padding: 0 12px; color: #5C6672; letter-spacing: .01em; }
    .split { display: flex; height: ${BODY_H}px; }
    img { display: block; }
    .sep { width: 1px; background: #CFD4DA; }
  </style></head><body>
    <div class="bar">
      <div class="dots"><i style="background:#FF5F57"></i><i style="background:#FEBC2E"></i><i style="background:#28C840"></i></div>
      <div class="url">${url}</div>
    </div>
    <div class="split">
      <img src="data:image/png;base64,${content}" width="${PAGE_W}" height="${BODY_H}">
      <div class="sep"></div>
      <img src="data:image/png;base64,${pane}" width="${PANE_W}" height="${BODY_H}">
    </div>
  </body></html>`;
}

async function compose(
  context: BrowserContext,
  opts: { content: Buffer; pane: Buffer; url: string; name: string },
): Promise<void> {
  const sheet = await context.newPage();
  await sheet.setViewportSize({ width: PAGE_W + PANE_W, height: BAR_H + BODY_H });
  await sheet.setContent(
    frame(opts.content.toString("base64"), opts.pane.toString("base64"), opts.url),
  );
  const shot = await sheet.screenshot({ type: "png" });
  writeFileSync(join(outDir, `${opts.name}.png`), shot);
  await sheet.close();
}

/** Both halves, taken as close together as the two tabs allow. */
async function shoot(
  context: BrowserContext,
  pageTab: Page,
  panel: Page,
  name: string,
  url: string,
): Promise<void> {
  const content = await pageTab.screenshot({ type: "png" });
  const pane = await panel.screenshot({ type: "png" });
  await compose(context, { content, pane, url, name });
}

test("captures the store screenshots", async ({ harness }) => {
  const { panel, context, fixture, aiss, extensionId } = harness;
  mkdirSync(outDir, { recursive: true });

  // By default the agent is scripted, so the frames are identical every time
  // and no model bill is spent on marketing assets. Set SKOPE_SHOT_RUNTIME to
  // shoot the same frames against a real agent instead — for assets that go
  // on a public listing, that is the honest version.
  const realRuntime = process.env.SKOPE_SHOT_RUNTIME;
  if (realRuntime) {
    aiss("models", "--set", realRuntime, process.env.SKOPE_SHOT_MODEL ?? "sonnet");
  } else {
    aiss("runtimes", "command", "custom:e2e", join(here, "agent.sh"));
    aiss("models", "--set", "custom:e2e", "demo");
  }

  // A folder with a readable path and plausible filenames: the harness's
  // scratch directory is a temp path nobody should see on a store listing.
  const demoDir = "/tmp/ai-skope-demo/northwind";
  rmSync("/tmp/ai-skope-demo", { recursive: true, force: true });
  mkdirSync(demoDir, { recursive: true });
  for (const [name, body] of [
    ["pricing-research.md", "# Pricing research\n\nCompetitor caps and overage rates.\n"],
    ["event-volume-forecast.md", "# Event volume\n\nQ3 ingest is trending at 38M/month.\n"],
    ["northwind-contract.md", "# Contract\n\nRenewal terms and the volume commitment.\n"],
  ]) {
    writeFileSync(join(demoDir, name), body);
  }
  aiss("folders", "add", demoDir);
  // Drop the harness's scratch folder so only the readable one is listed.
  for (const line of aiss("folders", "list").split("\n").slice(1)) {
    const [id, path] = line.trim().split(/\s{2,}/);
    if (id && path && !path.includes("ai-skope-demo")) aiss("folders", "remove", id);
  }

  await pair(harness);
  await panel.setViewportSize({ width: PANE_W, height: BODY_H });

  const pageTab = await context.newPage();
  await pageTab.setViewportSize({ width: PAGE_W, height: BODY_H });
  await pageTab.goto(fixture("pricing.html"));
  // Content scripts registered while the pane started apply to the next load.
  await pageTab.reload();
  const shownUrl = "northwind.example/pricing";

  // ---- 1. an answer, grounded in the page --------------------------------
  await panel.bringToFront();
  await panel.getByLabel("Message").fill("Is Growth enough for 40M events a month?");
  await panel.getByLabel("Send").click();
  const strip = panel.getByText("Send this page's text with your question?");
  if (await strip.isVisible({ timeout: 3000 }).catch(() => false)) {
    await panel.getByRole("button", { name: "Include page" }).click();
  }
  await expect(panel.locator(".sk-msg.ai .sk-ai-body strong").first()).toBeVisible({ timeout: 30000 });
  await expect(panel.locator(".sk-msg.ai .sk-ai-body li")).toHaveCount(2);
  await shoot(context, pageTab, panel, "1-answer", shownUrl);

  // ---- 2. the picker, outlining an element -------------------------------
  await panel.getByLabel("Pick an element from the page").click();
  await pageTab.bringToFront();
  const tier = pageTab.locator("article.pg-tier.featured");
  await tier.hover();
  await expect(pageTab.locator("#ai-skope-overlay-host")).toBeAttached();
  await shoot(context, pageTab, panel, "2-picker", shownUrl);
  await pageTab.keyboard.press("Escape");

  // ---- 3. the selection toolbar ------------------------------------------
  const lede = pageTab.locator("p.pg-lede");
  const box = (await lede.boundingBox())!;
  await pageTab.mouse.move(box.x + 4, box.y + box.height / 2);
  await pageTab.mouse.down();
  await pageTab.mouse.move(box.x + box.width - 6, box.y + box.height / 2, { steps: 12 });
  await pageTab.mouse.up();
  await expect(
    pageTab.locator("#ai-skope-overlay-host").getByRole("button", { name: "Add to chat" }),
  ).toBeVisible();
  await shoot(context, pageTab, panel, "3-selection", shownUrl);

  // ---- 4. asking about a file on disk ------------------------------------
  // Clear the selection, or the previous scene's toolbar sits in this frame.
  await pageTab.mouse.click(60, BODY_H - 60);
  await expect(
    pageTab.locator("#ai-skope-overlay-host").getByRole("button", { name: "Add to chat" }),
  ).toBeHidden();
  await panel.bringToFront();
  await panel.getByLabel("Add a file").click();
  await expect(panel.locator(".sk-filepick")).toBeVisible();
  // Searching is the point of the picker, so show it searching.
  await panel.getByPlaceholder("Find a file or folder").fill("pricing");
  await expect(panel.locator(".sk-filepick").getByText("pricing-research.md")).toBeVisible();
  await shoot(context, pageTab, panel, "4-files", shownUrl);
  await panel.keyboard.press("Escape");

  // ---- 5. the settings page, full width ----------------------------------
  const options = await context.newPage();
  await options.setViewportSize({ width: PAGE_W + PANE_W, height: BAR_H + BODY_H });
  await options.goto(`chrome-extension://${extensionId}/options.html`);
  const folders = options.getByRole("heading", { name: "Folders", exact: true });
  await expect(folders).toBeVisible();
  // Framed on folders and privacy, not the runtimes table: that table lists
  // absolute paths under the operator's home directory, and these frames go
  // on a public listing.
  await folders.evaluate((el) => el.scrollIntoView({ block: "start" }));
  await options.waitForTimeout(400);
  writeFileSync(join(outDir, "5-settings.png"), await options.screenshot({ type: "png" }));
  await options.close();

  // Every frame the listing needs, at the size the store wants.
  const sizes = execFileSync("sips", [
    "-g", "pixelWidth", "-g", "pixelHeight",
    ...["1-answer", "2-picker", "3-selection", "4-files", "5-settings"].map((n) =>
      join(outDir, `${n}.png`),
    ),
  ]).toString();
  expect(sizes).not.toContain("Error");
  console.log(`\nstore screenshots written to ${outDir}\n${sizes}`);
});
