<script setup lang="ts">
import Icon from "./Icon.vue";

const emit = defineEmits<{
  (e: "summarize"): void;
  (e: "pick"): void;
  (e: "file"): void;
  (e: "select"): void;
}>();
</script>

<template>
  <div class="sk-empty">
    <div class="ring"><Icon id="i-reticle-big" /></div>
    <h2>Aim at anything on this page</h2>
    <p>Pick an element or select text, then ask. Answers stay grounded in what's on screen.</p>
    <div class="sk-suggest">
      <button type="button" @click="emit('summarize')"><Icon id="i-summarize" />Summarize this page</button>
      <button type="button" class="aim" @click="emit('pick')"><Icon id="i-reticle" />Pick an element to ask about</button>
      <button type="button" @click="emit('file')"><Icon id="i-folder" />Ask about a local file or repo</button>
      <button type="button" @click="emit('select')"><Icon id="i-select-text" />Explain the selected text</button>
    </div>
  </div>
</template>
