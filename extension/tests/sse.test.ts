import { describe, expect, it } from "vitest";
import { parseSSE, parseJSONFrames } from "@/api/sse";

/** Turns strings into the byte stream fetch would give us, one chunk at a time. */
function streamOf(...chunks: string[]): ReadableStream<Uint8Array> {
  const enc = new TextEncoder();
  return new ReadableStream({
    start(controller) {
      for (const c of chunks) controller.enqueue(enc.encode(c));
      controller.close();
    },
  });
}

async function collect<T>(gen: AsyncGenerator<T>): Promise<T[]> {
  const out: T[] = [];
  for await (const v of gen) out.push(v);
  return out;
}

describe("SSE parsing", () => {
  it("reads well-formed frames", async () => {
    const frames = await collect(
      parseSSE(streamOf('event: text.delta\ndata: {"text":"hi"}\n\nevent: turn.end\ndata: {}\n\n')),
    );
    expect(frames.map((f) => f.event)).toEqual(["text.delta", "turn.end"]);
    expect(frames[0].data).toBe('{"text":"hi"}');
  });

  it("reassembles a frame split across chunks", async () => {
    // This is the normal case for a streaming answer: the network splits
    // wherever it likes, including mid-field and mid-JSON.
    const frames = await collect(
      parseSSE(streamOf("event: text.de", 'lta\ndata: {"te', 'xt":"hello"}', "\n\n")),
    );
    expect(frames).toHaveLength(1);
    expect(frames[0].data).toBe('{"text":"hello"}');
  });

  it("ignores comments used as keep-alives", async () => {
    const frames = await collect(
      parseSSE(streamOf(": ping\n\nevent: tool\ndata: {}\n\n: ping\n\n")),
    );
    expect(frames).toHaveLength(1);
    expect(frames[0].event).toBe("tool");
  });

  it("joins multi-line data and defaults the event name", async () => {
    const frames = await collect(parseSSE(streamOf("data: one\ndata: two\n\n")));
    expect(frames[0]).toEqual({ event: "message", data: "one\ntwo" });
  });

  it("tolerates CRLF line endings", async () => {
    const frames = await collect(parseSSE(streamOf("event: x\r\ndata: 1\r\n\r\n")));
    expect(frames).toEqual([{ event: "x", data: "1" }]);
  });

  it("emits a trailing frame that never got its blank line", async () => {
    // A turn that ends abruptly should not lose its last token.
    const frames = await collect(parseSSE(streamOf('event: text.delta\ndata: {"text":"end"}')));
    expect(frames).toHaveLength(1);
  });

  it("skips frames whose JSON is malformed rather than ending the stream", async () => {
    const frames = await collect(
      parseJSONFrames<{ text?: string }>(
        streamOf("event: a\ndata: {bad json}\n\nevent: b\ndata: {\"text\":\"ok\"}\n\n"),
      ),
    );
    expect(frames).toHaveLength(1);
    expect(frames[0].data.text).toBe("ok");
  });

  it("stops when the caller aborts", async () => {
    const enc = new TextEncoder();
    const controller = new AbortController();
    const stream = new ReadableStream<Uint8Array>({
      start(c) {
        c.enqueue(enc.encode("event: a\ndata: 1\n\n"));
        // Never closed: only the abort ends this.
      },
    });
    const frames: string[] = [];
    const run = (async () => {
      for await (const f of parseSSE(stream, controller.signal)) {
        frames.push(f.event);
        controller.abort();
      }
    })();
    await run;
    expect(frames).toEqual(["a"]);
  });
});
