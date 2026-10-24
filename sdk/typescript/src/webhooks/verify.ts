// SPDX-License-Identifier: Apache-2.0
/**
 * Standard Webhooks verification for apps (docs/api/webhooks.md 3, docs/spec/11-extensions.md
 * 7 and 9, ADR 0016): outgoing webhooks and requests to `interactions_url` alike.
 *
 * @module
 */

import {
  createHmac,
  timingSafeEqual,
  verify as verifySignature,
  type KeyObject,
} from "node:crypto";

import { DEFAULTS } from "../config.js";
import type { components } from "../generated/openapi.js";
import {
  decodeEvent,
  type KnownEvent,
  type UnknownEvent,
} from "../tolerance.js";
import { WebhookConfigError, WebhookVerificationError } from "./errors.js";
import {
  decodeStandardBase64,
  parseWebhookPublicKey,
  parseWebhookSecret,
} from "./keys.js";

/** The request headers: a Fetch API `Headers` object or a plain record such as Node's. */
export type WebhookHeaders =
  Headers | Readonly<Record<string, string | readonly string[] | undefined>>;

/** What {@link verifyWebhook} needs besides the request. */
export interface VerifyWebhookTrust {
  /**
   * Trusted `v1a` public keys: parsed keys or their `whpk_` form, normally the result of
   * `WebhookKeyCache.keys()`. Never taken from the request.
   */
  readonly keys?: readonly (KeyObject | string)[];
  /** Trusted `v1` secrets: their `whsec_` form or the raw key bytes. */
  readonly secrets?: readonly (Uint8Array | string)[];
  /**
   * Largest distance of `webhook-timestamp` to {@link VerifyWebhookTrust.now} in milliseconds;
   * default `webhook.receiver_tolerance`.
   */
  readonly toleranceMs?: number;
  /** The own clock, Unix milliseconds; default `Date.now`. */
  readonly now?: () => number;
}

/** The input of {@link verifyWebhook}. */
export interface VerifyWebhookOptions extends VerifyWebhookTrust {
  /**
   * The raw request body exactly as received; a string counts as its UTF-8 bytes. Never pass
   * a body that was parsed and serialized again.
   */
  readonly body: Uint8Array | string;
  /** The request headers, looked up case-insensitively. */
  readonly headers: WebhookHeaders;
}

/** A delivery that passed verification. */
export interface VerifiedWebhook {
  /** The `webhook-id`, equal to the event ID; the key for the app's duplicate check. */
  readonly id: string;
  /** The `webhook-timestamp`, Unix seconds of this attempt. */
  readonly timestamp: number;
  /** The scheme of the first valid entry of `webhook-signature`. */
  readonly scheme: "v1a" | "v1";
  /**
   * The envelope decoded by type alone (`decodeEvent`); a body that is no JSON is an
   * `UnknownEvent` with type `""` and no `raw` value.
   */
  readonly event: KnownEvent | UnknownEvent;
  /**
   * The delivery member `sync_epoch` when it is a string; a change means the instance was
   * restored (docs/api/webhooks.md 3 point 6).
   */
  readonly syncEpoch: components["schemas"]["Uuid"] | undefined;
}

const HEADER_ID = "webhook-id";
const HEADER_TIMESTAMP = "webhook-timestamp";
const HEADER_SIGNATURE = "webhook-signature";
// At most 15 digits keeps the value a safe integer; Unix seconds need 10 for centuries.
const UNIX_SECONDS = /^\d{1,15}$/;
const HMAC_BYTES = 32;

function header(headers: WebhookHeaders, name: string): string | undefined {
  if (headers instanceof Headers) return headers.get(name) ?? undefined;
  // Several members differing only in case, or an array value, are joined with ", " as the
  // Fetch API joins repeated headers.
  const values: string[] = [];
  for (const [key, value] of Object.entries(headers)) {
    if (key.toLowerCase() !== name || value === undefined) continue;
    if (typeof value === "string") values.push(value);
    else values.push(...value);
  }
  return values.length === 0 ? undefined : values.join(", ");
}

