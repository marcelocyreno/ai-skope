/**
 * The typed client for the AI Skope Server.
 *
 * It runs in the side panel document (not the service worker): MV3 kills an
 * idle worker, which would cut a turn off mid-answer, whereas the panel
 * document lives exactly as long as the panel is open.
 *
 * The extension's requests carry Origin: chrome-extension://<id>, which is
 * what the server's origin check expects, and 127.0.0.1 is in host_permissions
 * so they are never subject to CORS.
 */
import { ApiError, NetworkError } from "./errors";
import { parseJSONFrames } from "./sse";
import type {
  Capabilities,
  Chat,
  ContextItem,
  FileContent,
  FileEntry,
  Folder,
  Health,
  Message,
  ModelOption,
  Note,
  Provider,
  RuntimeInfo,
  SendRequest,
  Selection,
  ServerEvent,
  TurnEvent,
} from "./types";

export const DEFAULT_BASE_URL = "http://127.0.0.1:7331";

export interface ClientOptions {
  baseUrl?: string;
  token?: string;
  /** Called whenever the server rejects our pairing, so the pane can re-pair. */
  onUnauthorized?: () => void;
}

export class SkopeClient {
  baseUrl: string;
  token: string;
  private onUnauthorized?: () => void;

  constructor(opts: ClientOptions = {}) {
    this.baseUrl = (opts.baseUrl ?? DEFAULT_BASE_URL).replace(/\/+$/, "");
    this.token = opts.token ?? "";
    this.onUnauthorized = opts.onUnauthorized;
  }

  get paired(): boolean {
    return this.token !== "";
  }

  // ---- plumbing -----------------------------------------------------------

  private headers(json = true): HeadersInit {
    const h: Record<string, string> = {};
    if (json) h["Content-Type"] = "application/json";
    if (this.token) h["Authorization"] = `Bearer ${this.token}`;
    return h;
  }

  private async raw(path: string, init: RequestInit = {}): Promise<Response> {
    const url = this.baseUrl + path;
    let resp: Response;
    try {
      resp = await fetch(url, { ...init, headers: { ...this.headers(), ...(init.headers ?? {}) } });
    } catch (cause) {
      throw new NetworkError(url, cause);
    }
    if (resp.ok) return resp;

    // The server answers errors as {"error":{"code","message"}}.
    let code = "http_error";
    let message = `The server replied ${resp.status}.`;
    try {
      const body = (await resp.json()) as { error?: { code?: string; message?: string } };
      if (body.error?.code) code = body.error.code;
      if (body.error?.message) message = body.error.message;
    } catch {
      /* not JSON; keep the generic message */
    }
    const err = new ApiError(resp.status, code, message);
    if (err.needsPairing) this.onUnauthorized?.();
    throw err;
  }

  private async json<T>(path: string, init: RequestInit = {}): Promise<T> {
    const resp = await this.raw(path, init);
    if (resp.status === 204) return undefined as T;
    return (await resp.json()) as T;
  }

  private post<T>(path: string, body?: unknown): Promise<T> {
    return this.json<T>(path, { method: "POST", body: body === undefined ? undefined : JSON.stringify(body) });
  }

  private patch<T>(path: string, body: unknown): Promise<T> {
    return this.json<T>(path, { method: "PATCH", body: JSON.stringify(body) });
  }

  private del(path: string): Promise<void> {
    return this.json<void>(path, { method: "DELETE" });
  }

  // ---- connection ---------------------------------------------------------

  /** health needs no token: it is how the pane knows the server is up at all. */
  async health(): Promise<Health> {
    const url = `${this.baseUrl}/v1/health`;
    try {
      const resp = await fetch(url);
      if (!resp.ok) throw new ApiError(resp.status, "http_error", `The server replied ${resp.status}.`);
      return (await resp.json()) as Health;
    } catch (cause) {
      if (cause instanceof ApiError) throw cause;
      throw new NetworkError(url, cause);
    }
  }

