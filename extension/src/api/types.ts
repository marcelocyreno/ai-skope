// Types mirroring the AI Skope Server's JSON, endpoint for endpoint.
// The Go source of truth is server/internal/api and server/internal/store.

export interface Health {
  status: string;
  version: string;
  apiVersion: number;
  serverId: string;
  uptimeMs: number;
  paired: boolean;
}

export interface Capabilities {
  apiVersion: number;
  version: string;
  features: string[];
  providerKinds: { id: string; name: string; needsKey: boolean; baseUrl: string }[];
  maxFileBytes: number;
  maxContextBytes: number;
  fullTextSearch: boolean;
  keystore: string;
}

export type RuntimeStatus = "ok" | "degraded" | "offline";

export interface RuntimeInfo {
  id: string;
  name: string;
  version?: string;
  path?: string;
  available: boolean;
  enabled: boolean;
  variants?: string[];
  effortLevels?: string[];
  usesProviders: boolean;
  status: RuntimeStatus;
  latencyMs?: number;
  detail?: string;
}

/** One selectable row in the model switcher. */
export interface ModelOption {
  runtime: string;
  runtimeName: string;
  group?: string;
  provider?: string;
  model: string;
  label: string;
  ctx?: number;
  status: RuntimeStatus;
  latencyMs?: number;
  effortLevels?: string[];
  default?: boolean;
}

/** What the user picked: a runtime, a model, and optionally an effort level. */
export interface Selection {
  runtime: string;
  provider?: string;
  model: string;
  effort?: string;
}

export interface Provider {
  id: string;
  kind: string;
  name: string;
  baseUrl?: string;
  key: string; // masked; the plaintext never leaves the server
  availableTo: string[];
  createdAt: number;
  updatedAt: number;
  lastTestAt?: number;
  lastTestOk: boolean;
  lastTestMessage?: string;
  models?: { model: string; ctx?: number }[];
}

export interface Folder {
  id: string;
  path: string;
  display: string;
  access: "read" | "read+watch";
  fileCount: number;
  lastIndexedAt: number;
  createdAt: number;
}

export interface FileEntry {
  path: string;
  display: string;
  dir: string;
  name: string;
  ext?: string;
  size: number;
  mtime: number;
  isDir: boolean;
  snippet?: string;
}

export interface FileContent {
  path: string;
  display: string;
  name: string;
  ext: string;
  size: number;
  mtime: number;
  text: string;
  truncated?: boolean;
  title?: string;
}

export type ContextType = "element" | "text" | "file" | "page";

/** A piece of context attached to a message: what the user aimed at. */
export interface ContextItem {
  id?: string;
  type: ContextType;
  // element
  selector?: string;
  html?: string;
  rect?: number[];
  // text selection
  quote?: string;
  before?: string;
  after?: string;
  // file
  path?: string;
  // shared
  text?: string;
  url?: string;
  title?: string;
}

export interface ToolRecord {
  name: string;
  target?: string;
  detail?: string;
  state: "running" | "done" | "failed";
}

export interface Usage {
  inputTokens: number;
  outputTokens: number;
  ms: number;
}

export interface Message {
  id: string;
  chatId: string;
  role: "user" | "assistant";
  text: string;
  tools?: ToolRecord[];
  usage?: Usage;
  error?: string;
  model?: string;
  createdAt: number;
  context?: ContextItem[];
}

export interface Chat {
  id: string;
  title: string;
  url: string;
  host: string;
  pageTitle?: string;
  favicon?: string;
  runtime?: string;
  variant?: string;
  provider?: string;
  model?: string;
  effort?: string;
  createdAt: number;
  updatedAt: number;
  deletedAt?: number;
  messageCount: number;
}

export interface Note {
  id: string;
  url: string;
  host: string;
  title: string;
  favicon?: string;
  quote?: string;
  body: string;
  createdAt: number;
  updatedAt: number;
}

export interface PageRef {
  url: string;
  title: string;
  favicon?: string;
  /** Only sent when the user's page-access setting allows it. */
  text?: string;
}

export interface SendRequest {
  text: string;
  page?: PageRef;
  context?: ContextItem[];
  model?: Selection;
}

/** The events a turn streams back, one per SSE frame. */
export type TurnEventName =
  | "turn.start"
  | "tool"
  | "text.delta"
  | "text.done"
  | "usage"
  | "error"
  | "turn.end";

export interface TurnEvent {
  event: TurnEventName;
  messageId?: string;
  text?: string;
  tool?: ToolRecord;
  usage?: Usage;
  code?: string;
  message?: string;
  retryable?: boolean;
  model?: string;
}

/** Events pushed on /v1/events while the panel is open. */
export interface ServerEvent {
  type: "runtime.status" | "index.progress" | "folder.changed" | "server.notice";
  at: number;
  data: unknown;
}
