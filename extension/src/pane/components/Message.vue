<script setup lang="ts">
/**
 * One turn in the transcript. User messages are bubbles on the right with
 * their context above; the assistant reads as a transcript, not a bubble.
 */
import { computed } from "vue";
import type { Message } from "@/api/types";
import ContextChip from "./ContextChip.vue";
import Icon from "./Icon.vue";

const props = defineProps<{ message: Message; streaming?: boolean }>();
defineEmits<{ (e: "retry"): void }>();

const time = computed(() =>
  new Date(props.message.createdAt).toLocaleTimeString([], { hour: "2-digit", minute: "2-digit" }),
);

/** Paragraphs, so a long answer is readable without a Markdown renderer. */
const paragraphs = computed(() => props.message.text.split(/\n{2,}/).filter(Boolean));
</script>

<template>
  <div v-if="message.role === 'user'" class="sk-msg user">
    <div v-if="message.context?.length" class="sk-msg-ctx">
      <ContextChip v-for="(item, i) in message.context" :key="i" :item="item" />
    </div>
    <div class="sk-bubble">
      <p v-for="(p, i) in paragraphs" :key="i">{{ p }}</p>
    </div>
    <span class="sk-time">{{ time }}</span>
  </div>

  <div v-else class="sk-msg ai">
    <div class="sk-ai-head">
      <Icon id="i-reticle" />AI Skope<template v-if="message.model"> · {{ message.model }}</template>
    </div>

    <div v-for="(tool, i) in message.tools ?? []" :key="i" class="sk-tool">
      <span v-if="tool.state === 'running'" class="sk-spin" />
      <Icon v-else id="i-check" size="sm" />
      {{ tool.state === "running" ? "Reading" : "Read" }} <code>{{ tool.target || tool.name }}</code>
      <template v-if="tool.detail"> · {{ tool.detail }}</template>
    </div>

    <div v-if="message.text" class="sk-ai-body">
      <p v-for="(p, i) in paragraphs" :key="i">
        {{ p }}<span v-if="streaming && i === paragraphs.length - 1" class="sk-cursor" />
      </p>
    </div>
    <div v-else-if="streaming" class="sk-ai-body"><p><span class="sk-cursor" /></p></div>

    <div v-if="message.error" class="sk-err">
      <Icon id="i-alert" />
      <span>{{ message.error }}</span>
      <button type="button" class="act" @click="$emit('retry')">Retry</button>
    </div>

    <span v-if="!streaming" class="sk-time">{{ time }}</span>
  </div>
</template>
