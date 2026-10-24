// SPDX-License-Identifier: Apache-2.0
/**
 * The adapter's check of the invocation token (docs/spec/09-agents.md 4.5 points 1 to 10).
 *
 * {@link InvocationVerifier.check} decides one verdict per token in the precedence of M2-06's
 * reference verifier (`services/server/internal/runs/adapters/runstest/refverifier.go`) and of
 * the Python SDK's `InvocationVerifier`:
 *
 * 1. Points 1 and 2 and the token form: three segments of strict base64url, a header object
 *    without duplicate members, `alg` exactly `EdDSA`, `typ` exactly `agent-invocation+jwt`, no
 *    `crit`, a non-empty `kid`. Failure: `invalid`. Nothing is fetched before this holds.
 * 2. Point 3: the `kid` is in the cached JWKS, after at most one forced reload per
 *    `adapter.jwks_refresh_min_interval`. Failure: `unknown_key`. A token never adds a key.
 * 3. Point 4: the signature (`jose` `compactVerify` with `algorithms: ["EdDSA"]` only), the
 *    payload shape, then `iss` and `aud` byte-exactly (`aud` a single string). Failure: `invalid`.
 * 4. Point 6: `run_id`, `thread_id` and the triggering object equal the event's payload; a
 *    triggering object without counterpart in the payload fails closed. Failure: `invalid`.
 * 5. Point 9: the allow list of 17.7 when configured; a `sub` still missing after the reload
 *    rules is `invalid` (docs/DECISIONS.md D-M2-29-8).
 * 6. Point 8: `billing_mode` equals the actual mode, and `unknown` never matches
 *    (docs/spec/10-backend-policy.md 7). Failure: `billing_mode_mismatch`.
 * 7. Point 10: the actual tool class is not above `tool_class` (`chat` < `scoped` < `host`).
 *    Failure: `tool_class_mismatch`.
 * 8. Point 5: `iat` after now plus `adapter.invocation_token_leeway`, or `exp` not after `iat`,
 *    is `invalid`; now at or after `exp` plus that leeway is `expired`.
 * 9. Point 7: a `jti` seen before and not yet expired is `replay`. A passing token's `jti` is
 *    remembered until `exp` plus the leeway.
 *
 * Header, `iss`, `aud` and the times are compared by this module itself, not by `jose`'s
 * `jwtVerify`, so that the precedence above holds (an expired token with a billing mismatch is a
 * mismatch); `jose` only verifies the signature. The token itself never appears in a log or an
 * error text; a log line carries at most a short prefix of the SHA-256 hash of the `jti`
 * (SEC-LOG-2).
 *
 * @module
 */

import { createHash, createPublicKey, type KeyObject } from "node:crypto";

import { compactVerify } from "jose";
import type { Client } from "openapi-fetch";

import type { Secret } from "../auth/secret.js";
import { unwrap, unwrapDetailed } from "../client.js";
import { type Clock, systemClock } from "../clock.js";
import { type SdkSettings, resolveSettings } from "../config.js";
import type { components, paths } from "../generated/openapi.js";
import { type Logger, defaultLogger } from "../logger.js";
import {
  canonicalUuid,
  decodeSegment,
  isRecord,
  parseObject,
  type StrictObject,
} from "./json.js";

const ALG = "EdDSA";
const TYP = "agent-invocation+jwt";
const ED25519_KEY_BYTES = 32;
const JTI_HASH_CHARS = 12;
// Order of the tool classes, docs/spec/07-permissions.md 2.11.
// A Map, not an object literal: a lookup must never find an inherited member such as
// "constructor", which would make an unknown tool class pass point 10.
const TOOL_CLASS_RANK: ReadonlyMap<string, number> = new Map([
  ["chat", 0],
  ["scoped", 1],
  ["host", 2],
]);
const REQUIRED_TEXT = [
  "iss",
  "aud",
  "sub",
  "jti",
  "trigger_kind",
  "thread_id",
  "channel_id",
  "space_id",
  "level",
  "billing_mode",
  "tool_class",
] as const;
const OPTIONAL_TEXT = [
  "run_id",
  "message_id",
  "interaction_id",
  "command_id",
] as const;
const REQUIRED_INT = ["iat", "exp", "permission_version"] as const;
const TRIGGER_CLAIMS: ReadonlySet<string> = new Set([
  "message_id",
  "interaction_id",
  "command_id",
]);

