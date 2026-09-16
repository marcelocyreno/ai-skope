/**
 * The worker's only decision is whether the two context-menu entries exist.
 * It is driven here through a chrome stub rather than by exporting the
 * decision, because the wiring is the part that was wrong: which events are
 * listened to, and what a settings change does to a menu already on screen.
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

vi.stubGlobal("chrome", {
  runtime: { onInstalled: on("installed"), onStartup: on("startup"), onMessage: on("message") },
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
    session: { set: async () => {} },
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
