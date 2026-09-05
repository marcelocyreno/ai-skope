<script setup lang="ts">
import { ref } from "vue";
import { connection, api } from "@/stores/connection";
import { saveSettings } from "@/stores/storage";

const logs = ref<string[]>([]);
const showing = ref(false);

async function loadLogs() {
  showing.value = true;
  try {
    logs.value = (await api().logs(60)).lines;
  } catch (err) {
    logs.value = [err instanceof Error ? err.message : String(err)];
  }
}

async function unpair() {
  await saveSettings({ token: "", serverId: "" });
  location.reload();
}
</script>

<template>
  <section class="sk-opt-section" data-section="about">
    <h2>About</h2>
    <div class="sk-row">
      <div class="lbl"><b>AI Skope 0.1.0</b><small>Chrome extension</small></div>
    </div>
    <div class="sk-row">
      <div class="lbl">
        <b>AI Skope Server {{ connection.health?.version ?? "—" }}</b>
        <small class="mono">{{ connection.settings?.baseUrl }}</small>
      </div>
      <button class="sk-btn secondary sm" @click="loadLogs()">View logs</button>
    </div>
    <div v-if="showing" class="sk-tablewrap" style="max-height: 320px; overflow: auto">
      <pre class="mono" style="margin: 0; padding: 12px; font-size: 11px; white-space: pre-wrap">{{ logs.join("\n") }}</pre>
    </div>
    <div class="sk-row" style="border: 0">
      <div class="lbl"><b>Pairing</b><small>Forget this browser's token and pair again.</small></div>
      <button class="sk-btn danger sm" @click="unpair()">Unpair</button>
    </div>
  </section>
</template>
