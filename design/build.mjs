// AI Skope design kit — build.
// Inlines tokens, component CSS, the icon sprite and HTML partials into every
// preview so each file is self-contained (Claude Design renders cards in
// isolation, and file:// blocks cross-file SVG <use>).
//
//   node build.mjs          # builds components/, screens/, preview/
//
// Template syntax:  {{name}}  → src/partials/name.html  (recursive)
//                   {{tokens}} {{components}} {{sprite}} {{fonts}} are built-ins.
// Files under src/components and src/screens become full documents; files
// under src/preview stay fragments (the Artifact host supplies the shell).
// A first-line <!-- @dsCard … --> comment is preserved as the first line.

import { readFileSync, writeFileSync, readdirSync, mkdirSync } from "node:fs";
import { join, dirname } from "node:path";
import { fileURLToPath } from "node:url";

const root = dirname(fileURLToPath(import.meta.url));
const read = (p) => readFileSync(join(root, p), "utf8");

const FONTS = `<link rel="preconnect" href="https://fonts.googleapis.com">
<link rel="stylesheet" href="https://fonts.googleapis.com/css2?family=Manrope:wght@500;600;700&family=IBM+Plex+Sans:ital,wght@0,400;0,500;0,600;1,400&family=IBM+Plex+Mono:wght@400;500&display=swap">`;

const builtins = {
  tokens: () => read("tokens/tokens.css"),
  components: () => read("tokens/components.css"),
  sprite: () => read("icons/skope-icons.svg"),
  themes: () => read("tokens/themes.css"),
  fonts: () => FONTS,
  "addsrc-static": () => expand(read("src/partials/addsrc-inner.html"), "addsrc-static").replace(/ data-role="[^"]*"/g, "").replace(/ hidden>/g, ">").replace("Add a source</h2>", "Add a source</h2><span class=\"sk-topbar-spacer\"></span><button type=\"button\" class=\"sk-iconbtn\" data-dialog-close aria-label=\"Close\"><svg class=\"sk-ico\" aria-hidden=\"true\"><use href=\"#i-close\"/></svg></button>").replace('<button type="button" class="sk-iconbtn" aria-label="Back to settings"><svg class="sk-ico" aria-hidden="true"><use href="#i-arrow-left"/></svg></button>', "").replace("data-view=\"runtime\">", "data-view=\"runtime\" hidden>").replace("data-view=\"direct\">", "data-view=\"direct\" hidden>").replace('class="sk-btn ghost"', 'class="sk-btn ghost" data-dialog-close').replace('class="sk-btn primary"', 'class="sk-btn primary" data-dialog-close'),
  "settings-inner": () => { const l = read("src/partials/settings.html").trimEnd().split("\n"); return l.slice(1, -1).join("\n"); },
};

const partialCache = new Map();
function partial(name) {
  if (builtins[name]) return builtins[name]();
  if (!partialCache.has(name)) partialCache.set(name, expand(read(`src/partials/${name}.html`), name));
  return partialCache.get(name);
}
function expand(src, from = "?") {
  return src.replace(/\{\{([\w.-]+)\}\}/g, (_, name) => {
    try { return partial(name); }
    catch (e) { throw new Error(`Unknown partial {{${name}}} in ${from}`); }
  });
}

function buildDir(srcDir, outDir, { document }) {
  mkdirSync(join(root, outDir), { recursive: true });
  const files = readdirSync(join(root, srcDir)).filter((f) => f.endsWith(".html"));
  for (const f of files) {
    const raw = read(`${srcDir}/${f}`);
    let card = "", body = raw;
    const m = raw.match(/^<!--\s*@dsCard[^\n]*-->\n?/);
    if (m) { card = m[0].trimEnd() + "\n"; body = raw.slice(m[0].length); }
    let html = expand(body, f);
    if (document) {
      html = `${card}<!doctype html>\n<html lang="en">\n<head>\n<meta charset="utf-8">\n<meta name="viewport" content="width=device-width, initial-scale=1">\n${html}\n</html>\n`;
    } else {
      html = card + html;
    }
    writeFileSync(join(root, outDir, f), html);
    console.log(`  ${outDir}/${f}  ${(html.length / 1024).toFixed(1)} KB`);
  }
}

console.log("building AI Skope design kit");
buildDir("src/components", "components", { document: true });
buildDir("src/screens", "screens", { document: true });
buildDir("src/preview", "preview", { document: false });
console.log("done");
