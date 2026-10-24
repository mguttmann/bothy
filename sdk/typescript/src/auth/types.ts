// SPDX-License-Identifier: Apache-2.0
/**
 * Interfaces of DPoP keys and agent credentials (08 7.3, RFC 9449).
 *
 * @module
 */

import type { Secret } from "./secret.js";

/** JWS algorithms of DPoP proofs the SDK produces. */
export type DpopAlgorithm = "EdDSA" | "ES256";

/**
 * A DPoP key behind any key store (for example an HSM): its JWS algorithm, its public JWK and
 * a signer. An adapter plugs a key store without a new SDK dependency.
 */
export interface DpopSigner {
  /** The JWS `alg` of the proofs. */
  readonly alg: DpopAlgorithm;
  /** The public JWK without private members. */
  readonly publicJwk: Readonly<Record<string, string>>;
  /**
   * Returns the JWS signature of `data` in the form of `alg` (64-byte `R || S` for `ES256`).
   *
   * @param data - The JWS signing input.
   */
  sign(data: Uint8Array): Promise<Uint8Array>;
}

/** The credentials of an agent adapter or an app. */
export interface Credentials {
  /** The API token. */
  readonly apiToken: Secret;
  /** The key of a DPoP-bound token; absent sends the token as a bearer token. */
  readonly dpop?: DpopSigner;
}
