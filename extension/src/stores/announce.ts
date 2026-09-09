/**
 * A polite, invisible announcement.
 *
 * Some confirmations are too small for a toast — copying one message shouldn't
 * throw a card over the composer every time. The glyph swap says it to anyone
 * watching; this says it to anyone listening. Mutating a button's aria-label
 * would not: a name that changes under a screen reader is not an announcement.
 */
import { ref } from "vue";

export const announcement = ref("");

let timer: number | undefined;

export function announce(message: string): void {
  window.clearTimeout(timer);
  // Re-announcing the same string needs a clear in between, or the live region
  // never changes and nothing is read out.
  announcement.value = "";
  timer = window.setTimeout(() => {
    announcement.value = message;
    timer = window.setTimeout(() => (announcement.value = ""), 2000);
  }, 40);
}
