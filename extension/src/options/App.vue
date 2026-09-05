<script setup lang="ts">
/**
 * The full settings page: everything you configure once. It talks to the same
 * server the pane does, and shares the design kit's components.
 */
import { ref, onMounted } from "vue";
import { connection, initConnection } from "@/stores/connection";
import Icon from "@/pane/components/Icon.vue";
import General from "./sections/General.vue";
import Server from "./sections/Server.vue";
import Folders from "./sections/Folders.vue";
import Providers from "./sections/Providers.vue";
import Privacy from "./sections/Privacy.vue";
import Shortcuts from "./sections/Shortcuts.vue";
import About from "./sections/About.vue";

const sections = [
  { id: "general", label: "General" },
  { id: "server", label: "Server & runtimes" },
  { id: "folders", label: "Folders" },
  { id: "providers", label: "Providers & keys" },
  { id: "privacy", label: "Privacy" },
  { id: "shortcuts", label: "Shortcuts" },
  { id: "about", label: "About" },
];

const active = ref(location.hash.replace("#", "") || "general");

onMounted(async () => {
  await initConnection();
  scrollTo(active.value);
});

function go(id: string) {
  active.value = id;
  history.replaceState(null, "", `#${id}`);
  scrollTo(id);
}

function scrollTo(id: string) {
  requestAnimationFrame(() => {
    document.querySelector(`[data-section="${id}"]`)?.scrollIntoView({ block: "start", behavior: "smooth" });
  });
}
</script>

<template>
  <div class="sk-options">
    <div class="sk-opt-shell">
      <nav class="sk-opt-nav" aria-label="Settings sections">
        <div class="brand">
          <Icon id="i-reticle" /><span>AI Skope</span><small>Settings · 0.1.0</small>
        </div>
        <a
          v-for="s in sections"
          :key="s.id"
          href="#"
          :class="{ on: active === s.id }"
          @click.prevent="go(s.id)"
        >
          {{ s.label }}
        </a>
        <div class="srv">
          <span class="sk-dot" :class="connection.state === 'online' ? '' : 'is-offline'" />
          {{ connection.state === "online" ? "Server connected" : "Server unreachable" }}
          <small class="mono">{{ connection.settings?.baseUrl }}</small>
        </div>
      </nav>

      <main class="sk-opt-main">
        <General />
        <Server />
        <Folders />
        <Providers />
        <Privacy />
        <Shortcuts />
        <About />
      </main>
    </div>
  </div>
</template>
