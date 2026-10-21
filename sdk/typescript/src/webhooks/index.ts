// SPDX-License-Identifier: Apache-2.0
/**
 * Standard Webhooks verification for apps (`webhooks`, docs/spec/11-erweiterungen.md 11.2):
 * `v1a` (Ed25519 with the keys of `getWebhookKeys`) and `v1` (HMAC-SHA256 with a `whsec_`
 * secret) over the raw body, inside `webhook.receiver_tolerance`; also for requests to
 * `interactions_url` (11 9). Node.js only (`node:crypto`).
 *
 * @example
 * ```ts
 * const cache = new WebhookKeyCache({ client: createApiClient({ baseUrl }) });
 * const verified = await verifyWebhookRequest(request, { keys: await cache.keys() });
 * ```
 *
 * @module
 */

export { WebhookKeyCache, type WebhookKeyCacheOptions } from "./cache.js";
export {
  WebhookConfigError,
  WebhookVerificationError,
  type WebhookVerificationErrorCode,
} from "./errors.js";
export { parseWebhookPublicKey, parseWebhookSecret } from "./keys.js";
export {
  verifyWebhook,
  verifyWebhookRequest,
  type VerifiedWebhook,
  type VerifyWebhookOptions,
  type VerifyWebhookTrust,
  type WebhookHeaders,
} from "./verify.js";
