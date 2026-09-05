/**
 * The end-to-end harness: a real `aiss` on a scratch profile, a real Chrome
 * with the built extension loaded, and a fake agent standing in for a coding
 * agent so the test is deterministic and offline.
 */
import { test as base, chromium, type BrowserContext, type Page } from "@playwright/test";
import { execFileSync, spawn, type ChildProcess } from "node:child_process";
import { createServer, type Server } from "node:http";
import { createHash } from "node:crypto";
import { cpSync, mkdtempSync, readFileSync, realpathSync, rmSync, writeFileSync, mkdirSync } from "node:fs";
import { tmpdir } from "node:os";
import { join, resolve } from "node:path";
import { fileURLToPath } from "node:url";

const here = fileURLToPath(new URL(".", import.meta.url));
const extensionRoot = resolve(here, "../..");
const repoRoot = resolve(extensionRoot, "..");
const serverDir = join(repoRoot, "server");

export interface Harness {
  context: BrowserContext;
  extensionId: string;
  panel: Page;
  serverUrl: string;
  projectDir: string;
  aiss: (...args: string[]) => string;
  /** URL of a fixture page, served over http — extensions cannot script
   *  file:// URLs without a separate Chrome setting. */
  fixture: (name: string) => string;
  /** Stops the server and waits for it to actually be gone. */
  stopServer: () => Promise<void>;
  /** Registers a provider whose models the fake runtime can offer. */
  addStubProvider: () => Promise<void>;
  token: string;
}

/**
 * Chrome's id for an unpacked extension is a function of its path: sha256 of
 * the absolute path, first 32 hex digits mapped 0->a … f->p. The path must be
 * the *real* one — on macOS a temp dir under /var is a symlink to
 * /private/var, and hashing the symlink yields an id that does not exist.
 */
export function unpackedExtensionId(dir: string): string {
  const real = realpathSync(resolve(dir));
  const hash = createHash("sha256").update(real, "utf8").digest("hex").slice(0, 32);
  return [...hash].map((c) => String.fromCharCode(97 + parseInt(c, 16))).join("");
}

async function waitFor(check: () => Promise<boolean> | boolean, ms = 20000): Promise<void> {
  const deadline = Date.now() + ms;
  while (Date.now() < deadline) {
    if (await check()) return;
    await new Promise((r) => setTimeout(r, 200));
  }
  throw new Error("timed out waiting for a condition");
}

