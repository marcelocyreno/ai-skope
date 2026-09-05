<script setup lang="ts">
/** One piece of attached context: a picked element, a selection, or a file. */
import { computed } from "vue";
import type { ContextItem } from "@/api/types";
import Icon from "./Icon.vue";

const props = defineProps<{ item: ContextItem; removable?: boolean }>();
defineEmits<{ (e: "remove"): void }>();

const icon = computed(() =>
  props.item.type === "element" ? "i-reticle" : props.item.type === "file" ? "i-folder" : "i-select-text",
);

/** Files show their folder and size; elements show their measured size. */
const dim = computed(() => {
  if (props.item.type === "element" && props.item.rect?.length === 2) {
    return `${props.item.rect[0]} × ${props.item.rect[1]}`;
  }
  if (props.item.type === "file") return props.item.title ?? "";
  return "";
});

const name = computed(() => {
  if (props.item.type === "element") return props.item.selector ?? "element";
  if (props.item.type === "file") return props.item.path?.split("/").pop() ?? props.item.path ?? "file";
  return "";
});
</script>

<template>
  <span class="sk-ctx">
    <Icon :id="icon" />
    <span v-if="item.type === 'text'" class="q">{{ item.quote }}</span>
    <template v-else>
      <span class="sel">{{ name }}</span>
      <span v-if="dim" class="dim">{{ dim }}</span>
    </template>
    <button v-if="removable" type="button" class="sk-ctx-x" aria-label="Remove" @click="$emit('remove')">
      <Icon id="i-close" />
    </button>
  </span>
</template>
