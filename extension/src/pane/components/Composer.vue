<script setup lang="ts">
/**
 * Everything about the message being written: the context tray, the field, the
 * aiming tools, the model chip, and Send. Clear chat sits in the hint row —
 * near the field but far from Send, so a slip cannot discard a draft.
 */
import { ref, computed, nextTick } from "vue";
import { chat, removeContext, cancel, recallPrevious, recallNext, leaveHistory } from "@/stores/chat";
import { connection } from "@/stores/connection";
import { modelStatus } from "@/stores/models";
import { page } from "@/stores/page";
import { showToast } from "@/stores/toast";
import { saveSettings, CONCISENESS_NORMAL, type Conciseness } from "@/stores/storage";
import { copyText } from "@/pane/clipboard";
import { chatMarkdown } from "@/pane/transcript";
import { atFirstLine, atLastLine } from "@/pane/caret";
import ContextChip from "./ContextChip.vue";
import ModelChip from "./ModelChip.vue";
import Icon from "./Icon.vue";

const props = defineProps<{ switcherOpen?: boolean; pickerOpen?: boolean }>();
const emit = defineEmits<{
  (e: "submit"): void;
  (e: "pick"): void;
  (e: "select"): void;
  (e: "files"): void;
  (e: "switcher"): void;
  (e: "clear"): void;
}>();

const field = ref<HTMLTextAreaElement | null>(null);

const blocked = computed(() => connection.state !== "online" || modelStatus.value === "offline");
const canSend = computed(() => chat.draft.trim().length > 0 && !chat.sending && !blocked.value);

/**
 * Answer length, the other half of what the model chip offers: effort is how
 * hard the model thinks, this is how much it says. Four is neutral and sends
 * no instruction at all, so the row can be ignored entirely.
 *
 * The stop is chosen per message but kept in chrome.storage.local, which is
 * therefore also the value on screen — the next message starts where the last
 * one left off, and the options page sees the same setting.
 */
const stops: { value: Conciseness; label: string }[] = [
  { value: 1, label: "Extremely concise" },
  { value: 2, label: "Very concise" },
  { value: 3, label: "Concise" },
  { value: 4, label: "Normal" },
  { value: 5, label: "More detail" },
];

const conciseness = computed(() => connection.settings?.conciseness ?? CONCISENESS_NORMAL);
const setConciseness = (value: Conciseness) => void saveSettings({ conciseness: value });

const placeholder = computed(() => {
  if (connection.state === "offline") return "Waiting for the server to come back…";
  if (modelStatus.value === "offline") return "Waiting for the model to come back…";
  return "Ask about this page… pick an element or select text to add context";
});

/**
 * The whole conversation as Markdown. Chat-level actions live here rather than
 * in the top bar, which holds navigation only. This is the one copy that raises
 * a toast: what it took is mostly off-screen, so nothing else would say so.
 */
async function copyChat() {
  if (chat.messages.length === 0) return;
  if (!(await copyText(chatMarkdown(chat.chat, chat.messages)))) return;
  showToast("Conversation copied", { icon: "i-copy" });
}

/** The field starts at three rows and grows to eight. */
function autosize() {
  const el = field.value;
  if (!el) return;
  el.style.height = "auto";
  el.style.height = `${el.scrollHeight}px`;
}

async function submit() {
  if (!canSend.value) return;
  emit("submit");
  await nextTick();
  autosize();
}

function onKeydown(e: KeyboardEvent) {
  if (e.key === "Enter" && !e.shiftKey) {
    e.preventDefault();
    void submit();
    return;
  }
  if (e.key === "ArrowUp" || e.key === "ArrowDown") void onArrow(e);
}

/**
 * Up and Down walk the sent messages once the caret has nowhere left to go in
 * that direction — a shell's rule, and Claude Code's. See caret.ts for what
 * counts as the first and last line.
 */
async function onArrow(e: KeyboardEvent) {
  const el = field.value;
  if (!el) return;
  // A modifier or a live selection means the user is editing or navigating on
  // purpose; neither is a request for history.
  if (e.altKey || e.ctrlKey || e.metaKey || e.shiftKey) return;
  if (el.selectionStart !== el.selectionEnd) return;

  const up = e.key === "ArrowUp";
  const atEdge = up
    ? atFirstLine(el.value, el.selectionStart)
    : atLastLine(el.value, el.selectionEnd);
  if (!atEdge) return;

  const recalled = up ? recallPrevious() : recallNext();
  if (recalled === null) return; // nothing to recall: let the caret move

  e.preventDefault();
  chat.draft = recalled;
  await nextTick();
  autosize();
  el.setSelectionRange(recalled.length, recalled.length);
}

/** Typing is the end of recalling: what is in the field is the draft again. */
function onInput() {
  leaveHistory();
  autosize();
}

defineExpose({ focus: () => field.value?.focus() });
</script>

<template>
  <form class="sk-composer" autocomplete="off" @submit.prevent="submit()">
    <div v-if="chat.tray.length" class="sk-tray">
      <ContextChip
        v-for="(item, i) in chat.tray"
        :key="i"
        :item="item"
        removable
        @remove="removeContext(i)"
      />
    </div>

    <div class="sk-field">
      <textarea
        ref="field"
        v-model="chat.draft"
        rows="3"
        :placeholder="placeholder"
        :disabled="blocked"
        aria-label="Message"
        @input="onInput()"
        @keydown="onKeydown"
      />
      <div class="sk-toolrow">
        <button
          type="button"
          class="sk-iconbtn"
          :aria-pressed="page.picking"
          title="Pick element  ⌘⇧K"
          aria-label="Pick an element from the page"
          @click="emit('pick')"
        >
          <Icon id="i-reticle" />
        </button>
        <button
          type="button"
          class="sk-iconbtn"
          title="Add the selected text  ⌘⇧S"
          aria-label="Add the selected text"
          @click="emit('select')"
        >
          <Icon id="i-select-text" />
        </button>
        <button
          type="button"
          class="sk-iconbtn"
          :aria-pressed="props.pickerOpen"
          title="Add a file from this computer"
          aria-label="Add a file"
          @click="emit('files')"
        >
          <Icon id="i-folder" />
        </button>
        <ModelChip :expanded="props.switcherOpen" @open="emit('switcher')" />
        <span class="sk-seg mini sk-conc" role="group" aria-label="Answer length">
          <Icon id="i-verbosity" />
          <button
            v-for="stop in stops"
            :key="stop.value"
            type="button"
            :aria-pressed="conciseness === stop.value"
            :aria-label="stop.label"
            :title="`Answer length: ${stop.label}`"
            @click="setConciseness(stop.value)"
          >
            {{ stop.value }}
          </button>
        </span>
        <span class="grow" />
        <button
          v-if="chat.sending"
          type="button"
          class="sk-send"
          aria-label="Stop"
          style="background: var(--surface-3); color: var(--ink)"
          @click="cancel()"
        >
          <Icon id="i-close" />
        </button>
        <button v-else type="submit" class="sk-send" :disabled="!canSend" aria-label="Send">
          <Icon id="i-send" />
        </button>
      </div>
    </div>

    <div class="sk-hint">
      <span class="grow" />
      <button
        type="button"
        class="clear"
        :disabled="chat.messages.length === 0"
        title="Copy the whole conversation as Markdown"
        @click="copyChat()"
      >
        <Icon id="i-copy" size="sm" />Copy chat
      </button>
      <button
        type="button"
        class="clear"
        :disabled="chat.messages.length === 0"
        @click="emit('clear')"
      >
        <Icon id="i-chat-clear" size="sm" />Clear chat
      </button>
    </div>
  </form>
</template>