export const test = base.extend<{ harness: Harness }>({
  harness: async ({}, use) => {
    const work = mkdtempSync(join(tmpdir(), "skope-e2e-"));
    const port = 7400 + Math.floor(Math.random() * 200);
    const serverUrl = `http://127.0.0.1:${port}`;

    // A scratch HOME so the developer's real config, database and keychain
    // are never touched by a test run.
    const env = {
      ...process.env,
      XDG_CONFIG_HOME: join(work, "config"),
      XDG_DATA_HOME: join(work, "data"),
      XDG_STATE_HOME: join(work, "state"),
      AISS_PORT: String(port),
      AISS_KEYSTORE: "file",
    };

    const binary = join(work, "aiss");
    execFileSync("go", ["build", "-o", binary, "./cmd/aiss"], { cwd: serverDir, stdio: "inherit" });
    const aiss = (...args: string[]) =>
      execFileSync(binary, args, { env, encoding: "utf8" }).toString();

    const server: ChildProcess = spawn(binary, ["start", "--foreground"], { env, stdio: "ignore" });
    await waitFor(async () => {
      try {
        return (await fetch(`${serverUrl}/v1/health`)).ok;
      } catch {
        return false;
      }
    });

    // A project the server is allowed to read, and a fake agent to answer.
    const projectDir = join(work, "dev", "finme");
    mkdirSync(projectDir, { recursive: true });
    writeFileSync(
      join(projectDir, "README.md"),
      "# finme\n\n## Export format\nEach statement month is written as CSV and JSON.\n",
    );
    aiss("folders", "add", projectDir);
    aiss("runtimes", "command", "custom:e2e", join(serverDir, "testdata/fakes/claude-like.sh"));
    // Pin the default so the pane answers through the fake agent, not through
    // whatever real runtime happens to be installed on the machine.
    aiss("models", "--set", "custom:e2e", "fake-1");

    // Chrome cannot be asked to accept an optional-permission prompt from a
    // test, so the loaded copy declares host access up front. The prompt flow
    // itself is on the manual checklist.
    const loadDir = join(work, "extension");
    cpSync(join(extensionRoot, "dist"), loadDir, { recursive: true });
    const manifest = JSON.parse(readFileSync(join(loadDir, "manifest.json"), "utf8"));
    manifest.host_permissions = ["<all_urls>"];
    delete manifest.optional_host_permissions;
    writeFileSync(join(loadDir, "manifest.json"), JSON.stringify(manifest, null, 2));

    // Playwright's bundled Chromium, not the installed Chrome: Chrome 137+
    // ignores --load-extension entirely, so a released Chrome cannot load an
    // unpacked extension from the command line at all.
    // A stub model API, so a provider can be registered and the switcher has
    // something real to list.
    const models: Server = createServer((_req, res) => {
      res.writeHead(200, { "Content-Type": "application/json" });
      res.end(JSON.stringify({ data: [{ id: "glm-5.3-flash", context_length: 128000 }, { id: "glm-5.3" }] }));
    });
    await new Promise<void>((r) => models.listen(0, "127.0.0.1", r));
    const modelsPort = (models.address() as { port: number }).port;

    // Fixture pages are served over http so the content script can be
    // injected the same way it would be on a real site.
    const fixtures: Server = createServer((req, res) => {
      const name = (req.url ?? "/").replace(/^\/+|\?.*$/g, "") || "pricing.html";
      try {
        const body = readFileSync(join(here, "fixtures", name));
        res.writeHead(200, { "Content-Type": "text/html; charset=utf-8" });
        res.end(body);
      } catch {
        res.writeHead(404).end("not found");
      }
    });
    await new Promise<void>((r) => fixtures.listen(0, "127.0.0.1", r));
    const fixturePort = (fixtures.address() as { port: number }).port;
    const fixture = (name: string) => `http://127.0.0.1:${fixturePort}/${name}`;

    const context = await chromium.launchPersistentContext(join(work, "profile"), {
      headless: false, // extensions do not load in headless mode
      args: [`--disable-extensions-except=${loadDir}`, `--load-extension=${loadDir}`],
    });

    // An MV3 service worker does not start until something wakes it, so the
    // id is derived the way Chrome derives it for an unpacked extension:
    // sha256 of the absolute path, first 32 hex digits mapped 0->a … f->p.
    const extensionId = unpackedExtensionId(loadDir);

    // From here on any failure must still stop the server, or a broken setup
    // leaves an aiss process running for the rest of the session.
    const panel = await context.newPage();
    // Surface what the pane says: a silent failure in the panel document is
    // otherwise invisible to the test.
    panel.on("console", (m) => {
      if (m.type() === "error" || m.type() === "warning") console.log("[panel]", m.text());
    });
    panel.on("pageerror", (e) => console.log("[panel exception]", e.message));
    try {
      await panel.goto(`chrome-extension://${extensionId}/sidepanel.html`);
    } catch (err) {
      await context.close().catch(() => {});
      fixtures.close();
      server.kill("SIGKILL");
      rmSync(work, { recursive: true, force: true });
      throw err;
    }

    // Point the panel at this run's server before anything else happens.
    await panel.evaluate(
      async (url) => chrome.storage.local.set({ settings: { baseUrl: url } }),
      serverUrl,
    );
    await panel.reload();

    // `aiss stop` cannot be used here: the server is a child of this Node
    // process, and once it exits it stays a zombie until Node reaps it — so a
    // liveness check by signal would keep reporting it as running.
    let stopped = false;
    const stopServer = async () => {
      if (stopped) return;
      stopped = true;
      const exited = new Promise<void>((resolve) => server.once("exit", () => resolve()));
      server.kill("SIGTERM");
      await Promise.race([exited, new Promise((r) => setTimeout(r, 8000))]);
      await waitFor(async () => {
        try {
          await fetch(`${serverUrl}/v1/health`);
          return false;
        } catch {
          return true;
        }
      }, 10000);
    };

    // The pane pairs itself in the tests; this token is for direct API calls.
    const code = execFileSync(binary, ["pair"], { env, encoding: "utf8" }).split("\n")[0].split(":")[1].trim();
    const paired = await fetch(`${serverUrl}/v1/pair`, {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({ code, origin: `chrome-extension://${extensionId}`, label: "harness" }),
    }).then((r) => r.json() as Promise<{ token: string }>);
    const token = paired.token;

    const addStubProvider = async () => {
      const resp = await fetch(`${serverUrl}/v1/providers`, {
        method: "POST",
        headers: { "Content-Type": "application/json", Authorization: `Bearer ${token}` },
        body: JSON.stringify({
          kind: "openai-compatible",
          name: "stub.ai",
          baseUrl: `http://127.0.0.1:${modelsPort}`,
          key: "stub-key",
          availableTo: ["custom:e2e"],
        }),
      });
      if (!resp.ok) {
        throw new Error(`stub provider was refused: ${resp.status} ${await resp.text()}`);
      }
      const created = (await resp.json()) as { models?: unknown[] };
      if (!created.models?.length) {
        throw new Error("the stub provider registered but reported no models");
      }
    };

    await use({ context, extensionId, panel, serverUrl, projectDir, aiss, fixture, stopServer, addStubProvider, token });

    await context.close();
    fixtures.close();
    await stopServer().catch(() => {});
    models.close();
    rmSync(work, { recursive: true, force: true });
  },
});

export { expect } from "@playwright/test";

/** Reads a fresh pairing code from the CLI and enters it in the panel. */
export async function pair(h: Harness): Promise<void> {
  const code = h.aiss("pair").split("\n")[0].split(":")[1].trim();
  await h.panel.getByLabel("Pairing code").fill(code);
  await h.panel.getByRole("button", { name: "Pair", exact: true }).click();
  // Wait for the composer, not the top bar: the top bar is on screen even
  // before pairing, so waiting for it returns while the pairing request is
  // still in flight — and a reload then throws the pairing away.
  await h.panel.getByLabel("Message").waitFor({ state: "visible", timeout: 20000 });
}
