/**
 * The server-side file picker: recent files, search across the index, and
 * browsing the folders the user allowed. Nothing here touches the filesystem
 * directly — the server owns what may be read.
 */
import { reactive } from "vue";
import { api } from "./connection";
import type { FileEntry, Folder } from "@/api/types";

interface FilesStore {
  query: string;
  recent: FileEntry[];
  results: FileEntry[];
  folders: Folder[];
  browsing: string;
  entries: FileEntry[];
  loading: boolean;
  error: string;
}

export const files = reactive<FilesStore>({
  query: "",
  recent: [],
  results: [],
  folders: [],
  browsing: "",
  entries: [],
  loading: false,
  error: "",
});

export async function openPicker(): Promise<void> {
  files.query = "";
  files.results = [];
  files.browsing = "";
  files.entries = [];
  files.error = "";
  files.loading = true;
  try {
    const [recent, folders] = await Promise.all([api().recentFiles(12), api().folders()]);
    files.recent = recent;
    files.folders = folders;
  } catch (err) {
    files.error = err instanceof Error ? err.message : String(err);
  } finally {
    files.loading = false;
  }
}

export async function search(q: string): Promise<void> {
  files.query = q;
  if (!q.trim()) {
    files.results = [];
    return;
  }
  files.loading = true;
  try {
    files.results = await api().searchFiles(q, 40);
  } catch (err) {
    files.error = err instanceof Error ? err.message : String(err);
  } finally {
    files.loading = false;
  }
}

export async function browse(path: string): Promise<void> {
  files.loading = true;
  files.error = "";
  try {
    files.entries = await api().browseFiles(path);
    files.browsing = path;
  } catch (err) {
    files.error = err instanceof Error ? err.message : String(err);
  } finally {
    files.loading = false;
  }
}
