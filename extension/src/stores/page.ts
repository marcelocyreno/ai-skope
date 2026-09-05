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

export function pageRef(text?: string): PageRef {
  return { url: page.url, title: page.title, favicon: page.favicon, text };
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

export async function refreshActiveTab(): Promise<void> {
  // The pane must describe the page the user is looking at — never itself.
  // In the side panel that is simply the active tab; but the options page and
  // the test harness run as tabs, so extension pages are skipped.
  const inWindow = await chrome.tabs.query({ lastFocusedWindow: true });
  const usable = inWindow.filter((t) => t.url && !t.url.startsWith("chrome-extension://"));
  const tab = usable.find((t) => t.active) ?? usable[usable.length - 1];
  if (!tab?.id) {
    page.tabId = null;
    return;
  }
  page.tabId = tab.id;
  page.url = tab.url ?? "";
  page.title = tab.title ?? "";
  page.favicon = tab.favIconUrl ?? "";
}

/** Watches for tab changes so the pane always describes what is on screen. */
export function watchActiveTab(onChange: () => void): () => void {
  const activated = () => void refreshActiveTab().then(onChange);
  const updated = (id: number, info: chrome.tabs.TabChangeInfo) => {
    if (id === page.tabId && (info.url || info.title)) void refreshActiveTab().then(onChange);
  };
  chrome.tabs.onActivated.addListener(activated);
  chrome.tabs.onUpdated.addListener(updated);
  return () => {
    chrome.tabs.onActivated.removeListener(activated);
    chrome.tabs.onUpdated.removeListener(updated);
  };
}

/** Asks for access to this origin, from the user's click. */
export async function ensureAccess(): Promise<boolean> {
  if (!page.url || isRestricted(page.url)) {
    page.error = "Chrome does not allow extensions to read this page.";
    return false;
  }
  const origin = new URL(page.url).origin + "/*";
  if (await chrome.permissions.contains({ origins: [origin] })) return true;
  try {
    return await chrome.permissions.request({ origins: [origin] });
  } catch {
    return false;
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
export async function readPageText(): Promise<string> {
  if (!(await ensureAccess())) return "";
  const out = await callContent<{ text: string } | null>({ kind: "skope:pagetext" });
  return out?.text ?? "";
}
