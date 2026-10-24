// SPDX-License-Identifier: Apache-2.0
//
// Retry policy (docs/spec/11-extensions.md 11.2 row `retry`, docs/DECISIONS.md D-M2-26-7,
// D-M2-29-4): the exempt table equals openapi.yaml, only idempotent requests repeat, the waits
// stay inside their bounds and `Retry-After` reads both of its forms.

import { readFileSync } from "node:fs";
import { parse } from "yaml";
import { describe, expect, it } from "vitest";

import { DEFAULTS, resolveSettings } from "../src/config.js";
import {
  IDEMPOTENT_EXEMPT_OPERATIONS,
  RETRY_EXEMPT_REASONS,
  backoffMs,
  exemptOperationOf,
  isRetryableRequest,
  isRetryableStatus,
  parseRetryAfter,
} from "../src/retry/index.js";

type Operation = { "x-idempotency-exempt"?: string };

// Parsed once at import: the specification is large.
const SPEC = parse(
  readFileSync(
    new URL("../../../docs/api/openapi.yaml", import.meta.url),
    "utf8",
  ),
) as { paths: Record<string, Record<string, unknown>> };

function specExempt(): Map<string, string> {
  const spec = SPEC;
  const out = new Map<string, string>();
  for (const [path, item] of Object.entries(spec.paths)) {
    for (const [method, op] of Object.entries(item)) {
      const reason = (op as Operation | null)?.["x-idempotency-exempt"];
      if (typeof reason === "string")
        out.set(`${method.toUpperCase()} ${path}`, reason);
    }
  }
  return out;
}

/** A seeded linear congruential source in [0, 1). */
function seeded(seed: number): () => number {
  let state = seed >>> 0;
  return () => {
    state = (Math.imul(state, 1_664_525) + 1_013_904_223) >>> 0;
    return state / 2 ** 32;
  };
}

describe("IDEMPOTENT_EXEMPT_OPERATIONS", () => {
  it("equals every operation of openapi.yaml with reason agent_runtime, client_id or read_only", () => {
    const expected = [...specExempt()]
      .filter(([, reason]) => RETRY_EXEMPT_REASONS.includes(reason))
      .map(([key]) => key)
      .sort();
    expect(expected.length).toBeGreaterThan(0);
    expect([...IDEMPOTENT_EXEMPT_OPERATIONS.keys()].sort()).toEqual(expected);
    const all = specExempt();
    for (const [key, reason] of IDEMPOTENT_EXEMPT_OPERATIONS)
      expect(all.get(key)).toBe(reason);
    expect([...RETRY_EXEMPT_REASONS].sort()).toEqual([
      "agent_runtime",
      "client_id",
      "read_only",
    ]);
  });

  it("matches request URLs and prefers a literal route", () => {
    const base = "https://example.com/api/v1";
    expect(exemptOperationOf("POST", `${base}/runs/0192f4a0/start`)).toBe(
      "POST /runs/{run_id}/start",
    );
    expect(exemptOperationOf("post", "/api/v1/runs/x/finish?y=1")).toBe(
      "POST /runs/{run_id}/finish",
    );
    expect(exemptOperationOf("POST", "/agents/me/heartbeat")).toBe(
      "POST /agents/me/heartbeat",
    );
    expect(exemptOperationOf("POST", `${base}/runs/x/cancel`)).toBeUndefined();
    expect(
      exemptOperationOf("POST", `${base}/auth/password/login`),
    ).toBeUndefined();
    expect(exemptOperationOf("POST", "/runs/x/y/start")).toBeUndefined();
  });
});

