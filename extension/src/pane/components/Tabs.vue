<script setup lang="ts">
/**
 * Parked, not deleted. Notes is half built — see issue #11 — so the pane
 * renders no tab bar and no way into it. Nothing imports this component;
 * re-enabling the feature means putting the entry points back in App.vue.
 */
import Icon from "./Icon.vue";
import { notes } from "@/stores/notes";

defineProps<{ modelValue: "chat" | "notes" }>();
defineEmits<{ (e: "update:modelValue", v: "chat" | "notes"): void }>();
</script>

<template>
  <nav class="sk-tabs" role="tablist" aria-label="AI Skope sections">
    <button
      class="sk-tab"
      role="tab"
      :aria-selected="modelValue === 'chat'"
      @click="$emit('update:modelValue', 'chat')"
    >
      <Icon id="i-chat" />Chat
    </button>
    <button
      class="sk-tab"
      role="tab"
      :aria-selected="modelValue === 'notes'"
      @click="$emit('update:modelValue', 'notes')"
    >
      <Icon id="i-note" />Notes
      <span v-if="notes.notes.length" class="sk-badge">{{ notes.notes.length }}</span>
    </button>
  </nav>
</template>
