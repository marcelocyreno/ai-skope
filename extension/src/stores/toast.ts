/** The small floating confirmation at the bottom of the pane. */
import { reactive } from "vue";

interface ToastStore {
  message: string;
  icon: string;
  undoLabel: string;
  onUndo: (() => void) | null;
}

export const toast = reactive<ToastStore>({ message: "", icon: "i-reticle", undoLabel: "", onUndo: null });

let timer: number | undefined;

export function showToast(
  message: string,
  opts: { icon?: string; undoLabel?: string; onUndo?: () => void; ms?: number } = {},
): void {
  window.clearTimeout(timer);
  toast.message = message;
  toast.icon = opts.icon ?? "i-reticle";
  toast.undoLabel = opts.undoLabel ?? "";
  toast.onUndo = opts.onUndo
    ? () => {
        opts.onUndo?.();
        dismissToast();
      }
    : null;
  timer = window.setTimeout(dismissToast, opts.ms ?? (opts.undoLabel ? 6000 : 2600));
}

export function dismissToast(): void {
  window.clearTimeout(timer);
  toast.message = "";
  toast.undoLabel = "";
  toast.onUndo = null;
}
