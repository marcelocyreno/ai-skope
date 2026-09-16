import { describe, expect, it } from "vitest";
import { atFirstLine, atLastLine } from "@/pane/caret";

/** The caret's position, written as the text with a | where it sits. */
const at = (marked: string) => [marked.replace("|", ""), marked.indexOf("|")] as const;

describe("where the caret is in a draft", () => {
  it("an empty draft is both the first line and the last", () => {
    expect(atFirstLine("", 0)).toBe(true);
    expect(atLastLine("", 0)).toBe(true);
  });

  it("a single line is both, wherever the caret is in it", () => {
    for (const marked of ["|one line", "one |line", "one line|"]) {
      const [value, caret] = at(marked);
      expect(atFirstLine(value, caret)).toBe(true);
      expect(atLastLine(value, caret)).toBe(true);
    }
  });

  it("sees the lines above and below", () => {
    const [value, caret] = at("first\nsec|ond\nthird");
    expect(atFirstLine(value, caret)).toBe(false);
    expect(atLastLine(value, caret)).toBe(false);
  });

  it("the end of the first line is still the first line", () => {
    const [value, caret] = at("first|\nsecond");
    expect(atFirstLine(value, caret)).toBe(true);
    expect(atLastLine(value, caret)).toBe(false);
  });

  it("the start of the last line is already the last line", () => {
    const [value, caret] = at("first\n|second");
    expect(atFirstLine(value, caret)).toBe(false);
    expect(atLastLine(value, caret)).toBe(true);
  });

  it("a trailing newline leaves an empty last line of its own", () => {
    const [value, caret] = at("first\n|");
    expect(atLastLine(value, caret)).toBe(true);
    expect(atFirstLine(value, caret)).toBe(false);
  });
});
