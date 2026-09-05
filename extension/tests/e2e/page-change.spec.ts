import { test, expect, pair } from "./harness";
import type { Page } from "@playwright/test";

/** Asks, answering the page-text question the way a person would. */
async function ask(panel: Page, text: string): Promise<void> {
  await expect(panel.getByLabel("Message")).toBeVisible();
  await panel.getByLabel("Message").fill(text);
  await panel.getByLabel("Send").click();
  const strip = panel.getByText("Send this page's text with your question?");
  if (await strip.isVisible({ timeout: 2000 }).catch(() => false)) {
    await panel.getByRole("button", { name: "Include page" }).click();
  }
}

/** The fake agent echoes the prompt it was given, so the last answer shows
 *  exactly which page's text reached the runtime. */
function lastAnswer(panel: Page) {
  return panel.locator(".sk-msg.ai .sk-ai-body").last();
}

test.describe.configure({ mode: "serial" });

test("follows the page when the tab navigates", async ({ harness }) => {
  const { panel, context, fixture } = harness;
  await pair(harness);

  const tab = await context.newPage();
  await tab.goto(fixture("pricing.html"));
  await panel.bringToFront();

  await ask(panel, "What is on this page?");
  await expect(lastAnswer(panel)).toContainText("unlimited seats", { timeout: 30000 });

  // The same tab goes somewhere else.
  await tab.goto(fixture("changelog.html"));
  await panel.bringToFront();

  // The transcript belongs to the page. Carrying it over would carry the
  // agent's session with it, and the agent would answer from the page it was
  // shown before rather than the one on screen. The pane says so rather than
  // swapping the transcript silently.
  await expect(panel.getByText("Now asking about")).toBeVisible();
  await expect(panel.locator(".sk-msg.user")).toHaveCount(0);

  await ask(panel, "And now?");
  await expect(panel.locator(".sk-msg.user")).toHaveCount(1);
  await expect(lastAnswer(panel)).toContainText("v1 export API", { timeout: 30000 });
  await expect(lastAnswer(panel)).not.toContainText("unlimited seats");

  // Both conversations survive, one per page.
  await panel.getByLabel("Chat history").click();
  await expect(panel.locator(".sk-history").getByText("What is on this page?")).toBeVisible();
  await expect(panel.locator(".sk-history").getByText("And now?")).toBeVisible();
});

test("follows the page when the user switches tabs", async ({ harness }) => {
  const { panel, context, fixture } = harness;
  await pair(harness);

  const first = await context.newPage();
  await first.goto(fixture("pricing.html"));
  await panel.bringToFront();
  await ask(panel, "What is on this page?");
  await expect(lastAnswer(panel)).toContainText("unlimited seats", { timeout: 30000 });

  // A second tab, opened and focused the way a link in a new tab would be.
  const second = await context.newPage();
  await second.goto(fixture("changelog.html"));
  await second.bringToFront();
  await panel.bringToFront();

  await ask(panel, "And now?");
  await expect(lastAnswer(panel)).toContainText("v1 export API", { timeout: 30000 });
  await expect(lastAnswer(panel)).not.toContainText("unlimited seats");
});

test("follows a single-page app that changes content without a load", async ({ harness }) => {
  const { panel, context, fixture } = harness;
  await pair(harness);

  const tab = await context.newPage();
  await tab.goto(fixture("spa.html"));
  await panel.bringToFront();
  await ask(panel, "What is on this page?");
  await expect(lastAnswer(panel)).toContainText("ingest key", { timeout: 30000 });

  await tab.bringToFront();
  await tab.click("#go");
  await panel.bringToFront();

  await ask(panel, "And now?");
  await expect(lastAnswer(panel)).toContainText("first working day", { timeout: 30000 });
});
