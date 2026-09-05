/**
 * A small Markdown renderer for answers in the transcript.
 *
 * Models answer in Markdown, and the design's chat styles were written for it —
 * lists, inline code, bold. Rendering it as plain text turns a structured
 * answer into a wall of asterisks.
 *
 * This deliberately does not use a Markdown library. The text comes from a
 * model that has just been fed the contents of a web page, so it must never be
 * able to inject markup. Everything is HTML-escaped *first*, and the renderer
 * only ever emits tags it wrote itself: there is no path by which input becomes
 * markup, which is a stronger guarantee than parsing and then sanitising.
 *
 * The supported subset is what answers actually use: headings, paragraphs,
 * bullet and numbered lists (with one level of nesting), fenced and inline
 * code, bold, italic, links, block quotes and rules.
 */

/** Marks a code span while the rest of the inline formatting runs. Built from
 *  a control character, which cannot survive HTML-escaping of the input. */
const MARK = String.fromCharCode(0);

export function escapeHtml(s: string): string {
  return s
    .replace(/&/g, "&amp;")
    .replace(/</g, "&lt;")
    .replace(/>/g, "&gt;")
    .replace(/"/g, "&quot;");
}

/** Only schemes that cannot execute script. */
function safeHref(url: string): string | null {
  const trimmed = url.trim();
  return /^(https?:\/\/|mailto:)/i.test(trimmed) ? trimmed : null;
}

/** Inline formatting, applied to already-escaped text. */
function inline(text: string): string {
  // Code spans are set aside first so their contents are never re-formatted:
  // `**not bold**` inside backticks must stay literal.
  const codes: string[] = [];
  let s = text.replace(/`([^`\n]+)`/g, (_m, code: string) => {
    codes.push(code);
    return `${MARK}${codes.length - 1}${MARK}`;
  });

  s = s.replace(/\[([^\]\n]+)\]\(([^)\s]+)\)/g, (_m, label: string, url: string) => {
    const href = safeHref(url);
    // A link we will not follow is still worth reading, so keep its label.
    return href ? `<a href="${href}" target="_blank" rel="noopener noreferrer">${label}</a>` : label;
  });

  s = s.replace(/\*\*([^\n]+?)\*\*/g, "<strong>$1</strong>");
  s = s.replace(/__([^\n]+?)__/g, "<strong>$1</strong>");
  s = s.replace(/(^|[^*\w])\*([^*\n]+?)\*(?!\*)/g, "$1<em>$2</em>");
  s = s.replace(/(^|[^_\w])_([^_\n]+?)_(?![_\w])/g, "$1<em>$2</em>");

  return s.replace(
    new RegExp(`${MARK}(\\d+)${MARK}`, "g"),
    (_m, n: string) => `<code>${codes[Number(n)]}</code>`,
  );
}

const BULLET = /^(\s*)[-*+]\s+(.*)$/;
const NUMBER = /^(\s*)\d+[.)]\s+(.*)$/;
const HEADING = /^\s{0,3}(#{1,6})\s+(.*)$/;
// Block patterns run over already-escaped text, so a quote marker arrives as
// &gt; rather than >.
const QUOTE = /^\s{0,3}&gt;\s?(.*)$/;
const RULE = /^\s{0,3}([-*_])\s*(?:\1\s*){2,}$/;
const FENCE = /^\s*```/;

function isListItem(line: string): boolean {
  return BULLET.test(line) || NUMBER.test(line);
}

function isBlockStart(line: string): boolean {
  return (
    !line.trim() ||
    FENCE.test(line) ||
    HEADING.test(line) ||
    QUOTE.test(line) ||
    RULE.test(line) ||
    isListItem(line)
  );
}

/** Renders one list, including a single level of nesting. */
function renderList(lines: string[], start: number): [string, number] {
  const ordered = NUMBER.test(lines[start]) && !BULLET.test(lines[start]);
  const items: string[] = [];
  let i = start;
  let baseIndent = -1;

  while (i < lines.length) {
    const match = lines[i].match(BULLET) ?? lines[i].match(NUMBER);
    if (!match) {
      // A plain line directly under an item continues that item.
      if (items.length > 0 && lines[i].trim() && !isBlockStart(lines[i])) {
        items[items.length - 1] += " " + inline(lines[i].trim());
        i++;
        continue;
      }
      break;
    }

    const indent = match[1].length;
    if (baseIndent === -1) baseIndent = indent;

    if (indent > baseIndent && items.length > 0) {
      const [nested, next] = renderList(lines, i);
      items[items.length - 1] += nested;
      i = next;
      continue;
    }
    if (indent < baseIndent) break;

    items.push(inline(match[2].trim()));
    i++;
  }

  const tag = ordered ? "ol" : "ul";
  return [`<${tag}>${items.map((it) => `<li>${it}</li>`).join("")}</${tag}>`, i];
}

/** renderMarkdown turns an answer into safe HTML for the transcript. */
export function renderMarkdown(source: string): string {
  if (!source) return "";
  const lines = escapeHtml(source).split(/\r?\n/);
  const out: string[] = [];
  let i = 0;

  while (i < lines.length) {
    const line = lines[i];

    if (!line.trim()) {
      i++;
      continue;
    }

    if (FENCE.test(line)) {
      const body: string[] = [];
      i++;
      // An unterminated fence is normal mid-stream: render what has arrived.
      while (i < lines.length && !FENCE.test(lines[i])) {
        body.push(lines[i]);
        i++;
      }
      if (i < lines.length) i++; // closing fence
      out.push(`<pre><code>${body.join("\n")}</code></pre>`);
      continue;
    }

    const heading = line.match(HEADING);
    if (heading) {
      // Headings step down two levels: an h1 inside a 400px pane shouts.
      const level = Math.min(heading[1].length + 2, 6);
      out.push(`<h${level}>${inline(heading[2].trim())}</h${level}>`);
      i++;
      continue;
    }

    if (RULE.test(line)) {
      out.push("<hr>");
      i++;
      continue;
    }

    if (QUOTE.test(line)) {
      const body: string[] = [];
      while (i < lines.length && QUOTE.test(lines[i])) {
        body.push(lines[i].match(QUOTE)![1]);
        i++;
      }
      out.push(`<blockquote>${inline(body.join(" ").trim())}</blockquote>`);
      continue;
    }

    if (isListItem(line)) {
      const [html, next] = renderList(lines, i);
      out.push(html);
      i = next;
      continue;
    }

    const paragraph: string[] = [];
    while (i < lines.length && lines[i].trim() && !isBlockStart(lines[i])) {
      paragraph.push(lines[i].trim());
      i++;
    }
    if (paragraph.length) out.push(`<p>${inline(paragraph.join(" "))}</p>`);
  }

  return out.join("");
}

/** Plain text with its line breaks kept — for what the user typed. */
export function renderPlain(source: string): string {
  return escapeHtml(source).replace(/\r?\n/g, "<br>");
}

/**
 * Places the streaming cursor at the end of the last piece of text, rather than
 * on a line of its own below the answer.
 */
export function withCursor(html: string, cursor: string): string {
  const tail = /(<\/(?:p|li|h[1-6]|blockquote|code)>)((?:<\/(?:ul|ol|pre|blockquote)>)*)$/;
  return tail.test(html) ? html.replace(tail, `${cursor}$1$2`) : html + cursor;
}
