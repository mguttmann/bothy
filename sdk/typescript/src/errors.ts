// SPDX-License-Identifier: Apache-2.0
/**
 * Typed errors after the problem format of RFC 9457 (docs/spec/06-api.md 5).
 *
 * {@link ProblemError} carries what a `Problem` body says; one subclass per HTTP status family
 * and, below them, one class per code the hand layer reacts to. An unknown code keeps its
 * string and maps to the class of its status ("Clients treat unknown codes by HTTP status",
 * schema `Problem`, R-API-04). A `404` means "not there or not visible" and is never treated as
 * surely missing (docs/spec/11-erweiterungen.md 11.2). An error message shows the operation,
 * status, code, reason and `correlation_id`, never a body, a token, a proof or a message
 * content (SEC-LOG-2). Browser-safe: no `node:` import.
 *
 * @example
 * ```ts
 * const err = problemError(409, { code: "already_started", status: 409, title: "x" });
 * err instanceof AlreadyStarted; // true
 * err instanceof ConflictError; // true
 * ```
 *
 * @module
 */

import type { components } from "./generated/openapi.js";

/** A field error of `400 validation_failed` (the generated schema `FieldError`). */
export type ProblemFieldError = components["schemas"]["FieldError"];

/**
 * Every code of `x-problem-codes` of the schema `Problem` in docs/api/openapi.yaml with its
 * status; `test/errors.test.ts` holds this table equal to the specification.
 */
export const PROBLEM_STATUS: Readonly<Record<string, number>> = Object.freeze({
  cursor_invalid: 400,
  malformed_request: 400,
  validation_failed: 400,
  webhook_host_not_allowed: 400,
  invalid_dpop_proof: 401,
  session_revoked: 401,
  step_up_required: 401,
  token_expired: 401,
  unauthenticated: 401,
  use_dpop_nonce: 401,
  agent_invocation_denied: 403,
  channel_read_only: 403,
  device_pending: 403,
  feature_disabled: 403,
  forbidden: 403,
  invocation_token_invalid: 403,
  scope_missing: 403,
  terminal_denied: 403,
  not_found: 404,
  method_not_allowed: 405,
  already_started: 409,
  base_version_conflict: 409,
  billing_mode_mismatch: 409,
  component_used: 409,
  conflict: 409,
  file_blocked: 409,
  file_not_ready: 409,
  fingerprint_mismatch: 409,
  host_unavailable: 409,
  i1_violation: 409,
  i6_violation: 409,
  invalid_state_transition: 409,
  last_owner: 409,
  operator_mismatch: 409,
  request_in_progress: 409,
  title_manual: 409,
  tool_class_mismatch: 409,
  gone: 410,
  message_deleted: 410,
  resync_required: 410,
  version_conflict: 412,
  payload_too_large: 413,
  unsupported_media_type: 415,
  range_not_satisfiable: 416,
  idempotency_key_reuse: 422,
  precondition_required: 428,
  rate_limited: 429,
  internal_error: 500,
  maintenance: 503,
  setup_required: 503,
  unavailable: 503,
});

/** The codes of `x-problem-codes` this SDK knows; any other code is classified by status. */
export const KNOWN_PROBLEM_CODES: readonly string[] = Object.freeze(
  Object.keys(PROBLEM_STATUS),
);

/** The code of a problem whose body carries none (or is not a problem object at all). */
const UNKNOWN_CODE = "unknown";

/** The members of a {@link ProblemError} besides status and code. */
export interface ProblemDetails {
  /** The finer reason, for example `expired` of `invocation_token_invalid`. */
  readonly reason?: string | undefined;
  /** The short title of the problem. */
  readonly title?: string | undefined;
  /** The detail text; kept on the error, never part of its message. */
  readonly detail?: string | undefined;
  /** The correlation ID for support. */
  readonly correlationId?: string | undefined;
  /** The `instance` URI of the problem. */
  readonly instance?: string | undefined;
  /** Field errors of `400 validation_failed`. */
  readonly errors?: readonly ProblemFieldError[] | undefined;
  /** The current resource of a conflict (for example the run of `409 base_version_conflict`). */
  readonly current?: Readonly<Record<string, unknown>> | undefined;
  /** The wait of `Retry-After` in milliseconds, if the answer had one. */
  readonly retryAfterMs?: number | undefined;
  /** The operation that failed (an `operationId` or `METHOD /path/template`). */
  readonly operation?: string | undefined;
}

