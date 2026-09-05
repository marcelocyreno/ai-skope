<script setup lang="ts">
/**
 * The in-pane sheet: only what you change while working. Everything heavier —
 * server, runtimes, folders, providers — lives on the options page.
 */
import { computed } from "vue";
import { connection } from "@/stores/connection";
import { saveSettings, type Settings } from "@/stores/storage";
import Icon from "./Icon.vue";

const emit = defineEmits<{ (e: "close"): void; (e: "options", section: string): void }>();

const s = computed(() => connection.settings);

const palettes: { id: Settings["palette"]; label: string }[] = [
  { id: "graphite", label: "Graphite" },
  { id: "nocturne", label: "Nocturne" },
  { id: "sage", label: "Sage" },
  { id: "ember", label: "Ember" },
  { id: "arctic", label: "Arctic" },
  { id: "mono", label: "Mono" },
];

const set = (patch: Partial<Settings>) => void saveSettings(patch);

const serverSummary = computed(() => {
  if (connection.state !== "online") return "not reachable";
  const n = connection.runtimes.filter((r) => r.available).length;
  return `connected · v${connection.health?.version ?? "?"} · ${n} runtime${n === 1 ? "" : "s"}`;
});
</script>

<template>
  <div class="sk-settings is-open" role="dialog" aria-label="Settings">
    <div class="hd">
      <button type="button" class="sk-iconbtn" aria-label="Back to chat" @click="emit('close')">
        <Icon id="i-arrow-left" />
      </button>
      <h2>Settings</h2>
      <span class="sk-topbar-spacer" />
      <button type="button" class="sk-btn ghost sm" @click="emit('options', 'general')">
        All settings <Icon id="i-external" />
      </button>
    </div>

    <div class="bd">
      <section class="sk-set-section">
        <h3>Appearance</h3>
        <div class="sk-row">
          <div class="lbl"><b>Theme</b><small>Follows Chrome unless you choose one.</small></div>
          <div class="sk-seg" role="group" aria-label="Theme">
            <button type="button" :aria-pressed="s?.theme === 'light'" @click="set({ theme: 'light' })">
              <Icon id="i-sun" />Light
            </button>
            <button type="button" :aria-pressed="s?.theme === 'dark'" @click="set({ theme: 'dark' })">
              <Icon id="i-moon" />Dark
            </button>
            <button type="button" :aria-pressed="s?.theme === 'system'" @click="set({ theme: 'system' })">
              <Icon id="i-monitor" />System
            </button>
          </div>
        </div>
        <div class="sk-row">
          <div class="lbl">
            <b>Color</b>
            <small>{{ palettes.find((p) => p.id === s?.palette)?.label }}</small>
          </div>
          <div class="sk-swatches" role="group" aria-label="Color palette">
            <button
              v-for="p in palettes"
              :key="p.id"
              type="button"
              class="sk-swatch"
              :data-palette="p.id"
              data-theme="dark"
              :aria-pressed="s?.palette === p.id"
              :aria-label="p.label"
              :title="p.label"
              @click="set({ palette: p.id })"
            />
          </div>
        </div>
        <div class="sk-row">
          <div class="lbl"><b>Text size</b></div>
          <div class="sk-seg" role="group" aria-label="Text size">
            <button type="button" :aria-pressed="s?.textSize === 'small'" @click="set({ textSize: 'small' })">Small</button>
            <button type="button" :aria-pressed="s?.textSize === 'default'" @click="set({ textSize: 'default' })">Default</button>
            <button type="button" :aria-pressed="s?.textSize === 'large'" @click="set({ textSize: 'large' })">Large</button>
          </div>
        </div>
      </section>

      <section class="sk-set-section">
        <h3>Models</h3>
        <div class="sk-row">
          <div class="lbl">
            <b>AI Skope Server</b>
            <small>
              <span class="sk-dot" :class="connection.state === 'online' ? '' : 'is-offline'" style="display: inline-block; vertical-align: middle; margin-right: 5px" />
              {{ serverSummary }}
            </small>
          </div>
          <button type="button" class="sk-btn secondary sm" @click="emit('options', 'server')">Manage</button>
        </div>
        <div class="sk-row">
          <div class="lbl"><b>Providers &amp; keys</b><small>Held by the server, never by the browser.</small></div>
          <button type="button" class="sk-btn secondary sm" @click="emit('options', 'providers')">Manage</button>
        </div>
        <div class="sk-row">
          <div class="lbl"><b>Folders</b><small>What the server may read.</small></div>
          <button type="button" class="sk-btn secondary sm" @click="emit('options', 'folders')">Manage</button>
        </div>
      </section>

      <section class="sk-set-section">
        <h3>Page access</h3>
        <div class="sk-row">
          <div class="lbl">
            <b>Send page content to the model</b>
            <small>Only what you pick or select is sent unless you allow the whole page.</small>
          </div>
          <div class="sk-seg" role="group" aria-label="Page content">
            <button type="button" :aria-pressed="s?.pageAccess === 'ask'" @click="set({ pageAccess: 'ask' })">Ask</button>
            <button type="button" :aria-pressed="s?.pageAccess === 'always'" @click="set({ pageAccess: 'always' })">Always</button>
            <button type="button" :aria-pressed="s?.pageAccess === 'never'" @click="set({ pageAccess: 'never' })">Never</button>
          </div>
        </div>
        <div class="sk-row">
          <div class="lbl"><b>Blocked sites</b><small>{{ s?.blockedHosts.length ?? 0 }} site(s) never read.</small></div>
          <button type="button" class="sk-btn secondary sm" @click="emit('options', 'privacy')">Manage</button>
        </div>
      </section>

      <section class="sk-set-section">
        <div class="sk-row" style="border: 0">
          <div class="lbl">
            <b>AI Skope 0.1.0</b>
            <small>Paired with {{ connection.settings?.baseUrl }}</small>
          </div>
        </div>
      </section>
    </div>
  </div>
</template>