/** The result of one check, in the vocabulary of 09 4.5. */
export const TokenVerdict = Object.freeze({
  /** Every point holds. */
  VALID: "valid",
  /** Only point 5 fails because `exp` passed: renew (4.5.1); acceptance ignores it. */
  EXPIRED: "expired",
  /** Points 1, 2, 4, 6 or 9 fail, or `iat` is in the future: `invalid_invocation_token`. */
  INVALID: "invalid",
  /** The `kid` is missing after the allowed reload: neither accept nor acknowledge. */
  UNKNOWN_KEY: "unknown_key",
  /** Point 8: the actual billing mode differs or is unknown. */
  BILLING_MODE_MISMATCH: "billing_mode_mismatch",
  /** Point 10: the actual tool class is above the claim. */
  TOOL_CLASS_MISMATCH: "tool_class_mismatch",
  /** Point 7: the `jti` was seen before and has not expired. */
  REPLAY: "replay",
} as const);

/** One of the values of {@link TokenVerdict}. */
export type TokenVerdict = (typeof TokenVerdict)[keyof typeof TokenVerdict];

/** The event types that carry an `invocation_token` (09 4.4). */
export const TOKEN_EVENT_TYPES: ReadonlySet<string> = new Set([
  "agent.invocation",
  "agent.followup",
  "interaction.created",
  "command.invoked",
  "command.autocomplete_requested",
]);

/** What a verified token says; never the token itself. IDs are canonical lower-case UUIDs. */
export interface InvocationClaims {
  /** The issuing instance. */
  readonly iss: string;
  /** The agent the token is for. */
  readonly aud: string;
  /** The triggering principal. */
  readonly sub: string;
  /** The token ID. */
  readonly jti: string;
  /** Issued at, seconds since the epoch. */
  readonly iat: number;
  /** Expiry, seconds since the epoch. */
  readonly exp: number;
  /** `message`, `followup`, `interaction`, `command` or `autocomplete`. */
  readonly triggerKind: string;
  /** The thread. */
  readonly threadId: string;
  /** The channel. */
  readonly channelId: string;
  /** The space of the channel. */
  readonly spaceId: string;
  /** The effective level of `sub` on the channel. */
  readonly level: string;
  /** The configured billing mode. */
  readonly billingMode: string;
  /** The configured tool class. */
  readonly toolClass: string;
  /** The permission version at delivery time. */
  readonly permissionVersion: number;
  /** The run, when the token has one. */
  readonly runId: string | undefined;
  /** The triggering message, when it is one. */
  readonly messageId: string | undefined;
  /** The triggering interaction, when it is one. */
  readonly interactionId: string | undefined;
  /** The triggering command, when it is one. */
  readonly commandId: string | undefined;
}

/** What the event's payload says (point 6); an `undefined` run or thread is not compared. */
export interface Expectation {
  /** The thread of the payload, if it names one. */
  readonly threadId: string | undefined;
  /** The run of the payload, if it names one. */
  readonly runId: string | undefined;
  /**
   * The claim that names the triggering object: `message_id`, `interaction_id` or `command_id`
   * (empty for a trigger kind this SDK does not know).
   */
  readonly triggerClaim: string;
  /**
   * The triggering object of the payload; `undefined` when the payload has no counterpart,
   * which makes the check `invalid` (fail closed).
   */
  readonly triggerId: string | undefined;
}

/** The runtime's actual state for this run (points 8 and 10). */
export interface Actual {
  /** A `BillingMode` value, or `unknown` (never matches). */
  readonly billingMode: string;
  /** The tool class actually loaded: `chat`, `scoped` or `host`. */
  readonly toolClass: string;
}

