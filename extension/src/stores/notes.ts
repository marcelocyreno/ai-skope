/** Page-linked notes, optionally quoting a selection. */
import { reactive } from "vue";
import { api } from "./connection";
import { page } from "./page";
import type { Note } from "@/api/types";

interface NotesStore {
  notes: Note[];
  query: string;
  draft: string;
  loading: boolean;
  lastDeleted: Note | null;
}

export const notes = reactive<NotesStore>({
  notes: [],
  query: "",
  draft: "",
  loading: false,
  lastDeleted: null,
});

export async function loadNotes(): Promise<void> {
  notes.loading = true;
  try {
    notes.notes = await api().notes({ q: notes.query || undefined });
  } finally {
    notes.loading = false;
  }
}

export async function addNote(body: string, quote?: string): Promise<void> {
  if (!body.trim() && !quote?.trim()) return;
  await api().createNote({
    url: page.url,
    title: page.title,
    favicon: page.favicon,
    quote,
    body: body.trim(),
  });
  notes.draft = "";
  await loadNotes();
}

export async function deleteNote(n: Note): Promise<void> {
  await api().deleteNote(n.id);
  notes.lastDeleted = n;
  await loadNotes();
}
