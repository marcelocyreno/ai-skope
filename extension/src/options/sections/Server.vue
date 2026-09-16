<script setup lang="ts">
/** The server itself, and the coding agents it can drive. */
import { ref, computed, onMounted, watch } from "vue";
import { connection, api, setBaseUrl, connect } from "@/stores/connection";
import { models, loadModels, setDefaultModel, shortModel } from "@/stores/models";
import type { ModelOption, RuntimeInfo } from "@/api/types";
import Icon from "@/pane/components/Icon.vue";

const url = ref("");
const runtimes = ref<RuntimeInfo[]>([]);
const busy = ref(false);
const error = ref("");

/**
 * The default model: what every new chat and every fresh pane starts from.
 *
 * The server has stored one all along and nothing ever wrote it, so a pane
 * reload silently fell back to "the first runtime reporting OK, no effort" —
 * and an effort chosen in the switcher lived only until the pane closed.
 *
 * The key identifies an option across a reload, since runtime, provider and
 * model are what a Selection is made of. The <select> cannot carry an object.
 */
const keyOf = (o: { runtime: string; provider?: string; model: string }) =>
  `${o.runtime}\u0000${o.provider ?? ""}\u0000${o.model}`;

const chosen = ref("");
const savedEffort = ref("");
const saving = ref(false);
const saved = ref(false);

const chosenOption = computed<ModelOption | undefined>(() =>
  models.options.find((o) => keyOf(o) === chosen.value),
);

/** Only the runtimes that reason offer a level; the rest show no control. */
const efforts = computed<string[]>(() => chosenOption.value?.effortLevels ?? []);

/** How a row reads in the list: the runtime, then the model by its own name. */
function optionLabel(o: ModelOption): string {
  const model = o.provider ? `${o.provider} / ${shortModel(o.model)}` : shortModel(o.model);
  return `${o.runtimeName} · ${model}`;
}

/** Reads the stored default back into the controls. */
function showStoredDefault() {
  const current = models.options.find((o) => o.default);
  chosen.value = current ? keyOf(current) : "";
  savedEffort.value = models.stored?.effort ?? "";
}

async function saveDefault() {
  const option = chosenOption.value;
  if (!option) return;
  saving.value = true;
  saved.value = false;
  error.value = "";
  try {
    await setDefaultModel({
      runtime: option.runtime,
      provider: option.provider,
      model: option.model,
      // A runtime that reports no levels stores none: the agent decides.
      effort: efforts.value.includes(savedEffort.value) ? savedEffort.value : undefined,
    });
    showStoredDefault();
    saved.value = true;
  } catch (err) {
    error.value = err instanceof Error ? err.message : String(err);
  } finally {
    saving.value = false;
  }
}

onMounted(async () => {
  url.value = connection.settings?.baseUrl ?? "";
  await refresh();
});
// The page mounts before the connection is up, so load again once it is.
watch(() => connection.state, (state) => {
  if (state === "online") {
    url.value = connection.settings?.baseUrl ?? url.value;
    void refresh();
  }
});

async function refresh() {
  if (connection.state !== "online") return;
  try {
    runtimes.value = await api().runtimes();
    await loadModels();
    showStoredDefault();
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
      folders you allow. Install it with
      <code>brew install marcelocyreno/tap/aiss</code>, then start it with
      <code>aiss start</code>. The
      <a href="https://marcelocyreno.github.io/ai-skope/install" target="_blank" rel="noopener noreferrer">install guide</a>
      covers Linux and building from source.
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

    <div class="sk-row">
      <div class="lbl">
        <b>Default model</b>
        <small>What every new chat and every fresh pane starts from.</small>
      </div>
      <span class="sk-selectwrap">
        <select v-model="chosen" class="sk-select" style="width: 260px" aria-label="Default model">
          <option value="" disabled>Choose a model</option>
          <option v-for="o in models.options" :key="keyOf(o)" :value="keyOf(o)">
            {{ optionLabel(o) }}
          </option>
        </select>
        <Icon id="i-chevron-down" />
      </span>
      <button class="sk-btn secondary sm" :disabled="!chosenOption || saving" @click="saveDefault()">
        {{ saving ? "Saving…" : saved ? "Saved" : "Save" }}
      </button>
    </div>

    <div v-if="efforts.length" class="sk-row">
      <div class="lbl">
        <b>Effort</b>
        <small>{{ chosenOption?.runtimeName }} reasons harder the higher this goes.</small>
      </div>
      <span class="sk-seg" role="group" aria-label="Effort">
        <button
          v-for="level in efforts"
          :key="level"
          type="button"
          :aria-pressed="savedEffort === level"
          @click="savedEffort = level"
        >
          {{ level }}
        </button>
      </span>
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