/** The verdict of one check and, when the payload decoded, the claims. */
export interface TokenCheck {
  /** The verdict. */
  readonly verdict: TokenVerdict;
  /**
   * The claims; `undefined` when the header, signature or payload was refused or the key is
   * unknown.
   */
  readonly claims: InvocationClaims | undefined;
}

/**
 * Fetches the JWKS: resolves to its JSON body and the `max-age` of `Cache-Control` in
 * milliseconds (`undefined` when absent). {@link jwksFetcher} builds one on a client.
 */
export type JwksFetcher = () => Promise<
  readonly [body: unknown, maxAgeMs: number | undefined]
>;

/**
 * Loads the allow list of 17.7 (`getAgentTriggerers`); {@link triggererLoader} builds one on a
 * client.
 */
export type TriggererLoader = () => Promise<
  components["schemas"]["AgentTriggerers"]
>;

/** The `jti` store of point 7; the inbox stores implement it. Times are Unix milliseconds. */
export interface ReplayStore {
  /**
   * Returns whether `jti` is remembered beyond `nowMs`.
   *
   * @param jti - The token ID.
   * @param nowMs - The current wall time.
   */
  jtiSeen(jti: string, nowMs: number): Promise<boolean>;
  /**
   * Remembers `jti` until `untilMs` (an earlier end never shortens a later one).
   *
   * @param jti - The token ID.
   * @param untilMs - `exp` plus `adapter.invocation_token_leeway`.
   */
  rememberJti(jti: string, untilMs: number): Promise<void>;
}

/**
 * The actual state, or a function that returns it for the verified claims (sync or async).
 */
export type ActualSource =
  Actual | ((claims: InvocationClaims) => Actual | Promise<Actual>);

/** The options of {@link InvocationVerifier}. */
export interface InvocationVerifierOptions {
  /** The configured instance (`iss`); never read from discovery (09 4.5 point 4). */
  readonly instanceId: string;
  /** The own agent ID (`aud`). */
  readonly agentId: string;
  /** Fetches the JWKS (`getJwks`) with its `max-age`; see {@link jwksFetcher}. */
  readonly jwks: JwksFetcher;
  /** The `jti` store (point 7), usually the inbox store. */
  readonly replay: ReplayStore;
  /** `adapter.invocation_token_leeway` and `adapter.jwks_refresh_min_interval`; default the keys' defaults. */
  readonly settings?: Partial<SdkSettings> | undefined;
  /** The time of every check, of the JWKS cache and of the reload throttle. */
  readonly clock?: Clock | undefined;
  /** Loads the allow list of 17.7; absent skips point 9. */
  readonly allowlist?: TriggererLoader | undefined;
  /** Receives the refusals at `debug` (reason and a hash prefix of the `jti` only). */
  readonly logger?: Logger | undefined;
}

/**
 * Compares the claims with the runtime's actual state (points 8 and 10 only).
 *
 * The adapter asks again at start time, after `prepare`, with this function.
 *
 * @param claims - The verified claims (their billing mode and tool class).
 * @param actual - The actual billing mode and tool class.
 * @returns `valid`, `billing_mode_mismatch` (also for `unknown` or an empty mode) or
 *   `tool_class_mismatch` (also for a tool class this SDK does not know).
 *
 * @example
 * ```ts
 * compareActual(claims, { billingMode: "unknown", toolClass: "chat" }); // "billing_mode_mismatch"
 * ```
 */
export function compareActual(
  claims: Pick<InvocationClaims, "billingMode" | "toolClass">,
  actual: Actual,
): TokenVerdict {
  if (
    actual.billingMode === "" ||
    actual.billingMode === "unknown" ||
    actual.billingMode !== claims.billingMode
  )
    return TokenVerdict.BILLING_MODE_MISMATCH;
  const have = TOOL_CLASS_RANK.get(actual.toolClass);
  const allowed = TOOL_CLASS_RANK.get(claims.toolClass);
  if (have === undefined || allowed === undefined || have > allowed)
    return TokenVerdict.TOOL_CLASS_MISMATCH;
  return TokenVerdict.VALID;
}

