<script setup lang="ts">
/**
 * Providers whose keys the server holds. The key is typed here but stored by
 * the server in the OS keychain; the browser only ever sees a masked form.
 */
import { ref, onMounted, computed, watch } from "vue";
import { connection, api } from "@/stores/connection";
import type { Capabilities, Provider, RuntimeInfo } from "@/api/types";
import Icon from "@/pane/components/Icon.vue";

const providers = ref<Provider[]>([]);
const runtimes = ref<RuntimeInfo[]>([]);
const kinds = ref<Capabilities["providerKinds"]>([]);
const adding = ref(false);
const busy = ref(false);
const error = ref("");
const testResult = ref<Record<string, string>>({});

const form = ref({ kind: "", name: "", baseUrl: "", key: "", availableTo: [] as string[] });

const agentRuntimes = computed(() => runtimes.value.filter((r) => r.usesProviders));

onMounted(refresh);
// The page mounts before the connection is up, so load again once it is.
watch(() => connection.state, (state) => { if (state === "online") void refresh(); });

async function refresh() {
  if (connection.state !== "online") return;
  try {
    const [p, r, c] = await Promise.all([api().providers(), api().runtimes(), api().capabilities()]);
    providers.value = p;
    runtimes.value = r;
    kinds.value = c.providerKinds;
    if (!form.value.kind && kinds.value.length) form.value.kind = kinds.value[0].id;
  } catch (err) {
    error.value = err instanceof Error ? err.message : String(err);
  }
}

function toggleRuntime(id: string) {
  const list = form.value.availableTo;
  const at = list.indexOf(id);
  if (at === -1) list.push(id);
  else list.splice(at, 1);
}

async function create() {
  busy.value = true;
  error.value = "";
  try {
    await api().createProvider({ ...form.value });
    form.value = { kind: kinds.value[0]?.id ?? "", name: "", baseUrl: "", key: "", availableTo: [] };
    adding.value = false;
    await refresh();
  } catch (err) {
    error.value = err instanceof Error ? err.message : String(err);
  } finally {
    busy.value = false;
  }
}

async function test(p: Provider) {
  testResult.value[p.id] = "testing…";
  try {
    const out = await api().testProvider(p.id);
    testResult.value[p.id] = out.ok ? `key works · ${out.message}` : out.message;
  } catch (err) {
    testResult.value[p.id] = err instanceof Error ? err.message : String(err);
  }
  await refresh();
}

async function remove(p: Provider) {
  await api().deleteProvider(p.id);
  await refresh();
}

const needsKey = computed(() => kinds.value.find((k) => k.id === form.value.kind)?.needsKey ?? true);
</script>

<template>
  <section class="sk-opt-section" data-section="providers">
    <h2>Providers &amp; keys</h2>
    <p class="lead">
      Keys are stored by the AI Skope Server on this computer; the extension never
      sees them. Each provider is offered to the runtimes you choose.
    </p>

    <div class="sk-tablewrap">
      <table class="sk-table">
        <thead><tr><th>Provider</th><th>Key</th><th>Available to</th><th>Models</th><th /></tr></thead>
        <tbody>
          <tr v-for="p in providers" :key="p.id">
            <td><b>{{ p.name }}</b><small class="mono">{{ p.kind }}</small></td>
            <td class="mono">{{ p.key || "—" }}</td>
            <td class="mono">{{ p.availableTo.join(" · ") || "—" }}</td>
            <td>
              {{ p.models?.length ?? 0 }}
              <small v-if="testResult[p.id]" class="mono">{{ testResult[p.id] }}</small>
              <small v-else-if="p.lastTestMessage" class="mono">{{ p.lastTestMessage }}</small>
            </td>
            <td>
              <button class="sk-btn ghost sm" @click="test(p)">Test</button>
              <button class="sk-iconbtn sm" aria-label="Remove provider" @click="remove(p)"><Icon id="i-trash" /></button>
            </td>
          </tr>
          <tr v-if="providers.length === 0"><td colspan="5" class="mono">No providers yet.</td></tr>
        </tbody>
      </table>
    </div>

    <div v-if="!adding" class="sk-row" style="border: 0">
      <button class="sk-btn primary sm" @click="adding = true"><Icon id="i-plus" />Add a source</button>
    </div>

    <div v-else class="sk-form" style="padding: 0">
      <div class="sk-callout">
        <Icon id="i-server" />
        <span>The key is stored by the <b>AI Skope Server</b> on this computer. The extension never sees it — runtimes use it when they answer.</span>
      </div>
      <label class="lbl">
        Provider
        <span class="sk-selectwrap">
          <select v-model="form.kind" class="sk-select">
            <option v-for="k in kinds" :key="k.id" :value="k.id">{{ k.name }}</option>
          </select>
          <Icon id="i-chevron-down" />
        </span>
      </label>
      <label class="lbl">Name (optional)<input v-model="form.name" class="sk-input" placeholder="z.ai" /></label>
      <label class="lbl">Base URL (optional)<input v-model="form.baseUrl" class="sk-input mono" placeholder="https://…" /></label>
      <label v-if="needsKey" class="lbl">API key<input v-model="form.key" class="sk-input mono" type="password" placeholder="sk-…" /></label>
      <div class="lbl">
        Available to
        <div class="sk-chks" style="margin-top: 5px">
          <button
            v-for="r in agentRuntimes"
            :key="r.id"
            type="button"
            class="sk-chk"
            :aria-pressed="form.availableTo.includes(r.id)"
            @click="toggleRuntime(r.id)"
          >
            {{ r.name }}
          </button>
        </div>
      </div>
      <div class="sk-row" style="border: 0">
        <button class="sk-btn ghost" @click="adding = false">Cancel</button>
        <button class="sk-btn primary" :disabled="busy" @click="create()">{{ busy ? "Adding…" : "Add to server" }}</button>
      </div>
    </div>
    <p v-if="error" class="mono" style="color: var(--bad)">{{ error }}</p>
  </section>
</template>
