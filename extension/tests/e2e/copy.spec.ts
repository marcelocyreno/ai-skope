/**
 * Getting a message out of the pane: the per-message copy action, and the
 * chat-level one in the composer's hint row. Run against a real browser and a
 * real server, so the clipboard here is the system clipboard.
 */
import { test, expect, pair } from "./harness";
import type { Page } from "@playwright/test";

async function ask(panel: Page, text: string): Promise<void> {
  await expect(panel.getByLabel("Message")).toBeVisible();
  await panel.getByLabel("Message").fill(text);
  await panel.getByLabel("Send").click();
  const strip = panel.getByText("Send this page's text with your question?");
  if (await strip.isVisible({ timeout: 2000 }).catch(() => false)) {
    await panel.getByRole("button", { name: "Include page" }).click();
  }
  await expect(panel.locator(".sk-msg.ai .sk-ai-body")).toBeVisible({ timeout: 30000 });
}

const clipboard = (panel: Page) => panel.evaluate(() => navigator.clipboard.readText());

test("a message and an answer can each be copied", async ({ harness }) => {
  const { panel } = harness;
  await panel.context().grantPermissions(["clipboard-read", "clipboard-write"]);
  await pair(harness);

  const question = "Is Growth enough for 40M events?";
  await ask(panel, question);

  // The row is revealed by pointing at the message, as the timestamp already was.
  const user = panel.locator(".sk-msg.user").last();
  await user.hover();
  await user.getByRole("button", { name: "Copy message" }).click();

  // The glyph swap is the confirmation, and it is what says the write landed;
  // there is no toast for a single message.
  await expect(user.getByRole("button", { name: "Copied" })).toBeVisible();
  await expect(panel.locator(".sk-toast")).toHaveCount(0);
  expect(await clipboard(panel)).toBe(question);

  // An answer travels as the Markdown the model sent, not the rendered HTML.
  const ai = panel.locator(".sk-msg.ai").last();
  await ai.hover();
  await ai.getByRole("button", { name: "Copy answer" }).click();
  await expect(ai.getByRole("button", { name: "Copied" })).toBeVisible();

  const answer = await clipboard(panel);
  expect(answer).toContain("**Limits**");
  expect(answer).not.toContain("<strong>");
});

test("copy chat takes the whole conversation, without the pane's furniture", async ({ harness }) => {
  const { panel } = harness;
  await panel.context().grantPermissions(["clipboard-read", "clipboard-write"]);
  await pair(harness);

  const question = "Is Growth enough for 40M events?";
  await ask(panel, question);

  // This one is worth a toast: what it took is mostly off-screen.
  await panel.getByRole("button", { name: "Copy chat" }).click();
  await expect(panel.getByText("Conversation copied")).toBeVisible();

  const transcript = await clipboard(panel);
  expect(transcript.split("\n")[0]).toContain(question);
  expect(transcript).toContain("## You");
  expect(transcript).toContain(question);

  // Tool lines are progress, not content, so they stay behind.
  const toolLine = (await panel.locator(".sk-msg.ai .sk-tool").first().innerText()).trim();
  expect(toolLine.length).toBeGreaterThan(0);
  expect(transcript).not.toContain(toolLine);
});
