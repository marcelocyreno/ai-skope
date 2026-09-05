/**
 * Each state the design specifies, exercised in a real browser against a real
 * server: notes, history, the model switcher, quick settings and palettes, the
 * options page, the selection toolbar, and the first-run screen.
 */
import { test, expect, pair } from "./harness";
import type { Page } from "@playwright/test";

/**
 * Waits for the pane to be usable after a load, and says what it is showing if
 * it is not — a silent timeout here hides whether the pane failed to connect
 * or simply took its time.
 */
async function waitForPane(panel: Page): Promise<void> {
  try {
    await panel.getByLabel("Message").waitFor({ state: "visible", timeout: 20000 });
  } catch (err) {
    const shown = await panel.locator("#app").innerText().catch(() => "(no #app)");
    const stored = await panel
      .evaluate(async () => JSON.stringify((await chrome.storage.local.get("settings")).settings ?? null))
      .catch(() => "(unreadable)");
    throw new Error(
      `the pane never became usable.\nstored settings: ${stored}\nshowing:\n${shown.slice(0, 300)}`,
    );
  }
}

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

test("first run shows the empty state and its four suggestions", async ({ harness }) => {
  const { panel } = harness;
  await pair(harness);

  await expect(panel.getByRole("heading", { name: "Aim at anything on this page" })).toBeVisible();
  for (const label of [
    "Summarize this page",
    "Pick an element to ask about",
    "Ask about a local file or repo",
    "Explain the selected text",
  ]) {
    await expect(panel.getByRole("button", { name: label })).toBeVisible();
  }
});

test("notes: write, search, delete and undo", async ({ harness }) => {
  const { panel } = harness;
  await pair(harness);

  await panel.getByRole("tab", { name: /Notes/ }).click();
  await expect(panel.getByText(/No notes yet/)).toBeVisible();

  await panel.getByLabel("New note").fill("Ask finance about the SAFE");
  await panel.getByRole("button", { name: "Save note" }).click();
  await expect(panel.locator(".sk-note")).toHaveCount(1);
  await expect(panel.getByText("Ask finance about the SAFE")).toBeVisible();

  // The tab's badge counts what is stored.
  await expect(panel.getByRole("tab", { name: /Notes/ }).locator(".sk-badge")).toHaveText("1");

  await panel.getByLabel("Search notes").fill("nothing matches this");
  await expect(panel.getByText(/No notes match/)).toBeVisible();
  await panel.getByLabel("Search notes").fill("");
  await expect(panel.locator(".sk-note")).toHaveCount(1);

  await panel.locator(".sk-note").getByLabel("Delete note").click();
  await expect(panel.locator(".sk-note")).toHaveCount(0);
  await panel.getByRole("button", { name: "Undo" }).click();
  await expect(panel.locator(".sk-note")).toHaveCount(1);
});

test("history: a new chat archives the last one, and it can be reopened", async ({ harness }) => {
  const { panel } = harness;
  await pair(harness);

  await ask(panel, "First question about pricing");
  await panel.getByLabel("New chat").click();

  // The pane returns to first run, and the toast says where the old one went.
  await expect(panel.getByRole("heading", { name: "Aim at anything on this page" })).toBeVisible();
  await expect(panel.getByText(/New chat/)).toBeVisible();

  await panel.getByLabel("Chat history").click();
  const row = panel.locator(".sk-chat", { hasText: "First question about pricing" });
  await expect(row).toHaveCount(1);
  await row.click();
  await expect(panel.getByText("First question about pricing")).toBeVisible();
});

test("clear chat archives rather than destroys", async ({ harness }) => {
  const { panel } = harness;
  await pair(harness);

  await ask(panel, "Something worth keeping");
  await panel.getByRole("button", { name: "Clear chat" }).click();
  await expect(panel.getByRole("heading", { name: "Aim at anything on this page" })).toBeVisible();

  // It is in History, not gone.
  await panel.getByLabel("Chat history").click();
  await expect(panel.locator(".sk-chat", { hasText: "Something worth keeping" })).toHaveCount(1);
});

