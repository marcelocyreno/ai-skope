import { describe, expect, it, beforeEach, vi } from "vitest";
import type { ModelOption, RuntimeInfo, Selection } from "@/api/types";

// The store reaches the server through connection.api(); the runtimes it reads
// are the ones the runtime.status event maintains.
const h = vi.hoisted(() => ({
  connection: { runtimes: [] as RuntimeInfo[], state: "online" },
  server: { models: [] as ModelOption[], default: undefined as Selection | undefined },
  calls: { models: 0, saved: [] as Selection[] },
}));

vi.mock("@/stores/connection", () => ({
  connection: h.connection,
  api: () => ({
    models: async () => {
      h.calls.models++;
      return { models: h.server.models, default: h.server.default };
    },
    setDefaultModel: async (sel: Selection) => {
      h.calls.saved.push(sel);
      h.server.default = sel;
      return sel;
    },
  }),
}));

const {
  models,
  loadModels,
  refreshForRuntimes,
  saveCurrentAsDefault,
  selectionIsDefault,
  shortModel,
  shortModelLabel,
} = await import("@/stores/models");

function runtime(id: string, enabled: boolean, status = "ok"): RuntimeInfo {
  return { id, name: id, enabled, status } as RuntimeInfo;
}

function option(runtimeId: string, model: string, effortLevels?: string[]): ModelOption {
  return {
    runtime: runtimeId,
    runtimeName: runtimeId,
    model,
    label: model,
    status: "ok",
    effortLevels,
  } as ModelOption;
}

describe("model store and runtime availability", () => {
  beforeEach(() => {
    h.connection.runtimes = [];
    h.server.models = [];
    h.server.default = undefined;
    h.calls.models = 0;
    h.calls.saved = [];
    models.options = [];
    models.selection = null;
    models.stored = null;
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

describe("saving the selection as the default", () => {
  beforeEach(async () => {
    h.connection.runtimes = [runtime("pi", true)];
    h.server.models = [option("pi", "glm-4.7", ["low", "high"])];
    h.server.default = { runtime: "pi", model: "glm-4.7" };
    h.calls.models = 0;
    h.calls.saved = [];
    models.options = [];
    models.selection = null;
    models.stored = null;
    await loadModels();
  });

  it("sends all four fields, so the effort does not stay behind", async () => {
    const withProvider = { ...option("pi", "glm-4.7", ["low", "high"]), provider: "z.ai" };
    h.server.models = [withProvider as ModelOption];
    await loadModels();
    models.selection = { runtime: "pi", provider: "z.ai", model: "glm-4.7", effort: "high" };

    await saveCurrentAsDefault();

    expect(h.calls.saved).toEqual([
      { runtime: "pi", provider: "z.ai", model: "glm-4.7", effort: "high" },
    ]);
  });

  it("stores no effort for a runtime that offers no levels", async () => {
    // The options page saves none either; the agent decides, and the server
    // rejects a level the runtime does not take.
    h.server.models = [option("pi", "glm-4.7")];
    await loadModels();
    models.selection = { runtime: "pi", model: "glm-4.7", effort: "high" };

    await saveCurrentAsDefault();

    expect(h.calls.saved[0].effort).toBeUndefined();
  });

  it("re-reads the models, so the default tag moves without a reload", async () => {
    models.selection = { runtime: "pi", model: "glm-4.7", effort: "low" };
    const before = h.calls.models;

    await saveCurrentAsDefault();

    expect(h.calls.models).toBe(before + 1);
    expect(models.stored).toEqual({ runtime: "pi", model: "glm-4.7", effort: "low" });
    expect(selectionIsDefault.value).toBe(true);
  });

  it("counts a selection whose effort alone moved as not yet the default", () => {
    models.selection = { runtime: "pi", model: "glm-4.7", effort: "high" };
    expect(selectionIsDefault.value).toBe(false);

    models.selection = { runtime: "pi", model: "glm-4.7" };
    expect(selectionIsDefault.value).toBe(true);
  });
});

describe("reading a model's name", () => {
  it("keeps an id that is already a name", () => {
    expect(shortModel("opus")).toBe("opus");
    expect(shortModel("gpt-5.5")).toBe("gpt-5.5");
    expect(shortModel("glm-4.7")).toBe("glm-4.7");
  });

  it("drops the namespace a provider puts in front of the name", () => {
    // Every Fireworks model shares the first three segments, so truncating
    // the id from the right made seven rows read "accounts/fi\u2026".
    expect(shortModel("accounts/fireworks/models/glm-5p3-flash")).toBe("glm-5p3-flash");
    expect(shortModel("accounts/fireworks/models/qwen3-coder-480b")).toBe("qwen3-coder-480b");
  });

  it("survives the shapes an id is not supposed to have", () => {
    expect(shortModel("zai/")).toBe("zai");
    expect(shortModel("")).toBe("");
    expect(shortModel("/")).toBe("/");
  });

  it("shortens each half of a provider / model label", () => {
    expect(shortModelLabel("fireworks / accounts/fireworks/models/glm-5p3-flash")).toBe(
      "fireworks / glm-5p3-flash",
    );
    expect(shortModelLabel("opus")).toBe("opus");
  });
});
