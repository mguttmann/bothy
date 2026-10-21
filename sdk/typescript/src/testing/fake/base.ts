// SPDX-License-Identifier: Apache-2.0
/**
 * Shared pieces of the FakeServer: problems, times, timings and JSON helpers (private; port of
 * `bothy_sdk/testing/_fake_base.py`).
 *
 * @module
 */

import type { components } from "../../generated/openapi.js";
import { uuidv7 } from "../../ids.js";

/** A problem body of the API. */
export type ProblemBody = components["schemas"]["Problem"];
/** One field error of `400 validation_failed`. */
export type FieldErrorBody = components["schemas"]["FieldError"];
/** A JSON object of unknown members. */
export type JsonObject = components["schemas"]["JsonObject"];

/** The server keys the FakeServer plays. */
export const TIMING_KEYS = [
  "agent.invocation_token_ttl",
  "auth.clock_leeway",
  "agent.ack_timeout",
  "agent.ack_persist_interval",
  "sync.heartbeat_interval",
  "sync.resume_window",
  "agent.max_inflight_events",
  "command.autocomplete_timeout",
  "agent.jwks_max_age",
  "agent.cancel_grace",
  "auth.dpop_nonce_ttl",
  "auth.dpop_proof_max_age",
] as const;

/** One of {@link TIMING_KEYS}. */
export type TimingKey = (typeof TIMING_KEYS)[number];

/** The values of the server keys: durations in milliseconds, `agent.max_inflight_events` a count. */
export type Timings = Readonly<Record<TimingKey, number>>;

/** The defaults of the FakeServer's tests, the same values as the Python FakeServer's. */
export const DEFAULT_TIMINGS: Timings = Object.freeze({
  "agent.invocation_token_ttl": 60_000,
  "auth.clock_leeway": 30_000,
  "agent.ack_timeout": 30_000,
  "agent.ack_persist_interval": 1_000,
  "sync.heartbeat_interval": 30_000,
  "sync.resume_window": 300_000,
  "agent.max_inflight_events": 100,
  "command.autocomplete_timeout": 3_000,
  "agent.jwks_max_age": 300_000,
  "agent.cancel_grace": 30_000,
  "auth.dpop_nonce_ttl": 300_000,
  "auth.dpop_proof_max_age": 60_000,
});

/**
 * Merges `overrides` into the default timings.
 *
 * @param overrides - Server keys to replace.
 * @returns The complete, frozen timings.
 * @throws `RangeError` when a key is not one the FakeServer plays.
 */
export function timingsWithDefaults(
  overrides: Readonly<Record<string, number>> = {},
): Timings {
  const merged: Record<string, number> = { ...DEFAULT_TIMINGS };
  for (const [key, value] of Object.entries(overrides)) {
    if (!(key in DEFAULT_TIMINGS))
      throw new RangeError(`unknown FakeServer timing: ${key}`);
    merged[key] = value;
  }
  return Object.freeze(merged) as Timings;
}

// The status of every problem code the FakeServer answers (x-problem-codes of docs/api/openapi.yaml).
const STATUS: Readonly<Record<string, number>> = {
  malformed_request: 400,
  validation_failed: 400,
  invalid_dpop_proof: 401,
  unauthenticated: 401,
  use_dpop_nonce: 401,
  agent_invocation_denied: 403,
  forbidden: 403,
  invocation_token_invalid: 403,
  not_found: 404,
  already_started: 409,
  base_version_conflict: 409,
  billing_mode_mismatch: 409,
  conflict: 409,
  invalid_state_transition: 409,
  request_in_progress: 409,
  tool_class_mismatch: 409,
  gone: 410,
  resync_required: 410,
  version_conflict: 412,
  idempotency_key_reuse: 422,
  precondition_required: 428,
  rate_limited: 429,
  internal_error: 500,
};

