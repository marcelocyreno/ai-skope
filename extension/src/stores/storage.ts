/**
 * Everything the extension remembers between sessions lives in
 * chrome.storage.local: the pairing, the server address, and the look of the
 * pane. Nothing sensitive beyond the bearer token is kept here — chats, notes
 * and provider keys all live on the server.
 */
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