/**
 * A failed API call, decoded from its `Problem` body; prefer {@link problemError} to build one.
 *
 * @example
 * ```ts
 * try {
 *   await unwrap(api.POST("/runs/{run_id}/start", init), "startRun");
 * } catch (err) {
 *   if (err instanceof ProblemError && err.code === "already_started") return;
 *   throw err;
 * }
 * ```
 */
export class ProblemError extends Error {
  /** The HTTP status (`0` for a gateway error frame of an unknown code). */
  readonly status: number;
  /** The stable machine code as sent; may be a code this SDK does not know. */
  readonly code: string;
  /** The finer reason, for example `expired`. */
  readonly reason: string | undefined;
  /** The short title. */
  readonly title: string;
  /** The detail text; never part of the error `message`. */
  readonly detail: string | undefined;
  /** The correlation ID for support. */
  readonly correlationId: string | undefined;
  /** The `instance` URI. */
  readonly instance: string | undefined;
  /** Field errors of `400 validation_failed`. */
  readonly errors: readonly ProblemFieldError[];
  /** The current resource of a conflict, as a raw object. */
  readonly current: Readonly<Record<string, unknown>> | undefined;
  /** The wait of `Retry-After` in milliseconds. */
  readonly retryAfterMs: number | undefined;
  /** The operation that failed. */
  readonly operation: string | undefined;

  /**
   * Creates the error.
   *
   * @param status - The HTTP status.
   * @param code - The problem code.
   * @param details - The other members of the problem.
   */
  constructor(status: number, code: string, details: ProblemDetails = {}) {
    super(problemText(status, code, details));
    this.name = new.target.name;
    this.status = status;
    this.code = code;
    this.reason = details.reason;
    this.title = details.title ?? "";
    this.detail = details.detail;
    this.correlationId = details.correlationId;
    this.instance = details.instance;
    this.errors = Object.freeze([...(details.errors ?? [])]);
    this.current = details.current;
    this.retryAfterMs = details.retryAfterMs;
    this.operation = details.operation;
  }

  /**
   * Returns the members that are safe to log (no detail, no title, no current document).
   *
   * @returns The name, status, code, reason, correlation ID and operation.
   */
  toJSON(): Record<string, unknown> {
    return {
      name: this.name,
      status: this.status,
      code: this.code,
      reason: this.reason,
      correlationId: this.correlationId,
      operation: this.operation,
    };
  }

  /**
   * Returns the stack (name and message) for `util.inspect` and `console.log`, so the detail,
   * the title and the current document of the server never reach a log line.
   */
  [Symbol.for("nodejs.util.inspect.custom")](): string {
    return this.stack ?? `${this.name}: ${this.message}`;
  }
}

function problemText(
  status: number,
  code: string,
  details: ProblemDetails,
): string {
  const parts = [`${status} ${code}`];
  if (details.reason) parts.push(`reason=${details.reason}`);
  if (details.correlationId)
    parts.push(`correlation_id=${details.correlationId}`);
  const text = parts.join(" ");
  return details.operation ? `${details.operation}: ${text}` : text;
}

/** HTTP 400. */
export class BadRequestError extends ProblemError {}

/** HTTP 401. */
export class UnauthenticatedError extends ProblemError {}

/** HTTP 403. */
export class ForbiddenError extends ProblemError {}

/**
 * HTTP 404: the resource is not there or not visible.
 *
 * The server answers `404` for a target the caller may not see as for one that does not exist
 * (docs/spec/07-rechte.md), so this error never proves that a resource is missing.
 */
export class NotFoundError extends ProblemError {}

/** HTTP 409. */
export class ConflictError extends ProblemError {}

/** HTTP 410. */
export class GoneError extends ProblemError {}

/** HTTP 412. */
export class PreconditionFailedError extends ProblemError {}