function trustedKeys(keys: readonly (KeyObject | string)[]): KeyObject[] {
  return keys.map((k) =>
    typeof k === "string" ? parseWebhookPublicKey(k) : k,
  );
}

function trustedSecrets(
  secrets: readonly (Uint8Array | string)[],
): Uint8Array[] {
  return secrets.map((s) => {
    if (typeof s === "string") return parseWebhookSecret(s);
    if (s.length === 0)
      throw new WebhookConfigError("a webhook secret is non-empty");
    return s;
  });
}

function hmacMatches(
  secrets: readonly Uint8Array[],
  content: Buffer,
  signature: Buffer,
): boolean {
  // Length first: timingSafeEqual throws on unequal lengths.
  if (signature.length !== HMAC_BYTES) return false;
  let match = false;
  for (const secret of secrets) {
    const expected = createHmac("sha256", secret).update(content).digest();
    // No early exit, so the time does not tell which secret matched.
    if (timingSafeEqual(expected, signature)) match = true;
  }
  return match;
}

function ed25519Matches(
  keys: readonly KeyObject[],
  content: Buffer,
  signature: Buffer,
): boolean {
  for (const key of keys) {
    try {
      if (verifySignature(null, content, key, signature)) return true;
    } catch {
      // A key object of another type or a malformed signature counts as no match.
    }
  }
  return false;
}

// JSON.parse reviver as in the REST tolerance layer (docs/spec/06-api.md 2 "Kein null"): a member
// whose value is null is dropped; null items of an array stay.
function withoutNullMembers(
  this: unknown,
  _key: string,
  value: unknown,
): unknown {
  return value === null && !Array.isArray(this) ? undefined : value;
}

const UTF8 = new TextDecoder("utf-8", { fatal: true });

function decodeBody(body: Uint8Array): {
  event: KnownEvent | UnknownEvent;
  syncEpoch: string | undefined;
} {
  let raw: unknown;
  try {
    raw = JSON.parse(UTF8.decode(body), withoutNullMembers);
  } catch {
    // Authentic but no JSON: skipped by the caller like any event it does not know (04 7.1).
    return {
      event: { kind: "unknown", type: "", raw: undefined },
      syncEpoch: undefined,
    };
  }
  const epoch =
    typeof raw === "object" && raw !== null && !Array.isArray(raw)
      ? (raw as Record<string, unknown>).sync_epoch
      : undefined;
  return {
    event: decodeEvent(raw),
    syncEpoch: typeof epoch === "string" ? epoch : undefined,
  };
}

/**
 * Verifies one delivery of an outgoing webhook or a request to `interactions_url`.
 *
 * Steps, each with its error code (docs/decisions.d/M2-30.md D-M2-30-9): the three headers present
 * (`missing_header`); `webhook-timestamp` an integer of Unix seconds (`invalid_timestamp`);
 * its distance to `now()` at most `toleranceMs` in both directions
 * (`timestamp_out_of_tolerance`); at least one key or secret configured (`no_trusted_key`);
 * one entry of `webhook-signature` valid over `{webhook-id}.{webhook-timestamp}.{body}`
 * (`signature_mismatch`). Entries are split at spaces and at their first comma; an unknown
 * scheme or bad base64 is ignored. `v1a` entries are checked against every key with Ed25519,
 * `v1` entries against every secret with HMAC-SHA256 compared in constant time.
 *
 * Duplicate detection is the app's duty: remember {@link VerifiedWebhook.id} for at least
 * `webhook.retry_max_age` (docs/api/webhooks.md 3 point 4).
 *
 * @param options - The raw body, the headers and the trust list.
 * @returns The verified delivery with its decoded event.
 * @throws {@link WebhookVerificationError} when the delivery fails a step; its message never
 *   contains the body, a signature or a secret.
 * @throws {@link WebhookConfigError} when a configured key or secret is malformed.
 * @example
 * ```ts
 * const keys = await keyCache.keys();
 * const verified = verifyWebhook({ body: rawBody, headers: req.headers, keys });
 * if (verified.event.kind === "known") handle(verified.event.event);
 * ```
 */
