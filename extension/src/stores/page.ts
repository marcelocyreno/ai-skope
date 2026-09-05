/**
 * The tab the pane is looking at, and the bridge to its content script.
 *
 * The extension asks for no host permission at install time. The first time
 * the user aims at a page, chrome.permissions.request runs from their click —
 * which is what the design's "Page access: Ask" means in practice.
 */
import { reactive } from "vue";
import type { ContextItem, PageRef } from "@/api/types";

export interface PageState {
  tabId: number | null;
  url: string;
  title: string;
  favicon: string;
  /** True while the picker is armed in the page. */
  picking: boolean;
  selecting: boolean;
  error: string;
}

export const page = reactive<PageState>({
  tabId: null,
  url: "",
  title: "",
  favicon: "",
  picking: false,
  selecting: false,
  error: "",
});

/** A page as it was at the moment it was read. */
export interface PageSnapshot {
  url: string;
  title: string;
  text: string;
}

export function pageRef(read?: PageSnapshot | null): PageRef {
  // Prefer what the content script saw over what the tab list said: they only
  // differ when the page moved underneath us, and then the text is the truth.
  return {
    url: read?.url || page.url,
    title: read?.title || page.title,
    favicon: page.favicon,
    text: read?.text,
  };
}

/** A page we must never read, per the user's blocked-site list. */
export function isBlocked(blocked: string[]): boolean {
  if (!page.url) return false;
  try {
    const host = new URL(page.url).host;
    return blocked.some((b) => host === b || (b.startsWith("*.") && host.endsWith(b.slice(1))));
  } catch {
    return false;
  }
}

/** Pages Chrome will not let any extension touch. */
export function isRestricted(url: string): boolean {
  return /^(chrome|edge|about|devtools|chrome-extension|view-source):/i.test(url) ||
    url.startsWith("https://chrome.google.com/webstore") ||
    url.startsWith("https://chromewebstore.google.com");
}

/**
 * The window this pane belongs to.
 *
 * A side panel is per-window, so the tab it describes has to come from its own
 * window. `lastFocusedWindow` is not that: with a second window open — or a
 * detached devtools window — it resolves somewhere else entirely, and the pane
 * then answers about a page the user cannot even see.
 */
let ownWindowId: number | null = null;

/** The last tab with real content the pane saw, for when the pane is itself
 *  the active tab (the options page and the tests run it that way). */
let lastContentTabId: number | null = null;

async function paneWindowId(): Promise<number | null> {
  if (ownWindowId != null) return ownWindowId;
  try {
    const win = await chrome.windows.getCurrent();
    if (win?.id != null) ownWindowId = win.id;
  } catch {
    /* fall back to the last focused window below */
  }
  return ownWindowId;
}

/** A tab, reduced to what choosing between them needs. */
export interface TabLike {
  id?: number;
  url?: string;
  active?: boolean;
}

/**
 * Picks the tab the pane should describe.
 *
 * The active one, unless that is the pane itself — the options page and the
 * tests run the pane as a tab. Then the right answer is the content tab the
 * user last looked at, not whichever happens to sit last in the strip.
 */
export function chooseTab<T extends TabLike>(tabs: T[], lastSeenId: number | null): T | null {
  const usable = tabs.filter((t) => t.id != null && t.url && !t.url.startsWith("chrome-extension://"));
  return (
    usable.find((t) => t.active) ??
    usable.find((t) => t.id === lastSeenId) ??
    usable[usable.length - 1] ??
    null
  );
}

async function resolveTab(): Promise<chrome.tabs.Tab | null> {
  const id = await paneWindowId();
  const inWindow = await chrome.tabs.query(id != null ? { windowId: id } : { lastFocusedWindow: true });
  return chooseTab(inWindow, lastContentTabId);
}

/**
 * Re-reads which page is on screen.
 *
 * Cheap enough (one tabs.query) to call before every question rather than
 * trusting the last event to have arrived: a missed event is invisible, and
 * the symptom is an answer that is confidently about the previous page.
 */
export async function refreshActiveTab(): Promise<void> {
  const tab = await resolveTab();
  if (!tab?.id) {
    page.tabId = null;
    return;
  }
  page.tabId = tab.id;
  lastContentTabId = tab.id;
  page.url = tab.url ?? "";
  page.title = tab.title ?? "";
  page.favicon = tab.favIconUrl ?? "";
}

/**
 * Watches for page changes so the pane always describes what is on screen.
 *
 * Four things can change it: another tab is selected, the tab navigates (this
 * covers single-page apps too — tabs.onUpdated reports a pushState as a URL
 * change), the described tab closes, or the window regains focus after the
 * user was elsewhere.
 */
