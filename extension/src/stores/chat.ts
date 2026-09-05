/**
 * The conversation: the context tray, the transcript, and the live turn.
 *
 * Streaming appends to a reactive message object, so a delta updates that
 * message's text node and nothing else re-renders.
 */
import { reactive, computed } from "vue";
import { api, connection } from "./connection";
import { models } from "./models";
import { page, pageRef, readPageText } from "./page";
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
}

export async function openChat(id: string): Promise<void> {
  const got = await api().chat(id);
  chat.chat = got.chat;
  chat.messages = got.messages ?? [];
  chat.tray = [];
  chat.error = "";
}

export function addContext(item: ContextItem): void {
  chat.tray.push(item);
}

export function removeContext(index: number): void {
  chat.tray.splice(index, 1);
}

/** Sends the draft and streams the answer into the transcript. */
export async function send(): Promise<void> {
  const text = chat.draft.trim();
  if (!text || chat.sending) return;
  if (!chat.chat) await newChat();
  const chatId = chat.chat!.id;

  const settings = await loadSettings();
  let pageText: string | undefined;
  if (settings.pageAccess === "always") {
    pageText = await readPageText().catch(() => "");
  }

  const context = chat.tray.slice();
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
  chat.error = "";
  abort = new AbortController();

  try {
    for await (const ev of api().send(
      chatId,
      { text, page: pageRef(pageText), context, model: models.selection ?? undefined },
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
    // The server owns the transcript; re-read it so ids and usage are exact.
    void refresh();
  }
}

function applyTurnEvent(assistant: Message, ev: TurnEvent): void {
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
      const running = list.find((t) => t.name === tool.name && t.target === tool.target && t.state === "running");
      if (running) Object.assign(running, tool);
      else list.push({ ...tool });
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
export async function retryLast(): Promise<void> {
  const lastUser = [...chat.messages].reverse().find((m) => m.role === "user");
  if (!lastUser) return;
  chat.draft = lastUser.text;
  chat.tray = lastUser.context?.slice() ?? [];
  await send();
}
