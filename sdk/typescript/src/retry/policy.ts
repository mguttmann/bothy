// SPDX-License-Identifier: Apache-2.0
/**
 * Which requests and answers are repeated, and how long the client waits
 * (docs/spec/11-erweiterungen.md 11.2 row `retry`, docs/DECISIONS.md D-M2-26-7, D-M2-30-3).
 *
 * @module
 */

import type { SdkSettings } from "../config.js";
import { exemptOperationOf } from "./table.js";

const IDEMPOTENT_METHODS: ReadonlySet<string> = new Set([
  "GET",
  "HEAD",
  "PUT",
  "DELETE",
  "OPTIONS",
]);
const IDEMPOTENCY_KEY = "idempotency-key";
const DELTA_SECONDS = /^\d+$/;
// IMF-fixdate of RFC 9110 5.6.7, the form servers send.
const HTTP_DATE =
  /^[A-Z][a-z]{2}, \d{2} [A-Z][a-z]{2} \d{4} \d{2}:\d{2}:\d{2} GMT$/;

/** Request headers in any of the forms the client sees. */
export type HeadersLike =
  Headers | Readonly<Record<string, string | undefined>>;

function hasHeader(headers: HeadersLike, name: string): boolean {
  if (headers instanceof Headers) return headers.has(name);
  return Object.entries(headers).some(
    ([key, value]) => key.toLowerCase() === name && value !== undefined,
  );
}

/**
 * Tells whether a request may be sent again: method `GET`, `HEAD`, `PUT`, `DELETE` (or
 * `OPTIONS`), an `Idempotency-Key` header, or an operation of `IDEMPOTENT_EXEMPT_OPERATIONS`.
 *
 * @param method - The HTTP method.
 * @param pathTemplateOrUrl - The request URL, its path, or the path template.
 * @param headers - The request headers.
 * @returns Whether the request is idempotent.
 */
export function isRetryableRequest(
  method: string,
  pathTemplateOrUrl: string,
  headers: HeadersLike,
): boolean {
  const upper = method.toUpperCase();
  if (IDEMPOTENT_METHODS.has(upper)) return true;
  if (hasHeader(headers, IDEMPOTENCY_KEY)) return true;
  return exemptOperationOf(upper, pathTemplateOrUrl) !== undefined;
}

/**
 * Tells whether an answer is repeated (for an idempotent request): every `5xx` except
 * `503 setup_required` (the instance waits for its owner, a repeat cannot help), `429` and
 * `409 request_in_progress` (the same `Idempotency-Key` is still being processed); never
 * another `4xx`.
 *
 * @param status - The HTTP status.
 * @param code - The problem code of the body, if any.
 * @returns Whether to repeat.
 */
export function isRetryableStatus(
  status: number,
  code: string | undefined,
): boolean {
  if (status === 429) return true;
  if (status === 503) return code !== "setup_required";
  if (status >= 500) return true;
  return status === 409 && code === "request_in_progress";
}

/**
 * Reads `Retry-After` as milliseconds: delta-seconds or an HTTP date (IMF-fixdate).
 *
 * @param value - The header value.
 * @param nowMs - The current wall time, Unix milliseconds, for an HTTP date.
 * @returns The wait, never negative, or `undefined` when absent or unreadable.
 */
export function parseRetryAfter(
  value: string | null | undefined,
  nowMs: number,
): number | undefined {
  const text = value?.trim();
  if (!text) return undefined;
  if (DELTA_SECONDS.test(text)) return Number(text) * 1_000;
  if (!HTTP_DATE.test(text)) return undefined;
  const when = Date.parse(text);
  return Number.isNaN(when) ? undefined : Math.max(0, when - nowMs);
}

/**
 * Returns the wait before retry `attempt`: uniform in [`sdk.retry_backoff_base`,
 * min(`sdk.retry_backoff_cap`, `sdk.retry_backoff_base` * 2^(attempt-1))], the reading of "full
 * jitter between base and cap" of D-M2-26-7.
 *
 * @param attempt - The number of the retry, at least 1.
 * @param settings - Base `sdk.retry_backoff_base` and cap `sdk.retry_backoff_cap`.
 * @param random - A source in [0, 1).
 * @returns The wait in milliseconds.
 */
export function backoffMs(
  attempt: number,
  settings: Pick<SdkSettings, "retryBackoffBaseMs" | "retryBackoffCapMs">,
  random: () => number,
): number {
  const base = settings.retryBackoffBaseMs;
  const upper = Math.max(
    base,
    Math.min(settings.retryBackoffCapMs, base * 2 ** Math.max(0, attempt - 1)),
  );
  return base + random() * (upper - base);
}
