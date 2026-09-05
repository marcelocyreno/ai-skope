<script setup lang="ts">
/**
 * The strip above the composer, shown only when something stops the pane from
 * answering: the server is unreachable, or the chosen model is offline.
 */
import { computed } from "vue";
import { connection, connect } from "@/stores/connection";
import { modelStatus, models } from "@/stores/models";
import Icon from "./Icon.vue";

const emit = defineEmits<{ (e: "switch-model"): void }>();

const kind = computed<"offline" | "degraded" | null>(() => {
  if (connection.state === "offline") return "offline";
  if (modelStatus.value === "offline") return "offline";
  if (modelStatus.value === "degraded") return "degraded";
  return null;
});

const message = computed(() => {
  if (connection.state === "offline") return "AI Skope Server isn't reachable";
  if (modelStatus.value === "offline") return `${models.selection?.model ?? "That model"} is offline`;
  return `${models.selection?.model ?? "That model"} is slow right now`;
});

const isServer = computed(() => connection.state === "offline");
</script>

<template>
  <div v-if="kind" class="sk-strip" :class="kind === 'offline' ? 'is-offline' : 'is-degraded'" role="status">
    <Icon :id="kind === 'offline' ? 'i-offline' : 'i-alert'" />
    <span>
      {{ message }}
      <template v-if="isServer && connection.retryIn > 0">
        — retrying in <span class="cnt">{{ connection.retryIn }}s</span>
      </template>
    </span>
    <button v-if="isServer" type="button" class="act" @click="connect()">Retry now</button>
    <button v-else type="button" class="act" @click="emit('switch-model')">Switch model</button>
  </div>
</template>
