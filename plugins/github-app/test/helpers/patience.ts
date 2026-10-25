// SPDX-License-Identifier: Apache-2.0
/**
 * Waiting for a condition in tests without a wall-clock assumption, as the SDK's
 * `src/testing/patience.ts` does: the only upper bound is {@link PATIENCE_MS}, the value of
 * `quality.test.patience`, which only ends a test that would otherwise hang.
 *
 * @module
 */

import { performance } from "node:perf_hooks";
import { setTimeout as pause } from "node:timers/promises";

/** `quality.test.patience` in milliseconds (the Go tests' `testkit.Patience`). */
export const PATIENCE_MS = 180_000;

/**
 * Waits until `predicate` holds, with {@link PATIENCE_MS} as the only bound.
 *
 * @param predicate - The condition; it may return a promise.
 * @param what - Names the condition in the failure.
 * @throws `Error` when the condition did not hold within the patience.
 */
export async function eventually(
  predicate: () => unknown,
  what: string,
): Promise<void> {
  const deadline = performance.now() + PATIENCE_MS;
  for (;;) {
    if (await predicate()) return;
    if (performance.now() > deadline)
      throw new Error(`waited ${String(PATIENCE_MS / 1000)}s for ${what}`);
    await pause(5);
  }
}
