import { describe, expect, it, beforeEach, vi } from "vitest";
import type { ModelOption, RuntimeInfo, Selection } from "@/api/types";

// The store reaches the server through connection.api(); the runtimes it reads
// are the ones the runtime.status event maintains.
const h = vi.hoisted(() => ({
  connection: { runtimes: [] as RuntimeInfo[], state: "online" },
  server: { models: [] as ModelOption[], default: undefined as Selection | undefined },
  calls: { models: 0 },
}));

vi.mock("@/stores/connection", () => ({
  connection: h.connection,
  api: () => ({
    models: async () => {
      h.calls.models++;
      return { models: h.server.models, default: h.server.default };
    },
  }),
}));

const { models, loadModels, refreshForRuntimes } = await import("@/stores/models");

function runtime(id: string, enabled: boolean, status = "ok"): RuntimeInfo {
  return { id, name: id, enabled, status } as RuntimeInfo;
}

function option(runtimeId: string, model: string): ModelOption {
  return { runtime: runtimeId, runtimeName: runtimeId, model, label: model, status: "ok" } as ModelOption;
}

describe("model store and runtime availability", () => {
  beforeEach(() => {
    h.connection.runtimes = [];
    h.server.models = [];
    h.server.default = undefined;
    h.calls.models = 0;
    models.options = [];
    models.selection = null;
  });

  it("drops a selection whose runtime was switched off", async () => {
    h.connection.runtimes = [runtime("omp", false), runtime("claude-code", true)];
    h.server.models = [option("claude-code", "opus")];
    h.server.default = { runtime: "claude-code", model: "opus" };
    models.selection = { runtime: "omp", model: "glm-4.5" };

    await loadModels();

    expect(models.selection).toEqual({ runtime: "claude-code", model: "opus" });
  });

  it("keeps a selection that is merely absent from the options", async () => {
    // A custom runtime with no providers lists nothing yet can still answer,
    // which is the case modelStatus deliberately tolerates.
    h.connection.runtimes = [runtime("custom:mine", true)];
    h.server.models = [option("claude-code", "opus")];
    h.server.default = { runtime: "claude-code", model: "opus" };
    models.selection = { runtime: "custom:mine", model: "whatever" };

    await loadModels();

    expect(models.selection).toEqual({ runtime: "custom:mine", model: "whatever" });
  });

  it("falls through to a listed option when the server default is disabled too", async () => {
    h.connection.runtimes = [runtime("omp", false), runtime("pi", true)];
    h.server.models = [option("pi", "glm-4.7")];
    h.server.default = { runtime: "omp", model: "glm-4.5" }; // the stored default died with it
    models.selection = { runtime: "omp", model: "glm-4.5" };

    await loadModels();

    expect(models.selection).toMatchObject({ runtime: "pi", model: "glm-4.7" });
  });

  it("leaves no selection when nothing can answer", async () => {
    h.connection.runtimes = [runtime("omp", false)];
    models.selection = { runtime: "omp", model: "glm-4.5" };

    await loadModels();

    expect(models.selection).toBeNull();
  });

  it("refetches when a runtime is toggled, and not when the probe changed nothing", async () => {
    h.connection.runtimes = [runtime("omp", true)];
    h.server.models = [option("omp", "glm-4.5")];
    await loadModels();
    expect(h.calls.models).toBe(1);

    // runtime.status arrives on every scheduled probe; identical state must
    // not churn the switcher.
    await refreshForRuntimes();
    expect(h.calls.models).toBe(1);

    h.connection.runtimes = [runtime("omp", false)];
    await refreshForRuntimes();
    expect(h.calls.models).toBe(2);

    // Health changing is enough on its own: an offline runtime's models
    // should stop being offered as healthy.
    h.connection.runtimes = [runtime("omp", false, "offline")];
    await refreshForRuntimes();
    expect(h.calls.models).toBe(3);
  });
});
