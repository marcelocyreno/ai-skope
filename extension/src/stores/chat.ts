/**
 * The conversation: the context tray, the transcript, and the live turn.
 *
 * Streaming appends to a reactive message object, so a delta updates that
 * message's text node and nothing else re-renders.
 */
import { reactive, computed } from "vue";
import { api, connection } from "./connection";
import { models } from "./models";
import { page, pageRef, readPage, refreshActiveTab, type PageSnapshot } from "./page";
import { loadSettings } from "./storage";
import type { Chat, ContextItem, Message, TurnEvent } from "@/api/types";

interface ChatStore {
  chat: Chat | null;
  messages: Message[];
  /** Context attached to the next message, shown as chips in the composer. */
  tray: ContextItem[];
  draft: string;
  sending: boolean;
  error: string;
}

export const chat = reactive<ChatStore>({
  chat: null,
  messages: [],
  tray: [],
  draft: "",
  sending: false,
  error: "",
});

export const isEmpty = computed(() => chat.messages.length === 0);

let abort: AbortController | null = null;

/** Opens the chat for the current page, or starts one if there is none. */
export async function openForCurrentPage(): Promise<void> {
  if (connection.state !== "online") return;
  // Already the chat for this page. Re-opening it would throw away context the
  // user has attached, and any tab event — including clicking into the pane —
  // would land here.
  if (chat.chat && chat.chat.url === page.url) return;
  const existing = await api().chats({ url: page.url, limit: 1 });
  if (existing.length > 0) {
    await openChat(existing[0].id);
    return;
  }
  await newChat();
}

export async function newChat(): Promise<void> {
  chat.chat = await api().createChat({
    url: page.url,
    pageTitle: page.title,
    favicon: page.favicon,
  });
  chat.messages = [];
  chat.tray = [];
  chat.error = "";
  leaveHistory();
}

export async function openChat(id: string): Promise<void> {
  const got = await api().chat(id);
  chat.chat = got.chat;
  chat.messages = got.messages ?? [];
  chat.tray = [];
  chat.error = "";
  leaveHistory();
}

/**
 * Walking back through what has been sent, the way a shell's history or Claude
 * Code's prompt does.
 *
 * recall.at counts back from the newest message — 0 is the last thing sent —
 * and null means the user is writing rather than recalling. recall.draft holds
 * whatever they had in the field when they stepped into history, so stepping
 * back out returns it rather than the empty string.
 *
 * Nothing is stored separately: the messages are the history. Only the text
 * comes back, not the context that went with it — the tray is what the next
 * message is aimed at, and silently re-aiming it would be a surprise.
 */
const recall: { at: number | null; draft: string } = { at: null, draft: "" };

/** This chat's own turns, oldest first. */
function sentTexts(): string[] {
  return chat.messages.filter((m) => m.role === "user").map((m) => m.text);
}

/**
 * One step back through the sent messages. Returns the text to put in the
 * field, or null when there is nothing to recall and the caret should move
 * as it normally would.
 */
export function recallPrevious(): string | null {
  const sent = sentTexts();
  if (sent.length === 0) return null;
  if (recall.at === null) {
    recall.draft = chat.draft;
    recall.at = 0;
  } else if (recall.at < sent.length - 1) {
    recall.at += 1;
  }
  // At the oldest entry this returns the same text again, which is the point:
  // the key is still consumed, so the caret does not jump away from it.
  return sent[sent.length - 1 - recall.at];
}

/**
 * One step forward. Stepping past the newest entry returns the draft that was
 * being written when history was entered — which may be the empty string, and
 * so is not the same as null.
 */
export function recallNext(): string | null {
  if (recall.at === null) return null;
  const sent = sentTexts();
  if (recall.at === 0 || sent.length === 0) {
    recall.at = null;
    return recall.draft;
  }
  recall.at -= 1;
  return sent[sent.length - 1 - recall.at];
}

/**
 * Back to writing. Called when the user types, and after sending, so the next
 * step back reaches the message that was just sent rather than resuming from
 * wherever the cursor had wandered to.
 */
export function leaveHistory(): void {
  recall.at = null;
  recall.draft = "";
}

export function addContext(item: ContextItem): void {
  chat.tray.push(item);
}

export function removeContext(index: number): void {
  chat.tray.splice(index, 1);
}

/** How a message should treat the page's text. */
export interface SendOptions {
  /** Include the page's readable text with this message. */
  includePage?: boolean;
}

