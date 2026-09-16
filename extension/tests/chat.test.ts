/**
 * The pane draws a turn from the stream as it arrives and only re-reads the
 * server's transcript afterwards, so it has to merge tool frames by the same
 * rule the server does. pi announces one call in three frames that disagree
 * about the name, and they used to become three rows.
 */
import { beforeEach, describe, expect, it, vi } from "vitest";
import type { Message, ToolRecord, TurnEvent } from "@/api/types";

vi.mock("@/stores/connection", () => ({ connection: {}, api: () => ({}) }));
vi.mock("@/stores/models", () => ({ models: { selection: null } }));
vi.mock("@/stores/page", () => ({
  page: {},
  pageRef: () => null,
  readPage: async () => null,
  refreshActiveTab: async () => {},
}));
vi.mock("@/stores/storage", () => ({ loadSettings: async () => ({}) }));

const { applyTurnEvent, chat, recallPrevious, recallNext, leaveHistory } = await import(
  "@/stores/chat"
);

const blank = (): Message =>
  ({ id: "m", chatId: "c", role: "assistant", text: "", tools: [], createdAt: 0 }) as Message;

const tool = (t: ToolRecord): TurnEvent => ({ event: "tool", tool: t }) as TurnEvent;

describe("tool rows in a live turn", () => {
  it("folds a call's frames into one row, keeping the best name and target", () => {
    const m = blank();
    // The start knows the tool but not yet what it will read.
    applyTurnEvent(m, tool({ id: "call_7", name: "read", state: "running" }));
    // The end carries the assembled arguments.
    applyTurnEvent(m, tool({ id: "call_7", name: "read", target: "README.md", state: "running" }));
    applyTurnEvent(m, tool({ id: "call_7", name: "read", target: "README.md", state: "done" }));

    expect(m.tools).toEqual([{ id: "call_7", name: "read", target: "README.md", state: "done" }]);
  });

  it("keeps two calls to the same tool apart", () => {
    const m = blank();
    applyTurnEvent(m, tool({ id: "a", name: "read", target: "one.md", state: "running" }));
    applyTurnEvent(m, tool({ id: "b", name: "read", target: "two.md", state: "running" }));
    applyTurnEvent(m, tool({ id: "a", name: "read", target: "one.md", state: "done" }));

    expect(m.tools?.map((t) => t.state)).toEqual(["done", "running"]);
  });

  it("never blanks a target a later frame does not repeat", () => {
    const m = blank();
    applyTurnEvent(m, tool({ id: "call_7", name: "read", target: "README.md", state: "running" }));
    applyTurnEvent(m, tool({ id: "call_7", name: "read", state: "done" }));

    expect(m.tools?.[0].target).toBe("README.md");
  });

  it("still merges an agent that gives no id, by name and target", () => {
    const m = blank();
    applyTurnEvent(m, tool({ name: "grep", target: "TODO", state: "running" }));
    applyTurnEvent(m, tool({ name: "grep", target: "TODO", state: "done" }));

    expect(m.tools).toHaveLength(1);
    expect(m.tools?.[0].state).toBe("done");
  });

  it("carries a failure through rather than reporting it as done", () => {
    const m = blank();
    applyTurnEvent(m, tool({ id: "call_9", name: "read", state: "running" }));
    applyTurnEvent(m, tool({ id: "call_9", name: "read", target: "gone.md", state: "failed" }));

    expect(m.tools?.[0].state).toBe("failed");
  });
});

describe("walking back through what was sent", () => {
  /** A transcript of user turns with the answers in between, as it really is. */
  const transcript = (...asked: string[]) =>
    asked.flatMap((text, i) => [
      { id: `u${i}`, chatId: "c", role: "user", text, createdAt: i } as Message,
      { id: `a${i}`, chatId: "c", role: "assistant", text: "…", createdAt: i } as Message,
    ]);

  beforeEach(() => {
    chat.messages = transcript("first", "second", "third");
    chat.draft = "";
    leaveHistory();
  });

  it("steps back from the newest to the oldest", () => {
    expect(recallPrevious()).toBe("third");
    expect(recallPrevious()).toBe("second");
    expect(recallPrevious()).toBe("first");
  });

  it("stays on the oldest rather than falling off the end", () => {
    recallPrevious();
    recallPrevious();
    expect(recallPrevious()).toBe("first");
    expect(recallPrevious()).toBe("first");
  });

  it("gives back the draft that was interrupted, empty or not", () => {
    chat.draft = "half a question";
    expect(recallPrevious()).toBe("third");
    expect(recallNext()).toBe("half a question");

    // An empty draft comes back as "", which is a recall — not "nothing to do".
    chat.draft = "";
    leaveHistory();
    expect(recallPrevious()).toBe("third");
    expect(recallNext()).toBe("");
  });

  it("does nothing on the way down when the user is not in history", () => {
    // null is what tells the composer to leave the caret alone.
    expect(recallNext()).toBeNull();
  });

  it("has nothing to recall in a chat with no turns yet", () => {
    chat.messages = [];
    expect(recallPrevious()).toBeNull();
  });

  it("starts again from the newest once the user types", () => {
    recallPrevious();
    recallPrevious();
    leaveHistory(); // what @input does
    expect(recallPrevious()).toBe("third");
  });

  it("ignores the assistant's turns", () => {
    chat.messages = transcript("only mine");
    expect(recallPrevious()).toBe("only mine");
    expect(recallPrevious()).toBe("only mine");
  });
});
