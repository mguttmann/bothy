// SPDX-License-Identifier: Apache-2.0
/**
 * No test reaches a real service (PLAN 8 "Test strategy"; brief M2-49 Design note 18): this
 * setup file of every Vitest project replaces `globalThis.fetch` by {@link guardedFetch}, which
 * throws for every URL whose host is not `127.0.0.1` or `localhost`. The app opens outgoing
 * connections only through `fetch`, and `startApp` in tests gets the guarded one.
 *
 * @module
 */

const realFetch = globalThis.fetch.bind(globalThis);

/** The hosts a test may reach. */
export const LOOPBACK_HOSTS: readonly string[] = Object.freeze([
  "127.0.0.1",
  "localhost",
]);

/** A request to a host outside {@link LOOPBACK_HOSTS}. */
export class NetworkGuardError extends Error {
  /**
   * Creates the error.
   *
   * @param host - The refused host (a test value, never a secret).
   */
  constructor(host: string) {
    super(`the test tried to reach ${host}; only the loopback host is allowed`);
    this.name = "NetworkGuardError";
  }
}

/**
 * Throws {@link NetworkGuardError} unless `url` points at the loopback host.
 *
 * @param url - An absolute URL.
 */
export function assertLoopback(url: string): void {
  const host = new URL(url).hostname;
  if (!LOOPBACK_HOSTS.includes(host)) throw new NetworkGuardError(host);
}

/**
 * `fetch` that refuses every non-loopback URL before any connection.
 *
 * @param input - URL, string or `Request`.
 * @param init - The options of `fetch`.
 * @returns The response of the real `fetch`.
 */
export function guardedFetch(
  input: string | URL | Request,
  init?: RequestInit,
): Promise<Response> {
  const url =
    typeof input === "string"
      ? input
      : input instanceof URL
        ? input.href
        : input.url;
  try {
    assertLoopback(url);
  } catch (err) {
    return Promise.reject(err as Error);
  }
  // Only the first URL is checked here, so a redirect is never followed: a caller's own mode
  // stays (the GitHub client asks for "manual"), the default "follow" becomes "error".
  const mode =
    init?.redirect ?? (input instanceof Request ? input.redirect : "follow");
  return realFetch(input, {
    ...init,
    redirect: mode === "follow" ? "error" : mode,
  });
}

globalThis.fetch = guardedFetch;