const TITLES: Readonly<Record<string, string>> = {
  validation_failed: "Validation failed",
  unauthenticated: "Unauthenticated",
  use_dpop_nonce: "Use DPoP nonce",
  invalid_dpop_proof: "Invalid DPoP proof",
  invocation_token_invalid: "Invocation token invalid",
  agent_invocation_denied: "Agent invocation denied",
  forbidden: "Forbidden",
  not_found: "Not found",
  already_started: "Already started",
  invalid_state_transition: "Invalid state transition",
  conflict: "Conflict",
  base_version_conflict: "Base version conflict",
  billing_mode_mismatch: "Billing mode mismatch",
  tool_class_mismatch: "Tool class mismatch",
  version_conflict: "Version conflict",
  precondition_required: "Precondition required",
  idempotency_key_reuse: "Idempotency key reuse",
  request_in_progress: "Request in progress",
  rate_limited: "Rate limited",
  resync_required: "Resync required",
  gone: "Gone",
  internal_error: "Internal error",
  bad_gateway: "Bad gateway",
};

/** The optional members of a {@link FakeProblem}. */
export interface FakeProblemOptions {
  /** The HTTP status; the code's status when absent. */
  readonly status?: number;
  /** The detail text. */
  readonly detail?: string;
  /** The finer reason (for example `superseded`). */
  readonly reason?: string;
  /** The field errors of `400 validation_failed`. */
  readonly errors?: readonly FieldErrorBody[];
  /** The current document of `409 base_version_conflict`. */
  readonly current?: JsonObject;
  /** Extra response headers. */
  readonly headers?: Readonly<Record<string, string>>;
}

/** An error answer in the problem format: thrown by a rule, rendered by the router. */
export class FakeProblem extends Error {
  /** The problem code. */
  readonly code: string;
  /** The HTTP status. */
  readonly status: number;
  /** The detail text. */
  readonly detail: string | undefined;
  /** The finer reason. */
  readonly reason: string | undefined;
  /** The field errors. */
  readonly errors: readonly FieldErrorBody[] | undefined;
  /** The current document. */
  readonly current: JsonObject | undefined;
  /** Extra response headers. */
  readonly headers: Readonly<Record<string, string>>;

  /**
   * Creates the problem.
   *
   * @param code - The problem code.
   * @param options - The optional members.
   */
  constructor(code: string, options: FakeProblemOptions = {}) {
    super(code);
    this.code = code;
    this.status = options.status ?? STATUS[code] ?? 500;
    this.detail = options.detail;
    this.reason = options.reason;
    this.errors = options.errors;
    this.current = options.current;
    this.headers = options.headers ?? {};
  }

  /**
   * Returns the problem body without empty members.
   *
   * @param instance - The request path.
   * @returns The body.
   */
  body(instance: string): ProblemBody {
    const out: ProblemBody = {
      type: `/api/v1/problems/${this.code}`,
      title: TITLES[this.code] ?? capitalize(this.code.replaceAll("_", " ")),
      status: this.status,
      code: this.code,
      correlation_id: uuidv7(),
      instance,
    };
    if (this.detail) out.detail = this.detail;
    if (this.errors?.length) out.errors = [...this.errors];
    if (this.current && Object.keys(this.current).length)
      out.current = this.current;
    if (this.reason) out.reason = this.reason;
    return out;
  }
}

function capitalize(text: string): string {
  return text.charAt(0).toUpperCase() + text.slice(1).toLowerCase();
}

/**
 * Returns a `400 validation_failed` with one field error.
 *
 * @param pointer - The JSON pointer of the member.
 * @param code - The field error code.
 * @param detail - An optional detail text.
 * @returns The problem.
 */
export function fieldError(
  pointer: string,
  code: string,
  detail = "",
): FakeProblem {
  const error: FieldErrorBody = detail
    ? { pointer, code, detail }
    : { pointer, code };
  return new FakeProblem("validation_failed", { errors: [error] });
}

/**
 * Returns UTC time with milliseconds and `Z`, as the run views of the server write it.
 *
 * @param ms - Unix milliseconds.
 * @returns The timestamp text.
 */
export function iso(ms: number): string {
  return new Date(ms).toISOString();
}

/**
 * Encodes base64url without padding.
 *
 * @param data - Bytes or UTF-8 text.
 * @returns The encoding.
 */
export function b64url(data: Uint8Array | string): string {
  return Buffer.from(
    typeof data === "string" ? new TextEncoder().encode(data) : data,
  ).toString("base64url");
}

const B64URL = /^[A-Za-z0-9_-]*$/;

