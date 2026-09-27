<script setup lang="ts">
/**
 * Answer length, as a menu that names each stop. It rises from the button
 * beside the model chip; five inline stops there left the chip too little
 * room to show which model it was.
 */
import { computed, onMounted, ref } from "vue";
import { connection } from "@/stores/connection";
import { saveSettings, CONCISENESS_NORMAL } from "@/stores/storage";
import { LENGTH_STOPS, type LengthStop } from "@/pane/length";
import Icon from "./Icon.vue";

const emit = defineEmits<{ (e: "close"): void }>();

const current = computed(() => connection.settings?.conciseness ?? CONCISENESS_NORMAL);
const root = ref<HTMLElement | null>(null);

function choose(stop: LengthStop) {
  void saveSettings({ conciseness: stop.value });
  emit("close");
}

/** Arrow keys move between the stops, as they do in any menu. */
function onKeydown(e: KeyboardEvent) {
  if (e.key !== "ArrowDown" && e.key !== "ArrowUp") return;
  e.preventDefault();
  const items = [...(root.value?.querySelectorAll<HTMLButtonElement>("button") ?? [])];
  const at = items.indexOf(document.activeElement as HTMLButtonElement);
  const next = e.key === "ArrowDown" ? (at + 1) % items.length : (at - 1 + items.length) % items.length;
  items[next]?.focus();
}

// Focus lands on the stop in force, so Enter alone keeps it.
onMounted(() => root.value?.querySelector<HTMLButtonElement>('[aria-checked="true"]')?.focus());
</script>

<template>
  <div ref="root" class="sk-pop sk-menu" role="menu" aria-label="Answer length" @keydown="onKeydown">
    <div class="hd">Answer length</div>
    <button
      v-for="stop in LENGTH_STOPS"
      :key="stop.value"
      type="button"
      role="menuitemradio"
      :aria-checked="current === stop.value"
      @click="choose(stop)"
    >
      <span class="n">{{ stop.value }}</span>
      <span>{{ stop.label }}</span>
      <Icon id="i-check" class="chk" />
    </button>
    <p class="note">Stays set for the messages after this one. Normal adds no instruction.</p>
  </div>
</template>
