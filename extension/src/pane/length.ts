/**
 * The five answer-length stops. Four is neutral and sends no instruction at
 * all, so the control can be ignored entirely; the stop is kept in
 * chrome.storage.local, so the next message starts where the last left off
 * and the options page sees the same setting.
 */
import type { Conciseness } from "@/stores/storage";

export interface LengthStop {
  value: Conciseness;
  label: string;
}

export const LENGTH_STOPS: LengthStop[] = [
  { value: 1, label: "Extremely concise" },
  { value: 2, label: "Very concise" },
  { value: 3, label: "Concise" },
  { value: 4, label: "Normal" },
  { value: 5, label: "More detail" },
];

export function lengthStop(value: number): LengthStop {
  return LENGTH_STOPS.find((s) => s.value === value) ?? LENGTH_STOPS[3];
}
