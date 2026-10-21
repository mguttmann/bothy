// SPDX-License-Identifier: Apache-2.0
/**
 * Errors of the webhook verifier (docs/api/webhooks.md 3).
 *
 * Messages name the failed step only; they never contain the body, a signature, a key or a
 * secret (SEC-LOG-2), so a caller may log them as they are.
 *
 * @module
 */

/**
 * Why a delivery was refused, one value per step of the verification (docs/DECISIONS.md
 * D-M2-30-9):
 *
 * - `missing_header`: `webhook-id`, `webhook-timestamp` or `webhook-signature` is absent.
 * - `invalid_timestamp`: `webhook-timestamp` is no integer of Unix seconds.
 * - `timestamp_out_of_tolerance`: the timestamp lies farther than `webhook.receiver_tolerance`
 *   from the own clock, in either direction.
 * - `no_trusted_key`: neither a public key nor a secret is configured.
 * - `signature_mismatch`: no entry of `webhook-signature` is valid for a trusted key or secret.
 */
export type WebhookVerificationErrorCode =
  | "missing_header"
  | "invalid_timestamp"
  | "timestamp_out_of_tolerance"
  | "no_trusted_key"
  | "signature_mismatch";

/**
 * A delivery failed verification; answer it with `401` or `400` and do not process it.
 *
 * @example
 * ```ts
 * try {
 *   verifyWebhook({ body, headers, keys });
 * } catch (err) {
 *   if (err instanceof WebhookVerificationError) return reply(401, err.code);
 *   throw err;
 * }
 * ```
 */
export class WebhookVerificationError extends Error {
  /** The failed step. */
  readonly code: WebhookVerificationErrorCode;

  /**
   * Creates the error.
   *
   * @param code - The failed step.
   * @param message - A text naming the step, never a value of the request.
   */
  constructor(code: WebhookVerificationErrorCode, message: string) {
    super(message);
    this.name = "WebhookVerificationError";
    this.code = code;
  }
}

/**
 * A configured public key or secret is malformed (another prefix, bad base64, another key
 * type, empty). This is a fault of the receiver's configuration, not of a delivery.
 */
export class WebhookConfigError extends Error {
  /**
   * Creates the error.
   *
   * @param message - A text naming the rule, never the value.
   */
  constructor(message: string) {
    super(message);
    this.name = "WebhookConfigError";
  }
}