/** Sends the draft and streams the answer into the transcript. */
export async function send(opts: SendOptions = {}): Promise<void> {
  const text = chat.draft.trim();
  if (!text || chat.sending) return;

  // Held before anything can reset the tray below.
  const context = chat.tray.slice();

  // Re-read which page is on screen rather than trusting the last tab event to
  // have arrived. A missed event is invisible, and its symptom is an answer
  // confidently about the previous page.
  await refreshActiveTab().catch(() => {});
  if (chat.chat && chat.chat.url && page.url && chat.chat.url !== page.url) {
    // The page moved on. A fresh chat is not only the design's rule — the
    // agent session behind the old chat still holds the old page's text, and
    // the agent would answer from that rather than say it cannot tell.
    await openForCurrentPage();
  }
  if (!chat.chat) await newChat();
  const chatId = chat.chat!.id;

  // The page's text is only read when the user has allowed it — always, or
  // for this message. "Never" means never, whatever was asked for.
  const settings = await loadSettings();
  let read: PageSnapshot | null = null;
  const wantsPage = settings.pageAccess === "always" || (opts.includePage && settings.pageAccess !== "never");
  if (wantsPage) {
    read = await readPage().catch(() => null);
  }
  chat.messages.push({
    id: `local-${Date.now()}`,
    chatId,
    role: "user",
    text,
    createdAt: Date.now(),
    context,
  });
  const assistant = reactive<Message>({
    id: `pending-${Date.now()}`,
    chatId,
    role: "assistant",
    text: "",
    tools: [],
    createdAt: Date.now(),
    model: models.selection?.model,
  });
  chat.messages.push(assistant);

  chat.draft = "";
  chat.tray = [];
  chat.sending = true;
  leaveHistory();
  chat.error = "";
  abort = new AbortController();

  try {
    for await (const ev of api().send(
      chatId,
      {
        text,
        page: pageRef(read),
        context,
        model: models.selection ?? undefined,
        conciseness: settings.conciseness,
      },
      abort.signal,
    )) {
      applyTurnEvent(assistant, ev);
    }
  } catch (err) {
    const msg = err instanceof Error ? err.message : String(err);
    if (!abort.signal.aborted) {
      assistant.error = msg;
      chat.error = msg;
    }
  } finally {
    chat.sending = false;
    abort = null;
    // A tool cannot outlive the turn that called it, so no row is left
    // spinning — whatever the agent did or did not report about its end.
    for (const tool of assistant.tools ?? []) {
      if (tool.state === "running") tool.state = "done";
    }
    // The server owns the transcript; re-read it so ids and usage are exact.
    void refresh();
  }
}

/**
 * One event from the turn, folded into the message being built. Exported for
 * the tests: the tool-row merge below is the behaviour that was wrong, and it
 * is not reachable through send() without a server.
 */
export function applyTurnEvent(assistant: Message, ev: TurnEvent): void {
  switch (ev.event) {
    case "turn.start":
      if (ev.messageId) assistant.id = ev.messageId;
      if (ev.model) assistant.model = ev.model;
      break;
    case "text.delta":
      if (ev.text) assistant.text += ev.text;
      break;
    case "tool": {
      const tool = ev.tool;
      if (!tool) break;
      const list = assistant.tools ?? (assistant.tools = []);
      // The same rule the server's appendTool follows, because the pane draws
      // the turn from the stream and only re-reads the transcript afterwards:
      // the call's own id when both sides have one, else name and target.
      const row = list.find((t) =>
        tool.id && t.id ? t.id === tool.id : t.name === tool.name && t.target === tool.target && t.state === "running",
      );
      // A later frame fills in what an earlier one did not know, and never
      // blanks what it does: the start names the tool before the arguments
      // have finished arriving, and the end carries the target.
      if (row) {
        row.state = tool.state;
        if (tool.name && tool.name !== "tool") row.name = tool.name;
        if (tool.target) row.target = tool.target;
        if (tool.detail) row.detail = tool.detail;
      } else {
        list.push({ ...tool });
      }
      break;
    }
    case "usage":
      if (ev.usage) assistant.usage = ev.usage;
      break;
    case "error":
      assistant.error = ev.message ?? "The runtime reported an error.";
      break;
  }
}

/** Stops the running turn, both here and in the agent. */
export async function cancel(): Promise<void> {
  if (!chat.chat) return;
  abort?.abort();
  try {
    await api().cancelChat(chat.chat.id);
  } catch {
    /* the turn may already have finished */
  }
  chat.sending = false;
}

/** Re-reads the transcript from the server. */
export async function refresh(): Promise<void> {
  if (!chat.chat) return;
  try {
    const got = await api().chat(chat.chat.id);
    chat.chat = got.chat;
    chat.messages = got.messages ?? [];
  } catch {
    /* keep what is on screen if the refresh fails */
  }
}

/** Retries the last question after a failure. */
/**
 * What the user decided about each page's text this session — including a no,
 * so the pane asks once per page rather than before every message. Decisions
 * are not persisted: a new panel asks again.
 */
const pageDecisions = new Map<string, boolean>();

export function rememberPageConsent(url: string, allowed = true): void {
  if (url) pageDecisions.set(url, allowed);
}

export function pageConsentGiven(url: string): boolean {
  return pageDecisions.get(url) === true;
}

/** Whether the user has already answered for this page. */
export function pageDecided(url: string): boolean {
  return pageDecisions.has(url);
}

export async function retryLast(): Promise<void> {
  const lastUser = [...chat.messages].reverse().find((m) => m.role === "user");
  if (!lastUser) return;
  chat.draft = lastUser.text;
  chat.tray = lastUser.context?.slice() ?? [];
  await send({ includePage: pageConsentGiven(page.url) });
}
