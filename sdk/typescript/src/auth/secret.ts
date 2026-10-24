// SPDX-License-Identifier: Apache-2.0
/**
 * A secret string that never renders its value (SEC-LOG-2).
 *
 * @module
 */

const REDACTED = "[redacted]";
const VALUE = Symbol("secret");

/**
 * A secret string (API token, invocation token) that never renders its value.
 *
 * `String(secret)`, template literals, `JSON.stringify` and `util.inspect` all give
 * `[redacted]`; {@link Secret.reveal} returns the value for the one place that must send it.
 *
 * @example
 * ```ts
 * const token = new Secret("example-token");
 * `${token}`; // "[redacted]"
 * token.reveal(); // "example-token"
 * ```
 */
export class Secret {
  private readonly [VALUE]: string;

  /**
   * Wraps a secret value.
   *
   * @param value - The secret text; must not be empty.
   * @throws `TypeError` when the value is empty.
   */
  constructor(value: string) {
    if (!value) throw new TypeError("a secret must not be empty");
    this[VALUE] = value;
  }

  /**
   * Returns the secret value; call it only where the value is sent.
   *
   * @returns The secret text.
   */
  reveal(): string {
    return this[VALUE];
  }

  /**
   * Compares two secrets in constant time with respect to their content.
   *
   * @param other - The other secret.
   * @returns Whether both hold the same value.
   */
  equals(other: Secret): boolean {
    const a = new TextEncoder().encode(this[VALUE]);
    const b = new TextEncoder().encode(other[VALUE]);
    let diff = a.length ^ b.length;
    for (let i = 0; i < Math.max(a.length, b.length); i++)
      diff |= (a[i] ?? 0) ^ (b[i] ?? 0);
    return diff === 0;
  }

  /** Returns `[redacted]`. */
  toString(): string {
    return REDACTED;
  }

  /** Returns `[redacted]`, so a secret never lands in JSON output. */
  toJSON(): string {
    return REDACTED;
  }

  /** Returns `Secret([redacted])` for `util.inspect` and `console.log`. */
  [Symbol.for("nodejs.util.inspect.custom")](): string {
    return `Secret(${REDACTED})`;
  }
}

/**
 * Wraps a string into a {@link Secret}; a secret passes unchanged.
 *
 * @param value - A secret or its text.
 * @returns The secret.
 */
export function asSecret(value: Secret | string): Secret {
  return value instanceof Secret ? value : new Secret(value);
}
