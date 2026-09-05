<script setup lang="ts">
/** Identity and navigation only; anything about the message lives below. */
import Icon from "./Icon.vue";
import { chat } from "@/stores/chat";

defineEmits<{ (e: "new-chat"): void; (e: "history"): void; (e: "settings"): void }>();
</script>

<template>
  <header class="sk-topbar">
    <div class="sk-brand">
      <Icon id="i-reticle" />
      <span class="sk-brand-name">AI Skope</span>
    </div>
    <div class="sk-topbar-spacer" />
    <button
      type="button"
      class="sk-iconbtn"
      :disabled="chat.messages.length === 0"
      title="New chat — keeps this one in History"
      aria-label="New chat"
      @click="$emit('new-chat')"
    >
      <Icon id="i-chat-new" />
    </button>
    <button type="button" class="sk-iconbtn" title="History" aria-label="Chat history" @click="$emit('history')">
      <Icon id="i-history" />
    </button>
    <button type="button" class="sk-iconbtn" title="Settings" aria-label="Settings" @click="$emit('settings')">
      <Icon id="i-gear" />
    </button>
  </header>
</template>
