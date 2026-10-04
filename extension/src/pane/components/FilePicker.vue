<script setup lang="ts">
/**
 * The local-file picker. Everything shown here comes from the server's index,
 * so the pane can only ever offer files the user has allowed.
 */
import { ref, computed, onMounted } from "vue";
import { files, openPicker, search, browse } from "@/stores/files";
import type { FileEntry } from "@/api/types";
import Icon from "./Icon.vue";

const emit = defineEmits<{ (e: "choose", file: FileEntry): void; (e: "close"): void; (e: "manage"): void }>();
const query = ref("");

onMounted(() => void openPicker());

let debounce: number | undefined;
function onInput() {
  window.clearTimeout(debounce);
  debounce = window.setTimeout(() => void search(query.value), 160);
}

const showing = computed<{ label: string; items: FileEntry[] }[]>(() => {
  if (query.value.trim()) return [{ label: "Results", items: files.results }];
  const groups: { label: string; items: FileEntry[] }[] = [];
  if (files.browsing) {
    groups.push({ label: files.browsing, items: files.entries });
  } else {
    if (files.recent.length) groups.push({ label: "Recent", items: files.recent });
    groups.push({
      label: "Folders",
      items: files.folders.map((f) => ({
        path: f.path,
        display: f.display,
        dir: f.display,
        name: f.display,
        size: f.fileCount,
        mtime: f.lastIndexedAt,
        isDir: true,
      })),
    });
  }
  return groups.filter((g) => g.items.length > 0);
});

function activate(f: FileEntry) {
  if (f.isDir) void browse(f.path);
  else emit("choose", f);
}

/** A folder is known by its own name; the path above it is the detail. */
function split(path: string): { name: string; parent: string } {
  const trimmed = path.replace(/\/+$/, "");
  const at = trimmed.lastIndexOf("/");
  return at <= 0 ? { name: trimmed, parent: "" } : { name: trimmed.slice(at + 1), parent: trimmed.slice(0, at) };
}

/**
 * The end of a path is the part that tells files apart; its start is the same
 * for every row. Long ones keep their last two folders.
 */
function tail(path: string): string {
  const parts = path.split("/").filter(Boolean);
  return parts.length > 3 ? `…/${parts.slice(-2).join("/")}` : path;
}

const kb = (n: number) => (n < 1024 ? `${n} B` : n < 1048576 ? `${Math.round(n / 1024)} KB` : `${(n / 1048576).toFixed(1)} MB`);
</script>

<template>
  <div class="sk-pop sk-switcher sk-filepick" role="listbox" aria-label="Files">
    <div class="srv">
      <span class="sk-dot" />AI Skope Server ·
      {{ files.folders.map((f) => tail(f.display)).join(" · ") || "no folders allowed yet" }}
    </div>

    <div class="search">
      <Icon id="i-search" />
      <input
        v-model="query"
        placeholder="Find a file or folder"
        aria-label="Find a file or folder"
        autofocus
        @input="onInput()"
      />
    </div>

    <div class="list">
      <button
        v-if="files.browsing"
        type="button"
        class="sk-model sk-file two"
        @click="files.browsing = ''; void openPicker()"
      >
        <span class="ft dir"><Icon id="i-arrow-left" /></span>
        <span class="txt"><span class="nm"><span>Back</span></span></span>
      </button>

      <template v-for="g in showing" :key="g.label">
        <div class="sk-group"><span>{{ g.label }}</span></div>
        <button
          v-for="f in g.items"
          :key="f.path"
          type="button"
          class="sk-model sk-file two"
          role="option"
          :title="f.display || f.path"
          @click="activate(f)"
        >
          <span class="ft" :class="{ dir: f.isDir }">
            <Icon v-if="f.isDir" id="i-folder" />
            <template v-else>{{ (f.ext || "").replace(".", "").slice(0, 4).toUpperCase() || "TXT" }}</template>
          </span>
          <span class="txt">
            <span class="nm"><span>{{ f.isDir ? split(f.display || f.name).name : f.name }}</span></span>
            <span class="sub">
              <template v-if="f.isDir">{{ f.size }} files<template v-if="split(f.display || f.name).parent"> · {{ tail(split(f.display || f.name).parent) }}</template></template>
              <template v-else>{{ kb(f.size) }} · {{ tail(f.dir) }}</template>
            </span>
          </span>
        </button>
      </template>

      <div v-if="showing.length === 0 && !files.loading" class="sk-hist-empty">
        <template v-if="files.folders.length === 0">
          No folders are allowed yet. Add one in Settings, or run
          <code>aiss folders add ~/dev</code>.
        </template>
        <template v-else-if="query">Nothing matches "{{ query }}".</template>
        <template v-else>Nothing indexed yet.</template>
      </div>
      <div v-if="files.error" class="sk-hist-empty">{{ files.error }}</div>
    </div>

    <div class="foot">
      <span>Only folders you allowed</span>
      <a href="#" @click.prevent="emit('manage')">Manage folders →</a>
    </div>
  </div>
</template>