/** HTTP 429. */
export class RateLimitedError extends ProblemError {}

/** HTTP 5xx. */
export class ServerError extends ProblemError {}

/** `400 validation_failed`; {@link ProblemError.errors} names the fields (`pointer`, `code`). */
export class ValidationFailed extends BadRequestError {}

/** `401 use_dpop_nonce` after the one repeat with the new nonce. */
export class UseDpopNonce extends UnauthenticatedError {}

/**
 * `403 invocation_token_invalid` with its {@link ProblemError.reason}: `expired`,
 * `superseded`, `invalid` or `run_inactive` (docs/spec/09-agenten.md 4.5, 07 SR9).
 */
export class InvocationTokenInvalid extends ForbiddenError {}

/** `403 agent_invocation_denied`: the renewal check failed and the server ended the run. */
export class AgentInvocationDenied extends ForbiddenError {}

/** `409 already_started`: the run started elsewhere or is no longer waiting. */
export class AlreadyStarted extends ConflictError {}

/** `409 billing_mode_mismatch`: the reported mode differs; the server failed the run. */
export class BillingModeMismatch extends ConflictError {}

/** `409 tool_class_mismatch`: the reported tool class is above the claim. */
export class ToolClassMismatch extends ConflictError {}

/**
 * `409 base_version_conflict`; {@link BaseVersionConflict.run} is the current document.
 */
export class BaseVersionConflict extends ConflictError {
  /** The run of `current` as the generated type, if the answer had it. */
  get run(): components["schemas"]["Run"] | undefined {
    return this.current as components["schemas"]["Run"] | undefined;
  }
}

/** `412 version_conflict`: `If-Match` names an outdated version. */
export class VersionConflict extends PreconditionFailedError {}

/** `409 request_in_progress`: the same `Idempotency-Key` is still being processed. */
export class RequestInProgress extends ConflictError {}

/** `503 unavailable`. */
export class Unavailable extends ServerError {}

/**
 * An `error` frame of the gateway for one op (docs/spec/04-sync-protokoll.md 6.2).
 *
 * The status comes from {@link PROBLEM_STATUS}, `0` for an unknown code.
 */
export class GatewayOpError extends ProblemError {
  /** The op that failed. */
  readonly op: string;
  /** The `ref` the client sent with the op, if any. */
  readonly ref: string | undefined;

  /**
   * Creates the error.
   *
   * @param op - The op that failed.
   * @param code - The problem code of the frame.
   * @param options - The optional `detail` and `ref` of the frame.
   */
  constructor(
    op: string,
    code: string,
    options: { detail?: string | undefined; ref?: string | undefined } = {},
  ) {
    super(PROBLEM_STATUS[code] ?? 0, code, {
      detail: options.detail,
      operation: `gateway op ${op}`,
    });
    this.op = op;
    this.ref = options.ref;
  }
}

/** The constructor of a {@link ProblemError} class. */
export type ProblemErrorClass = new (
  status: number,
  code: string,
  details?: ProblemDetails,
) => ProblemError;

const BY_CODE: Readonly<Record<string, ProblemErrorClass>> = Object.freeze({
  validation_failed: ValidationFailed,
  use_dpop_nonce: UseDpopNonce,
  invocation_token_invalid: InvocationTokenInvalid,
  agent_invocation_denied: AgentInvocationDenied,
  already_started: AlreadyStarted,
  billing_mode_mismatch: BillingModeMismatch,
  tool_class_mismatch: ToolClassMismatch,
  base_version_conflict: BaseVersionConflict,
  version_conflict: VersionConflict,
  request_in_progress: RequestInProgress,
  unavailable: Unavailable,
});

const BY_STATUS: Readonly<Record<number, ProblemErrorClass>> = Object.freeze({
  400: BadRequestError,
  401: UnauthenticatedError,
  403: ForbiddenError,
  404: NotFoundError,
  409: ConflictError,
  410: GoneError,
  412: PreconditionFailedError,
  429: RateLimitedError,
});

/**
 * Returns the class for `code` when it belongs to `status`, else the class of the status
 * family.
 *
 * @param status - The HTTP status.
 * @param code - The problem code.
 * @returns The most specific class.
 */
