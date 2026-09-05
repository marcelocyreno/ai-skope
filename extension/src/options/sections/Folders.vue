<script setup lang="ts">
/** The read allow-list: nothing outside these folders is ever opened. */
import { ref, onMounted } from "vue";
import { connection, api } from "@/stores/connection";
import type { Folder } from "@/api/types";
import Icon from "@/pane/components/Icon.vue";

const folders = ref<Folder[]>([]);
const path = ref("");
const watchIt = ref(true);
const error = ref("");

onMounted(refresh);

async function refresh() {
  if (connection.state !== "online") return;
  try {
    folders.value = await api().folders();
  } catch (err) {
    error.value = err instanceof Error ? err.message : String(err);
  }
}

async function add() {
  if (!path.value.trim()) return;
  error.value = "";
  try {
    await api().addFolder(path.value.trim(), watchIt.value ? "read+watch" : "read");
    path.value = "";
    await refresh();
  } catch (err) {
    error.value = err instanceof Error ? err.message : String(err);
  }
}

async function remove(f: Folder) {
  await api().removeFolder(f.id);
  await refresh();
}

async function reindex(f: Folder) {
  await api().reindexFolder(f.id);
  window.setTimeout(refresh, 1500);
}

const ago = (ms: number) => (ms ? new Date(ms).toLocaleString() : "never");
</script>

<template>
  <section class="sk-opt-section" data-section="folders">
    <h2>Folders</h2>
    <p class="lead">
      Folders the server may read, so you can ask about local HTML and Markdown
      files, repos and notes. Nothing outside them is ever opened, and keys,
      credentials and shell history are refused even inside them.
    </p>

    <div class="sk-tablewrap">
      <table class="sk-table">
        <thead><tr><th>Folder</th><th>Files</th><th>Access</th><th>Last indexed</th><th /></tr></thead>
        <tbody>
          <tr v-for="f in folders" :key="f.id">
            <td class="mono"><b>{{ f.display }}</b></td>
            <td>{{ f.fileCount }}</td>
            <td>{{ f.access }}</td>
            <td class="mono">{{ ago(f.lastIndexedAt) }}</td>
            <td>
              <button class="sk-btn ghost sm" @click="reindex(f)">Reindex</button>
              <button class="sk-iconbtn sm" aria-label="Remove folder" @click="remove(f)"><Icon id="i-trash" /></button>
            </td>
          </tr>
          <tr v-if="folders.length === 0"><td colspan="5" class="mono">No folders allowed yet.</td></tr>
        </tbody>
      </table>
    </div>

    <div class="sk-row" style="border: 0">
      <input v-model="path" class="sk-input mono" placeholder="~/dev" style="flex: 1" aria-label="Folder path" @keydown.enter="add()" />
      <label class="lbl" style="flex: none; display: flex; align-items: center; gap: 6px">
        <button type="button" class="sk-switch" role="switch" :aria-checked="watchIt" aria-label="Watch for changes" @click="watchIt = !watchIt" />
        <small>watch for changes</small>
      </label>
      <button class="sk-btn primary sm" @click="add()"><Icon id="i-folder" />Add folder</button>
    </div>
    <p v-if="error" class="mono" style="color: var(--bad)">{{ error }}</p>
  </section>
</template>
