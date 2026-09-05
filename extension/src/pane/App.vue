<script setup lang="ts">
/**
 * The AI Pane. Chrome's side panel is the right half of the split canvas, so
 * this document renders the pane itself: top bar, tabs, body, composer, and
 * the overlays that rise from the composer.
 */
import { ref, computed, onMounted, onUnmounted, watch } from "vue";
import { connection, initConnection, stopConnection } from "@/stores/connection";
import { loadModels } from "@/stores/models";
import { chat, openForCurrentPage, newChat, addContext, send, rememberPageConsent, pageConsentGiven, pageDecided } from "@/stores/chat";
import { loadNotes, addNote } from "@/stores/notes";
import { page, refreshActiveTab, watchActiveTab, pickElement, cancelPick, readSelection, syncContentScripts } from "@/stores/page";
import { showToast } from "@/stores/toast";
import type { ContextItem, FileEntry } from "@/api/types";

import TopBar from "./components/TopBar.vue";
import Tabs from "./components/Tabs.vue";
import Thread from "./components/Thread.vue";
import Composer from "./components/Composer.vue";
import ModelSwitcher from "./components/ModelSwitcher.vue";
import FilePicker from "./components/FilePicker.vue";
import StatusStrip from "./components/StatusStrip.vue";
import EmptyState from "./components/EmptyState.vue";
import History from "./components/History.vue";
import Notes from "./components/Notes.vue";
import QuickSettings from "./components/QuickSettings.vue";
import Pairing from "./components/Pairing.vue";
import Icon from "./components/Icon.vue";
import Toast from "./components/Toast.vue";

type Overlay = "none" | "switcher" | "files";
type Panel = "none" | "history" | "settings";

const tab = ref<"chat" | "notes">("chat");
const overlay = ref<Overlay>("none");
const panel = ref<Panel>("none");
const composer = ref<InstanceType<typeof Composer> | null>(null);
/** True while the pane is asking whether to include the page's text. */
const askingPage = ref(false);

const ready = computed(() => connection.state === "online");
const showChat = computed(() => tab.value === "chat");

let unwatchTab: (() => void) | null = null;

onMounted(async () => {
  await refreshActiveTab();
  void syncContentScripts();
  unwatchTab = watchActiveTab(() => {
    // A new page means a new conversation context; the old one is in History.
    if (ready.value) void openForCurrentPage();
  });

  await initConnection();
  if (ready.value) {
    await Promise.all([loadModels(), openForCurrentPage(), loadNotes()]);
  }
  chrome.runtime.onMessage.addListener(onRuntimeMessage);
  void drainPending();
});

onUnmounted(() => {
  unwatchTab?.();
  stopConnection();
  chrome.runtime.onMessage.removeListener(onRuntimeMessage);
});

// Once the connection comes up (or comes back), load what needs the server.
watch(ready, async (isReady) => {
  if (!isReady) return;
  await Promise.all([loadModels(), openForCurrentPage(), loadNotes()]);
});

/** The selection toolbar in the page reports the user's choice here. */
function onRuntimeMessage(msg: { kind?: string; action?: string; selection?: ContextItem }): undefined {
  if (msg?.kind !== "skope:selection-action" || !msg.selection) return;
  applySelectionAction(msg.action ?? "add", msg.selection);
  return;
}

function applySelectionAction(action: string, selection: ContextItem) {
  if (action === "note") {
    void addNote("", selection.quote);
    tab.value = "notes";
    showToast("Saved to Notes", { icon: "i-note" });
    return;
  }
  addContext(selection);
  showToast("Added the selection to context", { icon: "i-select-text" });
  if (action === "ask") composer.value?.focus();
}

/** A command or context-menu click may have opened the panel; act on it. */
async function drainPending() {
  const got = await chrome.storage.session.get(["pendingCommand", "pendingAction"]);
  await chrome.storage.session.remove(["pendingCommand", "pendingAction"]);
  const cmd = got.pendingCommand as { command: string } | undefined;
  const action = got.pendingAction as { action: string; selection: ContextItem } | undefined;
  if (action?.selection) applySelectionAction(action.action, action.selection);
  if (cmd?.command === "pick-element") await doPick();
  if (cmd?.command === "add-selection") await doSelection();
  if (cmd?.command === "new-chat") await doNewChat();
}

async function doPick() {
  overlay.value = "none";
  const item = await pickElement();
  if (!item) return;
  addContext(item);
  showToast(`Added <code>${item.selector}</code> to context`, { icon: "i-reticle" });
  composer.value?.focus();
}

async function doSelection() {
  const item = await readSelection();
  if (!item) {
    showToast("Select some text on the page first", { icon: "i-select-text" });
    return;
  }
  addContext(item);
  showToast("Added the selection to context", { icon: "i-select-text" });
  composer.value?.focus();
}

