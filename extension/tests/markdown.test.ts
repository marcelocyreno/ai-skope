import { describe, expect, it } from "vitest";
import { renderMarkdown, renderPlain, withCursor } from "@/pane/markdown";

describe("rendering an answer", () => {
  it("renders paragraphs, not one wall of text", () => {
    const html = renderMarkdown("First thought.\n\nSecond thought.");
    expect(html).toBe("<p>First thought.</p><p>Second thought.</p>");
  });

  it("keeps a bullet list as a list", () => {
    // This is the bug from the first real answer: single newlines collapsed
    // and every item ran together into one paragraph.
    const html = renderMarkdown("What you get:\n- 61 lessons\n- 91 practice questions\n- Notes");
    expect(html).toContain("<ul><li>61 lessons</li><li>91 practice questions</li><li>Notes</li></ul>");
    expect(html).not.toContain("- 61 lessons");
  });

  it("renders numbered lists as ordered lists", () => {
    expect(renderMarkdown("1. first\n2. second")).toBe("<ol><li>first</li><li>second</li></ol>");
  });

  it("nests one level", () => {
    const html = renderMarkdown("- outer\n  - inner\n- next");
    expect(html).toContain("<li>outer<ul><li>inner</li></ul></li>");
    expect(html).toContain("<li>next</li>");
  });

  it("continues an item that wraps onto the next line", () => {
    const html = renderMarkdown("- an item that\n  keeps going\n- another");
    expect(html).toContain("<li>an item that keeps going</li>");
  });

  it("renders bold, italic and inline code", () => {
    expect(renderMarkdown("**bold** and *italic* and `code()`")).toBe(
      "<p><strong>bold</strong> and <em>italic</em> and <code>code()</code></p>",
    );
    expect(renderMarkdown("__also bold__ and _also italic_")).toContain("<strong>also bold</strong>");
  });

  it("leaves formatting inside code spans alone", () => {
    expect(renderMarkdown("use `**literal**` here")).toContain("<code>**literal**</code>");
  });

  it("does not treat snake_case as italics", () => {
    expect(renderMarkdown("call some_function_name now")).toBe("<p>call some_function_name now</p>");
  });

  it("renders fenced code, and an unfinished fence while streaming", () => {
    // The language tag on the fence is not part of the code.
    expect(renderMarkdown("```go\nfmt.Println()\n```")).toBe("<pre><code>fmt.Println()</code></pre>");
    expect(renderMarkdown("```\nhalf a fen")).toBe("<pre><code>half a fen</code></pre>");
  });

  it("renders headings a couple of levels down", () => {
    expect(renderMarkdown("# Title")).toBe("<h3>Title</h3>");
    expect(renderMarkdown("## Section")).toBe("<h4>Section</h4>");
  });

  it("renders quotes and rules", () => {
    expect(renderMarkdown("> quoted\n> still quoted")).toBe("<blockquote>quoted still quoted</blockquote>");
    expect(renderMarkdown("---")).toBe("<hr>");
  });

  it("renders a table as a table, not a line of pipes", () => {
    // The bug from an Amazon checkout summary: the totals table fell through
    // to the paragraph branch and every row ran together into pipe soup.
    const html = renderMarkdown("| Item | Cost |\n|---|---|\n| Frete | R$ 0,00 |\n| Total | R$ 114,59 |");
    expect(html).toBe(
      "<div class=\"sk-tbl\"><table><thead><tr><th>Item</th><th>Cost</th></tr></thead>" +
        "<tbody><tr><td>Frete</td><td>R$ 0,00</td></tr>" +
        "<tr><td>Total</td><td>R$ 114,59</td></tr></tbody></table></div>",
    );
  });

  it("starts a table that follows a line of text directly", () => {
    const html = renderMarkdown("**Totals**\n| | |\n|---|---|\n| Frete | R$ 0,00 |");
    expect(html).toContain("<p><strong>Totals</strong></p>");
    expect(html).toContain("<td>Frete</td>");
    expect(html).not.toContain("|---|");
  });

  it("drops a header row the model left empty", () => {
    // A key/value table is often written with no header at all.
    const html = renderMarkdown("| | |\n|---|---|\n| Itens | R$ 259,99 |");
    expect(html).not.toContain("<thead>");
    expect(html).toContain("<tbody><tr><td>Itens</td><td>R$ 259,99</td></tr></tbody>");
  });

  it("aligns columns the delimiter row marks", () => {
    const html = renderMarkdown("| a | b | c |\n| :-- | :-: | --: |\n| 1 | 2 | 3 |");
    expect(html).toContain('<td>1</td><td class="sk-c">2</td><td class="sk-r">3</td>');
  });

  it("formats inside cells, and pads rows that are short", () => {
    const html = renderMarkdown("| a | b |\n|---|---|\n| **bold** |");
    expect(html).toContain("<td><strong>bold</strong></td><td></td>");
  });

  it("ends the table at the next block", () => {
    const html = renderMarkdown("| a |\n|---|\n| 1 |\nAfter the table.");
    expect(html).toContain("<td>1</td>");
    expect(html).toContain("<p>After the table.</p>");
  });

  it("does not mistake a stray pipe for a table", () => {
    expect(renderMarkdown("run `a | b`\nand then some")).toBe(
      "<p>run <code>a | b</code> and then some</p>",
    );
    expect(renderMarkdown("a | b\n---")).toBe("<p>a | b</p><hr>");
  });

  it("links only to schemes that cannot execute", () => {
    expect(renderMarkdown("[docs](https://example.com/x)")).toContain(
      '<a href="https://example.com/x" target="_blank" rel="noopener noreferrer">docs</a>',
    );
    // A javascript: link keeps its label and loses its teeth.
    const evil = renderMarkdown("[click](javascript:alert(1))");
    expect(evil).not.toContain("javascript:");
    expect(evil).toContain("click");
  });
});

