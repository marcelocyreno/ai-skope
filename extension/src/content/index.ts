/**
 * The content script: the only part of AI Skope that touches the page.
 *
 * It is injected on demand — never declaratively — and only after the user has
 * granted access to that origin. It draws its overlay inside a Shadow DOM, so
 * the page cannot restyle the reticle and we cannot restyle the page. It reads;
 * it never writes to the document.
 */
import overlayCss from "./overlay.css?raw";
import { cssSelector, elementHtml, elementText } from "./selector";

interface PickResult {
  type: "element";
  selector: string;
  text: string;
  html: string;
  rect: number[];
}

interface SelectionResult {
  type: "text";
  quote: string;
  before: string;
  after: string;
}

const HOST_ID = "ai-skope-overlay-host";

class Overlay {
  private host: HTMLElement;
  private root: ShadowRoot;
  private aim: HTMLElement;
  private tag: HTMLElement;
  private hint: HTMLElement;
  private toolbar: HTMLElement;

  constructor() {
    const existing = document.getElementById(HOST_ID);
    if (existing) existing.remove();

    this.host = document.createElement("div");
    this.host.id = HOST_ID;
    // The host itself must not affect layout or catch clicks.
    Object.assign(this.host.style, {
      position: "absolute",
      top: "0",
      left: "0",
      width: "0",
      height: "0",
      pointerEvents: "none",
      zIndex: "2147483647",
    } as Partial<CSSStyleDeclaration>);

    this.root = this.host.attachShadow({ mode: "open" });
    const style = document.createElement("style");
    style.textContent = overlayCss;
    this.root.append(style);

    this.aim = document.createElement("div");
    this.aim.className = "aim";
    this.aim.innerHTML = "<i></i><i></i><i></i><i></i>";
    this.tag = document.createElement("span");
    this.tag.className = "tag";
    this.aim.append(this.tag);

    this.hint = document.createElement("div");
    this.hint.className = "hint";
    this.hint.innerHTML = "Click an element to add it to the chat <kbd>esc</kbd> cancel";

    this.toolbar = document.createElement("div");
    this.toolbar.className = "toolbar";

    document.documentElement.append(this.host);
  }

  showAim(el: Element, selector: string): void {
    const r = el.getBoundingClientRect();
    Object.assign(this.aim.style, {
      top: `${r.top + window.scrollY}px`,
      left: `${r.left + window.scrollX}px`,
      width: `${r.width}px`,
      height: `${r.height}px`,
    });
    this.tag.innerHTML = "";
    const name = document.createElement("b");
    name.textContent = selector;
    const dim = document.createElement("span");
    dim.className = "dim";
    dim.textContent = `${Math.round(r.width)} × ${Math.round(r.height)}`;
    this.tag.append(name, dim);
    // Flip the label below the outline when there is no room above it.
    this.tag.classList.toggle("below", r.top < 34);
    if (!this.aim.isConnected) this.root.append(this.aim);
  }

  showHint(): void {
    if (!this.hint.isConnected) this.root.append(this.hint);
  }

  hideAim(): void {
    this.aim.remove();
    this.hint.remove();
  }

  showToolbar(rect: DOMRect, actions: { label: string; aim?: boolean; run: () => void }[]): void {
    this.toolbar.innerHTML = "";
    actions.forEach((a, i) => {
      if (i === 1) {
        const sep = document.createElement("span");
        sep.className = "sep";
        this.toolbar.append(sep);
      }
      const btn = document.createElement("button");
      btn.textContent = a.label;
      if (a.aim) btn.className = "aim-action";
      btn.addEventListener("mousedown", (e) => {
        e.preventDefault();
        e.stopPropagation();
        a.run();
      });
      this.toolbar.append(btn);
    });
    Object.assign(this.toolbar.style, {
      top: `${rect.top + window.scrollY - 46}px`,
      left: `${rect.left + rect.width / 2 + window.scrollX}px`,
      pointerEvents: "auto",
    });
    this.host.style.pointerEvents = "none";
    if (!this.toolbar.isConnected) this.root.append(this.toolbar);
  }

  hideToolbar(): void {
    this.toolbar.remove();
  }

  destroy(): void {
    this.host.remove();
  }
}

let overlay: Overlay | null = null;
function ui(): Overlay {
  if (!overlay) overlay = new Overlay();
  return overlay;
}

// ---- element picking -------------------------------------------------------

let cancelPicking: (() => void) | null = null;