function idOf(
  holder: StrictObject,
  name: string,
  required: boolean,
): string | undefined {
  const value = holder[name];
  if (value === undefined && !required) return undefined;
  const id = typeof value === "string" ? canonicalUuid(value) : undefined;
  if (id === undefined)
    throw new TypeError(`the payload member ${name} is not a UUID`);
  return id;
}

/**
 * Returns what point 6 compares for an event (Design note 8 of M2-29).
 *
 * Rows: `agent.invocation` (run, thread, and by `trigger_kind` `message_id`, `interaction_id` or
 * `command_id`), `agent.followup` (run, thread, `message_id`), `interaction.created`
 * (`interaction.id`, its `run_id` and `thread_id`), `command.invoked` (`command_id`, `run_id`
 * and `thread_id` when present), `command.autocomplete_requested` (`command_id`, its thread when
 * present, no run). `AgentInvocationPayload` has no `command_id` yet (only
 * `command_invocation_id`), so `trigger_kind = command` has no counterpart and every such token
 * is `invalid`. IDs are compared in their canonical lower-case form.
 *
 * @param envelopeType - The envelope `type`.
 * @param event - The event (an envelope or a decoded event with `payload`).
 * @returns The expectation.
 * @throws `RangeError` when the type carries no invocation token; `TypeError` when the payload
 *   is not the payload of that type.
 *
 * @example
 * ```ts
 * const expect = expectationFor(envelope.type, envelope);
 * const check = await verifier.check(token, expect, actual);
 * ```
 */
export function expectationFor(
  envelopeType: string,
  event: unknown,
): Expectation {
  if (!TOKEN_EVENT_TYPES.has(envelopeType))
    throw new RangeError(
      `event type ${envelopeType} carries no invocation token`,
    );
  const payload = isRecord(event) ? event.payload : undefined;
  if (!isRecord(payload))
    throw new TypeError(`the event has no payload of ${envelopeType}`);
  switch (envelopeType) {
    case "agent.invocation": {
      const threadId = idOf(payload, "thread_id", true);
      const runId = idOf(payload, "run_id", true);
      const kind = payload.trigger_kind;
      if (typeof kind !== "string")
        throw new TypeError("the payload member trigger_kind is not a text");
      if (kind === "message")
        return {
          threadId,
          runId,
          triggerClaim: "message_id",
          triggerId: idOf(payload, "message_id", false),
        };
      if (kind === "interaction")
        return {
          threadId,
          runId,
          triggerClaim: "interaction_id",
          triggerId: idOf(payload, "interaction_id", false),
        };
      // No command_id in AgentInvocationPayload before M2-16: fail closed.
      if (kind === "command")
        return {
          threadId,
          runId,
          triggerClaim: "command_id",
          triggerId: undefined,
        };
      return { threadId, runId, triggerClaim: "", triggerId: undefined };
    }
    case "agent.followup":
      return {
        threadId: idOf(payload, "thread_id", true),
        runId: idOf(payload, "run_id", true),
        triggerClaim: "message_id",
        triggerId: idOf(payload, "message_id", true),
      };
    case "interaction.created": {
      const interaction = payload.interaction;
      if (!isRecord(interaction))
        throw new TypeError("the payload has no interaction");
      return {
        threadId: idOf(interaction, "thread_id", false),
        runId: idOf(interaction, "run_id", false),
        triggerClaim: "interaction_id",
        triggerId: idOf(interaction, "id", true),
      };
    }
    case "command.invoked":
      return {
        threadId: idOf(payload, "thread_id", false),
        runId: idOf(payload, "run_id", false),
        triggerClaim: "command_id",
        triggerId: idOf(payload, "command_id", true),
      };
    default:
      return {
        threadId: idOf(payload, "thread_id", false),
        runId: undefined,
        triggerClaim: "command_id",
        triggerId: idOf(payload, "command_id", true),
      };
  }
}

/** A verdict reached before the end of the check; `reason` names the point only. */
class Refused extends Error {
  constructor(
    readonly verdict: TokenVerdict,
    readonly reason: string,
    readonly claims?: InvocationClaims,
  ) {
    super(reason);
    this.name = "Refused";
  }
}

