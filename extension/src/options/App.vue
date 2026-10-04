<script setup lang="ts">
/**
 * The full settings page: everything you configure once. It talks to the same
 * server the pane does, and shares the design kit's components.
 */
import { ref, onMounted, onUnmounted } from "vue";
import { connection, initConnection } from "@/stores/connection";
import Icon from "@/pane/components/Icon.vue";
import { VERSION } from "@/version";
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
  window.addEventListener("scroll", onScroll, { passive: true });
  await initConnection();
  scrollTo(active.value);
});

onUnmounted(() => window.removeEventListener("scroll", onScroll));

/**
 * The nav follows the reader. It only ever changed on a click, so scrolling
 * to Privacy left "General" highlighted. A click's own smooth scroll passes
 * through every section on the way, so for that long the click decides.
 */
let clickedAt = 0;
function onScroll() {
  if (Date.now() - clickedAt < 800) return;
  const sections = [...document.querySelectorAll<HTMLElement>("[data-section]")];
  if (sections.length === 0) return;
  const atBottom = window.innerHeight + window.scrollY >= document.documentElement.scrollHeight - 2;
  let current = sections[0].dataset.section ?? "general";
  for (const el of sections) if (el.getBoundingClientRect().top <= 120) current = el.dataset.section ?? current;
  if (atBottom) current = sections[sections.length - 1].dataset.section ?? current;
  if (current !== active.value) {
    active.value = current;
    history.replaceState(null, "", `#${current}`);
  }
}

function go(id: string) {
  clickedAt = Date.now();
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
          <Icon id="i-reticle" /><span>AI Skope</span><small>Settings · {{ VERSION }}</small>
        </div>
        <a
          v-for="s in sections"
          :key="s.id"
          href="#"
          :class="{ on: active === s.id }"
          :aria-current="active === s.id ? 'location' : undefined"
          @click.prevent="go(s.id)"
        >
          {{ s.label }}
        </a>
        <div class="srv" :title="connection.state === 'online' ? 'Server connected' : 'Server unreachable'">
          <span class="sk-dot" :class="connection.state === 'online' ? '' : 'is-offline'" />
          <span class="txt">{{ connection.state === "online" ? "Server connected" : "Server unreachable" }}</span>
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
