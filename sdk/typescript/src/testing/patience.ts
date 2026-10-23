// SPDX-License-Identifier: Apache-2.0
/**
 * Waiting for a condition in tests without a wall-clock assumption.
 *
 * A test waits for a state, never a fixed time or a fixed number of loop turns: on a loaded
 * machine or a slow CI runner a turn takes longer, so a bound by turns or by a few seconds fails
 * a correct program. The only upper bound is {@link PATIENCE_MS}, the value of
 * `quality.test.patience` (the Go tests' `testkit.Patience`, the Python SDK's `PATIENCE`), which
 * only ends a test that would otherwise hang (docs/decisions.d/M2-30.md D-M2-30-15).
 */

import { performance } from "node:perf_hooks";
import { setTimeout as pause } from "node:timers/promises";

/** `quality.test.patience` in milliseconds: the same 3m as `testkit.Patience`. */
export const PATIENCE_MS = 180_000;

/** The condition did not hold within {@link PATIENCE_MS}; the test hangs on a real defect. */
export class PatienceExceededError extends Error {
  /**
   * Creates the error.
   *
   * @param what - Names the condition that never held.
   */
  constructor(what: string) {
    super(`waited ${PATIENCE_MS / 1000}s for ${what}`);
    this.name = "PatienceExceededError";
  }
}

/** Options of {@link eventually}. */
export interface EventuallyOptions {
  /** Runs between two checks (for example a fake clock advance); default: one short pause. */
  readonly step?: () => unknown;
  /** Real pause between two checks in milliseconds; default 5. */
  readonly pauseMs?: number;
}

/**
 * Waits until `predicate` holds, with {@link PATIENCE_MS} of real time as the only bound.
 *
 * @param predicate - The condition; it may return a promise.
 * @param what - Names the condition in the failure.
 * @param options - What runs between two checks.
 * @throws {@link PatienceExceededError} When the condition did not hold within the patience.
 */
export async function eventually(
  predicate: () => unknown,
  what: string,
  options: EventuallyOptions = {},
): Promise<void> {
  const deadline = performance.now() + PATIENCE_MS;
  const pauseMs = options.pauseMs ?? 5;
  for (;;) {
    if (await predicate()) return;
    if (performance.now() > deadline) throw new PatienceExceededError(what);
    if (options.step !== undefined) await options.step();
    await pause(pauseMs);
  }
}

/**
 * Awaits `promise` with {@link PATIENCE_MS} as the only bound.
 *
 * @param promise - What to wait for.
 * @param what - Names it in the failure.
 * @returns Its result.
 * @throws {@link PatienceExceededError} When it did not settle within the patience.
 */
export async function withinPatience<T>(
  promise: Promise<T>,
  what: string,
): Promise<T> {
  const timer = new AbortController();
  const expired = pause(PATIENCE_MS, undefined, { signal: timer.signal }).then(
    () => {
      throw new PatienceExceededError(what);
    },
  );
  try {
    return await Promise.race([promise, expired]);
  } finally {
    timer.abort();
    expired.catch(() => undefined);
  }
}