function pickElement(): Promise<PickResult | null> {
  cancelPicking?.();
  return new Promise((resolve) => {
    let hovered: Element | null = null;
    let selector = "";
    const cursor = document.createElement("style");
    cursor.textContent = "*{cursor:crosshair!important}";
    document.head.append(cursor);
    ui().showHint();

    const move = (e: MouseEvent) => {
      const el = document.elementFromPoint(e.clientX, e.clientY);
      if (!el || el === hovered || el.id === HOST_ID) return;
      hovered = el;
      selector = cssSelector(el);
      ui().showAim(el, selector);
    };
    const click = (e: MouseEvent) => {
      e.preventDefault();
      e.stopPropagation();
      if (!hovered) return;
      const r = hovered.getBoundingClientRect();
      const result: PickResult = {
        type: "element",
        selector,
        text: elementText(hovered),
        html: elementHtml(hovered),
        rect: [Math.round(r.width), Math.round(r.height)],
      };
      finish(result);
    };
    const key = (e: KeyboardEvent) => {
      if (e.key === "Escape") {
        e.preventDefault();
        finish(null);
      }
    };
    const finish = (result: PickResult | null) => {
      document.removeEventListener("mousemove", move, true);
      document.removeEventListener("click", click, true);
      document.removeEventListener("keydown", key, true);
      cursor.remove();
      ui().hideAim();
      cancelPicking = null;
      resolve(result);
    };
    cancelPicking = () => finish(null);

    document.addEventListener("mousemove", move, true);
    document.addEventListener("click", click, true);
    document.addEventListener("keydown", key, true);
  });
}

// ---- text selection --------------------------------------------------------

function readSelection(): SelectionResult | null {
  const sel = window.getSelection();
  if (!sel || sel.isCollapsed) return null;
  const quote = sel.toString().trim();
  if (!quote) return null;

  // A little surrounding text helps the model place the quote on the page.
  let before = "";
  let after = "";
  const node = sel.anchorNode;
  const block = node?.parentElement?.closest("p, li, td, h1, h2, h3, h4, blockquote, section, div");
  if (block) {
    const full = (block as HTMLElement).innerText ?? "";
    const at = full.indexOf(quote);
    if (at >= 0) {
      before = full.slice(Math.max(0, at - 200), at).trim();
      after = full.slice(at + quote.length, at + quote.length + 200).trim();
    }
  }
  return { type: "text", quote, before, after };
}

/**
 * The floating toolbar the design shows on a selection. It reports the user's
 * choice back to the panel, which owns what happens next.
 */
function watchSelection(): void {
  document.addEventListener("mouseup", () => {
    window.setTimeout(() => {
      const result = readSelection();
      if (!result) {
        ui().hideToolbar();
        return;
      }
      const range = window.getSelection()?.getRangeAt(0);
      if (!range) return;
      const send = (action: string) => {
        ui().hideToolbar();
        void chrome.runtime.sendMessage({ kind: "skope:selection-action", action, selection: result });
      };
      ui().showToolbar(range.getBoundingClientRect(), [
        { label: "Add to chat", aim: true, run: () => send("add") },
        { label: "Ask…", run: () => send("ask") },
        { label: "Save note", run: () => send("note") },
      ]);
    }, 0);
  });
  document.addEventListener("mousedown", () => ui().hideToolbar());
}

// ---- readable page text ----------------------------------------------------

function pageText(limit = 40000): string {
  const clone = document.body.cloneNode(true) as HTMLElement;
  clone.querySelectorAll("script, style, noscript, svg, iframe, nav, footer").forEach((n) => n.remove());
  const text = (clone.innerText ?? clone.textContent ?? "")
    .replace(/[ \t]+/g, " ")
    .replace(/\n{3,}/g, "\n\n")
    .trim();
  return text.length > limit ? text.slice(0, limit) + "\n…" : text;
}

// ---- message handling ------------------------------------------------------

type Request =
  | { kind: "skope:pick" }
  | { kind: "skope:cancel" }
  | { kind: "skope:selection" }
  | { kind: "skope:pagetext" }
  | { kind: "skope:ping" };

// Guard against a second injection: executeScript runs the file again on every
// pick, and listeners would otherwise stack up.
declare global {
  interface Window {
    __aiSkopeContentReady?: boolean;
  }
}

if (!window.__aiSkopeContentReady) {
  window.__aiSkopeContentReady = true;
  watchSelection();

  chrome.runtime.onMessage.addListener((msg: Request, _sender, sendResponse) => {
    switch (msg?.kind) {
      case "skope:ping":
        sendResponse({ ok: true });
        return false;
      case "skope:pick":
        void pickElement().then(sendResponse);
        return true; // the reply comes later
      case "skope:cancel":
        cancelPicking?.();
        ui().hideToolbar();
        sendResponse({ ok: true });
        return false;
      case "skope:selection":
        sendResponse(readSelection());
        return false;
      case "skope:pagetext":
        sendResponse({ text: pageText() });
        return false;
      default:
        return false;
    }
  });

  window.addEventListener("pagehide", () => {
    overlay?.destroy();
    overlay = null;
  });
}