function chooseFile(f: FileEntry) {
  overlay.value = "none";
  addContext({ type: "file", path: f.path, title: `${f.dir} · ${Math.max(1, Math.round(f.size / 1024))} KB` });
  showToast(`Added <code>${f.name}</code>`, { icon: "i-folder" });
  composer.value?.focus();
}

async function doNewChat() {
  if (chat.messages.length === 0) return;
  await newChat();
  panel.value = "none";
  showToast("New chat — the last one is in History", { icon: "i-history" });
}

/**
 * Summarising is an explicit request for the whole page, so clicking it is the
 * consent — there is nothing to ask about. It still respects "never".
 */
async function summarize() {
  if (connection.settings?.pageAccess === "never") {
    showToast("Page access is set to Never, so the page can't be summarized", { icon: "i-shield" });
    return;
  }
  chat.draft = "Summarize this page";
  rememberPageConsent(page.url);
  await submitMessage({ includePage: true });
}

/**
 * The composer asks to send. With page access on Ask — the default — a
 * question carrying no context would reach the model with nothing to go on, so
 * the pane asks first rather than sending a request that cannot be answered.
 */
async function submitMessage(opts: { includePage?: boolean } = {}) {
  const access = connection.settings?.pageAccess ?? "ask";
  const hasContext = chat.tray.length > 0;
  const decided = opts.includePage !== undefined;

  if (!decided && access === "ask" && !hasContext && !pageDecided(page.url) && page.url) {
    askingPage.value = true;
    return;
  }
  askingPage.value = false;
  await send({ includePage: opts.includePage ?? pageConsentGiven(page.url) });
}

async function answerWithPage() {
  rememberPageConsent(page.url);
  await submitMessage({ includePage: true });
}

async function answerWithoutPage() {
  // Remember the no as well, so the question is asked once per page.
  rememberPageConsent(page.url, false);
  await submitMessage({ includePage: false });
}

function openOptions(section: string) {
  void chrome.tabs.create({ url: chrome.runtime.getURL(`options.html#${section}`) });
}

/** Esc closes whatever is on top, including an armed picker. */
function onKeydown(e: KeyboardEvent) {
  if (e.key !== "Escape") return;
  if (page.picking) void cancelPick();
  else if (overlay.value !== "none") overlay.value = "none";
  else if (panel.value !== "none") panel.value = "none";
}
</script>

<template>
  <div class="sk-pane" @keydown="onKeydown">
    <TopBar
      @new-chat="doNewChat()"
      @history="panel = panel === 'history' ? 'none' : 'history'"
      @settings="panel = panel === 'settings' ? 'none' : 'settings'"
    />

    <template v-if="ready">
      <Tabs v-model="tab" />

      <div class="sk-body">
        <section v-show="showChat" class="sk-view">
          <Thread v-if="chat.messages.length" />
          <EmptyState
            v-else
            @summarize="summarize()"
            @pick="doPick()"
            @file="overlay = 'files'"
            @select="doSelection()"
          />
        </section>
        <section v-show="!showChat" class="sk-view sk-notes">
          <Notes />
        </section>
      </div>

      <StatusStrip v-if="showChat" @switch-model="overlay = 'switcher'" />

      <div v-if="showChat && askingPage" class="sk-strip is-ask" role="status">
        <Icon id="i-shield" />
        <span>Send this page's text with your question?</span>
        <button type="button" class="act" @click="answerWithPage()">Include page</button>
        <button type="button" class="act plain" @click="answerWithoutPage()">Without it</button>
      </div>

      <Composer
        v-if="showChat"
        ref="composer"
        @submit="submitMessage()"
        :switcher-open="overlay === 'switcher'"
        :picker-open="overlay === 'files'"
        @pick="doPick()"
        @select="doSelection()"
        @files="overlay = overlay === 'files' ? 'none' : 'files'"
        @switcher="overlay = overlay === 'switcher' ? 'none' : 'switcher'"
        @clear="doNewChat()"
      />

      <div v-if="overlay !== 'none'" class="sk-scrim" @click="overlay = 'none'" />
      <div v-if="overlay === 'switcher'" class="sk-anchor" style="left: 8px; right: 8px; bottom: 118px">
        <ModelSwitcher @close="overlay = 'none'" @manage="openOptions('providers')" />
      </div>
      <div v-if="overlay === 'files'" class="sk-anchor" style="left: 8px; right: 8px; bottom: 118px">
        <FilePicker @choose="chooseFile" @close="overlay = 'none'" @manage="openOptions('folders')" />
      </div>

      <History v-if="panel === 'history'" @close="panel = 'none'" @new-chat="doNewChat()" />
      <QuickSettings v-if="panel === 'settings'" @close="panel = 'none'" @options="openOptions" />
    </template>

    <Pairing v-else />

    <Toast />
  </div>
</template>
