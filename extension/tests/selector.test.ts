import { describe, expect, it, beforeEach } from "vitest";
import { cssSelector, elementText, elementHtml, shortLabel } from "@/content/selector";

function mount(html: string): void {
  document.body.innerHTML = html;
}

/** Every selector this returns must actually find that one element. */
function expectResolves(el: Element): string {
  const sel = cssSelector(el);
  const found = document.querySelectorAll(sel);
  expect(found.length, `selector "${sel}" matched ${found.length} elements`).toBe(1);
  expect(found[0]).toBe(el);
  return sel;
}

describe("cssSelector", () => {
  beforeEach(() => mount(""));

  it("prefers a real id", () => {
    mount('<div id="pricing"><p>hi</p></div>');
    expect(cssSelector(document.querySelector("#pricing")!)).toBe("#pricing");
  });

  it("uses tag and classes when they are already unique", () => {
    mount('<section class="pg-tiers"><article class="pg-tier featured">A</article></section>');
    const el = document.querySelector(".featured")!;
    expect(expectResolves(el)).toBe("article.pg-tier.featured");
  });

  it("disambiguates siblings that look identical", () => {
    mount(`<section class="tiers">
      <article class="tier">A</article>
      <article class="tier">B</article>
      <article class="tier">C</article>
    </section>`);
    const els = [...document.querySelectorAll(".tier")];
    const selectors = els.map((el) => expectResolves(el));
    expect(new Set(selectors).size).toBe(3);
    expect(selectors[1]).toContain("nth-of-type(2)");
  });

  it("walks up until the path is unambiguous", () => {
    mount(`<div class="a"><ul><li><span class="x">one</span></li></ul></div>
           <div class="b"><ul><li><span class="x">two</span></li></ul></div>`);
    const second = document.querySelectorAll(".x")[1];
    const sel = expectResolves(second);
    expect(sel).toContain(">");
  });

  it("ignores generated class names", () => {
    // Hashed class names change on every build, so a selector built from them
    // would be worthless the moment the page is redeployed.
    mount('<div class="css-1ab2c3 Card_root__x7f2a promo">x</div>');
    const sel = cssSelector(document.querySelector(".promo")!);
    expect(sel).toContain(".promo");
    expect(sel).not.toContain("css-1ab2c3");
    expect(sel).not.toContain("Card_root__x7f2a");
  });

  it("ignores transient state classes", () => {
    mount('<button class="tab is-active">A</button><button class="tab">B</button>');
    const sel = cssSelector(document.querySelector(".is-active")!);
    expect(sel).not.toContain("is-active");
  });

  it("escapes awkward characters in class names", () => {
    mount('<div class="md:flex w-1/2 keep">x</div>');
    const el = document.querySelector(".keep")!;
    expect(() => document.querySelectorAll(cssSelector(el))).not.toThrow();
    expectResolves(el);
  });

  it("names the document body and root without inventing a path", () => {
    expect(cssSelector(document.body)).toBe("body");
    expect(cssSelector(document.documentElement)).toBe("html");
  });

  it("handles a deeply nested element", () => {
    mount(`<main><div><div><div><div><div><div><div><div><div>
      <span class="deep">found</span>
    </div></div></div></div></div></div></div></div></div></main>`);
    expectResolves(document.querySelector(".deep")!);
  });
});

describe("element capture", () => {
  it("collapses whitespace and caps long text", () => {
    mount("<p id=p>hello   world</p>");
    const el = document.getElementById("p")!;
    Object.defineProperty(el, "innerText", { value: "hello   world\n\n\n\nagain", configurable: true });
    expect(elementText(el)).toBe("hello world\n\nagain");
    Object.defineProperty(el, "innerText", { value: "x".repeat(5000), configurable: true });
    expect(elementText(el, 100)).toHaveLength(101); // 100 chars plus the ellipsis
  });

  it("truncates markup with a visible marker", () => {
    mount(`<div id="big">${"<span>x</span>".repeat(2000)}</div>`);
    const html = elementHtml(document.getElementById("big")!, 500);
    expect(html.length).toBeLessThan(600);
    expect(html).toContain("truncated");
  });

  it("shortens a long selector for the chip", () => {
    expect(shortLabel("a > b > c > d")).toBe("… > c > d");
    expect(shortLabel("article.tier")).toBe("article.tier");
  });
});