  /** Redeems the one-time code from `aiss pair` for a bearer token. */
  async pair(code: string, label = "Chrome"): Promise<{ token: string; serverId: string }> {
    const origin = `chrome-extension://${chrome.runtime.id}`;
    const out = await this.json<{ token: string; serverId: string }>("/v1/pair", {
      method: "POST",
      body: JSON.stringify({ code: code.trim().toUpperCase(), origin, label }),
    });
    this.token = out.token;
    return out;
  }

  capabilities(): Promise<Capabilities> {
    return this.json<Capabilities>("/v1/capabilities");
  }

  /**
   * events subscribes to the server's push stream (runtime health, index
   * progress, folder changes). The caller abort()s to unsubscribe.
   */
  async *events(signal: AbortSignal): AsyncGenerator<ServerEvent> {
    const resp = await this.raw("/v1/events", { signal, headers: { Accept: "text/event-stream" } });
    if (!resp.body) return;
    for await (const frame of parseJSONFrames<unknown>(resp.body, signal)) {
      yield { type: frame.event as ServerEvent["type"], at: Date.now(), data: frame.data };
    }
  }

  // ---- runtimes and models ------------------------------------------------

  async runtimes(): Promise<RuntimeInfo[]> {
    return (await this.json<{ runtimes: RuntimeInfo[] }>("/v1/runtimes")).runtimes ?? [];
  }

  async detectRuntimes(): Promise<RuntimeInfo[]> {
    return (await this.post<{ runtimes: RuntimeInfo[] }>("/v1/runtimes/detect")).runtimes ?? [];
  }

  setRuntimeEnabled(id: string, enabled: boolean, command?: string): Promise<RuntimeInfo> {
    return this.patch<RuntimeInfo>(`/v1/runtimes/${encodeURIComponent(id)}`, { enabled, command });
  }

  models(): Promise<{ models: ModelOption[]; default: Selection }> {
    return this.json<{ models: ModelOption[]; default: Selection }>("/v1/models");
  }

  setDefaultModel(sel: Selection): Promise<Selection> {
    return this.json<Selection>("/v1/models/default", { method: "PUT", body: JSON.stringify(sel) });
  }

  // ---- providers ----------------------------------------------------------

  async providers(): Promise<Provider[]> {
    return (await this.json<{ providers: Provider[] }>("/v1/providers")).providers ?? [];
  }

  createProvider(input: {
    kind: string;
    name?: string;
    baseUrl?: string;
    key?: string;
    availableTo?: string[];
  }): Promise<Provider> {
    return this.post<Provider>("/v1/providers", input);
  }

  updateProvider(id: string, input: Record<string, unknown>): Promise<Provider> {
    return this.patch<Provider>(`/v1/providers/${id}`, input);
  }

  deleteProvider(id: string): Promise<void> {
    return this.del(`/v1/providers/${id}`);
  }

  testProvider(id: string): Promise<{ ok: boolean; message: string; models?: { model: string }[] }> {
    return this.post(`/v1/providers/${id}/test`);
  }

  // ---- folders and files --------------------------------------------------

  async folders(): Promise<Folder[]> {
    return (await this.json<{ folders: Folder[] }>("/v1/folders")).folders ?? [];
  }

  addFolder(path: string, access: "read" | "read+watch" = "read"): Promise<Folder> {
    return this.post<Folder>("/v1/folders", { path, access });
  }

  setFolderAccess(id: string, access: "read" | "read+watch"): Promise<Folder> {
    return this.patch<Folder>(`/v1/folders/${id}`, { access });
  }

  removeFolder(id: string): Promise<void> {
    return this.del(`/v1/folders/${id}`);
  }

  reindexFolder(id: string): Promise<{ folderId: string; state: string }> {
    return this.post(`/v1/folders/${id}/reindex`);
  }

  async searchFiles(q: string, limit = 50): Promise<FileEntry[]> {
    const qs = new URLSearchParams({ q, limit: String(limit) });
    return (await this.json<{ files: FileEntry[] }>(`/v1/files/search?${qs}`)).files ?? [];
  }

  async recentFiles(limit = 20): Promise<FileEntry[]> {
    return (await this.json<{ files: FileEntry[] }>(`/v1/files/recent?limit=${limit}`)).files ?? [];
  }