function checkHeader(head: Uint8Array): string {
  let members: StrictObject;
  try {
    members = parseObject(head);
  } catch {
    throw new Refused(TokenVerdict.INVALID, "header");
  }
  if (members.alg !== ALG) throw new Refused(TokenVerdict.INVALID, "alg");
  if (members.typ !== TYP) throw new Refused(TokenVerdict.INVALID, "typ");
  if (Object.hasOwn(members, "crit"))
    throw new Refused(TokenVerdict.INVALID, "crit");
  const kid = members.kid;
  if (typeof kid !== "string" || kid === "")
    throw new Refused(TokenVerdict.INVALID, "kid");
  return kid;
}

function isInt(value: unknown): value is number {
  // The parser reads fractions, exponent forms and integers beyond int64 as a NonInteger, so
  // only an int64 integer is a number here; a boolean never is.
  return typeof value === "number" && Number.isInteger(value);
}

function decodeClaims(body: Uint8Array): [InvocationClaims, StrictObject] {
  let members: StrictObject;
  try {
    members = parseObject(body);
  } catch {
    throw new Refused(TokenVerdict.INVALID, "payload");
  }
  const text = (name: string): string => {
    const value = members[name];
    if (typeof value !== "string" || value === "")
      throw new Refused(TokenVerdict.INVALID, `claim ${name}`);
    return value;
  };
  for (const name of REQUIRED_TEXT) text(name);
  for (const name of OPTIONAL_TEXT)
    if (Object.hasOwn(members, name)) text(name);
  for (const name of REQUIRED_INT)
    if (!isInt(members[name]))
      throw new Refused(TokenVerdict.INVALID, `claim ${name}`);
  if (!TOOL_CLASS_RANK.has(text("tool_class")))
    throw new Refused(TokenVerdict.INVALID, "claim tool_class");
  const uuidOf = (name: string): string | undefined => {
    if (!Object.hasOwn(members, name)) return undefined;
    const id = canonicalUuid(text(name));
    if (id === undefined)
      throw new Refused(TokenVerdict.INVALID, `claim ${name}`);
    return id;
  };
  const must = (name: string): string => {
    const id = uuidOf(name);
    // Unreachable: the required members were checked before.
    if (id === undefined) throw new Refused(TokenVerdict.INVALID, "claim");
    return id;
  };
  const claims: InvocationClaims = Object.freeze({
    iss: text("iss"),
    aud: text("aud"),
    sub: must("sub"),
    jti: text("jti"),
    iat: members.iat as number,
    exp: members.exp as number,
    triggerKind: text("trigger_kind"),
    threadId: must("thread_id"),
    channelId: must("channel_id"),
    spaceId: must("space_id"),
    level: text("level"),
    billingMode: text("billing_mode"),
    toolClass: text("tool_class"),
    permissionVersion: members.permission_version as number,
    runId: uuidOf("run_id"),
    messageId: uuidOf("message_id"),
    interactionId: uuidOf("interaction_id"),
    commandId: uuidOf("command_id"),
  });
  return [claims, members];
}

/**
 * Reads the claims of a token this adapter verified before, without checking it again.
 *
 * Only for tokens the adapter accepted into its inbox or received from `renewInvocationToken`
 * over its authenticated connection; never for a delivered token that has not passed
 * {@link InvocationVerifier.check}.
 *
 * @param token - The token.
 * @returns Its claims.
 * @throws `RangeError` when the token is not a JWS with a readable payload.
 */
export function storedClaims(token: Secret): InvocationClaims {
  const parts = token.reveal().split(".");
  // header, payload, signature
  if (parts.length !== 3) throw new RangeError("not a compact JWS");
  try {
    return decodeClaims(decodeSegment(parts[1] ?? ""))[0];
  } catch {
    throw new RangeError("unreadable claims");
  }
}

