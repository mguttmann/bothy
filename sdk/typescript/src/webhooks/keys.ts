// SPDX-License-Identifier: Apache-2.0
/**
 * Parsers of the `whpk_` public keys and `whsec_` secrets (docs/api/webhooks.md 2.1 point 4,
 * docs/DECISIONS.md D-M2-26-10 and D-M2-26-11).
 *
 * @module
 */

import { createPublicKey, type KeyObject } from "node:crypto";

import { WebhookConfigError } from "./errors.js";

const PUBLIC_KEY_PREFIX = "whpk_";
const SECRET_PREFIX = "whsec_";

// Standard base64 with padding and nothing else: Buffer.from(..., "base64") alone would skip
// foreign characters and accept the URL alphabet.
const STANDARD_BASE64 =
  /^(?:[A-Za-z0-9+/]{4})*(?:[A-Za-z0-9+/]{2}==|[A-Za-z0-9+/]{3}=)?$/;

/**
 * Decodes standard base64 with padding strictly.
 *
 * @param text - The encoded text.
 * @returns The bytes, or `undefined` when the text is no such encoding.
 * @internal
 */
export function decodeStandardBase64(text: string): Buffer | undefined {
  return STANDARD_BASE64.test(text) ? Buffer.from(text, "base64") : undefined;
}

/**
 * Reads a public key of `getWebhookKeys`: `whpk_` plus the standard base64 of the DER-encoded
 * SubjectPublicKeyInfo of an Ed25519 key (RFC 8410).
 *
 * @param value - The `public_key` of a `WebhookKey`.
 * @returns The Ed25519 public key.
 * @throws {@link WebhookConfigError} for another prefix, bad base64, bad DER or another key
 *   type; the message never repeats the value.
 * @example
 * ```ts
 * const key = parseWebhookPublicKey("whpk_MCowBQYDK2VwAyEA6h3uZ0Q4i1Yv1o1Wg8bQq3c0o8bq9oQm3Kc8n3m3v0I=");
 * key.asymmetricKeyType; // "ed25519"
 * ```
 */
export function parseWebhookPublicKey(value: string): KeyObject {
  if (!value.startsWith(PUBLIC_KEY_PREFIX)) {
    throw new WebhookConfigError("a webhook public key starts with whpk_");
  }
  const der = decodeStandardBase64(value.slice(PUBLIC_KEY_PREFIX.length));
  if (der === undefined || der.length === 0) {
    throw new WebhookConfigError(
      "a webhook public key is whpk_ and standard base64",
    );
  }
  let key: KeyObject;
  try {
    key = createPublicKey({ key: der, format: "der", type: "spki" });
  } catch {
    throw new WebhookConfigError(
      "a webhook public key is a DER SubjectPublicKeyInfo",
    );
  }
  if (key.asymmetricKeyType !== "ed25519") {
    throw new WebhookConfigError("a webhook public key is an Ed25519 key");
  }
  return key;
}

/**
 * Reads a `v1` secret: `whsec_` plus the standard base64 of the HMAC key, as the server shows
 * it once.
 *
 * @param value - The secret.
 * @returns The HMAC-SHA256 key bytes.
 * @throws {@link WebhookConfigError} when the value is empty, has another prefix, is no
 *   standard base64 or decodes to no bytes; the message never repeats the value.
 * @example
 * ```ts
 * const key = parseWebhookSecret(process.env.WEBHOOK_SECRET ?? "");
 * ```
 */
export function parseWebhookSecret(value: string): Uint8Array {
  if (!value.startsWith(SECRET_PREFIX)) {
    throw new WebhookConfigError("a webhook secret starts with whsec_");
  }
  const key = decodeStandardBase64(value.slice(SECRET_PREFIX.length));
  if (key === undefined || key.length === 0) {
    throw new WebhookConfigError(
      "a webhook secret is whsec_ and non-empty standard base64",
    );
  }
  return new Uint8Array(key);
}
