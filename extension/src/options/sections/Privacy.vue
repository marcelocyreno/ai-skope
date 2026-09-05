<script setup lang="ts">
import { computed, ref } from "vue";
import { connection, api } from "@/stores/connection";
import { saveSettings, type Settings } from "@/stores/storage";
import Icon from "@/pane/components/Icon.vue";

const s = computed(() => connection.settings);
const set = (patch: Partial<Settings>) => void saveSettings(patch);
const host = ref("");

function addHost() {
  const v = host.value.trim();
  if (!v) return;
  set({ blockedHosts: [...(s.value?.blockedHosts ?? []), v] });
  host.value = "";
}

function removeHost(h: string) {
  set({ blockedHosts: (s.value?.blockedHosts ?? []).filter((x) => x !== h) });
}

async function setRetention(days: string) {
  await api().saveSettings({ "privacy.retentionDays": days });
}
</script>

<template>
  <section class="sk-opt-section" data-section="privacy">
    <h2>Privacy</h2>
    <div class="sk-row">
      <div class="lbl">
        <b>Send page content to the model</b>
        <small>Only what you pick or select is sent unless you allow the whole page.</small>
      </div>
      <div class="sk-seg" role="group">
        <button type="button" :aria-pressed="s?.pageAccess === 'ask'" @click="set({ pageAccess: 'ask' })">Ask</button>
        <button type="button" :aria-pressed="s?.pageAccess === 'always'" @click="set({ pageAccess: 'always' })">Always</button>
        <button type="button" :aria-pressed="s?.pageAccess === 'never'" @click="set({ pageAccess: 'never' })">Never</button>
      </div>
    </div>

    <div class="sk-row">
      <div class="lbl"><b>Blocked sites</b><small>AI Skope never reads or sends anything here.</small></div>
    </div>
    <div class="sk-chks">
      <span v-for="h in s?.blockedHosts ?? []" :key="h" class="sk-chk" aria-pressed="true">
        {{ h }}
        <button type="button" class="sk-ctx-x" aria-label="Remove" @click="removeHost(h)"><Icon id="i-close" /></button>
      </span>
    </div>
    <div class="sk-row" style="border: 0">
      <input v-model="host" class="sk-input mono" placeholder="mail.google.com" style="width: 240px" @keydown.enter="addHost()" />
      <button class="sk-btn secondary sm" @click="addHost()"><Icon id="i-plus" />Add site</button>
    </div>

    <div class="sk-row">
      <div class="lbl"><b>Delete chats older than</b><small>Applied by the server.</small></div>
      <span class="sk-selectwrap">
        <select class="sk-select" @change="setRetention(($event.target as HTMLSelectElement).value)">
          <option value="0">Never</option>
          <option value="90">90 days</option>
          <option value="30">30 days</option>
          <option value="7">7 days</option>
        </select>
        <Icon id="i-chevron-down" />
      </span>
    </div>
  </section>
</template>
