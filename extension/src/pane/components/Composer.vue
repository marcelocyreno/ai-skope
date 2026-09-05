<script setup lang="ts">
/**
 * Everything about the message being written: the context tray, the field, the
 * aiming tools, the model chip, and Send. Clear chat sits in the hint row —
 * near the field but far from Send, so a slip cannot discard a draft.
 */
import { ref, computed, nextTick } from "vue";
import { chat, removeContext, send, cancel } from "@/stores/chat";
import { connection } from "@/stores/connection";
import { modelStatus } from "@/stores/models";
import { page } from "@/stores/page";
import ContextChip from "./ContextChip.vue";
import ModelChip from "./ModelChip.vue";
import Icon from "./Icon.vue";

const props = defineProps<{ switcherOpen?: boolean; pickerOpen?: boolean }>();
const emit = defineEmits<{
  (e: "pick"): void;
  (e: "select"): void;
  (e: "files"): void;
  (e: "switcher"): void;
  (e: "clear"): void;
}>();

const field = ref<HTMLTextAreaElement | null>(null);

const blocked = computed(() => connection.state !== "online" || modelStatus.value === "offline");
const canSend = computed(() => chat.draft.trim().length > 0 && !chat.sending && !blocked.value);

const placeholder = computed(() => {
  if (connection.state === "offline") return "Waiting for the server to come back…";
  if (modelStatus.value === "offline") return "Waiting for the model to come back…";
  return "Ask about this page… pick an element or select text to add context";
});

/** The field starts at three rows and grows to eight. */
function autosize() {
  const el = field.value;
  if (!el) return;
  el.style.height = "auto";
  el.style.height = `${el.scrollHeight}px`;
}

async function submit() {
  if (!canSend.value) return;
  await send();
  await nextTick();
  autosize();
}

function onKeydown(e: KeyboardEvent) {
  if (e.key === "Enter" && !e.shiftKey) {
    e.preventDefault();
    void submit();
  }
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
        @input="autosize()"
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
      <span><kbd>⏎</kbd> send</span>
      <span><kbd>⇧⏎</kbd> new line</span>
      <span><kbd>⌘⇧K</kbd> pick element</span>
      <span class="grow" />
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
