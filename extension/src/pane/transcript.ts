/**
 * The conversation as Markdown, for the clipboard.
 *
 * The rule throughout: copy what was said, not what the pane drew. Answers
 * travel as the Markdown the model sent rather than the HTML the transcript
 * rendered — message.text already holds it — and tool lines stay out, because
 * they are progress rather than content.
 *
 * Context chips are the one piece of pane furniture that survives, as a
 * blockquote under the turn that carried them: an answer read without knowing
 * what it was aimed at is a different answer.
 */
import type { Chat, ContextItem, Message } from "@/api/types";

const at = (ms: number) =>
  new Date(ms).toLocaleTimeString([], { hour: "2-digit", minute: "2-digit" });

const on = (ms: number) =>
  new Date(ms).toLocaleDateString([], { day: "numeric", month: "short", year: "numeric" });

/** How a piece of context reads on one line — the chip's label, in text. */
export function contextLine(item: ContextItem): string {
  if (item.type === "element") {
    const size = item.rect?.length === 2 ? ` · ${item.rect[0]} × ${item.rect[1]}` : "";
    return `${item.selector ?? "element"}${size}`;
  }
  if (item.type === "file") return item.path ?? item.title ?? "file";
  return `“${item.quote ?? ""}”`;
}

/** One turn: who, when, what it was aimed at, and what was said. */
export function turnMarkdown(m: Message): string {
  const who = m.role === "user" ? "You" : (m.model ?? "AI Skope");
  const context = (m.context ?? []).map((c) => `> ${contextLine(c)}`).join("\n");
  // A turn that failed is part of the record; a transcript that drops it lies.
  const body = [m.text.trim(), m.error ? `*${m.error}*` : ""].filter(Boolean).join("\n\n");
  return [`## ${who} — ${at(m.createdAt)}`, context, body].filter(Boolean).join("\n\n");
}

/**
 * The whole chat. The H1 is the chat's own auto-title — the first message,
 * capped at 60 characters, exactly as History labels it.
 */
export function chatMarkdown(chat: Chat | null, messages: Message[]): string {
  const first = messages[0];
  const title = chat?.title || first?.text.slice(0, 60) || "AI Skope chat";
  const source = [chat?.url, "AI Skope", chat?.model, first ? on(first.createdAt) : ""]
    .filter(Boolean)
    .join(" · ");

  return [`# ${title}\n${source}`, ...messages.map(turnMarkdown)].join("\n\n") + "\n";
}