function usableKey(jwk: unknown): [string, KeyObject] | undefined {
  // Usable: kty OKP, crv Ed25519, a kid, a 32-byte x in strict base64url, and alg and use,
  // when present, EdDSA and sig. Anything else is skipped, not an error of the whole set.
  if (!isRecord(jwk)) return undefined;
  const { kid, x, alg, use } = jwk;
  if (
    jwk.kty !== "OKP" ||
    jwk.crv !== "Ed25519" ||
    typeof kid !== "string" ||
    kid === "" ||
    typeof x !== "string" ||
    ![undefined, null, "", ALG].includes(alg as string) ||
    ![undefined, null, "", "sig"].includes(use as string)
  )
    return undefined;
  try {
    if (decodeSegment(x).length !== ED25519_KEY_BYTES) return undefined;
    return [
      kid,
      createPublicKey({
        key: { kty: "OKP", crv: "Ed25519", x },
        format: "jwk",
      }),
    ];
  } catch {
    return undefined;
  }
}

function jtiHash(jti: string): string {
  return createHash("sha256")
    .update(jti)
    .digest("hex")
    .slice(0, JTI_HASH_CHARS);
}

function checkExpectation(
  members: StrictObject,
  expect: Expectation,
  claims: InvocationClaims,
): void {
  // Byte-exact comparison of the claim text with the canonical form of the payload's ID, as
  // the reference does; a missing counterpart in the payload fails closed.
  for (const [want, name] of [
    [expect.runId, "run_id"],
    [expect.threadId, "thread_id"],
  ] as const) {
    if (want !== undefined && members[name] !== want)
      throw new Refused(TokenVerdict.INVALID, name, claims);
  }
  if (
    expect.triggerId === undefined ||
    !TRIGGER_CLAIMS.has(expect.triggerClaim)
  )
    throw new Refused(
      TokenVerdict.INVALID,
      "no triggering object in the payload",
      claims,
    );
  if (members[expect.triggerClaim] !== expect.triggerId)
    throw new Refused(TokenVerdict.INVALID, "triggering object", claims);
}

/**
 * The check of 09 4.5 for one agent; safe for concurrent use (checks run one at a time).
 *
 * @example
 * ```ts
 * const verifier = new InvocationVerifier({
 *   instanceId, agentId,
 *   jwks: jwksFetcher(api),
 *   replay: inbox,
 *   allowlist: triggererLoader(api),
 * });
 * await verifier.refresh();
 * const { verdict, claims } = await verifier.check(token, expectationFor(env.type, env), actual);
 * ```
 */
export class InvocationVerifier {
  private readonly iss: string;
  private readonly aud: string;
  private readonly jwks: JwksFetcher;
  private readonly replay: ReplayStore;
  private readonly settings: SdkSettings;
  private readonly clock: Clock;
  private readonly allowlist: TriggererLoader | undefined;
  private readonly logger: Logger;
  private tail: Promise<unknown> = Promise.resolve();
  private keys = new Map<string, KeyObject>();
  private loaded = false;
  private expires = 0;
  private lastForced: number | undefined;
  private fetchCount = 0;
  private forcedCount = 0;
  private triggerers: ReadonlySet<string> | undefined;
  private triggerersVersion = 0;
  private triggerersStale = false;

  /**
   * Creates the verifier; nothing is fetched before the first check or {@link refresh}.
   *
   * @param options - Instance, agent, JWKS fetcher, `jti` store, settings, clock, allow list
   *   loader and logger.
   */
  constructor(options: InvocationVerifierOptions) {
    this.iss = options.instanceId;
    this.aud = options.agentId;
    this.jwks = options.jwks;
    this.replay = options.replay;
    this.settings = resolveSettings(options.settings);
    this.clock = options.clock ?? systemClock;
    this.allowlist = options.allowlist;
    this.logger = options.logger ?? defaultLogger;
  }

  /** How often the JWKS was fetched, forced reloads included. */
  get fetches(): number {
    return this.fetchCount;
  }

  /** How many fetches an unknown `kid` caused. */
  get forcedReloads(): number {
    return this.forcedCount;
  }

  /** Reloads the allow list before the next check (on `access.changed`). */
  accessChanged(): void {
    this.triggerersStale = true;
  }

