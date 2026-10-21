// SPDX-License-Identifier: Apache-2.0
/**
 * Retry policy of the REST client (docs/spec/11-erweiterungen.md 11.2 row `retry`).
 *
 * Only idempotent requests are repeated: `GET`, `HEAD`, `PUT` and `DELETE`, a request with
 * `Idempotency-Key`, or an operation of {@link IDEMPOTENT_EXEMPT_OPERATIONS}. Repeated are
 * network errors, timeouts, `5xx` except `503 setup_required`, `429` and
 * `409 request_in_progress`. Retry `n` waits uniformly between `sdk.retry_backoff_base` and
 * min(`sdk.retry_backoff_cap`, base * 2^(n-1)), at least the `Retry-After` of the answer; at
 * most `sdk.retry_max_attempts` attempts (docs/DECISIONS.md D-M2-30-3).
 *
 * @example
 * ```ts
 * isRetryableRequest("POST", "https://example.com/api/v1/runs/r1/start", new Headers()); // true
 * isRetryableRequest("POST", "https://example.com/api/v1/runs/r1/cancel", new Headers()); // false
 * ```
 *
 * @module
 */

export {
  API_PREFIX,
  IDEMPOTENT_EXEMPT_OPERATIONS,
  RETRY_EXEMPT_REASONS,
  apiPath,
  exemptOperationOf,
} from "./table.js";
export {
  backoffMs,
  isRetryableRequest,
  isRetryableStatus,
  parseRetryAfter,
  type HeadersLike,
} from "./policy.js";
