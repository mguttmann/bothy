// SPDX-License-Identifier: Apache-2.0
/**
 * Credentials of an agent or app: secrets that never render, DPoP keys and proofs, and the
 * clear-text rule (docs/spec/11-extensions.md 11.2 row `auth`, 06 3.1, 08 7.3). Node.js only.
 *
 * @example
 * ```ts
 * const key = await loadDpopKey("dpop.pem");
 * const credentials: Credentials = { apiToken: new Secret("example-token"), dpop: key };
 * ```
 *
 * @module
 */

export { Secret, asSecret } from "./secret.js";
export type { Credentials, DpopAlgorithm, DpopSigner } from "./types.js";
export {
  DpopKey,
  createDpopProof,
  dpopThumbprint,
  gatewayHtu,
  loadDpopKey,
  normalizeHtu,
  requireSafeUrl,
  type DpopProofOptions,
} from "./dpop.js";
