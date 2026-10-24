// SPDX-License-Identifier: Apache-2.0
/**
 * Faults the FakeServer injects into one operation (docs/spec/09-agents.md 16.1).
 *
 * A {@link Fault} describes what goes wrong; `FakeServer.inject(target, fault, times)` applies it
 * to the next `times` calls of an `operationId` or a gateway op:
 *
 * - {@link Fault.storedThen502}: the operation runs and stores its effect, then the answer is
 *   `502` with a problem body, so a client that retries must not create a duplicate.
 * - {@link Fault.storedLate}: the answer comes after `delayMs` of the FakeServer's clock and the
 *   effect is stored at the end of that wait.
 * - {@link Fault.rateLimited}: `429 rate_limited` with `Retry-After`; nothing is stored.
 * - {@link Fault.disconnect}: the connection is dropped without an answer; nothing is stored. On
 *   a gateway op the WebSocket connection is dropped.
 *
 * The kinds carry the names of the Python SDK (`stored_then_502`, `stored_late`,
 * `rate_limited`, `disconnect`); the brief's names `store_then_502`, `store_late`, `rate_limit`
 * and `drop_connection` mean the same four faults.
 *
 * @example
 * ```ts
 * const fault = Fault.rateLimited(2);
 * [fault.kind, fault.retryAfter]; // ["rate_limited", 2]
 * ```
 *
 * @module
 */

/** The four faults of the FakeServer. */
export const FaultKind = Object.freeze({
  /** Run and store, then answer `502`. */
  STORED_THEN_502: "stored_then_502",
  /** Answer and store after a delay of the clock. */
  STORED_LATE: "stored_late",
  /** Answer `429 rate_limited` with `Retry-After`. */
  RATE_LIMITED: "rate_limited",
  /** Drop the connection without an answer. */
  DISCONNECT: "disconnect",
} as const);

/** One of the values of {@link FaultKind}. */
export type FaultKind = (typeof FaultKind)[keyof typeof FaultKind];

/** One fault; build it with the static methods, nothing else is accepted by `inject`. */
export class Fault {
  /** What goes wrong. */
  readonly kind: FaultKind;
  /** Milliseconds of the FakeServer's clock before a `stored_late` answer. */
  readonly delayMs: number;
  /** Whole seconds of the `Retry-After` header of `rate_limited` (the header's unit). */
  readonly retryAfter: number;

  private constructor(kind: FaultKind, delayMs = 0, retryAfter = 0) {
    this.kind = kind;
    this.delayMs = delayMs;
    this.retryAfter = retryAfter;
    Object.freeze(this);
  }

  /**
   * Runs and stores the operation, then answers `502` with a problem body.
   *
   * @returns The fault.
   */
  static storedThen502(): Fault {
    return new Fault(FaultKind.STORED_THEN_502);
  }

  /**
   * Answers after `delayMs` of the clock and stores at the end of the wait.
   *
   * @param delayMs - The wait in milliseconds; must not be negative.
   * @returns The fault.
   * @throws `RangeError` when `delayMs` is negative or not a number.
   */
  static storedLate(delayMs: number): Fault {
    if (!(delayMs >= 0)) throw new RangeError("delay must not be negative");
    return new Fault(FaultKind.STORED_LATE, delayMs);
  }

  /**
   * Answers `429 rate_limited` with `Retry-After` and stores nothing.
   *
   * @param retryAfter - Whole seconds for the `Retry-After` header; must not be negative.
   * @returns The fault.
   * @throws `RangeError` when `retryAfter` is negative or not an integer.
   */
  static rateLimited(retryAfter: number): Fault {
    if (!Number.isInteger(retryAfter) || retryAfter < 0)
      throw new RangeError("retryAfter must be a whole, non-negative number");
    return new Fault(FaultKind.RATE_LIMITED, 0, retryAfter);
  }

  /**
   * Drops the connection without an answer and stores nothing.
   *
   * @returns The fault.
   */
  static disconnect(): Fault {
    return new Fault(FaultKind.DISCONNECT);
  }
}