export function watchActiveTab(onChange: () => void): () => void {
  const recheck = () => void refreshActiveTab().then(onChange);
  const activated = () => recheck();
  const updated = (id: number, info: chrome.tabs.TabChangeInfo) => {
    if (id === page.tabId && (info.url || info.title)) recheck();
  };
  const removed = (id: number) => {
    if (id === page.tabId) recheck();
  };
  const focused = (id: number) => {
    if (id !== chrome.windows.WINDOW_ID_NONE) recheck();
  };
  chrome.tabs.onActivated.addListener(activated);
  chrome.tabs.onUpdated.addListener(updated);
  chrome.tabs.onRemoved.addListener(removed);
  chrome.windows.onFocusChanged.addListener(focused);
  return () => {
    chrome.tabs.onActivated.removeListener(activated);
    chrome.tabs.onUpdated.removeListener(updated);
    chrome.tabs.onRemoved.removeListener(removed);
    chrome.windows.onFocusChanged.removeListener(focused);
  };
}

/** Asks for access to this origin, from the user's click. */
export async function ensureAccess(): Promise<boolean> {
  if (!page.url || isRestricted(page.url)) {
    page.error = "Chrome does not allow extensions to read this page.";
    return false;
  }
  const origin = new URL(page.url).origin + "/*";
  if (await chrome.permissions.contains({ origins: [origin] })) {
    await enableOnSite(origin);
    return true;
  }
  try {
    const granted = await chrome.permissions.request({ origins: [origin] });
    if (granted) await enableOnSite(origin);
    return granted;
  } catch {
    return false;
  }
}

/**
 * Selecting text has to work without the user asking for it first — the design
 * shows the toolbar appearing on any drag-select. That means the content
 * script must already be in the page, so once a site is allowed it is
 * registered there permanently rather than injected per action.
 */
async function enableOnSite(origin: string): Promise<void> {
  await registerContentScript(origin);
  // A registration only affects future loads, so the page already on screen
  // gets the script injected now — otherwise the first drag-select after
  // granting access would do nothing.
  if (page.tabId != null) {
    try {
      await chrome.scripting.executeScript({ target: { tabId: page.tabId }, files: ["content.js"] });
    } catch {
      /* the tab may have navigated; the registration covers the next load */
    }
  }
}

async function registerContentScript(origin: string): Promise<void> {
  const id = `skope-${origin}`;
  try {
    const existing = await chrome.scripting.getRegisteredContentScripts({ ids: [id] });
    if (existing.length > 0) return;
    await chrome.scripting.registerContentScripts([
      {
        id,
        js: ["content.js"],
        matches: [origin],
        runAt: "document_idle",
        persistAcrossSessions: true,
      },
    ]);
  } catch {
    // Registration is an optimisation: picking still works by injecting on
    // demand, so a failure here must not stop the user.
  }
}

/**
 * Re-registers on every allowed origin at start-up, so a site allowed in an
 * earlier session still shows the selection toolbar.
 */
export async function syncContentScripts(): Promise<void> {
  try {
    const perms = await chrome.permissions.getAll();
    for (const origin of perms.origins ?? []) {
      // A granted origin can be a specific site or a broad pattern such as
      // <all_urls>; both are worth registering on.
      if (origin.startsWith("http") || origin === "<all_urls>" || origin.startsWith("*://")) {
        await registerContentScript(origin === "<all_urls>" ? "*://*/*" : origin);
      }
    }
  } catch {
    /* nothing to sync */
  }
}

/** Injects the content script, then sends it a command and awaits the reply. */
async function callContent<T>(message: unknown): Promise<T | null> {
  if (page.tabId == null) return null;
  try {
    await chrome.scripting.executeScript({ target: { tabId: page.tabId }, files: ["content.js"] });
  } catch (err) {
    page.error = err instanceof Error ? err.message : String(err);
    return null;
  }
  try {
    return (await chrome.tabs.sendMessage(page.tabId, message)) as T;
  } catch {
    return null;
  }
}

/** Arms the element picker; resolves with the chip when the user clicks one. */
export async function pickElement(): Promise<ContextItem | null> {
  if (!(await ensureAccess())) return null;
  page.picking = true;
  page.error = "";
  try {
    const got = await callContent<ContextItem | null>({ kind: "skope:pick" });
    return got ?? null;
  } finally {
    page.picking = false;
  }
}

export async function cancelPick(): Promise<void> {
  page.picking = false;
  if (page.tabId != null) {
    try {
      await chrome.tabs.sendMessage(page.tabId, { kind: "skope:cancel" });
    } catch {
      /* the page may have navigated away; nothing to cancel */
    }
  }
}

/** Reads whatever the user has selected on the page right now. */
export async function readSelection(): Promise<ContextItem | null> {
  if (!(await ensureAccess())) return null;
  return callContent<ContextItem | null>({ kind: "skope:selection" });
}

/** Extracts the page's readable text, for "send page content". */
export async function readPage(): Promise<PageSnapshot | null> {
  if (!(await ensureAccess())) return null;
  const out = await callContent<PageSnapshot | null>({ kind: "skope:pagetext" });
  return out?.text ? out : null;
}
