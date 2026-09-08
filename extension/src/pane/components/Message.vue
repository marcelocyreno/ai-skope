<script setup lang="ts">
/**
 * One turn in the transcript. User messages are bubbles on the right with
 * their context above; the assistant reads as a transcript, not a bubble.
 *
 * Under each finished message sits a meta row — the copy action, then the
 * timestamp — revealed by the same :hover / :focus-within the timestamp
 * already used. Fenced blocks inside an answer carry their own button.
 */
import { computed, onBeforeUnmount, ref } from "vue";
import type { Message } from "@/api/types";
import { renderMarkdown, renderPlain, withCursor } from "@/pane/markdown";
import { copyText } from "@/pane/clipboard";
import { announce } from "@/stores/announce";
import ContextChip from "./ContextChip.vue";
import Icon from "./Icon.vue";

const props = defineProps<{ message: Message; streaming?: boolean }>();
defineEmits<{ (e: "retry"): void }>();

/** How long the check stays up. Long enough to read, short enough to repeat. */
const COPIED_MS = 1400;

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

const isUser = computed(() => props.message.role === "user");
const copyLabel = computed(() => (isUser.value ? "Copy message" : "Copy answer"));

const copied = ref(false);
let timer: number | undefined;

/** The clipboard takes what was said: message.text, not the rendered HTML. */
async function copy() {
  if (!(await copyText(props.message.text))) return;
  copied.value = true;
  announce(isUser.value ? "Message copied" : "Answer copied");
  window.clearTimeout(timer);
  timer = window.setTimeout(() => (copied.value = false), COPIED_MS);
}

/**
 * A fenced block's button lives inside v-html, out of Vue's reach, so the click
 * is caught on the way up and the copied state is set on the node itself.
 */
async function onBodyClick(e: MouseEvent) {
  const button = (e.target as HTMLElement | null)?.closest<HTMLButtonElement>(".sk-copy-code");
  if (!button) return;
  // The code without its fence markers — what belongs in an editor.
  const code = button.parentElement?.querySelector("pre code")?.textContent ?? "";
  if (!code || !(await copyText(code))) return;
  announce("Code copied");
  flash(button);
}

function flash(button: HTMLButtonElement) {
  const glyph = button.querySelector("use");
  button.classList.add("is-copied");
  button.setAttribute("aria-label", "Copied");
  glyph?.setAttribute("href", "#i-check");
  window.setTimeout(() => {
    button.classList.remove("is-copied");
    button.setAttribute("aria-label", "Copy code");
    glyph?.setAttribute("href", "#i-copy");
  }, COPIED_MS);
}

onBeforeUnmount(() => window.clearTimeout(timer));
</script>

<template>
  <div v-if="message.role === 'user'" class="sk-msg user">
    <div v-if="message.context?.length" class="sk-msg-ctx">
      <ContextChip v-for="(item, i) in message.context" :key="i" :item="item" />
    </div>
    <div class="sk-bubble"><p v-html="asked"></p></div>
    <div class="sk-msg-meta">
      <button
        type="button"
        class="sk-iconbtn sm"
        :class="{ 'is-copied': copied }"
        :title="copied ? 'Copied' : copyLabel"
        :aria-label="copied ? 'Copied' : copyLabel"
        @click="copy()"
      >
        <Icon :id="copied ? 'i-check' : 'i-copy'" />
      </button>
      <span class="sk-time">{{ time }}</span>
    </div>
  </div>

  <div v-else class="sk-msg ai" :class="{ 'is-streaming': streaming }">
    <div class="sk-ai-head">
      <Icon id="i-reticle" />AI Skope<template v-if="message.model"> · {{ message.model }}</template>
    </div>

    <div v-for="(tool, i) in message.tools ?? []" :key="i" class="sk-tool">
      <span v-if="tool.state === 'running'" class="sk-spin" />
      <Icon v-else id="i-check" size="sm" />
      {{ tool.state === "running" ? "Reading" : "Read" }} <code>{{ tool.target || tool.name }}</code>
      <template v-if="tool.detail"> · {{ tool.detail }}</template>
    </div>

    <div v-if="message.text || streaming" class="sk-ai-body" v-html="answer" @click="onBodyClick"></div>

    <div v-if="message.error" class="sk-err">
      <Icon id="i-alert" />
      <span>{{ message.error }}</span>
      <button type="button" class="act" @click="$emit('retry')">Retry</button>
    </div>

    <div v-if="!streaming" class="sk-msg-meta">
      <button
        type="button"
        class="sk-iconbtn sm"
        :class="{ 'is-copied': copied }"
        :title="copied ? 'Copied' : copyLabel"
        :aria-label="copied ? 'Copied' : copyLabel"
        @click="copy()"
      >
        <Icon :id="copied ? 'i-check' : 'i-copy'" />
      </button>
      <span class="sk-time">{{ time }}</span>
    </div>
  </div>
</template>
