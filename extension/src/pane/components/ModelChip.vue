<script setup lang="ts">
/**
 * The model chip lives in the composer, because choosing a model is a
 * per-message decision. Latency stays in the switcher and the tooltip so the
 * model's name is never squeezed out.
 */
import { computed } from "vue";
import { models, modelStatus, currentOption } from "@/stores/models";
import { connection } from "@/stores/connection";
import Icon from "./Icon.vue";

defineProps<{ expanded?: boolean }>();
defineEmits<{ (e: "open"): void }>();

const glyph = computed(() => {
  const runtime = models.selection?.runtime ?? "";
  if (runtime.startsWith("claude")) return "r-claude-code";
  if (runtime.startsWith("codex")) return "r-codex";
  if (runtime.startsWith("opencode")) return "r-opencode";
  if (runtime.startsWith("omp")) return "r-omp";
  if (runtime.startsWith("pi")) return "r-pi";
  return "i-server";
});

const name = computed(() => models.selection?.model ?? "No model");

const title = computed(() => {
  const o = currentOption.value;
  if (!o) return "Choose a model";
  const parts = [o.runtimeName];
  if (o.provider) parts.push(`${o.provider} / ${o.model}`);
  else parts.push(o.model);
  if (models.selection?.effort) parts.push(models.selection.effort);
  parts.push(connection.state === "online" ? "via AI Skope Server" : "server unreachable");
  if (o.latencyMs) parts.push(`${o.latencyMs} ms`);
  return parts.join(" · ");
});

const dotClass = computed(() =>
  modelStatus.value === "degraded" ? "is-degraded" : modelStatus.value === "offline" ? "is-offline" : "",
);
</script>

<template>
  <button
    type="button"
    class="sk-chip-model"
    :aria-expanded="expanded"
    aria-haspopup="listbox"
    :title="title"
    @click="$emit('open')"
  >
    <Icon :id="glyph" />
    <span class="name">{{ name }}</span>
    <span v-if="models.selection?.effort" class="eff sk-tag">{{ models.selection.effort }}</span>
    <span class="sk-dot" :class="dotClass" />
    <Icon id="i-chevron-down" class="chev" />
  </button>
</template>