test("the model switcher lists what the server offers and changes the chip", async ({ harness }) => {
  const { panel, addStubProvider } = harness;
  await pair(harness);
  await addStubProvider();
  await panel.reload();
  await waitForPane(panel);

  await panel.locator(".sk-chip-model").click();
  const switcher = panel.locator(".sk-switcher");
  await expect(switcher).toBeVisible();

  // The server line names where the models come from.
  await expect(switcher.locator(".srv")).toContainText("connected");

  // Models discovered from the provider are grouped under their runtime.
  const option = switcher.getByRole("option", { name: /glm-5.3-flash/ });
  if (!(await option.isVisible({ timeout: 5000 }).catch(() => false))) {
    const auth = { Authorization: `Bearer ${harness.token}` };
    const offered = await fetch(`${harness.serverUrl}/v1/models`, { headers: auth }).then((r) => r.text());
    const providers = await fetch(`${harness.serverUrl}/v1/providers`, { headers: auth }).then((r) => r.text());
    throw new Error(
      `the switcher did not offer the provider's models.\nproviders: ${providers}\nmodels: ${offered.slice(0, 300)}`,
    );
  }
  await option.click();

  await expect(switcher).toBeHidden();
  await expect(panel.locator(".sk-chip-model")).toContainText("glm-5.3-flash");
});

test("quick settings applies the theme and the palette", async ({ harness }) => {
  const { panel } = harness;
  await pair(harness);

  await panel.getByLabel("Settings").click();
  await expect(panel.getByRole("heading", { name: "Settings" })).toBeVisible();

  await panel.getByRole("button", { name: "Dark" }).click();
  await expect(panel.locator("html")).toHaveAttribute("data-theme", "dark");

  await panel.getByRole("button", { name: "Nocturne" }).click();
  await expect(panel.locator("html")).toHaveAttribute("data-palette", "nocturne");
  // The palette really changes the accent, not just an attribute.
  const accent = await panel.evaluate(() =>
    getComputedStyle(document.documentElement).getPropertyValue("--accent").trim(),
  );
  expect(accent.toLowerCase()).toBe("#d9a94a");

  await panel.getByRole("button", { name: "Light" }).click();
  await expect(panel.locator("html")).toHaveAttribute("data-theme", "light");
});

test("the selection toolbar adds a quote to the chat", async ({ harness }) => {
  const { panel, context, fixture } = harness;
  await pair(harness);

  const pageTab = await context.newPage();
  await pageTab.goto(fixture("pricing.html"));
  // Content scripts registered while the pane started apply to the next load.
  await pageTab.reload();

  // Drag across a paragraph the way a person would, so the content script's
  // mouseup handler sees a real selection.
  const lede = pageTab.locator("p.pg-lede");
  const box = (await lede.boundingBox())!;
  await pageTab.mouse.move(box.x + 4, box.y + box.height / 2);
  await pageTab.mouse.down();
  await pageTab.mouse.move(box.x + box.width - 6, box.y + box.height / 2, { steps: 12 });
  await pageTab.mouse.up();

  // The toolbar lives in the overlay's shadow root.
  const toolbar = pageTab.locator("#ai-skope-overlay-host");
  await expect(toolbar).toBeAttached();
  await pageTab.locator("#ai-skope-overlay-host").getByRole("button", { name: "Add to chat" }).click();

  await panel.bringToFront();
  await expect(panel.locator(".sk-tray .sk-ctx .q")).toContainText("unlimited seats", { timeout: 15000 });
});

test("the options page manages folders against the real server", async ({ harness }) => {
  const { context, extensionId } = harness;
  await pair(harness); // the options page talks to the server too
  const options = await context.newPage();
  await options.goto(`chrome-extension://${extensionId}/options.html`);

  // Every section the design specifies is present.
  for (const heading of [
    "General",
    "Server & runtimes",
    "Folders",
    "Providers & keys",
    "Privacy",
    "Shortcuts",
    "About",
  ]) {
    await expect(options.getByRole("heading", { name: heading, exact: true })).toBeVisible();
  }

  // The folder the harness allowed is listed, and a new one can be added.
  const listed = options.getByText(/dev\/northwind/).first();
  if (!(await listed.isVisible({ timeout: 10000 }).catch(() => false))) {
    throw new Error(`the options page did not list the allowed folder. It showed:\n${(
      await options.locator(".sk-opt-section[data-section='folders']").innerText()
    ).slice(0, 400)}`);
  }
  await options.getByLabel("Folder path").fill(harness.projectDir);
  await options.getByRole("button", { name: "Add folder" }).click();
  await expect(options.locator("table").filter({ hasText: "Folder" }).locator("tbody tr")).toHaveCount(1);

  // Runtimes are listed with their real versions.
  await expect(options.getByText("custom:fake").or(options.getByText("Claude Code"))).toBeVisible();
});
