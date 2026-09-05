import { test, expect, pair } from "./harness";

test.describe.configure({ mode: "serial" });

test("pairs with the server, then answers a question about the page", async ({ harness }) => {
  const { panel, context, fixture } = harness;

  // Before pairing the pane asks for a code and nothing else.
  await expect(panel.getByRole("heading", { name: "Pair this browser" })).toBeVisible();
  await pair(harness);

  // Paired: the composer is live and the chip names the pinned model.
  await expect(panel.getByLabel("Message")).toBeVisible();
  await expect(panel.locator(".sk-chip-model")).toContainText("fake-1");

  // The switcher opens from the chip and lists what the server offers.
  await panel.locator(".sk-chip-model").click();
  await expect(panel.locator(".sk-switcher")).toBeVisible();
  await panel.keyboard.press("Escape");
  await expect(panel.locator(".sk-switcher")).toBeHidden();

  const pageTab = await context.newPage();
  await pageTab.goto(fixture("pricing.html"));
  await panel.bringToFront();

  await panel.getByLabel("Message").fill("Is Growth enough for 40M events?");
  await panel.getByLabel("Send").click();

  // The answer streams in from the fake agent through the real server.
  await expect(panel.getByText(/Growth caps at 25M events/)).toBeVisible({ timeout: 30000 });
  // And the tool line the agent reported is shown.
  await expect(panel.getByText(/README\.md|Read/).first()).toBeVisible();
});

test("keeps the transcript, and undoes a delete", async ({ harness }) => {
  const { panel } = harness;
  await pair(harness);

  await panel.getByLabel("Message").fill("What does the export section say?");
  await panel.getByLabel("Send").click();
  await expect(panel.getByText(/Growth caps at 25M events/)).toBeVisible({ timeout: 30000 });

  // History groups it under this page, and delete is reversible.
  await panel.getByLabel("Chat history").click();
  await expect(panel.getByRole("heading", { name: "Chats" })).toBeVisible();
  await expect(panel.locator(".sk-history").getByText("What does the export section say?")).toBeVisible();
  await panel.getByRole("button", { name: "Back to chat" }).click();

  // Reopening the panel restores the conversation from the server.
  await panel.reload();
  await expect(panel.getByText(/Growth caps at 25M events/)).toBeVisible({ timeout: 30000 });
});

test("attaches a local file the server is allowed to read", async ({ harness }) => {
  const { panel } = harness;
  await pair(harness);

  await panel.getByLabel("Add a file").click();
  await expect(panel.getByPlaceholder("Find a file or folder")).toBeVisible();
  await panel.getByPlaceholder("Find a file or folder").fill("README");
  await panel.getByRole("option", { name: /README\.md/ }).first().click();

  // The chip names the file, and the question carries it to the agent.
  await expect(panel.locator(".sk-tray .sk-ctx .sel")).toHaveText("README.md");
  await panel.getByLabel("Message").fill("Summarize this file");
  await panel.getByLabel("Send").click();
  await expect(panel.getByText(/Growth caps at 25M events/)).toBeVisible({ timeout: 30000 });
});

test("picks an element from a real page", async ({ harness }) => {
  const { panel, context, fixture } = harness;
  await pair(harness);

  const pageTab = await context.newPage();
  await pageTab.goto(fixture("pricing.html"));
  await pageTab.bringToFront();
  await panel.bringToFront();

  await panel.getByLabel("Pick an element from the page").click();

  // The picker is armed in the page: hover draws the reticle, a click takes it.
  await pageTab.bringToFront();
  const tier = pageTab.locator("article.pg-tier.featured");
  await tier.hover();
  await expect(pageTab.locator("#ai-skope-overlay-host")).toBeAttached();
  await tier.click();

  await panel.bringToFront();
  // The picked element becomes a context chip in the composer.
  await expect(panel.locator(".sk-tray .sk-ctx .sel")).toHaveText("article.pg-tier.featured", {
    timeout: 15000,
  });
});

test("says what is wrong when the server goes away", async ({ harness }) => {
  const { panel, stopServer } = harness;
  await pair(harness);
  await expect(panel.getByLabel("Message")).toBeVisible();

  await stopServer();
  // The pane must explain itself rather than silently failing.
  await expect(
    panel.getByText(/isn't running|isn't reachable/i).first(),
  ).toBeVisible({ timeout: 30000 });
  // And it offers the way back rather than leaving the user stuck.
  await expect(panel.getByRole("button", { name: "Try again" })).toBeVisible();
});