  /**
   * Fetches the JWKS now, as at adapter start; it counts as a fetch, not a forced reload.
   *
   * @returns A promise that resolves when the key set is replaced.
   * @throws Whatever the fetcher throws, or `TypeError` for a body that is not a key set.
   */
  refresh(): Promise<void> {
    return this.exclusive(() => this.fetch());
  }

  /**
   * Checks a token against the event's payload and the runtime's actual state.
   *
   * @param token - The delivered `invocation_token`.
   * @param expect - What the event's payload says ({@link expectationFor}).
   * @param actual - The actual billing mode and tool class, or a function that returns them
   *   for the claims once signature, `iss`, `aud`, the payload and the allow list passed (the
   *   adapter asks its handler with the verified claims there, so the precedence of the points
   *   stays that of the reference).
   * @returns The verdict and, unless the token form, the signature or the payload was refused
   *   or the key is unknown, the claims.
   * @throws A failure of the JWKS fetcher, of the allow list loader or of `actual`; the event
   *   is then neither accepted nor refused.
   */
  async check(
    token: Secret,
    expect: Expectation,
    actual: ActualSource,
  ): Promise<TokenCheck> {
    try {
      const claims = await this.checkText(token.reveal(), expect, actual);
      return Object.freeze({ verdict: TokenVerdict.VALID, claims });
    } catch (error) {
      if (!(error instanceof Refused)) throw error;
      this.logger.debug("invocation token refused", {
        reason: error.reason,
        verdict: error.verdict,
        ...(error.claims === undefined
          ? {}
          : { jtiHash: jtiHash(error.claims.jti) }),
      });
      return Object.freeze({ verdict: error.verdict, claims: error.claims });
    }
  }

  /** Returns a summary without keys, so the verifier never renders more than its counters. */
  toJSON(): Record<string, number> {
    return { fetches: this.fetchCount, forcedReloads: this.forcedCount };
  }

  private exclusive<T>(fn: () => Promise<T>): Promise<T> {
    const run = this.tail.then(fn, fn);
    this.tail = run.catch(() => undefined);
    return run;
  }

  private async checkText(
    text: string,
    expect: Expectation,
    actual: ActualSource,
  ): Promise<InvocationClaims> {
    const parts = text.split(".");
    if (parts.length !== 3)
      throw new Refused(TokenVerdict.INVALID, "not three segments");
    let head: Uint8Array;
    let body: Uint8Array;
    try {
      head = decodeSegment(parts[0] ?? "");
      body = decodeSegment(parts[1] ?? "");
      decodeSegment(parts[2] ?? "");
    } catch {
      throw new Refused(TokenVerdict.INVALID, "segments");
    }
    const kid = checkHeader(head);
    return this.exclusive(async () => {
      const key = await this.key(kid);
      if (key === undefined) throw new Refused(TokenVerdict.UNKNOWN_KEY, "kid");
      try {
        await compactVerify(text, key, { algorithms: [ALG] });
      } catch {
        throw new Refused(TokenVerdict.INVALID, "signature");
      }
      const [claims, members] = decodeClaims(body);
      if (claims.iss !== this.iss || claims.aud !== this.aud)
        throw new Refused(TokenVerdict.INVALID, "iss or aud", claims);
      checkExpectation(members, expect, claims);
      await this.checkAllowlist(claims);
      const resolved =
        typeof actual === "function" ? await actual(claims) : actual;
      const verdict = compareActual(claims, resolved);
      if (verdict !== TokenVerdict.VALID)
        throw new Refused(verdict, verdict, claims);
      return this.checkTimeAndReplay(claims);
    });
  }

  private async checkTimeAndReplay(
    claims: InvocationClaims,
  ): Promise<InvocationClaims> {
    const leeway = this.settings.invocationTokenLeewayMs;
    const now = this.clock.now();
    if (claims.iat * 1000 > now + leeway || claims.exp <= claims.iat)
      throw new Refused(
        TokenVerdict.INVALID,
        "iat in the future or exp not after iat",
        claims,
      );
    const until = claims.exp * 1000 + leeway;
    if (now >= until)
      throw new Refused(TokenVerdict.EXPIRED, "expired", claims);
    if (await this.replay.jtiSeen(claims.jti, now))
      throw new Refused(TokenVerdict.REPLAY, "replay", claims);
    await this.replay.rememberJti(claims.jti, until);
    return claims;
  }