/**
 * Decodes strict base64url without padding.
 *
 * @param text - The encoding.
 * @returns The bytes.
 * @throws `SyntaxError` on padding, a character outside the alphabet or an impossible length.
 */
export function b64urlDecode(text: string): Uint8Array {
  if (!B64URL.test(text) || text.length % 4 === 1)
    throw new SyntaxError("not base64url");
  return new Uint8Array(Buffer.from(text, "base64url"));
}

/**
 * Encodes JSON without spaces, the way the server encodes JOSE parts.
 *
 * @param value - The value.
 * @returns The UTF-8 bytes.
 */
export function compactJson(value: unknown): Uint8Array {
  return new TextEncoder().encode(JSON.stringify(value));
}

/**
 * Encodes JSON with sorted object members (the `sort_keys` form of the Python fake).
 *
 * @param value - The value.
 * @returns The text.
 */
export function canonicalJson(value: unknown): string {
  return JSON.stringify(value, (_key, v: unknown) =>
    isPlainObject(v)
      ? Object.fromEntries(
          Object.keys(v)
            .sort()
            .map((k) => [k, v[k]]),
        )
      : v,
  );
}

/**
 * Tells whether `value` is a JSON object (not an array, not `null`).
 *
 * @param value - Any value.
 * @returns Whether it is a plain object.
 */
export function isPlainObject(
  value: unknown,
): value is Record<string, unknown> {
  return typeof value === "object" && value !== null && !Array.isArray(value);
}

/**
 * Decodes a JSON object and refuses duplicate members.
 *
 * @param raw - UTF-8 bytes or text.
 * @returns The object.
 * @throws `SyntaxError` when it is not a JSON object or a member appears twice.
 */
export function strictObject(
  raw: Uint8Array | string,
): Record<string, unknown> {
  const text =
    typeof raw === "string"
      ? raw
      : new TextDecoder("utf-8", { fatal: true }).decode(raw);
  const value: unknown = JSON.parse(text);
  if (!isPlainObject(value)) throw new SyntaxError("not an object");
  rejectDuplicates(text);
  return value;
}

// Walks valid JSON text and throws when one object names a member twice (JSON.parse keeps the last).
function rejectDuplicates(text: string): void {
  const stack: (Set<string> | undefined)[] = [];
  for (let i = 0; i < text.length; i++) {
    const ch = text[i];
    if (ch === "{") stack.push(new Set());
    else if (ch === "[") stack.push(undefined);
    else if (ch === "}" || ch === "]") stack.pop();
    else if (ch === '"') {
      let end = i + 1;
      while (text[end] !== '"') end += text[end] === "\\" ? 2 : 1;
      const keys = stack.at(-1);
      let next = end + 1;
      while (/\s/.test(text[next] ?? "")) next++;
      if (keys !== undefined && text[next] === ":") {
        const key = JSON.parse(text.slice(i, end + 1)) as string;
        if (keys.has(key)) throw new SyntaxError("duplicate member");
        keys.add(key);
      }
      i = end;
    }
  }
}

const UUID_HEX = /^[0-9a-f]{32}$/i;

/**
 * Returns the canonical lower-case form of a UUID in any form Python's `UUID()` reads
 * (hyphens, braces, `urn:uuid:`), or `undefined`.
 *
 * @param text - The candidate.
 * @returns The canonical UUID, or `undefined` when `text` is not one.
 */
export function canonicalUuid(text: string): string | undefined {
  const hex = text
    .replace("urn:", "")
    .replace("uuid:", "")
    .replace(/^\{+|\}+$/g, "")
    .replaceAll("-", "");
  if (!UUID_HEX.test(hex)) return undefined;
  const h = hex.toLowerCase();
  return `${h.slice(0, 8)}-${h.slice(8, 12)}-${h.slice(12, 16)}-${h.slice(16, 20)}-${h.slice(20)}`;
}

/**
 * Tells whether `text` is a UUIDv7 (version 7, RFC 4122 variant) in any form `UUID()` reads.
 *
 * @param text - The candidate.
 * @returns Whether it is one.
 */
export function isUuid7(text: string): boolean {
  const canonical = canonicalUuid(text);
  return (
    canonical !== undefined &&
    canonical[14] === "7" &&
    "89ab".includes(canonical[19] ?? "")
  );
}
