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

export async function loadModels(): Promise<void> {
  models.loading = true;
  models.error = "";
  try {
    const out = await api().models();
    models.options = out.models ?? [];
    if (!models.selection && out.default?.runtime) models.selection = out.default;
  } catch (err) {
    models.error = err instanceof Error ? err.message : String(err);
  } finally {
    models.loading = false;
  }
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
