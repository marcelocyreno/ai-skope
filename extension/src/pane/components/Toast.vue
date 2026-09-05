<script setup lang="ts">
import { toast, dismissToast } from "@/stores/toast";
import Icon from "./Icon.vue";
</script>

<template>
  <div v-if="toast.message" class="sk-toast" role="status">
    <Icon :id="toast.icon" />
    <span v-html="toast.message" />
    <button v-if="toast.undoLabel" type="button" class="undo" @click="toast.onUndo?.()">
      <Icon id="i-undo" size="sm" />{{ toast.undoLabel }}
    </button>
    <button
      v-else
      type="button"
      class="undo"
      aria-label="Dismiss"
      style="padding: 0 8px"
      @click="dismissToast()"
    >
      <Icon id="i-close" size="sm" />
    </button>
  </div>
</template>
