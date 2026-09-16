/** The model switcher's data: what can answer, and what is selected. */
import { reactive, computed } from "vue";
import { api, connection } from "./connection";
import type { ModelOption, Selection } from "@/api/types";

/**
 * The name a model is known by, for a list that has to be read at a glance.
 *
 * Providers that namespace their catalogue return ids like
 * `accounts/fireworks/models/glm-5p3-flash`, where every model shares the
 * first three segments. A row that truncates such an id keeps the shared
 * prefix and drops the one part that tells it apart, so seven different
 * models all render as `accounts/fi…`. The last segment is the name; the path
 * in front of it is addressing, and belongs in the tooltip.
 *
 * An id that is not a path — `opus`, `gpt-5.5` — is already the name.
 */
export function shortModel(id: string): string {
  return id.split("/").filter(Boolean).pop() ?? id;
}

/**
 * The same, for a label the server already assembled as `provider / model`:
 * each half is shortened, so the provider survives rather than being mistaken
 * for one more path segment.
 */
export function shortModelLabel(label: string): string {
  return label.split(" / ").map(shortModel).join(" / ");
}

interface ModelStore {
  options: ModelOption[];
  selection: Selection | null;
  /** What the server has stored as the default — what a fresh pane starts on. */
  stored: Selection | null;
  loading: boolean;
  error: string;
}

export const models = reactive<ModelStore>({
  options: [],
  selection: null,
  stored: null,
  loading: false,
  error: "",
});

/**
 * runtimeSignature captures the runtime facts the switcher's contents depend
 * on, so a status event that changed nothing does not churn the list.
 */
function runtimeSignature(): string {
  return connection.runtimes.map((r) => `${r.id}:${r.enabled}:${r.status}`).join("|");
}

let loadedSignature = "";

/**
 * runtimeDisabled reports that the server has this runtime switched off.
 *
 * Being absent from the options is deliberately *not* the test: a custom
 * runtime with no providers configured contributes no options yet can still
 * answer, which is the same reason modelStatus trusts the connection over the
 * list. Only an explicit "off" invalidates a selection.
 */
function runtimeDisabled(sel: Selection | null | undefined): boolean {
  if (!sel?.runtime) return false;
  const rt = connection.runtimes.find((r) => r.id === sel.runtime);
  return rt ? !rt.enabled : false;
}

function asSelection(opt: ModelOption | undefined): Selection | null {
  if (!opt) return null;
  return { runtime: opt.runtime, provider: opt.provider, model: opt.model };
}

export async function loadModels(): Promise<void> {
  models.loading = true;
  models.error = "";
  loadedSignature = runtimeSignature();
  try {
    const out = await api().models();
    models.options = out.models ?? [];
    models.stored = out.default ?? null;
    // A runtime switched off in settings must not stay selected, or the
    // composer chip names something that cannot answer. The stored default can
    // be the disabled one too, so fall through to whatever is still listed.
    if (!models.selection || runtimeDisabled(models.selection)) {
      const preferred = out.default?.runtime && !runtimeDisabled(out.default) ? out.default : null;
      models.selection = preferred ?? asSelection(models.options[0]);
    }
  } catch (err) {
    models.error = err instanceof Error ? err.message : String(err);
  } finally {
    models.loading = false;
  }
}

/**
 * refreshForRuntimes reloads the options after a runtime is enabled, disabled
 * or changes health. The server already excludes disabled runtimes from
 * /v1/models, but nothing refetched it, so the switcher kept offering models
 * from a runtime the user had just switched off.
 *
 * runtime.status also arrives on every scheduled probe, unchanged, hence the
 * signature check.
 */
export async function refreshForRuntimes(): Promise<void> {
  if (runtimeSignature() === loadedSignature) return;
  await loadModels();
}

export function selectModel(opt: ModelOption): void {
  models.selection = {
    runtime: opt.runtime,
    provider: opt.provider,
    model: opt.model,
    effort: models.selection?.effort,
  };
}

export function setEffort(effort: string): void {
  if (models.selection) models.selection.effort = effort;
}

export async function setDefaultModel(sel: Selection): Promise<void> {
  await api().setDefaultModel(sel);
  await loadModels();
}

/** The option matching the current selection, for the chip and the switcher. */
export const currentOption = computed<ModelOption | undefined>(() =>
  models.options.find(
    (o) =>
      o.runtime === models.selection?.runtime &&
      o.model === models.selection?.model &&
      (o.provider ?? "") === (models.selection?.provider ?? ""),
  ),
);

/** Effort levels offered by the selected runtime, empty when unsupported. */
export const effortLevels = computed<string[]>(() => currentOption.value?.effortLevels ?? []);

/**
 * The dot next to the model name: can this model answer right now.
 *
 * When the selection is not among the listed options — a custom runtime with
 * no providers configured, say — the server is still the authority on whether
 * it can answer, so the connection's own state is used rather than assuming
 * the worst and blocking the composer.
 */
export const modelStatus = computed<"ok" | "degraded" | "offline">(() => {
  if (connection.state !== "online") return "offline";
  const option = currentOption.value;
  if (!option) return models.selection?.runtime ? "ok" : "offline";
  return option.status;
});

/** Options grouped for the switcher: runtimes, with the agent family together. */
export const groupedOptions = computed(() => {
  const groups = new Map<string, { key: string; label: string; options: ModelOption[] }>();
  for (const o of models.options) {
    const key = o.group || o.runtime;
    const label = o.group ? "pi · omp · opencode" : o.runtimeName;
    if (!groups.has(key)) groups.set(key, { key, label, options: [] });
    groups.get(key)!.options.push(o);
  }
  return [...groups.values()];
});
