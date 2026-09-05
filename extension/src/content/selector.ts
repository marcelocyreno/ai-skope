/**
 * Naming an element the user picked.
 *
 * The selector has two jobs: identify the element uniquely *now*, so the
 * server can be told exactly what was aimed at, and read well in a chip — the
 * design shows it as `article.pg-tier.featured`. It is built shortest-first
 * and every candidate is verified with querySelectorAll before it is returned.
 */

/** Classes that are generated rather than authored, and so mean nothing. */
function isNoiseClass(cls: string): boolean {
  if (!cls || cls.length > 40) return true;
  // CSS-modules and styled-components hashes: name_hash, sc-xyz123, css-1ab2c3
  if (/^(css|sc|jsx|emotion)-[a-z0-9]{4,}$/i.test(cls)) return true;
  if (/^[\w-]+__[\w-]+___[a-z0-9]{4,}$/i.test(cls)) return true;
  if (/^[\w-]*_[a-z0-9]{5,}$/i.test(cls)) return true;
  // Long hex-ish blobs
  if (/^[a-f0-9]{8,}$/i.test(cls)) return true;
  // Transient state classes change from moment to moment.
  if (/^(is-)?(active|hover|focus|open|visible|hidden|selected|current)$/i.test(cls)) return true;
  return false;
}

function stableClasses(el: Element): string[] {
  return Array.from(el.classList).filter((c) => !isNoiseClass(c)).slice(0, 3);
}

function esc(value: string): string {
  return typeof CSS !== "undefined" && CSS.escape ? CSS.escape(value) : value.replace(/([^\w-])/g, "\\$1");
}

/** A readable segment for one element: tag plus its meaningful classes. */
function segment(el: Element): string {
  const tag = el.tagName.toLowerCase();
  const classes = stableClasses(el);
  return tag + classes.map((c) => "." + esc(c)).join("");
}

/** Position among siblings sharing a tag, used only when needed. */
function nthOfType(el: Element): string {
  const parent = el.parentElement;
  if (!parent) return "";
  const sameTag = Array.from(parent.children).filter((c) => c.tagName === el.tagName);
  if (sameTag.length < 2) return "";
  return `:nth-of-type(${sameTag.indexOf(el) + 1})`;
}

function isUnique(root: Document | ShadowRoot, selector: string, el: Element): boolean {
  try {
    const found = root.querySelectorAll(selector);
    return found.length === 1 && found[0] === el;
  } catch {
    return false; // an invalid selector is never a candidate
  }
}

/**
 * cssSelector returns the shortest selector that uniquely identifies el,
 * falling back to an nth-of-type path when classes are not enough.
 */
export function cssSelector(el: Element, root: Document | ShadowRoot = document): string {
  if (!el || el === document.documentElement) return "html";
  if (el === document.body) return "body";

  // An id is the clearest name there is, when it is real and unique.
  const id = el.getAttribute("id");
  if (id && !/^\d/.test(id) && !isNoiseClass(id)) {
    const candidate = `#${esc(id)}`;
    if (isUnique(root, candidate, el)) return candidate;
  }

  // Then the element on its own, if its classes already single it out.
  const own = segment(el);
  if (isUnique(root, own, el)) return own;
  const ownNth = own + nthOfType(el);
  if (ownNth !== own && isUnique(root, ownNth, el)) return ownNth;

  // Otherwise walk up, adding ancestors until the path is unambiguous.
  let path = ownNth;
  let node: Element | null = el.parentElement;
  let depth = 0;
  while (node && depth < 8) {
    const ancestorId = node.getAttribute("id");
    if (ancestorId && !/^\d/.test(ancestorId) && !isNoiseClass(ancestorId)) {
      const candidate = `#${esc(ancestorId)} > ${path}`;
      if (isUnique(root, candidate, el)) return candidate;
      const loose = `#${esc(ancestorId)} ${path}`;
      if (isUnique(root, loose, el)) return loose;
    }
    path = `${segment(node)}${nthOfType(node)} > ${path}`;
    if (isUnique(root, path, el)) return path;
    node = node.parentElement;
    depth++;
  }
  return path;
}

/** A short label for the chip when a selector would be unreadably long. */
export function shortLabel(selector: string): string {
  const parts = selector.split(">").map((p) => p.trim());
  return parts.length <= 2 ? selector : `… > ${parts.slice(-2).join(" > ")}`;
}

/** The element's own visible text, collapsed and capped. */
export function elementText(el: Element, limit = 4000): string {
  const text = (el as HTMLElement).innerText ?? el.textContent ?? "";
  const collapsed = text.replace(/[ \t]+/g, " ").replace(/\n{3,}/g, "\n\n").trim();
  return collapsed.length > limit ? collapsed.slice(0, limit) + "…" : collapsed;
}

/** Trimmed markup: enough for the model to see structure, not the whole tree. */
export function elementHtml(el: Element, limit = 6000): string {
  const html = el.outerHTML ?? "";
  return html.length > limit ? html.slice(0, limit) + "\n<!-- truncated -->" : html;
}
