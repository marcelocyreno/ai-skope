<script setup lang="ts">
/** Chats grouped by this page, this site, everywhere — deletable, undoable. */
import { onMounted } from "vue";
import { history, loadHistory, groups, deleteChat, undoDelete } from "@/stores/history";
import { chat, openChat } from "@/stores/chat";
import { showToast } from "@/stores/toast";
import type { Chat } from "@/api/types";
import Icon from "./Icon.vue";

const emit = defineEmits<{ (e: "close"): void; (e: "new-chat"): void }>();

onMounted(() => void loadHistory());

let debounce: number | undefined;
function onSearch() {
  window.clearTimeout(debounce);
  debounce = window.setTimeout(() => void loadHistory(), 160);
}

async function open(c: Chat) {
  await openChat(c.id);
  emit("close");
}

async function remove(c: Chat, e: Event) {
  e.stopPropagation();
  await deleteChat(c);
  showToast("Chat deleted", {
    icon: "i-trash",
    undoLabel: "Undo",
    onUndo: () => void undoDelete(),
  });
}

const when = (ms: number) => {
  const d = new Date(ms);
  const today = new Date();
  return d.toDateString() === today.toDateString()
    ? d.toLocaleTimeString([], { hour: "2-digit", minute: "2-digit" })
    : d.toLocaleDateString([], { month: "short", day: "numeric" });
};
</script>

<template>
  <div class="sk-settings sk-history is-open" role="dialog" aria-label="Chat history">
    <div class="hd">
      <button type="button" class="sk-iconbtn" aria-label="Back to chat" @click="emit('close')">
        <Icon id="i-arrow-left" />
      </button>
      <h2>Chats</h2>
      <span class="sk-topbar-spacer" />
      <button type="button" class="sk-btn secondary sm" @click="emit('new-chat')">
        <Icon id="i-chat-new" />New chat
      </button>
    </div>

    <div class="sk-notes-search" style="margin: 10px 12px 2px">
      <Icon id="i-search" />
      <input v-model="history.query" type="search" placeholder="Search chats" aria-label="Search chats" @input="onSearch()" />
    </div>

    <div class="bd">
      <section v-for="g in groups" :key="g.key" class="sk-set-section sk-hist-group">
        <h3>{{ g.label }} <span v-if="g.detail" class="url">{{ g.detail }}</span></h3>
        <button
          v-for="c in g.chats"
          :key="c.id"
          type="button"
          class="sk-chat"
          :class="{ 'is-current': c.id === chat.chat?.id }"
          @click="open(c)"
        >
          <span class="fav">{{ (c.host || "?").slice(0, 1).toUpperCase() }}</span>
          <span class="body">
            <span class="ttl">{{ c.title || "Untitled chat" }}</span>
            <span class="meta">
              {{ c.model || c.runtime || "—" }} · {{ c.messageCount }} message{{ c.messageCount === 1 ? "" : "s" }} ·
              {{ when(c.updatedAt) }}<template v-if="g.showHost"> · {{ c.host }}</template>
            </span>
          </span>
          <span v-if="c.id === chat.chat?.id" class="sk-tag">open</span>
          <span v-else class="del sk-iconbtn sm" role="button" aria-label="Delete chat" @click="remove(c, $event)">
            <Icon id="i-trash" />
          </span>
        </button>
      </section>
      <div v-if="groups.length === 0" class="sk-hist-empty">
        <template v-if="history.query">No chats match "{{ history.query }}".</template>
        <template v-else>No chats yet.</template>
      </div>
    </div>
  </div>
</template>
