// SPDX-License-Identifier: Apache-2.0
//
// Typed errors after the problem format (docs/spec/06-api.md 5, R-API-04): every code of
// `x-problem-codes` is known with its status, an unknown code keeps its string and is classified
// by status, and an error text never carries a body, a token or a message content (SEC-LOG-2).

import { readFileSync } from "node:fs";
import { inspect } from "node:util";
import { parse } from "yaml";
import { describe, expect, it } from "vitest";

import {
  AgentInvocationDenied,
  AlreadyStarted,
  BadRequestError,
  BaseVersionConflict,
  BillingModeMismatch,
  ConflictError,
  ForbiddenError,
  GatewayOpError,
  GoneError,
  InvocationTokenInvalid,
  KNOWN_PROBLEM_CODES,
  NotFoundError,
  PROBLEM_STATUS,
  PreconditionFailedError,
  ProblemError,
  RateLimitedError,
  RequestInProgress,
  ServerError,
  TimeoutError,
  ToolClassMismatch,
  TransportError,
  UnauthenticatedError,
  Unavailable,
  UseDpopNonce,
  ValidationFailed,
  VersionConflict,
  errorClass,
  isNotFound,
  problemError,
} from "../src/errors.js";

const CORRELATION = "0192f4a0-7b1c-7000-8a00-000000000009";

interface ProblemCode {
  code: string;
  status: number;
}

// Parsed once at import: the specification is large.
const SPEC = parse(
  readFileSync(
    new URL("../../../docs/api/openapi.yaml", import.meta.url),
    "utf8",
  ),
) as {
  components: { schemas: { Problem: { "x-problem-codes": ProblemCode[] } } };
};

function specProblemCodes(): ProblemCode[] {
  return SPEC.components.schemas.Problem["x-problem-codes"];
}

function problem(
  status: number,
  code: string,
  extra: Record<string, unknown> = {},
): Record<string, unknown> {
  return {
    type: "about:blank",
    title: code,
    status,
    code,
    correlation_id: CORRELATION,
    ...extra,
  };
}

describe("problem codes", () => {
  it("KNOWN_PROBLEM_CODES and PROBLEM_STATUS equal x-problem-codes of openapi.yaml", () => {
    const codes = specProblemCodes();
    expect([...KNOWN_PROBLEM_CODES].sort()).toEqual(
      codes.map((c) => c.code).sort(),
    );
    expect(PROBLEM_STATUS).toEqual(
      Object.fromEntries(codes.map((c) => [c.code, c.status])),
    );
  });

  it("every specific class belongs to a code of the spec with its status", () => {
    const cases: [string, unknown][] = [
      ["validation_failed", ValidationFailed],
      ["use_dpop_nonce", UseDpopNonce],
      ["invocation_token_invalid", InvocationTokenInvalid],
      ["agent_invocation_denied", AgentInvocationDenied],
      ["already_started", AlreadyStarted],
      ["billing_mode_mismatch", BillingModeMismatch],
      ["tool_class_mismatch", ToolClassMismatch],
      ["base_version_conflict", BaseVersionConflict],
      ["version_conflict", VersionConflict],
      ["request_in_progress", RequestInProgress],
      ["unavailable", Unavailable],
    ];
    for (const [code, cls] of cases) {
      const status = PROBLEM_STATUS[code];
      expect(status, code).toBeTypeOf("number");
      if (status === undefined) continue;
      expect(errorClass(status, code)).toBe(cls);
      expect(problemError(status, problem(status, code))).toBeInstanceOf(cls);
    }
  });

  it("an unknown code keeps its string and is classified by status", () => {
    const err = problemError(409, problem(409, "brand_new_code"));
    expect(err.constructor).toBe(ConflictError);
    expect(err.code).toBe("brand_new_code");
    expect(problemError(502, problem(502, "bad_gateway"))).toBeInstanceOf(
      ServerError,
    );
    expect(problemError(418, {}).constructor).toBe(ProblemError);
    expect(problemError(418, {}).code).toBe("unknown");
    // A known code with a status that does not belong to it is classified by status.
    expect(errorClass(400, "already_started")).toBe(BadRequestError);
    const families: [number, unknown][] = [
      [400, BadRequestError],
      [401, UnauthenticatedError],
      [403, ForbiddenError],
      [404, NotFoundError],
      [409, ConflictError],
      [410, GoneError],
      [412, PreconditionFailedError],
      [429, RateLimitedError],
      [500, ServerError],
      [599, ServerError],
    ];
    for (const [status, cls] of families)
      expect(errorClass(status, "x_unknown")).toBe(cls);
  });

  it("a body that is not a problem object maps by status with code unknown", () => {
    for (const body of ["<html>bad gateway</html>", undefined, null, 42, []]) {
      const err = problemError(502, body);
      expect(err).toBeInstanceOf(ServerError);
      expect(err.code).toBe("unknown");
    }
  });
});

