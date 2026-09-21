/**
 * The composer's answer-length stop is chosen per message and kept in
 * chrome.storage.local, so the only thing that can go wrong invisibly is the
 * hand-off: send() has to read it at send time and put it on the request.
 * Nothing else in the pane carries it, and the server reads it from there.
 */
import { beforeEach, describe, expect, it, vi } from "vitest";
import type { SendRequest } from "@/api/types";

const sent: SendRequest[] = [];
const settings = { pageAccess: "never" as const, conciseness: 4 };

vi.mock("@/stores/connection", () => ({
  connection: { state: "online" },
  api: () => ({
    createChat: async () => ({ id: "c1", url: "https://example.test", title: "" }),
    chat: async () => ({ chat: { id: "c1" }, messages: [] }),
    send: async function* (_chatId: string, req: SendRequest) {
      sent.push(req);
    },
  }),
}));
vi.mock("@/stores/models", () => ({ models: { selection: null } }));
vi.mock("@/stores/page", () => ({
  page: { url: "https://example.test" },
  pageRef: () => null,
  readPage: async () => null,
  refreshActiveTab: async () => {},
}));
vi.mock("@/stores/storage", () => ({ loadSettings: async () => settings }));

const { chat, send } = await import("@/stores/chat");

describe("the answer-length stop", () => {
  beforeEach(() => {
    sent.length = 0;
    chat.chat = null;
    chat.messages = [];
    chat.tray = [];
    chat.draft = "";
  });

  it("travels with the message it was chosen for", async () => {
    settings.conciseness = 2;
    chat.draft = "what does this page charge for overage?";
    await send();

    expect(sent).toHaveLength(1);
    expect(sent[0].conciseness).toBe(2);
  });

  it("sends the neutral stop rather than leaving it out", async () => {
    // 4 has to reach the server as 4: the server treats it and "nothing" the
    // same, but a panel that omits the field cannot be told from an old one.
    settings.conciseness = 4;
    chat.draft = "and this one?";
    await send();

    expect(sent[0].conciseness).toBe(4);
  });
});