export function verifyWebhook(options: VerifyWebhookOptions): VerifiedWebhook {
  const keys = trustedKeys(options.keys ?? []);
  const secrets = trustedSecrets(options.secrets ?? []);

  const id = header(options.headers, HEADER_ID);
  const timestampText = header(options.headers, HEADER_TIMESTAMP);
  const signatures = header(options.headers, HEADER_SIGNATURE);
  if (
    id === undefined ||
    timestampText === undefined ||
    signatures === undefined
  ) {
    throw new WebhookVerificationError(
      "missing_header",
      "a webhook needs the headers webhook-id, webhook-timestamp and webhook-signature",
    );
  }
  if (!UNIX_SECONDS.test(timestampText)) {
    throw new WebhookVerificationError(
      "invalid_timestamp",
      "webhook-timestamp is no integer of Unix seconds",
    );
  }
  const timestamp = Number(timestampText);
  const now = (options.now ?? Date.now)();
  // webhook.receiver_tolerance
  const tolerance = options.toleranceMs ?? DEFAULTS.webhookReceiverToleranceMs;
  if (Math.abs(now - timestamp * 1000) > tolerance) {
    throw new WebhookVerificationError(
      "timestamp_out_of_tolerance",
      "webhook-timestamp lies outside webhook.receiver_tolerance",
    );
  }
  if (keys.length === 0 && secrets.length === 0) {
    throw new WebhookVerificationError(
      "no_trusted_key",
      "no webhook public key and no webhook secret is configured",
    );
  }

  const body =
    typeof options.body === "string"
      ? new TextEncoder().encode(options.body)
      : options.body;
  // The canonical decimal form of the timestamp, as the signer writes it.
  const content = Buffer.concat([
    Buffer.from(`${id}.${String(timestamp)}.`, "utf8"),
    body,
  ]);
  let scheme: "v1a" | "v1" | undefined;
  for (const entry of signatures.split(" ")) {
    const comma = entry.indexOf(",");
    if (comma < 0) continue;
    const name = entry.slice(0, comma);
    if (name !== "v1a" && name !== "v1") continue;
    const signature = decodeStandardBase64(entry.slice(comma + 1));
    if (signature === undefined) continue;
    const valid =
      name === "v1a"
        ? ed25519Matches(keys, content, signature)
        : hmacMatches(secrets, content, signature);
    if (valid) {
      scheme = name;
      break;
    }
  }
  if (scheme === undefined) {
    throw new WebhookVerificationError(
      "signature_mismatch",
      "no entry of webhook-signature is valid for a trusted key or secret",
    );
  }
  return { id, timestamp, scheme, ...decodeBody(body) };
}

/**
 * Verifies a Fetch API `Request` (an outgoing webhook or an `interactions_url` request, 11 9),
 * reading its body once as bytes.
 *
 * @param request - The unread request.
 * @param options - The trust list, tolerance and clock as for {@link verifyWebhook}.
 * @returns The verified delivery with its decoded event.
 * @throws {@link WebhookVerificationError} or {@link WebhookConfigError} as
 *   {@link verifyWebhook} does.
 * @example
 * ```ts
 * export default {
 *   async fetch(request: Request): Promise<Response> {
 *     const verified = await verifyWebhookRequest(request, { keys: await cache.keys() });
 *     return new Response(null, { status: 204 });
 *   },
 * };
 * ```
 */
export async function verifyWebhookRequest(
  request: Request,
  options: VerifyWebhookTrust = {},
): Promise<VerifiedWebhook> {
  const body = new Uint8Array(await request.arrayBuffer());
  return verifyWebhook({ ...options, body, headers: request.headers });
}
