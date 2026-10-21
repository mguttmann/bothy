// SPDX-License-Identifier: Apache-2.0
/**
 * A clock that moves only when a test calls {@link FakeClock.advance}.
 *
 * @module
 */

import type { Clock } from "../clock.js";

const SETTLE_TURNS = 20;

interface Sleeper {
  deadline: number;
  order: number;
  resolve: () => void;
  done: boolean;
}

/** Lets woken tasks run their synchronous parts before the time moves on. */
export async function settle(turns: number = SETTLE_TURNS): Promise<void> {
  for (let i = 0; i < turns; i++)
    await new Promise<void>((r) => setImmediate(r));
}

/**
 * A clock that moves only when a test calls {@link FakeClock.advance}.
 *
 * `now` starts at `start` (Unix milliseconds, default 2026-01-01T00:00:00Z) and `monotonic` at
 * zero; `sleep` waits until `advance` passes its deadline, and `advance` wakes sleepers in
 * deadline order, letting each woken task run before the time moves on.
 *
 * @example
 * ```ts
 * const clock = new FakeClock();
 * const done = clock.sleep(5_000);
 * await clock.advance(5_000);
 * await done;
 * clock.monotonic(); // 5000
 * ```
 */
export class FakeClock implements Clock {
  private readonly start: number;
  private offset = 0;
  private order = 0;
  private sleepersList: Sleeper[] = [];

  /**
   * Creates the clock.
   *
   * @param start - The wall time at monotonic zero, Unix milliseconds.
   */
  constructor(start: number = Date.UTC(2026, 0, 1)) {
    this.start = start;
  }

  /** Returns the fake wall time, Unix milliseconds. */
  now(): number {
    return this.start + this.offset;
  }

  /** Returns the milliseconds advanced since the start. */
  monotonic(): number {
    return this.offset;
  }

  /**
   * Waits until the clock has advanced by `ms`.
   *
   * @param ms - The wait; zero or negative yields once.
   * @param signal - Optional abort signal; an abort rejects with its reason.
   */
  sleep(ms: number, signal?: AbortSignal): Promise<void> {
    if (signal?.aborted) return Promise.reject(signal.reason as Error);
    if (ms <= 0) return new Promise<void>((r) => setImmediate(r));
    return new Promise<void>((resolve, reject) => {
      const entry: Sleeper = {
        deadline: this.offset + ms,
        order: this.order++,
        resolve: () => {
          entry.done = true;
          signal?.removeEventListener("abort", onAbort);
          resolve();
        },
        done: false,
      };
      const onAbort = (): void => {
        entry.done = true;
        reject(signal?.reason as Error);
      };
      signal?.addEventListener("abort", onAbort, { once: true });
      this.sleepersList.push(entry);
      this.sleepersList.sort(
        (a, b) => a.deadline - b.deadline || a.order - b.order,
      );
    });
  }

  /** The number of tasks currently waiting in `sleep`. */
  get sleepers(): number {
    return this.sleepersList.filter((s) => !s.done).length;
  }

  /**
   * Moves the time forward by `ms`, waking sleepers in deadline order.
   *
   * @param ms - How far to move; must not be negative.
   * @throws `RangeError` when `ms` is negative.
   */
  async advance(ms: number): Promise<void> {
    if (ms < 0) throw new RangeError("time only moves forward");
    const target = this.offset + ms;
    for (;;) {
      const next = this.sleepersList[0];
      if (next === undefined || next.deadline > target) break;
      this.sleepersList.shift();
      this.offset = Math.max(this.offset, next.deadline);
      if (!next.done) next.resolve();
      await settle();
    }
    this.offset = target;
    await settle();
  }
}
