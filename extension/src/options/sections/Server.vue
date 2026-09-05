<script setup lang="ts">
/** The server itself, and the coding agents it can drive. */
import { ref, onMounted } from "vue";
import { connection, api, setBaseUrl, connect } from "@/stores/connection";
import type { RuntimeInfo } from "@/api/types";
import Icon from "@/pane/components/Icon.vue";

const url = ref("");
const runtimes = ref<RuntimeInfo[]>([]);
const busy = ref(false);
const error = ref("");

onMounted(async () => {
  url.value = connection.settings?.baseUrl ?? "";
  await refresh();
});

async function refresh() {
  if (connection.state !== "online") return;
  try {
    runtimes.value = await api().runtimes();
  } catch (err) {
    error.value = err instanceof Error ? err.message : String(err);
  }
}

async function detect() {
  busy.value = true;
  error.value = "";
  try {
    runtimes.value = await api().detectRuntimes();
  } catch (err) {
    error.value = err instanceof Error ? err.message : String(err);
  } finally {
    busy.value = false;
  }
}

async function toggle(r: RuntimeInfo) {
  await api().setRuntimeEnabled(r.id, !r.enabled);
  await refresh();
}

const glyph = (id: string) =>
  id.startsWith("claude") ? "r-claude-code"
  : id.startsWith("codex") ? "r-codex"
  : id.startsWith("opencode") ? "r-opencode"
  : id.startsWith("omp") ? "r-omp"
  : id.startsWith("pi") ? "r-pi"
  : "i-server";
</script>

<template>
  <section class="sk-opt-section" data-section="server">
    <h2>Server &amp; runtimes</h2>
    <p class="lead">
      The server drives the coding agents installed on this computer and reads the
      folders you allow. Start it with <code>aiss start</code>.
    </p>

    <div class="sk-row">
      <div class="lbl">
        <b>Server address</b>
        <small>
          <span class="sk-dot" :class="connection.state === 'online' ? '' : 'is-offline'" style="display: inline-block; vertical-align: middle; margin-right: 5px" />
          {{ connection.state === "online" ? `connected · v${connection.health?.version}` : "not reachable" }}
        </small>
      </div>
      <input v-model="url" class="sk-input mono" style="width: 240px" aria-label="Server URL" />
      <button class="sk-btn secondary sm" @click="setBaseUrl(url)">Save</button>
      <button class="sk-btn ghost sm" @click="connect()">Reconnect</button>
    </div>

    <div class="sk-tablewrap">
      <table class="sk-table">
        <thead>
          <tr><th /><th>Runtime</th><th>Path</th><th>Status</th><th /></tr>
        </thead>
        <tbody>
          <tr v-for="r in runtimes" :key="r.id">
            <td><span class="glyph"><Icon :id="glyph(r.id)" /></span></td>
            <td><b>{{ r.name }}</b><small class="mono">{{ r.version || "—" }}</small></td>
            <td class="mono">{{ r.path || r.detail || "not found" }}</td>
            <td>
              <span class="sk-dot" :class="r.status === 'ok' ? '' : r.status === 'degraded' ? 'is-degraded' : 'is-offline'" />
              {{ r.available ? r.status : "not installed" }}
            </td>
            <td>
              <button
                v-if="r.available"
                type="button"
                class="sk-switch"
                role="switch"
                :aria-checked="r.enabled"
                :aria-label="r.name"
                @click="toggle(r)"
              />
              <a
                v-else
                class="sk-btn secondary sm"
                :href="`https://www.google.com/search?q=install+${encodeURIComponent(r.name)}+cli`"
                target="_blank"
                rel="noreferrer"
              >Install guide</a>
            </td>
          </tr>
          <tr v-if="runtimes.length === 0">
            <td colspan="5" class="mono">No runtimes detected yet.</td>
          </tr>
        </tbody>
      </table>
    </div>
    <div class="sk-row" style="border: 0">
      <button class="sk-btn secondary sm" :disabled="busy" @click="detect()">
        <Icon id="i-refresh" />{{ busy ? "Detecting…" : "Detect again" }}
      </button>
      <span v-if="error" class="mono" style="color: var(--bad)">{{ error }}</span>
    </div>
  </section>
</template>
