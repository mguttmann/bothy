// SPDX-License-Identifier: Apache-2.0
/**
 * Invocation (`invocation`, docs/spec/11-extensions.md 11.2): the adapter's check of the
 * invocation token (docs/spec/09-agents.md 4.5) and the inbox of accepted events (09 4.3).
 * Node.js only: it handles tokens.
 *
 * - {@link InvocationVerifier}: points 1 to 10 of 09 4.5 with a JWKS cache ({@link jwksFetcher})
 *   that reloads once for an unknown `kid` and at most once per
 *   `adapter.jwks_refresh_min_interval`, the allow list of 17.7 ({@link triggererLoader}) and a
 *   `jti` store ({@link ReplayStore}).
 * - {@link expectationFor} and {@link compareActual}: what point 6 compares and the comparison of
 *   points 8 and 10 with the runtime's actual state.
 * - {@link MemoryInboxStore} and {@link FileInboxStore}: the inbox with the entry states of
 *   {@link EntryState} and the redelivery rule (a newer token replaces the stored one while the
 *   run has not started); both are the `jti` store as well.
 *
 * @example
 * ```ts
 * const inbox = new FileInboxStore(join(stateDir, "inbox"));
 * const verifier = new InvocationVerifier({
 *   instanceId, agentId, jwks: jwksFetcher(api), replay: inbox, allowlist: triggererLoader(api),
 * });
 * const { verdict } = await verifier.check(token, expectationFor(env.type, env), actual);
 * ```
 *
 * @module
 */

export { FileInboxStore } from "./file-store.js";
export {
  EntryState,
  InboxEntry,
  MemoryInboxStore,
  type InboxEntryFields,
  type InboxStore,
} from "./inbox.js";
export {
  InvocationVerifier,
  TOKEN_EVENT_TYPES,
  TokenVerdict,
  compareActual,
  expectationFor,
  jwksFetcher,
  storedClaims,
  triggererLoader,
  type Actual,
  type ActualSource,
  type Expectation,
  type InvocationClaims,
  type InvocationVerifierOptions,
  type JwksFetcher,
  type ReplayStore,
  type TokenCheck,
  type TriggererLoader,
} from "./verifier.js";
