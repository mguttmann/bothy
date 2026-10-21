// SPDX-License-Identifier: Apache-2.0
/**
 * Time source of the hand layer.
 *
 * Every time decision of the SDK (token expiry, throttle, backoff, autocomplete guard, key list
 * age) goes through a {@link Clock}, so tests drive time with `FakeClock` of `./testing`
 * instead of sleeping (docs/DECISIONS.md D-M2-29-2). Times are milliseconds.
 *
 * @example
 * ```ts
 * const clock = systemClock;
 * await clock.sleep(10);
 * ```
 *
 * @module
 */

/** Wall time for token checks, a monotonic time for waits, and a sleep. */
export interface Clock {
  /** Returns the current wall time as Unix milliseconds. */
  now(): number;
  /** Returns a monotonic time in milliseconds for intervals. */
  monotonic(): number;
  /**
   * Waits for `ms` milliseconds; zero or negative resolves at the next turn.
   *
   * @param ms - The wait.
   * @param signal - Optional abort signal; an abort rejects with its reason.
   */
  sleep(ms: number, signal?: AbortSignal): Promise<void>;
}

/** The real clock: `Date.now`, `performance.now` and `setTimeout`. */
export const systemClock: Clock = Object.freeze({
  now: () => Date.now(),
  monotonic: () => performance.now(),
  sleep: (ms: number, signal?: AbortSignal) =>
    new Promise<void>((resolve, reject) => {
      if (signal?.aborted) {
        reject(signal.reason as Error);
        return;
      }
      const timer = setTimeout(
        () => {
          signal?.removeEventListener("abort", onAbort);
          resolve();
        },
        Math.max(0, ms),
      );
      const onAbort = (): void => {
        clearTimeout(timer);
        reject(signal?.reason as Error);
      };
      signal?.addEventListener("abort", onAbort, { once: true });
    }),
});
