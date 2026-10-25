// SPDX-License-Identifier: Apache-2.0
/**
 * Check of `X-Hub-Signature-256` of a GitHub delivery (docs/spec/11-extensions.md 10.1 row
 * "Input"; brief M2-49 Design note 5).
 *
 * The header must match `^sha256=[0-9a-f]{64}$`; anything else is refused without computing.
 * The app computes HMAC-SHA256 with the webhook secret over the raw body bytes and compares the
 * 32-byte digest with the 32 bytes of the header through `crypto.timingSafeEqual` only, never as
 * strings. `X-Hub-Signature` (SHA-1) is never used. Source:
 * https://docs.github.com/en/webhooks/using-webhooks/validating-webhook-deliveries (retrieved
 * 2026-10-10).
 *
 * @example
 * ```ts
 * const result = verifyGitHubSignature(secret, rawBody, request.headers["x-hub-signature-256"]);
 * if (!result.ok) return reply(401, result.reason);
 * ```
 *
 * @module
 */

import { createHmac, timingSafeEqual } from "node:crypto";

/** Why a signature was refused; the only value a log line names. */
export type SignatureFailure = "missing" | "malformed" | "mismatch";

/** The result of {@link verifyGitHubSignature}. */
export type SignatureResult =
  | {
      /** The signature is valid. */
      readonly ok: true;
    }
  | {
      /** The signature is refused. */
      readonly ok: false;
      /** Why. */
      readonly reason: SignatureFailure;
    };

const HEADER_FORMAT = /^sha256=([0-9a-f]{64})$/;

/**
 * Verifies `X-Hub-Signature-256` over the raw body.
 *
 * @param secret - The webhook secret as configured in GitHub.
 * @param body - The raw request body exactly as received.
 * @param header - The header value, `undefined` when it is absent (a repeated header arrives
 *   as an array and is malformed).
 * @returns `ok` or the reason of the refusal; it never throws for any header value.
 */
export function verifyGitHubSignature(
  secret: string,
  body: Uint8Array,
  header: string | readonly string[] | undefined,
): SignatureResult {
  if (header === undefined) return { ok: false, reason: "missing" };
  if (typeof header !== "string") return { ok: false, reason: "malformed" };
  const match = HEADER_FORMAT.exec(header);
  if (!match) return { ok: false, reason: "malformed" };
  const expected = Buffer.from(match[1] ?? "", "hex");
  const actual = createHmac("sha256", secret).update(body).digest();
  return timingSafeEqual(actual, expected)
    ? { ok: true }
    : { ok: false, reason: "mismatch" };
}
