<script setup lang="ts">
/**
 * One turn in the transcript. User messages are bubbles on the right with
 * their context above; the assistant reads as a transcript, not a bubble.
 */
import { computed } from "vue";
import type { Message } from "@/api/types";
import { renderMarkdown, renderPlain, withCursor } from "@/pane/markdown";
import ContextChip from "./ContextChip.vue";
import Icon from "./Icon.vue";

const props = defineProps<{ message: Message; streaming?: boolean }>();
defineEmits<{ (e: "retry"): void }>();

const time = computed(() =>
  new Date(props.message.createdAt).toLocaleTimeString([], { hour: "2-digit", minute: "2-digit" }),
);

/** What the user typed, kept literal — only its line breaks are honoured. */
const asked = computed(() => renderPlain(props.message.text));

const CURSOR = '<span class="sk-cursor"></span>';

/** Answers arrive as Markdown; see markdown.ts for why nothing is parsed as HTML. */
const answer = computed(() => {
  const html = renderMarkdown(props.message.text);
  return props.streaming ? withCursor(html, CURSOR) : html;
});
</script>

<template>
  <div v-if="message.role === 'user'" class="sk-msg user">
    <div v-if="message.context?.length" class="sk-msg-ctx">
      <ContextChip v-for="(item, i) in message.context" :key="i" :item="item" />
    </div>
    <div class="sk-bubble"><p v-html="asked"></p></div>
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

    <div v-if="message.text || streaming" class="sk-ai-body" v-html="answer"></div>

    <div v-if="message.error" class="sk-err">
      <Icon id="i-alert" />
      <span>{{ message.error }}</span>
      <button type="button" class="act" @click="$emit('retry')">Retry</button>
    </div>

    <span v-if="!streaming" class="sk-time">{{ time }}</span>
  </div>
</template>
