/**
 * Server-sent events over fetch.
 *
 * EventSource cannot carry an Authorization header and cannot POST, and the
 * turn endpoint is a POST that streams — so the extension parses SSE itself.
 * The same parser serves both the turn stream and /v1/events.
 */

export interface SSEFrame {
  event: string;
  data: string;
}

/**
 * parseSSE turns a byte stream into frames. It is deliberately tolerant:
 * comment lines (": ping") are ignored, an event without a name defaults to
 * "message", and a frame split across chunk boundaries is reassembled.
 */
export async function* parseSSE(
  body: ReadableStream<Uint8Array>,
  signal?: AbortSignal,
): AsyncGenerator<SSEFrame> {
  const reader = body.getReader();
  const decoder = new TextDecoder();
  let buffer = "";

  const abort = () => reader.cancel().catch(() => {});
  signal?.addEventListener("abort", abort, { once: true });

  try {
    while (true) {
      const { done, value } = await reader.read();
      if (done) break;
      buffer += decoder.decode(value, { stream: true });

      // Frames are separated by a blank line; \r\n is tolerated.
      let sep: number;
      while ((sep = indexOfFrameEnd(buffer)) !== -1) {
        const raw = buffer.slice(0, sep);
        buffer = buffer.slice(sep).replace(/^(\r?\n){2}/, "");
        const frame = parseFrame(raw);
        if (frame) yield frame;
      }
    }
    const tail = parseFrame(buffer);
    if (tail) yield tail;
  } finally {
    signal?.removeEventListener("abort", abort);
    reader.releaseLock?.();
  }
}

function indexOfFrameEnd(buf: string): number {
  const a = buf.indexOf("\n\n");
  const b = buf.indexOf("\r\n\r\n");
  if (a === -1) return b;
  if (b === -1) return a;
  return Math.min(a, b);
}

function parseFrame(raw: string): SSEFrame | null {
  let event = "message";
  const data: string[] = [];
  for (const line of raw.split(/\r?\n/)) {
    if (line === "" || line.startsWith(":")) continue; // blank or comment
    const colon = line.indexOf(":");
    const field = colon === -1 ? line : line.slice(0, colon);
    let value = colon === -1 ? "" : line.slice(colon + 1);
    if (value.startsWith(" ")) value = value.slice(1);
    if (field === "event") event = value;
    else if (field === "data") data.push(value);
  }
  if (data.length === 0) return null;
  return { event, data: data.join("\n") };
}

/** parseJSONFrames decodes each frame's data as JSON, skipping bad frames. */
export async function* parseJSONFrames<T>(
  body: ReadableStream<Uint8Array>,
  signal?: AbortSignal,
): AsyncGenerator<{ event: string; data: T }> {
  for await (const frame of parseSSE(body, signal)) {
    try {
      yield { event: frame.event, data: JSON.parse(frame.data) as T };
    } catch {
      // A frame we cannot parse is skipped rather than ending the stream.
    }
  }
}
