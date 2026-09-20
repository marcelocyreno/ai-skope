<script setup lang="ts">
/**
 * The switcher rises from the chip in the composer. Its shape follows the
 * server's own hierarchy: runtime → provider / model → effort.
 */
import { ref, computed } from "vue";
import {
  models,
  groupedOptions,
  selectModel,
  setEffort,
  effortLevels,
  currentOption,
  saveCurrentAsDefault,
  selectionIsDefault,
  shortModel,
} from "@/stores/models";
import { connection } from "@/stores/connection";
import type { ModelOption } from "@/api/types";
import Icon from "./Icon.vue";

const emit = defineEmits<{ (e: "close"): void; (e: "manage"): void }>();
const filter = ref("");

const groups = computed(() => {
  const q = filter.value.trim().toLowerCase();
  if (!q) return groupedOptions.value;
  return groupedOptions.value
    .map((g) => ({
      ...g,
      options: g.options.filter((o) =>
        [o.model, o.provider, o.runtimeName, o.label].some((v) => v?.toLowerCase().includes(q)),
      ),
    }))
    .filter((g) => g.options.length > 0);
});

const glyphFor = (o: ModelOption) => {
  if (o.runtime.startsWith("claude")) return "r-claude-code";
  if (o.runtime.startsWith("codex")) return "r-codex";
  if (o.runtime.startsWith("opencode")) return "r-opencode";
  if (o.runtime.startsWith("omp")) return "r-omp";
  if (o.runtime.startsWith("pi")) return "r-pi";
  return "i-server";
};

const isSelected = (o: ModelOption) =>
  o.runtime === models.selection?.runtime &&
  o.model === models.selection?.model &&
  (o.provider ?? "") === (models.selection?.provider ?? "");

function choose(o: ModelOption) {
  selectModel(o);
  emit("close");
}

const savingDefault = ref(false);
const savedDefault = ref(false);
const defaultError = ref("");

/**
 * Keeps the selection: the popup stays open afterwards so the `default` tag
 * can be seen moving onto the row, which is the only confirmation that the
 * next fresh pane will start here.
 */
async function saveDefault() {
  savingDefault.value = true;
  defaultError.value = "";
  try {
    await saveCurrentAsDefault();
    savedDefault.value = true;
  } catch (err) {
    defaultError.value = err instanceof Error ? err.message : String(err);
  } finally {
    savingDefault.value = false;
  }
}

/** Effort is part of what gets saved, so moving it makes the action live again. */
function chooseEffort(level: string) {
  setEffort(level);
  savedDefault.value = false;
}

const serverLine = computed(() => {
  const base = connection.settings?.baseUrl.replace(/^https?:\/\//, "") ?? "";
  if (connection.state !== "online") return `${base} · not reachable`;
  const n = connection.runtimes.filter((r) => r.available).length;
  return `${base} · connected · ${n} runtime${n === 1 ? "" : "s"} · v${connection.health?.version ?? "?"}`;
});
</script>

<template>
  <div class="sk-pop sk-switcher" role="listbox" aria-label="Models">
    <div class="srv">
      <span class="sk-dot" :class="connection.state === 'online' ? '' : 'is-offline'" />{{ serverLine }}
    </div>

    <div class="search">
      <Icon id="i-search" />
      <input v-model="filter" placeholder="Find a model or runtime" aria-label="Find a model" autofocus />
    </div>

    <div class="list">
      <template v-for="g in groups" :key="g.key">
        <div class="sk-group">
          <span><Icon :id="glyphFor(g.options[0])" />{{ g.label }}</span>
        </div>
        <button
          v-for="o in g.options"
          :key="`${o.runtime}/${o.provider ?? ''}/${o.model}`"
          type="button"
          class="sk-model"
          :class="{ 'is-off': o.status === 'offline' }"
          role="option"
          :aria-checked="isSelected(o)"
          :title="o.label || o.model"
          @click="choose(o)"
        >
          <span
            class="sk-dot"
            :class="o.status === 'degraded' ? 'is-degraded' : o.status === 'offline' ? 'is-offline' : ''"
          />
          <span class="nm">
            <span v-if="o.provider" class="prov">{{ o.provider }} /</span>
            <span class="mdl">{{ shortModel(o.model) }}</span>
            <span v-if="o.default" class="sk-tag">default</span>
          </span>
          <span class="meta">
            <template v-if="o.latencyMs">{{ o.latencyMs }} ms</template>
            <template v-if="o.ctx"> · {{ Math.round(o.ctx / 1000) }}K ctx</template>
          </span>
          <Icon id="i-check" class="chk" />
        </button>
      </template>
      <div v-if="groups.length === 0" class="sk-hist-empty">
        <template v-if="models.options.length === 0">
          No runtimes are installed. Run <code>aiss doctor</code> to see what's missing.
        </template>
        <template v-else>Nothing matches "{{ filter }}".</template>
      </div>
    </div>

    <div v-if="effortLevels.length" class="effort">
      <span>
        Effort
        <small>{{ currentOption?.runtimeName }} · {{ shortModel(models.selection?.model ?? "") }}</small>
      </span>
      <span class="sk-seg mini" role="group" aria-label="Effort">
        <button
          v-for="level in effortLevels"
          :key="level"
          type="button"
          :aria-pressed="models.selection?.effort === level"
          @click="chooseEffort(level)"
        >
          {{ level }}
        </button>
      </span>
    </div>

    <div class="foot">
      <span v-if="defaultError" class="fail" :title="defaultError">Could not save the default</span>
      <span v-else>{{ connection.runtimes.filter((r) => r.available).length }} available</span>
      <span class="acts">
        <button
          type="button"
          class="dft"
          :disabled="!models.selection || selectionIsDefault || savingDefault"
          :title="
            selectionIsDefault
              ? 'Already what a new chat starts on'
              : 'New chats and fresh panes start on this runtime, model and effort'
          "
          @click="saveDefault()"
        >
          {{ savingDefault ? "Saving…" : savedDefault ? "Saved" : "Save as default" }}
        </button>
        <a href="#" @click.prevent="emit('manage')">Manage sources →</a>
      </span>
    </div>
  </div>
</template>
