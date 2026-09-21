<script setup lang="ts">
import { computed } from "vue";
import { connection } from "@/stores/connection";
import { saveSettings, type Settings } from "@/stores/storage";
import Icon from "@/pane/components/Icon.vue";

const s = computed(() => connection.settings);
const set = (patch: Partial<Settings>) => void saveSettings(patch);
const palettes: Settings["palette"][] = ["graphite", "nocturne", "sage", "ember", "arctic", "mono"];
</script>

<template>
  <section class="sk-opt-section" data-section="general">
    <h2>General</h2>
    <div class="sk-row">
      <div class="lbl"><b>Theme</b><small>Follows Chrome unless you choose one.</small></div>
      <div class="sk-seg" role="group">
        <button type="button" :aria-pressed="s?.theme === 'light'" @click="set({ theme: 'light' })"><Icon id="i-sun" />Light</button>
        <button type="button" :aria-pressed="s?.theme === 'dark'" @click="set({ theme: 'dark' })"><Icon id="i-moon" />Dark</button>
        <button type="button" :aria-pressed="s?.theme === 'system'" @click="set({ theme: 'system' })"><Icon id="i-monitor" />System</button>
      </div>
    </div>
    <div class="sk-row">
      <div class="lbl"><b>Color</b><small>{{ s?.palette }}</small></div>
      <div class="sk-swatches" role="group" aria-label="Color palette">
        <button
          v-for="p in palettes"
          :key="p"
          type="button"
          class="sk-swatch"
          :data-palette="p"
          data-theme="dark"
          :aria-pressed="s?.palette === p"
          :aria-label="p"
          @click="set({ palette: p })"
        />
      </div>
    </div>
    <div class="sk-row">
      <div class="lbl"><b>Text size</b></div>
      <div class="sk-seg" role="group">
        <button type="button" :aria-pressed="s?.textSize === 'small'" @click="set({ textSize: 'small' })">Small</button>
        <button type="button" :aria-pressed="s?.textSize === 'default'" @click="set({ textSize: 'default' })">Default</button>
        <button type="button" :aria-pressed="s?.textSize === 'large'" @click="set({ textSize: 'large' })">Large</button>
      </div>
    </div>
    <div class="sk-row">
      <div class="lbl"><b>Open automatically</b><small>Show the pane when Chrome starts.</small></div>
      <button
        type="button"
        class="sk-switch"
        role="switch"
        :aria-checked="s?.openAutomatically"
        @click="set({ openAutomatically: !s?.openAutomatically })"
      />
    </div>
  </section>
</template>
