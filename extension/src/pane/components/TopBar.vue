<script setup lang="ts">
/**
 * The page this chat is about, then navigation; anything about the message
 * lives below. No brand row: Chrome draws its own side-panel header above this
 * one, from the `<title>` in sidepanel.html, so a second "AI Skope" would just
 * repeat it. What Chrome cannot say is which page the pane is answering about
 * — and the transcript swaps when the tab does — so that is what goes here.
 */
import { computed } from "vue";
import Icon from "./Icon.vue";
import { chat } from "@/stores/chat";
import { page } from "@/stores/page";

defineEmits<{ (e: "new-chat"): void; (e: "history"): void; (e: "settings"): void }>();

const host = computed(() => {
  try {
    const u = new URL(page.url);
    return u.protocol === "file:" ? "local file" : u.host;
  } catch {
    return "";
  }
});

const title = computed(() => page.title || host.value || "No page");
</script>

<template>
  <header class="sk-topbar">
    <div class="sk-page" :title="page.url || undefined">
      <span class="fav" aria-hidden="true">{{ (host || "?").slice(0, 1).toUpperCase() }}</span>
      <span class="txt">
        <span class="ttl">{{ title }}</span>
        <span v-if="host && page.title" class="host">{{ host }}</span>
      </span>
    </div>
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
