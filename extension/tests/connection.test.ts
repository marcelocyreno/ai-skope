/**
 * The version gate, driven through a faked `/v1/health`.
 *
 * The interesting part is not the comparison but what the pane does around it:
 * a server speaking another API must stop the connection before anything else
 * is asked of it, and a server speaking this one must go on costing exactly
 * the two requests it always did.
 */
import { describe, expect, it, beforeEach, afterEach, vi } from "vitest";

const EXTENSION_VERSION = "9.9.9";

const server = { version: "0.1.3", apiVersion: 1 };
let requested: string[] = [];

vi.stubGlobal("chrome", {
  runtime: { getManifest: () => ({ version: EXTENSION_VERSION }), id: "abcdef" },
  storage: {
    local: {
      get: async () => ({ settings: { baseUrl: "http://127.0.0.1:7331", token: "paired-token" } }),
      set: async () => {},
    },
    onChanged: { addListener: () => {}, removeListener: () => {} },
  },
});

const json = (body: unknown) =>
  new Response(JSON.stringify(body), {
    status: 200,
    headers: { "Content-Type": "application/json" },
  });

vi.stubGlobal("fetch", async (input: RequestInfo | URL) => {
  const url = String(input);
  requested.push(new URL(url).pathname);
  if (url.endsWith("/v1/health")) {
    return json({
      status: "ok",
      version: server.version,
      apiVersion: server.apiVersion,
      serverId: "s1",
      uptimeMs: 1000,
      paired: true,
    });
  }
  if (url.endsWith("/v1/runtimes")) return json({ runtimes: [] });
  // The push stream stays open, the way a live one does.
  if (url.endsWith("/v1/events")) return new Response(new ReadableStream({ start() {} }));
  throw new Error(`unexpected request to ${url}`);
});

const { connection, initConnection, stopConnection } = await import("@/stores/connection");

describe("the server API version gate", () => {
  beforeEach(() => {
    requested = [];
    server.version = "0.1.3";
    server.apiVersion = 1;
    connection.error = "";
    connection.state = "connecting";
  });

  afterEach(() => stopConnection());

  it("connects as before when the server speaks the same API", async () => {
    await initConnection();

    expect(connection.state).toBe("online");
    expect(connection.error).toBe("");
    expect(requested).toContain("/v1/runtimes");
  });

  it("refuses a server speaking a newer API, and says to update the extension", async () => {
    server.version = "0.9.0";
    server.apiVersion = 2;

    await initConnection();

    expect(connection.state).toBe("incompatible");
    expect(connection.error).toContain("0.9.0");
    expect(connection.error).toContain("v2");
    expect(connection.error).toContain(EXTENSION_VERSION);
    expect(connection.error).toContain("v1");
    expect(connection.error).toContain("update the extension");
    // Nothing past health is a shape this build can trust, so nothing past
    // health is asked for — the gate is what makes Send dead, not a retry.
    expect(requested).toEqual(["/v1/health"]);
  });

  it("refuses a server speaking an older API, and says to update the server", async () => {
    server.version = "0.0.9";
    server.apiVersion = 0;

    await initConnection();

    expect(connection.state).toBe("incompatible");
    expect(connection.error).toContain("update the server");
    expect(requested).toEqual(["/v1/health"]);
  });
});
