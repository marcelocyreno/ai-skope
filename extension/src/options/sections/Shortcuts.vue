<script setup lang="ts">
/** Chrome owns the key bindings, so this links to where they are edited. */
const shortcuts = [
  { action: "Open AI Skope", keys: ["⌘", "⇧", "A"] },
  { action: "Pick element", keys: ["⌘", "⇧", "K"] },
  { action: "Add the selected text", keys: ["⌘", "⇧", "S"] },
  { action: "New chat", keys: ["⌘", "⇧", "N"] },
  // Chrome binds four suggested shortcuts per extension and the four above
  // spend them, so this one ships unassigned rather than not at all.
  { action: "Copy the last answer", keys: [] },
];

function openChromeShortcuts() {
  void chrome.tabs.create({ url: "chrome://extensions/shortcuts" });
}
</script>

<template>
  <section class="sk-opt-section" data-section="shortcuts">
    <h2>Shortcuts</h2>
    <p class="lead">Chrome manages extension shortcuts; change them on its shortcuts page.</p>
    <div class="sk-tablewrap">
      <table class="sk-table">
        <thead><tr><th>Action</th><th>Keys</th></tr></thead>
        <tbody>
          <tr v-for="s in shortcuts" :key="s.action">
            <td>{{ s.action }}</td>
            <td>
              <span v-if="s.keys.length" class="kbds"><kbd v-for="k in s.keys" :key="k">{{ k }}</kbd></span>
              <span v-else style="color: var(--ink-3)">Unassigned — set it in Chrome</span>
            </td>
          </tr>
        </tbody>
      </table>
    </div>
    <div class="sk-row" style="border: 0">
      <button class="sk-btn secondary sm" @click="openChromeShortcuts()">Edit in Chrome</button>
    </div>
  </section>
</template>
