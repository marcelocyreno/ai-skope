/**
 * Chat history, grouped the way the design shows it: this page, this site,
 * everywhere. New chat archives rather than destroys, and deleting is
 * reversible, so nothing the user asked is ever lost by accident.
 */
import { reactive, computed } from "vue";
import { api } from "./connection";
import { page } from "./page";
import type { Chat } from "@/api/types";

interface HistoryStore {
  chats: Chat[];
  query: string;
  loading: boolean;
  /** The chat just deleted, kept so the toast can offer Undo. */
  lastDeleted: Chat | null;
}

export const history = reactive<HistoryStore>({
  chats: [],
  query: "",
  loading: false,
  lastDeleted: null,
});

export async function loadHistory(): Promise<void> {
  history.loading = true;
  try {
    history.chats = await api().chats({ q: history.query || undefined, limit: 200 });
  } finally {
    history.loading = false;
  }
}

function hostOf(url: string): string {
  try {
    const u = new URL(url);
    return u.protocol === "file:" ? "local file" : u.host;
  } catch {
    return "";
  }
}

/**
 * The three groups the design shows. They are a strict partition: a chat
 * appears exactly once, whichever group claims it first, so the list never
 * repeats itself when the pane has no page (or an unparseable one).
 */
export const groups = computed(() => {
  const host = hostOf(page.url);
  const thisPage: Chat[] = [];
  const thisSite: Chat[] = [];
  const everywhere: Chat[] = [];

  for (const c of history.chats) {
    if (page.url && c.url === page.url) thisPage.push(c);
    else if (host && c.host === host) thisSite.push(c);
    else everywhere.push(c);
  }

  return [
    { key: "page", label: "This page", detail: page.title || page.url, chats: thisPage, showHost: false },
    { key: "site", label: "This site", detail: host, chats: thisSite, showHost: false },
    { key: "all", label: "Everywhere", detail: "", chats: everywhere, showHost: true },
  ].filter((g) => g.chats.length > 0);
});

export async function deleteChat(c: Chat): Promise<void> {
  await api().deleteChat(c.id);
  history.lastDeleted = c;
  await loadHistory();
}

export async function undoDelete(): Promise<Chat | null> {
  const c = history.lastDeleted;
  if (!c) return null;
  await api().restoreChat(c.id);
  history.lastDeleted = null;
  await loadHistory();
  return c;
}
