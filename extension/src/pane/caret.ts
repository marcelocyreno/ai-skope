/**
 * Where the caret sits in the composer, for the keys that mean one thing in
 * the middle of a draft and another at its edge.
 *
 * Lines are counted in the text, not on screen. A long line that soft wraps is
 * still one line, so Up recalls history from anywhere inside it. Measuring the
 * wrapped rows instead would make the same keystroke do different things
 * depending on how wide the pane happens to be, which is not something the
 * user can see or reason about.
 */

/** Is there no line above the caret? */
export function atFirstLine(value: string, caret: number): boolean {
  return !value.slice(0, caret).includes("\n");
}

/** Is there no line below the caret? */
export function atLastLine(value: string, caret: number): boolean {
  return !value.slice(caret).includes("\n");
}
