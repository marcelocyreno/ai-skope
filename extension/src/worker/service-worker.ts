/**
 * The service worker does only what must survive the panel being closed:
 * opening the panel, keyboard commands, and the context menu. It deliberately
 * holds no connection to the server — MV3 stops an idle worker, which would
 * cut a streaming answer off mid-sentence, so the panel owns that.
 */
import { loadSettings, onSettingsChanged } from "@/stores/storage";

const MENU_ASK = "skope-ask";

/**
 * What the menu currently looks like, so an unrelated settings change — a
 * palette, a blocked host — does not tear the entry down and build it again.
 * It is null in a worker that has just started and does not yet know, which
 * is exactly when the work should be done rather than skipped.
 */
let menuShown: boolean | null = null;

function rebuildMenu(shown: boolean): void {
  // removeAll first either way: a worker that restarts inherits whatever
  // entries the last one left behind, including none.
  chrome.contextMenus.removeAll(() => {
    if (!shown) return;
    chrome.contextMenus.create({
      id: MENU_ASK,
      title: "Ask AI Skope about this",
      contexts: ["selection"],
    });
  });
}

/**
 * The right-click menu is opt-in. A fresh install leaves the page's own menu
 * untouched; turning the setting on adds the entry there and then, without
 * reloading the extension.
 */
async function syncMenu(shown?: boolean): Promise<void> {
  const want = shown ?? (await loadSettings()).contextMenu;
  if (want === menuShown) return;
  menuShown = want;
  rebuildMenu(want);
}

chrome.runtime.onInstalled.addListener(() => {
  // Clicking the toolbar icon opens the side panel.
  void chrome.sidePanel.setPanelBehavior({ openPanelOnActionClick: true }).catch(() => {});
  void syncMenu();
});

// Context menus do not survive a browser restart, so the setting is applied
// again on the way up, not only when the extension is installed or updated.
chrome.runtime.onStartup.addListener(() => {
  void syncMenu();
});

onSettingsChanged((s) => {
  void syncMenu(s.contextMenu);
});

/**
 * A side panel is normally already open when you right-click, so the click is
 * offered to it live first — the same message the in-page selection toolbar
 * sends, so the pane needs no second path for it. Only when nothing answers is
 * the intent queued for a pane that has yet to start.
 *
 * The reply is what tells the two situations apart: the worker cannot see
 * whether a pane is running, and an action both delivered and queued would be
 * added to the chat twice.
 */
async function deliverSelection(quote: string): Promise<void> {
  const selection = { type: "text", quote };
  const reply = (await chrome.runtime
    .sendMessage({ kind: "skope:selection-action", action: "add", selection })
    .catch(() => undefined)) as { received?: boolean } | undefined;
  if (reply?.received) return;
  await chrome.storage.session.set({ pendingAction: { action: "add", selection, at: Date.now() } });
}

chrome.contextMenus.onClicked.addListener((info, tab) => {
  if (!tab?.windowId) return;
  // open() has to be reached while the user gesture is still live, so it goes
  // first and the delivery sorts itself out behind it.
  void chrome.sidePanel.open({ windowId: tab.windowId });
  void deliverSelection(info.selectionText ?? "");
});

chrome.commands.onCommand.addListener(async (command) => {
  const [tab] = await chrome.tabs.query({ active: true, lastFocusedWindow: true });
  if (!tab?.windowId) return;
  await chrome.sidePanel.open({ windowId: tab.windowId });
  await chrome.storage.session.set({ pendingCommand: { command, at: Date.now() } });
});

// The panel asks for this when it opens, to learn why it was opened.
chrome.runtime.onMessage.addListener((msg, _sender, sendResponse) => {
  if (msg?.kind === "skope:worker-ping") {
    sendResponse({ ok: true });
    return false;
  }
  return false;
});
