/**
 * Everything the extension remembers between sessions lives in
 * chrome.storage.local: the pairing, the server address, and the look of the
 * pane. Nothing sensitive beyond the bearer token is kept here — chats, notes
 * and provider keys all live on the server.
 */

/**
 * How long an answer should be, on the composer's five-stop scale: 1 is a
 * sentence, 3 is short, 5 asks for the reasoning as well. 4 is neutral and
 * says nothing to the model at all, so a control nobody touches leaves the
 * prompt exactly as the server would have built it.
 */
export type Conciseness = 1 | 2 | 3 | 4 | 5;

export const CONCISENESS_NORMAL: Conciseness = 4;

export interface Settings {
  baseUrl: string;
  token: string;
  serverId: string;
  theme: "light" | "dark" | "system";
  palette: "graphite" | "nocturne" | "sage" | "ember" | "arctic" | "mono";
  textSize: "small" | "default" | "large";
  /** Whether the whole page's text may be sent with a question. */
  pageAccess: "ask" | "always" | "never";
  /** Sites the extension never reads. */
  blockedHosts: string[];
  openAutomatically: boolean;
  /** Where the composer's answer-length control starts the next message. */
  conciseness: Conciseness;
  /**
   * Whether AI Skope puts its entry in the page's right-click menu.
   * Off by default: the menu belongs to the page, and the pane's own
   * selection toolbar and shortcuts already do the same.
   */
  contextMenu: boolean;
}

export const DEFAULTS: Settings = {
  baseUrl: "http://127.0.0.1:7331",
  token: "",
  serverId: "",
  theme: "system",
  palette: "graphite",
  textSize: "default",
  pageAccess: "ask",
  blockedHosts: [],
  openAutomatically: false,
  conciseness: CONCISENESS_NORMAL,
  contextMenu: false,
};

const KEY = "settings";

export async function loadSettings(): Promise<Settings> {
  const got = await chrome.storage.local.get(KEY);
  return { ...DEFAULTS, ...((got[KEY] as Partial<Settings>) ?? {}) };
}

export async function saveSettings(patch: Partial<Settings>): Promise<Settings> {
  const next = { ...(await loadSettings()), ...patch };
  await chrome.storage.local.set({ [KEY]: next });
  return next;
}

/** Calls back whenever another surface (options page, worker) changes settings. */
export function onSettingsChanged(fn: (s: Settings) => void): () => void {
  const listener = (changes: Record<string, chrome.storage.StorageChange>, area: string) => {
    if (area === "local" && changes[KEY]) {
      fn({ ...DEFAULTS, ...(changes[KEY].newValue as Partial<Settings>) });
    }
  };
  chrome.storage.onChanged.addListener(listener);
  return () => chrome.storage.onChanged.removeListener(listener);
}