describe("safety", () => {
  // The answer contains whatever the model read on the page, so markup in it
  // must never become markup here.
  const attacks = [
    '<img src=x onerror="alert(1)">',
    "<script>alert(1)</script>",
    '<a href="javascript:alert(1)">x</a>',
    "</p><svg onload=alert(1)>",
    '"><iframe src=evil>',
  ];

  for (const attack of attacks) {
    it(`escapes ${attack.slice(0, 24)}…`, () => {
      const html = renderMarkdown(attack);
      expect(html).not.toMatch(/<(script|img|svg|iframe|a\s+href="javascript)/i);
      expect(html).toContain("&lt;");
    });
  }

  it("escapes markup inside code fences too", () => {
    expect(renderMarkdown("```\n<script>alert(1)</script>\n```")).toContain("&lt;script&gt;");
  });

  it("escapes what the user typed, and keeps their line breaks", () => {
    expect(renderPlain("a <b> line\nand another")).toBe("a &lt;b&gt; line<br>and another");
  });
});

describe("withCursor", () => {
  const C = '<span class="sk-cursor"></span>';

  it("sits inside the last paragraph", () => {
    expect(withCursor("<p>one</p><p>two</p>", C)).toBe(`<p>one</p><p>two${C}</p>`);
  });

  it("sits inside the last list item, not after the list", () => {
    expect(withCursor("<ul><li>a</li><li>b</li></ul>", C)).toBe(`<ul><li>a</li><li>b${C}</li></ul>`);
  });

  it("sits inside the last cell, not below the table", () => {
    expect(withCursor('<div class="sk-tbl"><table><tbody><tr><td>a</td></tr></tbody></table></div>', C)).toBe(
      `<div class="sk-tbl"><table><tbody><tr><td>a${C}</td></tr></tbody></table></div>`,
    );
  });

  it("stands alone when there is nothing yet", () => {
    expect(withCursor("", C)).toBe(C);
  });
});
