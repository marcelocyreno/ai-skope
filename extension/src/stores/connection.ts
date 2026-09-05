/**
 * The pane's link to the server: pairing state, health, and the push stream.
 *
 * The model chip's dot and the offline strip both read from here, so there is
 * one source of truth for "can we answer a question right now".
 */
import { reactive, computed } from "vue";
import { SkopeClient } from "@/api/client";
import { ApiError, NetworkError } from "@/api/errors";
import type { Health, RuntimeInfo, ServerEvent } from "@/api/types";
import { loadSettings, saveSettings, onSettingsChanged, type Settings } from "./storage";

export type ConnectionState = "connecting" | "unpaired" | "online" | "offline";

interface ConnectionStore {
  state: ConnectionState;
  settings: Settings | null;
  health: Health | null;
  runtimes: RuntimeInfo[];
  error: string;
  /** Seconds until the next reconnect attempt, for the offline strip. */
  retryIn: number;
}

export const connection = reactive<ConnectionStore>({
  state: "connecting",
  settings: null,
  health: null,
  runtimes: [],
  error: "",
  retryIn: 0,
});

export const isOnline = computed(() => connection.state === "online");

let client: SkopeClient | null = null;
let eventsAbort: AbortController | null = null;
let retryTimer: number | undefined;
let countdownTimer: number | undefined;
let backoffMs = 1000;

/** api returns the client, which only exists once settings have loaded. */
export function api(): SkopeClient {
  if (!client) throw new Error("The connection is not ready yet.");
  return client;
}

export async function initConnection(): Promise<void> {
  const settings = await loadSettings();
  connection.settings = settings;
  applyAppearance(settings);

  client = new SkopeClient({
    baseUrl: settings.baseUrl,
    token: settings.token,
    onUnauthorized: () => {
      // Losing a pairing is rare and confusing; say so in the console, or a
      // pane that silently returns to the pairing screen looks like a bug.
      console.warn("[ai-skope] the server rejected our pairing; clearing it");
      void saveSettings({ token: "" });
      connection.state = "unpaired";
      connection.error = "This browser is no longer paired with the server.";
    },
  });

  onSettingsChanged((s) => {
    connection.settings = s;
    applyAppearance(s);
    if (client && (client.baseUrl !== s.baseUrl.replace(/\/+$/, "") || client.token !== s.token)) {
      client.baseUrl = s.baseUrl.replace(/\/+$/, "");
      client.token = s.token;
      void connect();
    }
  });

  await connect();
}

/** connect probes health, then subscribes to the push stream. */
export async function connect(): Promise<void> {
  if (!client) return;
  window.clearTimeout(retryTimer);
  window.clearInterval(countdownTimer);
  connection.retryIn = 0;

  try {
    connection.health = await client.health();
    connection.error = "";
    if (!client.paired) {
      connection.state = "unpaired";
      return;
    }
    // A token can be stale even when the server is up; runtimes proves it.
    connection.runtimes = await client.runtimes();
    connection.state = "online";
    backoffMs = 1000;
    startEventStream();
  } catch (err) {
    if (err instanceof ApiError && err.needsPairing) {
      connection.state = "unpaired";
      connection.error = err.message;
      return;
    }
    connection.state = "offline";
    connection.error =
      err instanceof NetworkError
        ? "The AI Skope Server isn't reachable."
        : err instanceof Error
          ? err.message
          : String(err);
    scheduleRetry();
  }
}

/** scheduleRetry backs off, and counts down so the strip can show the wait. */
function scheduleRetry(): void {
  const wait = backoffMs;
  backoffMs = Math.min(backoffMs * 2, 30000);
  connection.retryIn = Math.ceil(wait / 1000);
  countdownTimer = window.setInterval(() => {
    connection.retryIn = Math.max(0, connection.retryIn - 1);
  }, 1000);
  retryTimer = window.setTimeout(() => {
    window.clearInterval(countdownTimer);
    void connect();
  }, wait);
}

function startEventStream(): void {
  eventsAbort?.abort();
  const abort = new AbortController();
  eventsAbort = abort;
  void (async () => {
    try {
      for await (const ev of api().events(abort.signal)) {
        handleServerEvent(ev);
      }
    } catch {
      // The stream ends when the panel closes or the server goes away; the
      // health probe below decides which and reconnects if needed.
    }
    if (!abort.signal.aborted) void connect();
  })();
}

function handleServerEvent(ev: ServerEvent): void {
  if (ev.type === "runtime.status" && Array.isArray(ev.data)) {
    connection.runtimes = ev.data as RuntimeInfo[];
  }
}

export function stopConnection(): void {
  eventsAbort?.abort();
  window.clearTimeout(retryTimer);
  window.clearInterval(countdownTimer);
}

/** pair redeems the code shown by `aiss pair`. */
export async function pair(code: string): Promise<void> {
  if (!client) throw new Error("The connection is not ready yet.");
  const out = await client.pair(code);
  await saveSettings({ token: out.token, serverId: out.serverId });
  connection.state = "connecting";
  await connect();
}

export async function setBaseUrl(url: string): Promise<void> {
  await saveSettings({ baseUrl: url });
  if (client) client.baseUrl = url.replace(/\/+$/, "");
  await connect();
}

/** applyAppearance puts the theme and palette on the document root. */
export function applyAppearance(s: Settings): void {
  const root = document.documentElement;
  if (s.theme === "system") root.removeAttribute("data-theme");
  else root.setAttribute("data-theme", s.theme);
  if (s.palette === "graphite") root.removeAttribute("data-palette");
  else root.setAttribute("data-palette", s.palette);
  root.style.fontSize = s.textSize === "small" ? "13px" : s.textSize === "large" ? "16px" : "";
}
