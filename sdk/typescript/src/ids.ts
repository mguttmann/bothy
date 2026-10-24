// SPDX-License-Identifier: Apache-2.0
/**
 * UUIDv7 for the identifiers the adapter chooses (RFC 9562 section 5.7).
 *
 * `client_msg_id`, `Idempotency-Key`, DPoP `jti` and run step IDs are UUIDv7: time ordered, so a
 * server index stays compact, and monotonic within one millisecond of one process (RFC 9562 6.2,
 * method 1: a 12-bit counter in `rand_a` seeded randomly each millisecond). Uses Web Crypto
 * only, so the browser-safe entry `./core` may export it.
 *
 * @example
 * ```ts
 * const a = uuidv7();
 * const b = uuidv7();
 * a < b; // true
 * ```
 *
 * @module
 */

const COUNTER_MAX = 0xfff;
let lastMs = -1;
let counter = 0;

function randomBits(bytes: number): Uint8Array {
  const out = new Uint8Array(bytes);
  globalThis.crypto.getRandomValues(out);
  return out;
}

function randomCounter(): number {
  const b = randomBits(2);
  // 11 bits: a random start in the lower half leaves room to count within the millisecond.
  return (((b[0] ?? 0) << 8) | (b[1] ?? 0)) & 0x7ff;
}

/**
 * Returns a new UUIDv7 in its canonical lower-case form, strictly greater than every earlier
 * one of this process.
 *
 * @returns The UUID (version 7, variant RFC 9562).
 */
export function uuidv7(): string {
  let ms = Date.now();
  if (ms <= lastMs) {
    // Same (or an earlier, after a clock step back) millisecond: count on from the last value
    // so the order holds; an exhausted counter borrows the next millisecond.
    ms = lastMs;
    counter += 1;
    if (counter > COUNTER_MAX) {
      ms += 1;
      counter = randomCounter();
    }
  } else {
    counter = randomCounter();
  }
  lastMs = ms;
  const bytes = new Uint8Array(16);
  let t = ms;
  for (let i = 5; i >= 0; i--) {
    bytes[i] = t % 256;
    t = Math.floor(t / 256);
  }
  bytes[6] = 0x70 | ((counter >> 8) & 0x0f);
  bytes[7] = counter & 0xff;
  const rand = randomBits(8);
  for (let i = 0; i < 8; i++) bytes[8 + i] = rand[i] ?? 0;
  bytes[8] = ((bytes[8] ?? 0) & 0x3f) | 0x80;
  const hex = Array.from(bytes, (b) => b.toString(16).padStart(2, "0")).join(
    "",
  );
  return `${hex.slice(0, 8)}-${hex.slice(8, 12)}-${hex.slice(12, 16)}-${hex.slice(16, 20)}-${hex.slice(20)}`;
}
