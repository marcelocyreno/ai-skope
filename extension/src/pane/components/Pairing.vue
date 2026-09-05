<script setup lang="ts">
/**
 * What the pane shows before it is paired, or when the server is down. The
 * code comes from `aiss pair` in a terminal — the server never sends it here,
 * which is what stops a web page from pairing itself.
 */
import { ref, computed } from "vue";
import { connection, pair, connect, setBaseUrl } from "@/stores/connection";
import Icon from "./Icon.vue";

const code = ref("");
const busy = ref(false);
const error = ref("");
const editingUrl = ref(false);
const url = ref(connection.settings?.baseUrl ?? "http://127.0.0.1:7331");

const offline = computed(() => connection.state === "offline");

async function submit() {
  if (code.value.trim().length < 4) return;
  busy.value = true;
  error.value = "";
  try {
    await pair(code.value);
    code.value = "";
  } catch (err) {
    error.value = err instanceof Error ? err.message : String(err);
  } finally {
    busy.value = false;
  }
}

async function saveUrl() {
  editingUrl.value = false;
  await setBaseUrl(url.value.trim());
}
</script>

<template>
  <div class="sk-connect">
    <div class="ring"><Icon id="i-reticle-big" /></div>

    <template v-if="offline">
      <h2>The server isn't running</h2>
      <p>
        AI Skope answers through a small server on this computer. Start it with
        <code>aiss start</code>, then try again.
      </p>
      <div class="sk-suggest" style="max-width: 260px">
        <button type="button" @click="connect()"><Icon id="i-refresh" />Try again</button>
      </div>
      <p v-if="connection.retryIn > 0" class="sk-muted">retrying in {{ connection.retryIn }}s</p>
    </template>

    <template v-else>
      <h2>Pair this browser</h2>
      <p>
        Run <code>aiss pair</code> in a terminal and type the code it shows. It is
        valid once, for ten minutes.
      </p>
      <input
        v-model="code"
        class="sk-code-input"
        maxlength="8"
        placeholder="XXXXXXXX"
        aria-label="Pairing code"
        autofocus
        @keydown.enter="submit()"
      />
      <button class="sk-btn primary" :disabled="busy || code.trim().length < 4" @click="submit()">
        {{ busy ? "Pairing…" : "Pair" }}
      </button>
      <p v-if="error" class="err">{{ error }}</p>
    </template>

    <p class="sk-muted">
      <template v-if="!editingUrl">
        {{ connection.settings?.baseUrl }}
        <button type="button" class="sk-btn ghost sm" @click="editingUrl = true">Change</button>
      </template>
      <template v-else>
        <input v-model="url" class="sk-input mono" style="width: 180px" @keydown.enter="saveUrl()" />
        <button type="button" class="sk-btn ghost sm" @click="saveUrl()">Save</button>
      </template>
    </p>
  </div>
</template>
