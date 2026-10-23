// SPDX-License-Identifier: Apache-2.0
/**
 * Invocation tokens of the FakeServer: issuer, server-side check and forgeries (private; port
 * of `bothy_sdk/testing/_fake_tokens.py`).
 *
 * The issuer follows M2-06 (docs/spec/05-actions.md 5, docs/spec/09-agents.md 4.5): an
 * Ed25519 JWS with `alg = EdDSA`, `typ = agent-invocation+jwt` and the RFC 7638 thumbprint as
 * `kid`. The check is the server's `Verify`: the first failing rule gives `invalid`; `expired`
 * only when every other rule holds. Signing uses `node:crypto` synchronously, so a delivery
 * decides, issues and records its token in one step like the server's transaction, and a
 * forgery can sign a header no JOSE library would produce (`alg = HS256` over an Ed25519
 * signature).
 *
 * @module
 */

import {
  createHash,
  generateKeyPairSync,
  sign,
  verify,
  type KeyObject,
} from "node:crypto";

import { Secret } from "../../auth/secret.js";
import type { components } from "../../generated/openapi.js";
import { uuidv7 } from "../../ids.js";
import type { ForgedVariant } from "../scenario-server.js";
import { FORGED_VARIANTS } from "../scenario-server.js";
import { b64url, b64urlDecode, compactJson, strictObject } from "./base.js";

/** The `typ` of an invocation token. */
export const TYP = "agent-invocation+jwt";

/** The trigger kinds of 05 5 with the claim that names their object. */
export const TRIGGER_OBJECT = {
  message: "message_id",
  followup: "message_id",
  interaction: "interaction_id",
  command: "command_id",
  autocomplete: "command_id",
} as const;

/** One trigger kind of the claims. */
export type TriggerKind = keyof typeof TRIGGER_OBJECT;

/** The claims of an invocation token (05 5). */
export interface InvocationClaims {
  /** The instance. */
  iss: string;
  /** The agent. */
  aud: string;
  /** The triggering person. */
  sub: string;
  /** The token ID. */
  jti: string;
  /** Issued at, Unix seconds. */
  iat: number;
  /** Expires at, Unix seconds. */
  exp: number;
  /** What triggered the agent. */
  trigger_kind: TriggerKind;
  /** The run; absent for `autocomplete`. */
  run_id?: string;
  /** The message of `message` and `followup`. */
  message_id?: string;
  /** The interaction of `interaction`. */
  interaction_id?: string;
  /** The command of `command` and `autocomplete`. */
  command_id?: string;
  /** The thread (the channel when the trigger has no thread). */
  thread_id: string;
  /** The channel. */
  channel_id: string;
  /** The space. */
  space_id: string;
  /** The access level. */
  level: "read" | "write" | "trigger";
  /** The billing mode. */
  billing_mode: components["schemas"]["BillingMode"];
  /** The tool class. */
  tool_class: components["schemas"]["ToolClass"];
  /** The permission version. */
  permission_version: number;
}

/** The claims before {@link Issuer.issue} adds `iss` and `jti`. */
export type UnsignedClaims = Omit<InvocationClaims, "iss" | "jti">;

/** The verdict of a token check and the claims when the token decoded. */
export interface Checked {
  /** `valid`, `expired` or `invalid`. */
  readonly verdict: "valid" | "expired" | "invalid";
  /** The claims when the signature held. */
  readonly claims: Record<string, unknown> | undefined;
}

