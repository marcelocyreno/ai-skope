/** The model switcher's data: what can answer, and what is selected. */
import { reactive, computed } from "vue";
import { api, connection } from "./connection";
import type { ModelOption, Selection } from "@/api/types";

interface ModelStore {
  options: ModelOption[];
  selection: Selection | null;
  loading: boolean;
  error: string;
}

export const models = reactive<ModelStore>({
  options: [],
  selection: null,
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
