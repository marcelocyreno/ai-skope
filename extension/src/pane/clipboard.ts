/** Writing to the clipboard, and saying so when the browser refuses. */
import { showToast } from "@/stores/toast";

/**
 * Clipboard access can be denied — a policy, a permission the user withheld,
 * a document that wasn't focused. It is worth telling them, because the glyph
 * swap otherwise claims a copy that never happened.
 */
export async function copyText(text: string): Promise<boolean> {
  try {
    await navigator.clipboard.writeText(text);
    return true;
  } catch {
    showToast("The browser wouldn't allow copying", { icon: "i-alert" });
    return false;
  }
}
