import { describe, expect, it } from "vitest";
import { chatMarkdown, contextLine, turnMarkdown } from "@/pane/transcript";
import type { Chat, Message } from "@/api/types";

const at = Date.parse("2026-09-08T14:02:00Z");

const message = (over: Partial<Message>): Message => ({
  id: "m1",
  chatId: "c1",
  role: "user",
  text: "Is the Growth plan enough for about 40M events a month?",
  createdAt: at,
  ...over,
});

const chat: Chat = {
  id: "c1",
  title: "Is the Growth plan enough for about 40M events a month?",
  url: "https://example.com/pricing",
  host: "example.com",
  model: "Opus 5",
  createdAt: at,
  updatedAt: at,
  messageCount: 2,
};

describe("a piece of context on one line", () => {
  it("reads an element as its selector and measured size", () => {
    expect(contextLine({ type: "element", selector: "article.pg-tier", rect: [320, 412] })).toBe(
      "article.pg-tier · 320 × 412",
    );
  });

  it("leaves the size off an element that was never measured", () => {
    expect(contextLine({ type: "element", selector: "article.pg-tier" })).toBe("article.pg-tier");
  });

  it("reads a file as its path, not its basename — a transcript outlives the pane", () => {
    expect(contextLine({ type: "file", path: "~/dev/pricing.md" })).toBe("~/dev/pricing.md");
  });

  it("quotes a text selection", () => {
    expect(contextLine({ type: "text", quote: "25M events" })).toBe("“25M events”");
  });
});

describe("one turn", () => {
  it("carries what was said, and what it was aimed at", () => {
    const md = turnMarkdown(
      message({ context: [{ type: "element", selector: "table.pg-table", rect: [320, 412] }] }),
    );
    expect(md).toContain("## You — ");
    expect(md).toContain("> table.pg-table · 320 × 412");
    expect(md).toContain("Is the Growth plan enough");
  });

  it("titles an answer with the model that gave it", () => {
    expect(turnMarkdown(message({ role: "assistant", model: "Opus 5", text: "Not on its own." }))).toContain(
      "## Opus 5 — ",
    );
  });

  it("falls back to the product name when the model went unrecorded", () => {
    expect(turnMarkdown(message({ role: "assistant", model: undefined, text: "Not on its own." }))).toContain(
      "## AI Skope — ",
    );
  });

  it("keeps a failed turn in the record", () => {
    const md = turnMarkdown(message({ role: "assistant", text: "", error: "Opus 5 didn't answer in time." }));
    expect(md).toContain("*Opus 5 didn't answer in time.*");
  });

  it("leaves tool lines out — they are progress, not content", () => {
    const md = turnMarkdown(
      message({
        role: "assistant",
        text: "Not on its own.",
        tools: [{ name: "read", target: "table.pg-table", state: "done", detail: "7 rows" }],
      }),
    );
    expect(md).not.toContain("table.pg-table");
    expect(md).not.toContain("7 rows");
  });
});

describe("the whole chat", () => {
  const messages = [
    message({ context: [{ type: "element", selector: "article.pg-tier", rect: [320, 412] }] }),
    message({ id: "m2", role: "assistant", model: "Opus 5", text: "Not on its own." }),
  ];

  it("opens on the chat's own title and where it happened", () => {
    const md = chatMarkdown(chat, messages);
    const [title, source] = md.split("\n");
    expect(title).toBe("# Is the Growth plan enough for about 40M events a month?");
    expect(source).toContain("https://example.com/pricing");
    expect(source).toContain("AI Skope");
    expect(source).toContain("Opus 5");
  });

  it("runs the turns in order under it", () => {
    const md = chatMarkdown(chat, messages);
    expect(md.indexOf("## You")).toBeLessThan(md.indexOf("## Opus 5"));
    expect(md.endsWith("\n")).toBe(true);
  });

  it("falls back to the first message when there is no chat record yet", () => {
    expect(chatMarkdown(null, messages).split("\n")[0]).toBe(
      "# Is the Growth plan enough for about 40M events a month?".slice(0, 62),
    );
  });

  it("says something rather than nothing for an empty chat", () => {
    expect(chatMarkdown(null, [])).toContain("# AI Skope chat");
  });
});
