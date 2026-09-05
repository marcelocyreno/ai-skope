import { describe, expect, it } from "vitest";
import { chooseTab } from "@/stores/page";

const content = (id: number, url: string, active = false) => ({ id, url, active });
const pane = (id: number) => ({ id, url: "chrome-extension://abc/sidepanel.html", active: true });

describe("chooseTab", () => {
  it("describes the active tab", () => {
    const tabs = [content(1, "https://a.example"), content(2, "https://b.example", true)];
    expect(chooseTab(tabs, null)?.id).toBe(2);
  });

  it("never describes the pane itself", () => {
    const tabs = [content(1, "https://a.example"), pane(2)];
    expect(chooseTab(tabs, null)?.id).toBe(1);
  });

  it("falls back to the tab the user last looked at, not the last in the strip", () => {
    // The pane is active, so no content tab is: without the memory this would
    // answer about whichever tab happens to sit last.
    const tabs = [content(1, "https://a.example"), content(3, "https://c.example"), pane(2)];
    expect(chooseTab(tabs, 1)?.id).toBe(1);
  });

  it("prefers a real active tab over the remembered one", () => {
    const tabs = [content(1, "https://a.example"), content(3, "https://c.example", true)];
    expect(chooseTab(tabs, 1)?.id).toBe(3);
  });

  it("has nothing to describe when every tab is an extension page", () => {
    expect(chooseTab([pane(2)], null)).toBeNull();
  });

  it("ignores tabs whose URL Chrome withheld", () => {
    expect(chooseTab([{ id: 1 }, content(2, "https://b.example")], null)?.id).toBe(2);
  });
});
