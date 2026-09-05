<script setup lang="ts">
/** The transcript, pinned to the bottom while an answer streams in. */
import { ref, watch, nextTick, computed } from "vue";
import { chat, retryLast } from "@/stores/chat";
import Message from "./Message.vue";

const el = ref<HTMLElement | null>(null);

/** Only the last assistant message can be mid-stream. */
const streamingId = computed(() =>
  chat.sending ? chat.messages[chat.messages.length - 1]?.id : undefined,
);

async function toBottom() {
  await nextTick();
  el.value?.scrollTo({ top: el.value.scrollHeight });
}

watch(() => chat.messages.length, toBottom);
watch(
  () => chat.messages[chat.messages.length - 1]?.text,
  () => {
    // Follow the stream only when the reader is already at the bottom.
    const node = el.value;
    if (!node) return;
    const atBottom = node.scrollHeight - node.scrollTop - node.clientHeight < 120;
    if (atBottom) void toBottom();
  },
);

const today = new Date().toLocaleDateString([], { weekday: "long" });
</script>

<template>
  <div ref="el" class="sk-thread" aria-live="polite">
    <div class="sk-day">{{ today }}</div>
    <Message
      v-for="m in chat.messages"
      :key="m.id"
      :message="m"
      :streaming="m.id === streamingId"
      @retry="retryLast()"
    />
  </div>
</template>