  async browseFiles(path: string): Promise<FileEntry[]> {
    const qs = new URLSearchParams({ path });
    const out = await this.json<{ entries?: FileEntry[] }>(`/v1/files/browse?${qs}`);
    return out.entries ?? [];
  }

  readFile(path: string): Promise<FileContent> {
    return this.json<FileContent>(`/v1/files/read?${new URLSearchParams({ path })}`);
  }

  /** Turns a file:// URL from the browser into a path the server may read. */
  resolveFileUrl(url: string): Promise<{ path: string; display: string; folderId: string }> {
    return this.post("/v1/files/resolve", { url });
  }

  // ---- chats --------------------------------------------------------------

  async chats(filter: { url?: string; host?: string; q?: string; limit?: number } = {}): Promise<Chat[]> {
    const qs = new URLSearchParams();
    for (const [k, v] of Object.entries(filter)) if (v) qs.set(k, String(v));
    return (await this.json<{ chats: Chat[] }>(`/v1/chats?${qs}`)).chats ?? [];
  }

  createChat(input: { url?: string; title?: string; pageTitle?: string; favicon?: string }): Promise<Chat> {
    return this.post<Chat>("/v1/chats", input);
  }

  chat(id: string): Promise<{ chat: Chat; messages: Message[]; running: boolean }> {
    return this.json(`/v1/chats/${id}`);
  }

  renameChat(id: string, title: string): Promise<Chat> {
    return this.patch<Chat>(`/v1/chats/${id}`, { title });
  }

  deleteChat(id: string): Promise<void> {
    return this.del(`/v1/chats/${id}`);
  }

  restoreChat(id: string): Promise<Chat> {
    return this.post<Chat>(`/v1/chats/${id}/restore`);
  }

  cancelChat(id: string): Promise<{ cancelled: boolean }> {
    return this.post(`/v1/chats/${id}/cancel`);
  }

  /**
   * send posts a message and yields the turn's events as they arrive.
   * Aborting the signal stops reading; call cancelChat to stop the agent.
   */
  async *send(chatId: string, req: SendRequest, signal?: AbortSignal): AsyncGenerator<TurnEvent> {
    const resp = await this.raw(`/v1/chats/${chatId}/messages`, {
      method: "POST",
      body: JSON.stringify(req),
      signal,
      headers: { Accept: "text/event-stream" },
    });
    if (!resp.body) return;
    for await (const frame of parseJSONFrames<TurnEvent>(resp.body, signal)) {
      yield frame.data;
    }
  }

  // ---- notes --------------------------------------------------------------

  async notes(filter: { url?: string; host?: string; q?: string } = {}): Promise<Note[]> {
    const qs = new URLSearchParams();
    for (const [k, v] of Object.entries(filter)) if (v) qs.set(k, String(v));
    return (await this.json<{ notes: Note[] }>(`/v1/notes?${qs}`)).notes ?? [];
  }

  createNote(note: Partial<Note>): Promise<Note> {
    return this.post<Note>("/v1/notes", note);
  }

  updateNote(id: string, note: Partial<Note>): Promise<Note> {
    return this.patch<Note>(`/v1/notes/${id}`, note);
  }

  deleteNote(id: string): Promise<void> {
    return this.del(`/v1/notes/${id}`);
  }

  // ---- settings -----------------------------------------------------------

  settings(): Promise<{ settings: Record<string, string>; server: Record<string, unknown> }> {
    return this.json("/v1/settings");
  }

  saveSettings(patch: Record<string, string>): Promise<{ settings: Record<string, string> }> {
    return this.patch("/v1/settings", patch);
  }

  logs(tail = 200): Promise<{ lines: string[]; path: string }> {
    return this.json(`/v1/logs?tail=${tail}`);
  }
}

/** Context items are built here so every caller shapes them identically. */
export const context = {
  element(selector: string, text: string, html: string, rect: number[]): ContextItem {
    return { type: "element", selector, text, html, rect };
  },
  selection(quote: string, before?: string, after?: string): ContextItem {
    return { type: "text", quote, before, after };
  },
  file(path: string): ContextItem {
    return { type: "file", path };
  },
};
