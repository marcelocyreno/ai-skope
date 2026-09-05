/**
 * The service worker does only what must survive the panel being closed:
 * opening the panel, keyboard commands, and the context menu. It deliberately
 * holds no connection to the server — MV3 stops an idle worker, which would
 * cut a streaming answer off mid-sentence, so the panel owns that.
 */

const MENU_ASK = "skope-ask";
const MENU_NOTE = "skope-note";

chrome.runtime.onInstalled.addListener(() => {
  // Clicking the toolbar icon opens the side panel.
  void chrome.sidePanel.setPanelBehavior({ openPanelOnActionClick: true }).catch(() => {});

  chrome.contextMenus.removeAll(() => {
    chrome.contextMenus.create({
      id: MENU_ASK,
      title: "Ask AI Skope about this",
      contexts: ["selection"],
    });
    chrome.contextMenus.create({
      id: MENU_NOTE,
      title: "Save selection as a note",
      contexts: ["selection"],
    });
  });
});

chrome.contextMenus.onClicked.addListener((info, tab) => {
  if (!tab?.windowId) return;
  void chrome.sidePanel.open({ windowId: tab.windowId });
  // The panel may still be starting, so the intent is queued rather than sent.
  void chrome.storage.session.set({
    pendingAction: {
      action: info.menuItemId === MENU_NOTE ? "note" : "add",
      selection: { type: "text", quote: info.selectionText ?? "" },
      at: Date.now(),
    },
  });
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
