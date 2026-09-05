<script setup lang="ts">
/**
 * What the pane shows before it is paired, or when the server is down. The
 * code comes from `aiss pair` in a terminal — the server never sends it here,
 * which is what stops a web page from pairing itself.
 */
import { ref, computed } from "vue";
import { connection, pair, connect, setBaseUrl } from "@/stores/connection";
import Icon from "./Icon.vue";

const INSTALL_GUIDE = "https://marcelocyreno.github.io/ai-skope/install";

const code = ref("");
const busy = ref(false);
const error = ref("");
const editingUrl = ref(false);
const url = ref(connection.settings?.baseUrl ?? "http://127.0.0.1:7331");

const offline = computed(() => connection.state === "offline");

/**
 * A token means this browser has paired before, so the server exists and is
 * merely stopped. Without one, the likeliest reason nothing answers is that
 * the reader has installed the extension and has never heard of `aiss` — and
 * telling them to start something they do not have is useless.
 */
const neverPaired = computed(() => !connection.settings?.token);

const copied = ref("");
async function copy(text: string) {
  try {
    await navigator.clipboard.writeText(text);
    copied.value = text;
    window.setTimeout(() => {
      if (copied.value === text) copied.value = "";
    }, 1600);
  } catch {
    // Clipboard access can be refused; the command is on screen to be typed.
  }
}

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
      <h2>{{ neverPaired ? "One thing left to install" : "The server isn't running" }}</h2>
      <p v-if="neverPaired">
        Answers come from coding agents on this computer, driven by a small
        server you run yourself. Nothing is sent anywhere else.
      </p>
      <p v-else>
        AI Skope answers through a small server on this computer, and it is not
        responding. Start it, then try again.
      </p>

      <ol class="sk-steps">
        <li v-if="neverPaired">
          <code>brew install marcelocyreno/tap/aiss</code>
          <button
            type="button"
            class="cp"
            aria-label="Copy the install command"
            @click="copy('brew install marcelocyreno/tap/aiss')"
          >
            <Icon :id="copied === 'brew install marcelocyreno/tap/aiss' ? 'i-check' : 'i-copy'" />
          </button>
        </li>
        <li>
          <code>aiss start</code>
          <button type="button" class="cp" aria-label="Copy the start command" @click="copy('aiss start')">
            <Icon :id="copied === 'aiss start' ? 'i-check' : 'i-copy'" />
          </button>
        </li>
      </ol>

      <div class="sk-suggest" style="max-width: 260px">
        <button type="button" @click="connect()"><Icon id="i-refresh" />Try again</button>
      </div>
      <p v-if="connection.retryIn > 0" class="sk-muted">retrying in {{ connection.retryIn }}s</p>
      <p v-if="neverPaired" class="sk-muted">
        <a :href="INSTALL_GUIDE" target="_blank" rel="noopener noreferrer">Full install guide</a>
        · macOS and Linux
      </p>
    </template>

    <template v-else>
      <h2>Pair this browser</h2>
      <p>
        Run <code>aiss pair</code> in a terminal — a second one, if the server is
        running in the first — and type the code it shows. It is valid once, for
        ten minutes.
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
