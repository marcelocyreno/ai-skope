<script setup lang="ts">
/**
 * The switcher rises from the chip in the composer. Its shape follows the
 * server's own hierarchy: runtime → provider / model → effort.
 */
import { ref, computed } from "vue";
import { models, groupedOptions, selectModel, setEffort, effortLevels, currentOption } from "@/stores/models";
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
          @click="choose(o)"
        >
          <span
            class="sk-dot"
            :class="o.status === 'degraded' ? 'is-degraded' : o.status === 'offline' ? 'is-offline' : ''"
          />
          <span class="nm">
            <span><span v-if="o.provider" class="prov">{{ o.provider }} /</span> {{ o.model }}</span>
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
        <small>{{ currentOption?.runtimeName }} · {{ models.selection?.model }}</small>
      </span>
      <span class="sk-seg mini" role="group" aria-label="Effort">
        <button
          v-for="level in effortLevels"
          :key="level"
          type="button"
          :aria-pressed="models.selection?.effort === level"
          @click="setEffort(level)"
        >
          {{ level }}
        </button>
      </span>
    </div>

    <div class="foot">
      <span>{{ connection.runtimes.filter((r) => r.available).length }} available</span>
      <a href="#" @click.prevent="emit('manage')">Manage sources →</a>
    </div>
  </div>
</template>