describe("isRetryableRequest", () => {
  const h = (init: Record<string, string> = {}): Headers => new Headers(init);

  it("follows method, Idempotency-Key and the exempt table", () => {
    expect(isRetryableRequest("GET", "/runs/x", h())).toBe(true);
    expect(isRetryableRequest("HEAD", "/runs/x", h())).toBe(true);
    expect(isRetryableRequest("PUT", "/x", h())).toBe(true);
    expect(isRetryableRequest("DELETE", "/commands/x", h())).toBe(true);
    expect(
      isRetryableRequest(
        "POST",
        "/runs/x/cancel",
        h({ "Idempotency-Key": "k" }),
      ),
    ).toBe(true);
    expect(isRetryableRequest("POST", "/threads/t/messages", h())).toBe(true); // client_id
    expect(isRetryableRequest("POST", "/runs/r/start", h())).toBe(true); // agent_runtime
    expect(isRetryableRequest("PATCH", "/runs/r", h())).toBe(true); // agent_runtime
    expect(isRetryableRequest("POST", "/grants/preview", h())).toBe(true); // read_only
    expect(isRetryableRequest("POST", "/runs/r/cancel", h())).toBe(false);
    expect(isRetryableRequest("POST", "/auth/password/login", h())).toBe(false); // auth_flow
    expect(isRetryableRequest("POST", "/client-metrics", h())).toBe(false); // telemetry
    expect(isRetryableRequest("POST", "/imports/i/batches", h())).toBe(false); // origin_key
    expect(isRetryableRequest("PATCH", "/threads/t", h())).toBe(false);
  });

  it("accepts a path template as well as a URL", () => {
    expect(isRetryableRequest("POST", "/runs/{run_id}/start", h())).toBe(true);
  });
});

describe("isRetryableStatus", () => {
  it("retries 5xx, 429 and 409 request_in_progress, never 503 setup_required or another 4xx", () => {
    expect(isRetryableStatus(500, "internal_error")).toBe(true);
    expect(isRetryableStatus(502, undefined)).toBe(true);
    expect(isRetryableStatus(503, "unavailable")).toBe(true);
    expect(isRetryableStatus(503, "maintenance")).toBe(true);
    expect(isRetryableStatus(503, "setup_required")).toBe(false);
    expect(isRetryableStatus(429, "rate_limited")).toBe(true);
    expect(isRetryableStatus(409, "request_in_progress")).toBe(true);
    expect(isRetryableStatus(409, "already_started")).toBe(false);
    for (const status of [400, 401, 403, 404, 410, 412, 422])
      expect(isRetryableStatus(status, "x")).toBe(false);
    expect(isRetryableStatus(200, undefined)).toBe(false);
  });
});

describe("parseRetryAfter", () => {
  const now = Date.UTC(2026, 0, 1, 12, 0, 0);

  it("reads delta-seconds and HTTP dates", () => {
    expect(parseRetryAfter("7", now)).toBe(7_000);
    expect(parseRetryAfter(" 0 ", now)).toBe(0);
    expect(parseRetryAfter("Thu, 01 Jan 2026 12:00:30 GMT", now)).toBe(30_000);
    expect(parseRetryAfter("Thu, 01 Jan 2026 11:00:00 GMT", now)).toBe(0);
  });

  it("gives undefined for absent or unreadable values", () => {
    for (const value of [null, undefined, "", "soon", "-1", "1.5", "1e3"])
      expect(parseRetryAfter(value, now)).toBeUndefined();
  });
});

describe("backoffMs", () => {
  it("attempt n waits uniformly in [base, min(cap, base * 2^(n-1))]", () => {
    const settings = resolveSettings({
      retryBackoffBaseMs: 500,
      retryBackoffCapMs: 4_000,
    });
    const random = seeded(7);
    for (let attempt = 1; attempt <= 10; attempt++) {
      const upper = Math.min(4_000, 500 * 2 ** (attempt - 1));
      for (let i = 0; i < 50; i++) {
        const wait = backoffMs(attempt, settings, random);
        expect(wait).toBeGreaterThanOrEqual(500);
        expect(wait).toBeLessThanOrEqual(upper);
      }
    }
    expect(backoffMs(1, settings, () => 0.999)).toBe(500);
    expect(backoffMs(4, settings, () => 0)).toBe(500);
    expect(backoffMs(4, settings, () => 0.999_999)).toBeCloseTo(4_000, 0);
  });

  it("uses the defaults sdk.retry_backoff_base and sdk.retry_backoff_cap", () => {
    expect(backoffMs(100, DEFAULTS, () => 0.999_999)).toBeLessThanOrEqual(
      DEFAULTS.retryBackoffCapMs,
    );
    expect(backoffMs(1, DEFAULTS, () => 0.5)).toBe(DEFAULTS.retryBackoffBaseMs);
  });
});
