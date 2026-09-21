/**
 * The worker's two jobs around the right-click menu: whether the entry exists,
 * and where a click on it goes. Both are driven here through a chrome stub
 * rather than by exporting the decisions, because the wiring is the part that
 * was wrong — which events are listened to, what a settings change does to a
 * menu already on screen, and whether a pane that is already open hears the
 * click at all.
 */
import { describe, expect, it, beforeEach, vi } from "vitest";
import { DEFAULTS, type Settings } from "@/stores/storage";

type Listener = (...args: unknown[]) => void;

const listeners: Record<string, Listener[]> = {};
const on = (name: string) => ({
  addListener: (fn: Listener) => (listeners[name] ??= []).push(fn),
  removeListener: () => {},
});
const fire = async (name: string, ...args: unknown[]) => {
  for (const fn of listeners[name] ?? []) fn(...args);
  // syncMenu reads storage, so give the microtask queue a turn.
  await Promise.resolve();
  await Promise.resolve();
};

let stored: Partial<Settings> = {};
const created: string[] = [];
let removeAllCalls = 0;

/**
 * A pane that is open and listening, and what it answers with. null is a pane
 * that is not running: Chrome rejects sendMessage when no extension page is
 * there to hear it, which is the only signal the worker gets.
 */
let openPane: (() => unknown) | null = null;
const heard: Record<string, unknown>[] = [];
const queued: Record<string, unknown>[] = [];

vi.stubGlobal("chrome", {
  runtime: {
    onInstalled: on("installed"),
    onStartup: on("startup"),
    onMessage: on("message"),
    sendMessage: async (msg: Record<string, unknown>) => {
      if (!openPane) throw new Error("Could not establish connection. Receiving end does not exist.");
      heard.push(msg);
      return openPane();
    },
  },
  commands: { onCommand: on("command") },
  contextMenus: {
    onClicked: on("clicked"),
    create: (o: { id: string }) => created.push(o.id),
    removeAll: (cb?: () => void) => {
      removeAllCalls++;
      cb?.();
    },
  },
  storage: {
    local: { get: async () => ({ settings: stored }) },
    session: {
      set: async (o: Record<string, unknown>) => {
        queued.push(o);
      },
    },
    onChanged: on("changed"),
  },
  sidePanel: { setPanelBehavior: async () => {}, open: async () => {} },
  tabs: { query: async () => [] },
});

/** What the options page does: write the settings, then notify every surface. */
const setSetting = (patch: Partial<Settings>) => {
  stored = { ...stored, ...patch };
  return fire("changed", { settings: { newValue: stored } }, "local");
};

await import("@/worker/service-worker");

describe("the right-click menu is opt-in", () => {
  beforeEach(() => {
    created.length = 0;
    removeAllCalls = 0;
  });

  it("leaves a fresh install's menu alone", async () => {
    expect(DEFAULTS.contextMenu).toBe(false);
    stored = {};
    await fire("installed");
    expect(created).toEqual([]);
  });

  it("adds the entry when the setting is turned on, without a reload", async () => {
    await setSetting({ contextMenu: true });
    expect(created).toEqual(["skope-ask"]);
  });

  it("does not churn the menu when an unrelated setting changes", async () => {
    await setSetting({ contextMenu: true });
    created.length = 0;
    removeAllCalls = 0;

    await setSetting({ palette: "ember" });

    expect(created).toEqual([]);
    expect(removeAllCalls).toBe(0);
  });

  it("removes the entry when the setting is turned off", async () => {
    await setSetting({ contextMenu: true });
    created.length = 0;
    removeAllCalls = 0;

    await setSetting({ contextMenu: false });

    expect(removeAllCalls).toBe(1);
    expect(created).toEqual([]);
  });

  it("puts the entry back after a browser restart", async () => {
    // Menus do not survive a restart, and onInstalled does not fire on one.
    stored = { contextMenu: true };
    await fire("startup");
    expect(created).toEqual(["skope-ask"]);
  });
});

describe("a click on the entry reaches the pane", () => {
  /** Delivery decides after an await, so let the whole chain settle. */
  const click = async () => {
    await fire("clicked", { selectionText: "the quoted words" }, { windowId: 1 });
    await new Promise((r) => setTimeout(r, 0));
  };

  const selection = { type: "text", quote: "the quoted words" };

  beforeEach(() => {
    heard.length = 0;
    queued.length = 0;
    openPane = null;
  });

  it("hands the selection straight to a pane that is already open", async () => {
    openPane = () => ({ received: true });

    await click();

    expect(heard).toEqual([{ kind: "skope:selection-action", action: "add", selection }]);
    // Acted on live, so nothing is left behind to replay on the next open.
    expect(queued).toEqual([]);
  });

  it("queues the selection for a pane that is not running yet", async () => {
    await click();

    expect(heard).toEqual([]);
    expect(queued).toHaveLength(1);
    expect(queued[0].pendingAction).toMatchObject({ action: "add", selection });
  });

  it("queues it as well when a pane hears the click but does not answer", async () => {
    // An unanswered message is indistinguishable from an absent pane, and
    // dropping the click would be worse than a chip arriving a moment late.
    openPane = () => undefined;

    await click();

    expect(queued).toHaveLength(1);
  });
});
