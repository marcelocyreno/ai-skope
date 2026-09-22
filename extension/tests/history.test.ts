import { describe, expect, it, beforeEach, vi } from "vitest";
import type { Chat } from "@/api/types";

// The history store reaches the connection store, which now reads the manifest
// at import time to know which server API this build speaks. Every context the
// extension actually runs in has that; a bare test environment does not.
vi.stubGlobal("chrome", { runtime: { getManifest: () => ({ version: "0.0.0-test" }) } });

const { history, groups } = await import("@/stores/history");
const { page } = await import("@/stores/page");

function chat(id: string, url: string, host: string): Chat {
  return {
    id, url, host, title: id, createdAt: 0, updatedAt: 0, messageCount: 1,
  } as Chat;
}

describe("history grouping", () => {
  beforeEach(() => {
    history.chats = [];
    page.url = "";
    page.title = "";
  });

  it("puts each chat in exactly one group", () => {
    page.url = "https://n.example/pricing";
    history.chats = [
      chat("a", "https://n.example/pricing", "n.example"),
      chat("b", "https://n.example/docs", "n.example"),
      chat("c", "https://other.example/x", "other.example"),
    ];
    const seen = groups.value.flatMap((g) => g.chats.map((c) => c.id));
    expect(seen).toHaveLength(3);
    expect(new Set(seen).size).toBe(3);
    expect(groups.value.map((g) => g.key)).toEqual(["page", "site", "all"]);
  });

  it("does not repeat chats when there is no current page", () => {
    // The pane can open before a tab is known; a chat must still appear once.
    history.chats = [chat("a", "https://n.example/pricing", "n.example")];
    const seen = groups.value.flatMap((g) => g.chats.map((c) => c.id));
    expect(seen).toEqual(["a"]);
  });

  it("groups local-file chats under their own host", () => {
    page.url = "file:///Users/me/dev/readme.md";
    history.chats = [
      chat("local", "file:///Users/me/dev/readme.md", "local file"),
      chat("web", "https://n.example/x", "n.example"),
    ];
    expect(groups.value[0].chats.map((c) => c.id)).toEqual(["local"]);
    expect(groups.value[1].chats.map((c) => c.id)).toEqual(["web"]);
  });
});
