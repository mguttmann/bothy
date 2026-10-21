// SPDX-License-Identifier: Apache-2.0
/**
 * Signing of test deliveries the way the server signs them (docs/api/webhooks.md 2.1 point 4,
 * docs/DECISIONS.md D-M2-26-10), for receivers under test and the FakeServer.
 *
 * @module
 */

import {
  createHmac,
  createPrivateKey,
  createPublicKey,
  sign,
  type KeyObject,
} from "node:crypto";

import { parseWebhookSecret } from "../webhooks/keys.js";

// PKCS #8 prefix of an Ed25519 private key (RFC 8410); the 32-byte seed follows.
const ED25519_PKCS8_PREFIX = Buffer.from(
  "302e020100300506032b657004220420",
  "hex",
);
const SEED_BYTES = 32;

/** The input of {@link signWebhook}. */
export interface SignWebhookOptions {
  /** The `webhook-id`, equal to the event ID. */
  readonly id: string;
  /** The `webhook-timestamp`, Unix seconds. */
  readonly timestamp: number;
  /** The body exactly as sent; a string counts as its UTF-8 bytes. */
  readonly body: Uint8Array | string;
  /** 32-byte Ed25519 private seeds, one `v1a` entry each, in signer order. */
  readonly seeds?: readonly Uint8Array[];
  /** `v1` secrets in `whsec_` form or as raw key bytes, newest generation first. */
  readonly secrets?: readonly (Uint8Array | string)[];
}

function privateKey(seed: Uint8Array): KeyObject {
  if (seed.length !== SEED_BYTES) {
    throw new RangeError("an Ed25519 seed has 32 bytes");
  }
  return createPrivateKey({
    key: Buffer.concat([ED25519_PKCS8_PREFIX, seed]),
    format: "der",
    type: "pkcs8",
  });
}

/**
 * Returns the `webhook-signature` value the server would send: the `v1a` entries first in
 * seed order, then the `v1` entries in the given order, separated by one space, each
 * `<scheme>,<standard base64>` over `{id}.{timestamp}.{body}`.
 *
 * @param options - The delivery, the seeds and the secrets.
 * @returns The header value.
 * @throws `RangeError` for a seed of another length; `WebhookConfigError` for a malformed
 *   secret.
 * @example
 * ```ts
 * const header = signWebhook({ id, timestamp: 1760346000, body, seeds: [seed] });
 * ```
 */
export function signWebhook(options: SignWebhookOptions): string {
  const body =
    typeof options.body === "string"
      ? Buffer.from(options.body, "utf8")
      : options.body;
  const content = Buffer.concat([
    Buffer.from(`${options.id}.${String(options.timestamp)}.`, "utf8"),
    body,
  ]);
  const entries: string[] = [];
  for (const seed of options.seeds ?? []) {
    entries.push(
      `v1a,${sign(null, content, privateKey(seed)).toString("base64")}`,
    );
  }
  for (const secret of options.secrets ?? []) {
    const key =
      typeof secret === "string" ? parseWebhookSecret(secret) : secret;
    entries.push(
      `v1,${createHmac("sha256", key).update(content).digest("base64")}`,
    );
  }
  return entries.join(" ");
}

/**
 * Returns the published form of the public key of a seed: `whpk_` plus the standard base64 of
 * its DER SubjectPublicKeyInfo, as `getWebhookKeys` lists it.
 *
 * @param seed - A 32-byte Ed25519 private seed.
 * @returns The `whpk_` value.
 * @throws `RangeError` for a seed of another length.
 */
export function webhookPublicKey(seed: Uint8Array): string {
  const der = createPublicKey(privateKey(seed)).export({
    format: "der",
    type: "spki",
  });
  return `whpk_${der.toString("base64")}`;
}