export function errorClass(status: number, code: string): ProblemErrorClass {
  const byCode = Object.hasOwn(BY_CODE, code) ? BY_CODE[code] : undefined;
  if (byCode !== undefined && PROBLEM_STATUS[code] === status) return byCode;
  if (status >= 500) return ServerError;
  return BY_STATUS[status] ?? ProblemError;
}

function text(value: unknown): string | undefined {
  return typeof value === "string" && value !== "" ? value : undefined;
}

function isRecord(value: unknown): value is Record<string, unknown> {
  return typeof value === "object" && value !== null && !Array.isArray(value);
}

function fieldErrors(value: unknown): ProblemFieldError[] {
  if (!Array.isArray(value)) return [];
  return value.filter(
    (item): item is ProblemFieldError =>
      isRecord(item) &&
      typeof item.pointer === "string" &&
      typeof item.code === "string",
  );
}

/**
 * Builds the typed error of a failed answer.
 *
 * @param status - The HTTP status.
 * @param body - The decoded body; anything that is not a problem object (an HTML page of a
 *   proxy, an empty body) gives the code `unknown` and the class of the status.
 * @param options - `retryAfterMs` of `Retry-After` and the `operation` for the message.
 * @returns The error, not thrown.
 *
 * @example
 * ```ts
 * problemError(404, undefined, { operation: "getRun" }).message; // "getRun: 404 unknown"
 * ```
 */
export function problemError(
  status: number,
  body: unknown,
  options: {
    retryAfterMs?: number | undefined;
    operation?: string | undefined;
  } = {},
): ProblemError {
  const data = isRecord(body) ? body : {};
  const code = text(data.code) ?? UNKNOWN_CODE;
  const current = data.current;
  const Cls = errorClass(status, code);
  return new Cls(status, code, {
    reason: text(data.reason),
    title: text(data.title),
    detail: text(data.detail),
    correlationId: text(data.correlation_id),
    instance: text(data.instance),
    errors: fieldErrors(data.errors),
    current: isRecord(current) ? current : undefined,
    retryAfterMs: options.retryAfterMs,
    operation: options.operation,
  });
}

/**
 * Tells whether `err` is a `404`.
 *
 * A `404` means "not there or not visible": the server answers it for a target the caller may
 * not see as for one that does not exist, so it never proves that a resource is missing.
 *
 * @param err - Any thrown value.
 * @returns Whether it is a {@link ProblemError} with status `404`.
 */
export function isNotFound(err: unknown): err is ProblemError {
  return err instanceof ProblemError && err.status === 404;
}

/**
 * A request that failed on the network after the retries: no answer arrived.
 *
 * The message names the operation (`METHOD /path/template`) only, never the URL, which can
 * carry a token (`/hooks/{webhook_id}/{token}`).
 */
export class TransportError extends Error {
  /** The operation, `METHOD /path/template`. */
  readonly operation: string;

  /**
   * Creates the error.
   *
   * @param operation - The operation, `METHOD /path/template`.
   * @param options - The `cause` (the error of `fetch`).
   * @param message - The message; defaults to `transport error on <operation>`.
   */
  constructor(
    operation: string,
    options: { cause?: unknown } = {},
    message?: string,
  ) {
    super(message ?? `transport error on ${operation}`, options);
    this.name = new.target.name;
    this.operation = operation;
  }
}

/** An attempt that ran out of `sdk.http_timeout` (the last one, after the retries). */
export class TimeoutError extends TransportError {
  /** The bound of one attempt in milliseconds. */
  readonly timeoutMs: number;

  /**
   * Creates the error.
   *
   * @param operation - The operation, `METHOD /path/template`.
   * @param timeoutMs - The bound of one attempt in milliseconds.
   * @param options - The `cause`.
   */
  constructor(
    operation: string,
    timeoutMs: number,
    options: { cause?: unknown } = {},
  ) {
    super(operation, options, `timeout after ${timeoutMs} ms on ${operation}`);
    this.timeoutMs = timeoutMs;
  }
}
