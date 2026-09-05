<script setup lang="ts">
/** One symbol from the sprite the document inlines at start-up. */
defineProps<{ id: string; size?: "sm" | "lg" }>();
</script>

<template>
  <svg class="sk-ico" :class="size" aria-hidden="true"><use :href="`#${id}`" /></svg>
</template>