const REQUIRED_STRINGS = [
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
const ID_CLAIMS = [
  "sub",
  "jti",
  "run_id",
  "message_id",
  "interaction_id",
  "command_id",
  "thread_id",
  "channel_id",
  "space_id",
] as const;
const ENUMS: Readonly<Record<string, readonly string[]>> = {
  level: ["read", "write", "trigger"],
  billing_mode: ["personal", "organization"],
  tool_class: ["chat", "scoped", "host"],
  trigger_kind: Object.keys(TRIGGER_OBJECT),
};
const FORBIDDEN_HEADER = ["crit", "jku", "x5u", "x5c", "zip"];
const ORDER = [
  "iss",
  "aud",
  "sub",
  "jti",
  "iat",
  "exp",
  "trigger_kind",
  "run_id",
  "message_id",
  "interaction_id",
  "command_id",
  "thread_id",
  "channel_id",
  "space_id",
  "level",
  "billing_mode",
  "tool_class",
  "permission_version",
] as const;
const V7 = /^[0-9a-f]{8}-[0-9a-f]{4}-7[0-9a-f]{3}-[0-9a-f]{4}-[0-9a-f]{12}$/;

function isInt(value: unknown): value is number {
  return typeof value === "number" && Number.isInteger(value);
}

/**
 * Returns the claims shape of the server's check (required members, types, IDs, trigger
 * table).
 *
 * @param claims - The decoded claims.
 * @returns Whether the shape holds.
 */
export function shapeOk(claims: Record<string, unknown>): boolean {
  if (
    REQUIRED_STRINGS.some(
      (n) => typeof claims[n] !== "string" || claims[n] === "",
    )
  )
    return false;
  if (!["iat", "exp", "permission_version"].every((n) => isInt(claims[n])))
    return false;
  for (const name of ID_CLAIMS)
    if (
      name in claims &&
      (typeof claims[name] !== "string" || !V7.test(claims[name]))
    )
      return false;
  for (const [name, values] of Object.entries(ENUMS))
    if (!values.includes(claims[name] as string)) return false;
  const kind = claims.trigger_kind as TriggerKind;
  const present = (
    ["message_id", "interaction_id", "command_id"] as const
  ).filter((n) => n in claims);
  if (present.length !== 1 || present[0] !== TRIGGER_OBJECT[kind]) return false;
  if ("run_id" in claims === (kind === "autocomplete")) return false;
  return (
    (claims.exp as number) > (claims.iat as number) &&
    (claims.permission_version as number) >= 0
  );
}

function signJws(header: object, claims: object, key: KeyObject): string {
  const input = `${b64url(compactJson(header))}.${b64url(compactJson(claims))}`;
  return `${input}.${b64url(sign(null, Buffer.from(input, "ascii"), key))}`;
}

function sorted(value: Record<string, unknown>): Record<string, unknown> {
  return Object.fromEntries(
    Object.keys(value)
      .sort()
      .map((k) => [k, value[k]]),
  );
}

/** The inputs of {@link Issuer.check} besides the token. */
export interface CheckOptions {
  /** The expected `aud`. */
  readonly aud: string;
  /** The wall time, Unix milliseconds. */
  readonly nowMs: number;
  /** `auth.clock_leeway` in milliseconds. */
  readonly leewayMs: number;
  /** The expected `run_id`, if any. */
  readonly runId?: string;
  /** Skip the expiry rule (the start rules judge it themselves). */
  readonly ignoreExpiry?: boolean;
}

/** The instance's invocation key: issues, checks and forges tokens. */
export class Issuer {
  /** The `iss` of every token. */
  readonly instanceId: string;
  /** The RFC 7638 thumbprint of the public key. */
  readonly kid: string;
  private readonly privateKey: KeyObject;
  private readonly publicKey: KeyObject;
  private readonly x: string;

  /**
   * Creates a new Ed25519 key for the instance.
   *
   * @param instanceId - The `iss` of every token.
   */
  constructor(instanceId: string) {
    this.instanceId = instanceId;
    const { privateKey, publicKey } = generateKeyPairSync("ed25519");
    this.privateKey = privateKey;
    this.publicKey = publicKey;
    this.x = String(publicKey.export({ format: "jwk" }).x);
    const canonical = JSON.stringify({ crv: "Ed25519", kty: "OKP", x: this.x });
    this.kid = b64url(createHash("sha256").update(canonical).digest());
  }

  /**
   * Returns the JWKS body with the one active key.
   *
   * @returns The JWKS.
   */
  jwks(): components["schemas"]["Jwks"] {
    return {
      keys: [
        {
          kty: "OKP",
          crv: "Ed25519",
          x: this.x,
          kid: this.kid,
          alg: "EdDSA",
          use: "sig",
        },
      ],
    };
  }

  /**
   * Signs `claims` as a new token, adding `iss` and a fresh `jti`.
   *
   * @param claims - The claims without `iss` and `jti`.
   * @returns The compact token and its full claims.
   */
  issue(claims: UnsignedClaims): [string, InvocationClaims] {
    const full: InvocationClaims = {
      iss: this.instanceId,
      jti: uuidv7(),
      ...claims,
    };
    const record = full as unknown as Record<string, unknown>;
    const ordered = Object.fromEntries(
      ORDER.filter((n) => n in full).map((n) => [n, record[n]]),
    ) as unknown as InvocationClaims;
    return [
      signJws(
        { alg: "EdDSA", typ: TYP, kid: this.kid },
        ordered,
        this.privateKey,
      ),
      ordered,
    ];
  }

  /**
   * The server's `Verify`: header, key, signature, shape, expected values, expiry.
   *
   * @param token - The compact token.
   * @param options - Expected values and the time.
   * @returns `valid`, `expired` (only when every other rule holds) or `invalid`.
   */
  check(token: string, options: CheckOptions): Checked {
    const claims = this.decode(token);
    if (claims === undefined || !shapeOk(claims))
      return { verdict: "invalid", claims: undefined };
    if (claims.iss !== this.instanceId || claims.aud !== options.aud)
      return { verdict: "invalid", claims };
    if (options.runId !== undefined && claims.run_id !== options.runId)
      return { verdict: "invalid", claims };
    const stamp = options.nowMs / 1000;
    const leeway = options.leewayMs / 1000;
    if ((claims.iat as number) > stamp + leeway)
      return { verdict: "invalid", claims };
    if (!options.ignoreExpiry && stamp >= (claims.exp as number) + leeway)
      return { verdict: "expired", claims };
    return { verdict: "valid", claims };
  }

  private decode(token: string): Record<string, unknown> | undefined {
    const parts = token.split(".");
    if (parts.length !== 3 || parts.some((p) => p === "")) return undefined;
    let header: Record<string, unknown>;
    let claims: Record<string, unknown>;
    let signature: Uint8Array;
    try {
      header = strictObject(b64urlDecode(parts[0] ?? ""));
      claims = strictObject(b64urlDecode(parts[1] ?? ""));
      signature = b64urlDecode(parts[2] ?? "");
    } catch {
      return undefined;
    }
    if (header.alg !== "EdDSA" || header.typ !== TYP) return undefined;
    if (
      FORBIDDEN_HEADER.some((n) => n in header) ||
      !header.kid ||
      header.kid !== this.kid
    )
      return undefined;
    if (signature.length !== 64) return undefined;
    const input = Buffer.from(`${parts[0]}.${parts[1]}`, "ascii");
    return verify(null, input, this.publicKey, signature) ? claims : undefined;
  }

  /**
   * Builds one of the forged tokens of AG-08 from the claims of a delivered token.
   *
   * Like the server's tests, header and claims are encoded with sorted members and always
   * signed with Ed25519: `bad_signature` with a new key under the same `kid`, `alg_hs256` with
   * `alg = HS256`, `typ_access` with `typ = at+jwt` and `aud_other_agent` with the other
   * agent's ID, each validly signed otherwise.
   *
   * @param claims - The claims of a delivered (or would-be) token.
   * @param variant - The forgery.
   * @param otherAgentId - Texter's ID for `aud_other_agent`.
   * @returns The forged token.
   * @throws `RangeError` on an unknown variant.
   */
  forge(
    claims: InvocationClaims,
    variant: ForgedVariant,
    otherAgentId: string,
  ): Secret {
    const header: Record<string, unknown> = {
      alg: "EdDSA",
      kid: this.kid,
      typ: TYP,
    };
    const body = sorted({ ...claims });
    let key = this.privateKey;
    if (variant === "bad_signature")
      key = generateKeyPairSync("ed25519").privateKey;
    else if (variant === "alg_hs256") header.alg = "HS256";
    else if (variant === "typ_access") header.typ = "at+jwt";
    else if (variant === "aud_other_agent") body.aud = otherAgentId;
    else throw new RangeError(`unknown forge variant: ${String(variant)}`);
    return new Secret(signJws(sorted(header), body, key));
  }
}

/** The variants {@link Issuer.forge} builds (the shared list of the scenarios). */
export const FORGE_VARIANTS: readonly ForgedVariant[] = FORGED_VARIANTS;
