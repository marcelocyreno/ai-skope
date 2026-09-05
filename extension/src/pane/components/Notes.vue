<script setup lang="ts">
/** Page-linked notes: search, cards with an optional quote, and a composer. */
import { onMounted } from "vue";
import { notes, loadNotes, addNote, deleteNote } from "@/stores/notes";
import { showToast } from "@/stores/toast";
import { api } from "@/stores/connection";
import type { Note } from "@/api/types";
import Icon from "./Icon.vue";

onMounted(() => void loadNotes());

let debounce: number | undefined;
function onSearch() {
  window.clearTimeout(debounce);
  debounce = window.setTimeout(() => void loadNotes(), 160);
}

async function remove(n: Note) {
  await deleteNote(n);
  showToast("Note deleted", {
    icon: "i-trash",
    undoLabel: "Undo",
    onUndo: async () => {
      await api().createNote({ url: n.url, title: n.title, quote: n.quote, body: n.body });
      await loadNotes();
    },
  });
}

async function copy(n: Note) {
  await navigator.clipboard.writeText([n.quote, n.body].filter(Boolean).join("\n\n"));
  showToast("Note copied", { icon: "i-copy" });
}

const when = (ms: number) =>
  new Date(ms).toLocaleTimeString([], { hour: "2-digit", minute: "2-digit" });
</script>

<template>
  <div class="sk-notes">
    <div class="sk-notes-search">
      <Icon id="i-search" />
      <input v-model="notes.query" type="search" placeholder="Search notes" aria-label="Search notes" @input="onSearch()" />
    </div>

    <div class="sk-notes-list">
      <article v-for="n in notes.notes" :key="n.id" class="sk-note">
        <div class="src">
          <span class="fav">{{ (n.host || "?").slice(0, 1).toUpperCase() }}</span>
          <span class="ttl">{{ n.title || n.host }}</span>
          <span class="url">{{ n.host }}</span>
          <span class="when">{{ when(n.createdAt) }}</span>
        </div>
        <blockquote v-if="n.quote" class="sk-quote">{{ n.quote }}</blockquote>
        <div v-if="n.body" class="txt"><p>{{ n.body }}</p></div>
        <div class="acts">
          <button class="sk-iconbtn sm" aria-label="Copy note" @click="copy(n)"><Icon id="i-copy" /></button>
          <button class="sk-iconbtn sm" aria-label="Delete note" @click="remove(n)"><Icon id="i-trash" /></button>
        </div>
      </article>
      <div v-if="notes.notes.length === 0" class="sk-hist-empty">
        <template v-if="notes.query">No notes match "{{ notes.query }}".</template>
        <template v-else>No notes yet. Select text on the page and choose “Save note”.</template>
      </div>
    </div>

    <form class="sk-note-new" @submit.prevent="addNote(notes.draft)">
      <div class="sk-field">
        <textarea
          v-model="notes.draft"
          rows="2"
          placeholder="New note about this page…"
          aria-label="New note"
          style="min-height: 54px"
        />
        <div class="sk-toolrow">
          <span class="sk-hint" style="padding: 0 4px"><Icon id="i-reticle" size="sm" />{{ notes.notes.length }} saved</span>
          <span class="grow" />
          <button type="submit" class="sk-btn primary sm" :disabled="!notes.draft.trim()">Save note</button>
        </div>
      </div>
    </form>
  </div>
</template>