describe("ProblemError", () => {
  it("carries the members of the problem", () => {
    const err = problemError(
      400,
      problem(400, "validation_failed", {
        errors: [
          { pointer: "/text", code: "too_long" },
          "not an object",
          { pointer: 1 },
        ],
        instance: "/api/v1/threads/x/messages",
      }),
      { retryAfterMs: 2_000, operation: "createMessage" },
    );
    expect(err).toBeInstanceOf(ValidationFailed);
    expect(err.status).toBe(400);
    expect(err.correlationId).toBe(CORRELATION);
    expect(err.instance).toBe("/api/v1/threads/x/messages");
    expect(err.retryAfterMs).toBe(2_000);
    expect(err.operation).toBe("createMessage");
    expect(err.errors).toEqual([{ pointer: "/text", code: "too_long" }]);
    expect(err.name).toBe("ValidationFailed");
  });

  it("keeps reason and current", () => {
    const run = { id: "r", version: 7 };
    const err = problemError(
      409,
      problem(409, "base_version_conflict", { current: run }),
    );
    expect(err).toBeInstanceOf(BaseVersionConflict);
    expect(err.current).toEqual(run);
    const denied = problemError(
      403,
      problem(403, "invocation_token_invalid", { reason: "expired" }),
    );
    expect(denied.reason).toBe("expired");
    expect(
      problemError(409, problem(409, "conflict", { current: "x" })).current,
    ).toBeUndefined();
  });

  it("the message names status, code and operation, never a body or detail", () => {
    const content = "secret content of a message 31337";
    const err = problemError(
      403,
      problem(403, "invocation_token_invalid", {
        reason: "invalid",
        detail: content,
        title: content,
      }),
      { operation: "finishRun" },
    );
    expect(err.message).toBe(
      `finishRun: 403 invocation_token_invalid reason=invalid correlation_id=${CORRELATION}`,
    );
    const rendered = `${err.message} ${String(err)} ${inspect(err)} ${JSON.stringify(err)}`;
    expect(rendered).not.toContain(content);
  });

  it("isNotFound tells a 404 apart", () => {
    expect(isNotFound(problemError(404, problem(404, "not_found")))).toBe(true);
    expect(isNotFound(problemError(404, {}))).toBe(true);
    expect(isNotFound(problemError(403, problem(403, "forbidden")))).toBe(
      false,
    );
    expect(isNotFound(new Error("x"))).toBe(false);
    expect(isNotFound(undefined)).toBe(false);
  });
});

describe("other errors", () => {
  it("GatewayOpError takes the status of a known code", () => {
    const err = new GatewayOpError("ack", "rate_limited", { ref: "r-1" });
    expect(err).toBeInstanceOf(ProblemError);
    expect(err.status).toBe(429);
    expect(err.op).toBe("ack");
    expect(err.ref).toBe("r-1");
    expect(err.message).toBe("gateway op ack: 429 rate_limited");
    expect(new GatewayOpError("identify", "brand_new").status).toBe(0);
  });

  it("TransportError and TimeoutError name the operation only", () => {
    const cause = new TypeError("fetch failed");
    const t = new TransportError("GET /runs/{run_id}", { cause });
    expect(t.message).toBe("transport error on GET /runs/{run_id}");
    expect(t.cause).toBe(cause);
    expect(t.operation).toBe("GET /runs/{run_id}");
    const timeout = new TimeoutError("GET /runs/{run_id}", 30_000);
    expect(timeout).toBeInstanceOf(TransportError);
    expect(timeout.message).toBe(
      "timeout after 30000 ms on GET /runs/{run_id}",
    );
    expect(timeout.timeoutMs).toBe(30_000);
  });
});
