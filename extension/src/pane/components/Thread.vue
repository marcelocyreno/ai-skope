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

/**
 * When the conversation started. It used to be today's weekday whatever the
 * chat — so one reopened from History said "Sunday" about last month.
 */
const day = computed(() => {
  const first = chat.messages[0]?.createdAt;
  if (!first) return "";
  const d = new Date(first);
  const now = new Date();
  const midnight = (x: Date) => new Date(x.getFullYear(), x.getMonth(), x.getDate()).getTime();
  const days = Math.round((midnight(now) - midnight(d)) / 86_400_000);
  if (days === 0) return "Today";
  if (days === 1) return "Yesterday";
  if (days < 7) return d.toLocaleDateString([], { weekday: "long" });
  return d.toLocaleDateString([], {
    month: "long",
    day: "numeric",
    year: d.getFullYear() === now.getFullYear() ? undefined : "numeric",
  });
});
</script>

<template>
  <div ref="el" class="sk-thread" aria-live="polite">
    <div v-if="day" class="sk-day">{{ day }}</div>
    <Message
      v-for="m in chat.messages"
      :key="m.id"
      :message="m"
      :streaming="m.id === streamingId"
      @retry="retryLast()"
    />
  </div>
</template>