  private async checkAllowlist(claims: InvocationClaims): Promise<void> {
    if (this.allowlist === undefined) return;
    if (
      this.triggerers === undefined ||
      this.triggerersStale ||
      claims.permissionVersion > this.triggerersVersion
    ) {
      const loaded = await this.allowlist();
      this.triggerers = new Set(
        loaded.principal_ids.map((id) => canonicalUuid(id) ?? id),
      );
      this.triggerersVersion = loaded.permission_version;
      this.triggerersStale = false;
    }
    if (!this.triggerers.has(claims.sub))
      throw new Refused(
        TokenVerdict.INVALID,
        "sub not on the allow list",
        claims,
      );
  }

  private async key(kid: string): Promise<KeyObject | undefined> {
    const now = this.clock.monotonic();
    if (!this.loaded || now >= this.expires) await this.fetch();
    const key = this.keys.get(kid);
    if (key !== undefined) return key;
    // At most one forced reload per adapter.jwks_refresh_min_interval.
    const interval = this.settings.jwksRefreshMinIntervalMs;
    if (this.lastForced !== undefined && now - this.lastForced < interval)
      return undefined;
    this.lastForced = now;
    this.forcedCount++;
    await this.fetch();
    return this.keys.get(kid);
  }

  private async fetch(): Promise<void> {
    this.fetchCount++;
    const [body, maxAgeMs] = await this.jwks();
    const listed = isRecord(body) ? body.keys : undefined;
    if (!Array.isArray(listed))
      throw new TypeError("the JWKS body is not a key set");
    const usable = new Map<string, KeyObject>();
    for (const jwk of listed) {
      const found = usableKey(jwk);
      if (found !== undefined) usable.set(found[0], found[1]);
    }
    this.keys = usable;
    this.loaded = true;
    this.expires = this.clock.monotonic() + Math.max(0, maxAgeMs ?? 0);
  }
}

function maxAgeMs(cacheControl: string | null): number | undefined {
  for (const directive of (cacheControl ?? "").split(",")) {
    const trimmed = directive.trim();
    const at = trimmed.indexOf("=");
    const name = at < 0 ? trimmed : trimmed.slice(0, at);
    if (name.toLowerCase() !== "max-age") continue;
    const value = (at < 0 ? "" : trimmed.slice(at + 1)).replace(/^"+|"+$/g, "");
    return /^\d+(?:\.\d+)?$/.test(value) ? Number(value) * 1000 : undefined;
  }
  return undefined;
}

/**
 * Builds the {@link JwksFetcher} of a client: `getJwks` with the `max-age` of its
 * `Cache-Control` (the server sends `agent.jwks_max_age`).
 *
 * @param client - A client of the public API, for example of `createApiClient`; only `GET` is
 *   used.
 * @returns The fetcher; it throws the typed `ProblemError` of a failed answer.
 *
 * @example
 * ```ts
 * const verifier = new InvocationVerifier({ instanceId, agentId, jwks: jwksFetcher(api), replay });
 * ```
 */
export function jwksFetcher(client: Pick<Client<paths>, "GET">): JwksFetcher {
  return async () => {
    const { data, response } = await unwrapDetailed(
      client.GET("/instance/jwks"),
      "getJwks",
    );
    return [data, maxAgeMs(response.headers.get("Cache-Control"))] as const;
  };
}

/**
 * Builds the {@link TriggererLoader} of a client: `getAgentTriggerers`.
 *
 * @param client - A client of the public API with the agent's credentials; only `GET` is used.
 * @returns The loader; it throws the typed `ProblemError` of a failed answer.
 *
 * @example
 * ```ts
 * const verifier = new InvocationVerifier({ ..., allowlist: triggererLoader(api) });
 * ```
 */
export function triggererLoader(
  client: Pick<Client<paths>, "GET">,
): TriggererLoader {
  return () =>
    unwrap(client.GET("/agents/me/triggerers"), "getAgentTriggerers");
}
