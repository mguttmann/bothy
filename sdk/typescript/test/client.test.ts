// SPDX-License-Identifier: Apache-2.0
//
// REST client of the hand layer (M2-30 Design note 4): auth schemes, DPoP proofs and nonces,
// retries of idempotent requests only, `Retry-After`, timeouts, typed problems, deprecation
// warnings and the run binding of `Invocation-Token` (07 SR9). Every request goes to a fake
// `fetch` on https://example.com/api/v1; nothing leaves the process. Tokens are compared by a
// SHA-256 fingerprint and never printed.

import { createHash, createPublicKey, verify } from "node:crypto";
import { readFileSync } from "node:fs";
import { inspect } from "node:util";
import { parse } from "yaml";
import { afterEach, describe, expect, it, vi } from "vitest";

import { DpopKey, Secret } from "../src/auth/index.js";
import {
  INVOCATION_TOKEN_HEADER,
  IDEMPOTENCY_KEY_HEADER,
  SDK_VERSION,
  createApiClient,
  unwrap,
  unwrapDetailed,
  userAgentHeader,
  type ApiClientOptions,
} from "../src/client.js";
import {
  ConflictError,
  InvocationTokenInvalid,
  KNOWN_PROBLEM_CODES,
  NotFoundError,
  RateLimitedError,
  ServerError,
  TimeoutError,
  TransportError,
  Unavailable,
  UseDpopNonce,
} from "../src/errors.js";
import { defaultLogger, silentLogger, type Logger } from "../src/logger.js";

const BASE = "https://example.com/api/v1";
const CANARY = "canary-secret-token-777";
const RUN = "0192f4a0-7b1c-7000-8a00-000000000001";
const RUN_B = "0192f4a0-7b1c-7000-8a00-000000000002";
const CORRELATION = "0192f4a0-7b1c-7000-8a00-000000000009";
const NOW = Date.UTC(2026, 0, 1, 12, 0, 0);

const spec = parse(
  readFileSync(
    new URL("../../../docs/api/openapi.yaml", import.meta.url),
    "utf8",
  ),
) as {
  components: {
    parameters: Record<string, { name: string; in: string }>;
    schemas: { Problem: { "x-problem-codes": { code: string }[] } };
  };
};

function fingerprint(value: string | null | undefined): string | undefined {
  if (value === null || value === undefined) return undefined;
  return createHash("sha256").update(value).digest("hex").slice(0, 12);
}

interface Seen {
  method: string;
  url: string;
  headers: Headers;
  body: string;
}

type Answer = (seen: Seen, index: number) => Response | Promise<Response>;

/** A fake fetch that records every attempt and answers with `answer`. */
function fakeFetch(answer: Answer): {
  fetch: (request: Request) => Promise<Response>;
  seen: Seen[];
} {
  const seen: Seen[] = [];
  return {
    seen,
    fetch: async (request: Request) => {
      const entry: Seen = {
        method: request.method,
        url: request.url,
        headers: new Headers(request.headers),
        body: await request.text(),
      };
      seen.push(entry);
      return answer(entry, seen.length - 1);
    },
  };
}

function json(
  status: number,
  body: unknown,
  headers: Record<string, string> = {},
): Response {
  return new Response(JSON.stringify(body), {
    status,
    headers: { "content-type": "application/json", ...headers },
  });
}

function problem(
  status: number,
  code: string,
  headers: Record<string, string> = {},
  extra = {},
) {
  return new Response(
    JSON.stringify({
      type: "about:blank",
      title: code,
      status,
      code,
      correlation_id: CORRELATION,
      ...extra,
    }),
    {
      status,
      headers: { "content-type": "application/problem+json", ...headers },
    },
  );
}

const ok = (): Response => json(200, { id: RUN });

/** Retry options that record the waits instead of sleeping. */
function recordedRetry(extra: ApiClientOptions["retry"] = {}): {
  retry: NonNullable<ApiClientOptions["retry"]>;
  waits: number[];
} {
  const waits: number[] = [];
  return {
    waits,
    retry: {
      random: () => 0.5,
      sleep: (ms: number) => {
        waits.push(ms);
        return Promise.resolve();
      },
      ...extra,
    },
  };
}

function recordingLogger(): Logger & { lines: string[] } {
  const lines: string[] = [];
  const record =
    (level: string) =>
    (msg: string, fields?: Record<string, unknown>): void => {
      lines.push(`${level} ${msg} ${fields ? inspect(fields) : ""}`);
    };
  return {
    lines,
    debug: record("debug"),
    info: record("info"),
    warn: record("warn"),
    error: record("error"),
  };
}

const getRun = (api: ReturnType<typeof createApiClient>, runId = RUN) =>
  api.GET("/runs/{run_id}", { params: { path: { run_id: runId } } });

function decodeProof(proof: string | null): {
  header: Record<string, unknown>;
  claims: Record<string, unknown>;
  signature: Buffer;
} {
  const [h, c, s] = (proof ?? "").split(".");
  if (!h || !c || !s) throw new Error("no proof");
  const header = JSON.parse(Buffer.from(h, "base64url").toString()) as Record<
    string,
    unknown
  >;
  const claims = JSON.parse(Buffer.from(c, "base64url").toString()) as Record<
    string,
    unknown
  >;
  const signature = Buffer.from(s, "base64url");
  const key = createPublicKey({
    key: header.jwk as Record<string, string>,
    format: "jwk",
  });
  const valid = verify(
    "sha256",
    Buffer.from(`${h}.${c}`),
    { key, dsaEncoding: "ieee-p1363" },
    signature,
  );
  expect(valid).toBe(true);
  return { header, claims, signature };
}

afterEach(() => {
  vi.restoreAllMocks();
});

describe("auth schemes", () => {
  it("sends Bearer without a DPoP key", async () => {
    const f = fakeFetch(ok);
    const api = createApiClient({
      baseUrl: BASE,
      token: CANARY,
      fetch: f.fetch,
    });
    await unwrap(getRun(api), "getRun");
    const auth = f.seen[0]?.headers.get("authorization");
    expect(auth?.startsWith("Bearer ")).toBe(true);
    expect(fingerprint(auth?.slice("Bearer ".length))).toBe(
      fingerprint(CANARY),
    );
    expect(f.seen[0]?.headers.get("dpop")).toBeNull();
  });

  it("sends no Authorization without a token", async () => {
    const f = fakeFetch(ok);
    const api = createApiClient({ baseUrl: BASE, fetch: f.fetch });
    await unwrap(getRun(api), "getRun");
    expect(f.seen[0]?.headers.get("authorization")).toBeNull();
  });

  it("sends DPoP with a fresh ES256 proof per attempt", async () => {
    const key = await DpopKey.generate();
    const f = fakeFetch((_, i) =>
      i === 0 ? problem(503, "unavailable") : ok(),
    );
    const { retry } = recordedRetry();
    const api = createApiClient({
      baseUrl: BASE,
      token: new Secret(CANARY),
      dpopKey: key,
      fetch: f.fetch,
      retry,
      clock: {
        now: () => NOW,
        monotonic: () => 0,
        sleep: () => Promise.resolve(),
      },
    });
    await unwrap(
      api.GET("/runs/{run_id}", { params: { path: { run_id: RUN } } }),
      "getRun",
    );
    expect(f.seen).toHaveLength(2);
    const proofs = f.seen.map((s) => decodeProof(s.headers.get("dpop")));
    for (const [i, { header, claims, signature }] of proofs.entries()) {
      expect(f.seen[i]?.headers.get("authorization")?.startsWith("DPoP ")).toBe(
        true,
      );
      expect(header.typ).toBe("dpop+jwt");
      expect(header.alg).toBe("ES256");
      expect(header.jwk).toEqual(key.publicJwk);
      expect(signature).toHaveLength(64);
      expect(claims.htm).toBe("GET");
      expect(claims.htu).toBe(`${BASE}/runs/${RUN}`);
      expect(claims.iat).toBe(NOW / 1000);
      expect(claims.ath).toBe(
        createHash("sha256").update(CANARY).digest("base64url"),
      );
    }
    expect(proofs[0]?.claims.jti).not.toBe(proofs[1]?.claims.jti);
  });
});

describe("DPoP nonce", () => {
  it("repeats exactly once on 401 use_dpop_nonce with the new nonce, outside the retry count", async () => {
    const key = await DpopKey.generate();
    const f = fakeFetch((_, i) =>
      i === 0 ? problem(401, "use_dpop_nonce", { "DPoP-Nonce": "n-1" }) : ok(),
    );
    const api = createApiClient({
      baseUrl: BASE,
      token: CANARY,
      dpopKey: key,
      fetch: f.fetch,
      retry: { maxAttempts: 1 },
    });
    await unwrap(getRun(api), "getRun");
    expect(f.seen).toHaveLength(2);
    expect(
      decodeProof(f.seen[0]?.headers.get("dpop") ?? null).claims,
    ).not.toHaveProperty("nonce");
    expect(
      decodeProof(f.seen[1]?.headers.get("dpop") ?? null).claims.nonce,
    ).toBe("n-1");
  });

  it("surfaces a second use_dpop_nonce as UseDpopNonce", async () => {
    const key = await DpopKey.generate();
    const f = fakeFetch((_, i) =>
      problem(401, "use_dpop_nonce", { "DPoP-Nonce": `n-${i + 1}` }),
    );
    const api = createApiClient({
      baseUrl: BASE,
      token: CANARY,
      dpopKey: key,
      fetch: f.fetch,
    });
    await expect(unwrap(getRun(api), "getRun")).rejects.toBeInstanceOf(
      UseDpopNonce,
    );
    expect(f.seen).toHaveLength(2);
  });

  it("does not repeat use_dpop_nonce without a DPoP key", async () => {
    const f = fakeFetch(() =>
      problem(401, "use_dpop_nonce", { "DPoP-Nonce": "n" }),
    );
    const api = createApiClient({
      baseUrl: BASE,
      token: CANARY,
      fetch: f.fetch,
    });
    await expect(unwrap(getRun(api), "getRun")).rejects.toBeInstanceOf(
      UseDpopNonce,
    );
    expect(f.seen).toHaveLength(1);
  });

  it("keeps the newest nonce of any response", async () => {
    const key = await DpopKey.generate();
    const f = fakeFetch((_, i) =>
      json(200, {}, { "DPoP-Nonce": `n-${i + 1}` }),
    );
    const api = createApiClient({
      baseUrl: BASE,
      token: CANARY,
      dpopKey: key,
      fetch: f.fetch,
    });
    for (let i = 0; i < 3; i++) await unwrap(getRun(api), "getRun");
    expect(
      f.seen.map((s) => decodeProof(s.headers.get("dpop")).claims.nonce),
    ).toEqual([undefined, "n-1", "n-2"]);
    expect(api.dpopNonce()).toBe("n-3");
  });
});

describe("retries", () => {
  it("never repeats a plain POST without Idempotency-Key", async () => {
    const f = fakeFetch(() => problem(503, "unavailable"));
    const { retry, waits } = recordedRetry();
    const api = createApiClient({
      baseUrl: BASE,
      token: CANARY,
      fetch: f.fetch,
      retry,
    });
    const call = api.POST("/runs/{run_id}/cancel", {
      params: { path: { run_id: RUN } },
      body: { reason: "Wrong ticket" },
    });
    await expect(unwrap(call, "cancelRun")).rejects.toBeInstanceOf(Unavailable);
    expect(f.seen).toHaveLength(1);
    expect(waits).toEqual([]);
    expect(f.seen[0]?.headers.get(IDEMPOTENCY_KEY_HEADER)).toBeNull();
  });

  it("repeats a POST with Idempotency-Key with the same key and body", async () => {
    const f = fakeFetch((_, i) =>
      i < 2 ? problem(503, "unavailable") : json(202, {}),
    );
    const { retry } = recordedRetry();
    const api = createApiClient({
      baseUrl: BASE,
      token: CANARY,
      fetch: f.fetch,
      retry,
    });
    await unwrap(
      api.POST("/runs/{run_id}/cancel", {
        params: { path: { run_id: RUN } },
        body: { reason: "Wrong ticket" },
        headers: {
          [IDEMPOTENCY_KEY_HEADER]: "0192f4a0-7b1c-7000-8a00-0000000000aa",
        },
      }),
      "cancelRun",
    );
    expect(f.seen).toHaveLength(3);
    expect(
      new Set(f.seen.map((s) => s.headers.get(IDEMPOTENCY_KEY_HEADER))).size,
    ).toBe(1);
    expect(f.seen.map((s) => s.body)).toEqual(
      Array(3).fill('{"reason":"Wrong ticket"}'),
    );
  });

  it("repeats an exempt agent_runtime operation", async () => {
    const f = fakeFetch((_, i) =>
      i === 0 ? problem(502, "bad_gateway") : json(200, {}),
    );
    const { retry } = recordedRetry();
    const api = createApiClient({
      baseUrl: BASE,
      token: CANARY,
      fetch: f.fetch,
      retry,
    });
    await unwrap(
      api.POST("/runs/{run_id}/start", {
        params: { path: { run_id: RUN } },
        body: {
          invocation_token: "placeholder",
          billing_mode_used: "organization",
          tool_class_used: "chat",
        },
      }),
      "startRun",
    );
    expect(f.seen).toHaveLength(2);
  });

  it("503 unavailable and 429 rate_limited with Retry-After wait at least that long", async () => {
    for (const [status, code] of [
      [503, "unavailable"],
      [429, "rate_limited"],
    ] as const) {
      const f = fakeFetch((_, i) =>
        i < 2 ? problem(status, code, { "Retry-After": "7" }) : ok(),
      );
      const { retry, waits } = recordedRetry();
      const api = createApiClient({ baseUrl: BASE, fetch: f.fetch, retry });
      await unwrap(getRun(api), "getRun");
      expect(f.seen).toHaveLength(3);
      expect(waits).toHaveLength(2);
      for (const w of waits) expect(w).toBeGreaterThanOrEqual(7_000);
    }
  });

  it("an HTTP date in Retry-After counts from the client clock", async () => {
    const f = fakeFetch((_, i) =>
      i === 0
        ? problem(429, "rate_limited", {
            "Retry-After": "Thu, 01 Jan 2026 12:00:20 GMT",
          })
        : ok(),
    );
    const { retry, waits } = recordedRetry();
    const api = createApiClient({
      baseUrl: BASE,
      fetch: f.fetch,
      retry,
      clock: {
        now: () => NOW,
        monotonic: () => 0,
        sleep: () => Promise.resolve(),
      },
    });
    await unwrap(getRun(api), "getRun");
    expect(waits).toEqual([20_000]);
  });

  it("waits within the backoff bounds and stops after sdk.retry_max_attempts", async () => {
    const f = fakeFetch(() => problem(500, "internal_error"));
    const randoms = [0.999, 0.999, 0.999, 0.999];
    const { retry, waits } = recordedRetry({
      random: () => randoms.shift() ?? 0,
    });
    const api = createApiClient({
      baseUrl: BASE,
      fetch: f.fetch,
      retry,
      settings: {
        retryMaxAttempts: 4,
        retryBackoffBaseMs: 100,
        retryBackoffCapMs: 250,
      },
    });
    await expect(unwrap(getRun(api), "getRun")).rejects.toBeInstanceOf(
      ServerError,
    );
    expect(f.seen).toHaveLength(4);
    expect(waits).toHaveLength(3);
    const uppers = [100, 200, 250];
    waits.forEach((w, i) => {
      expect(w).toBeGreaterThanOrEqual(100);
      expect(w).toBeLessThanOrEqual(uppers[i] ?? 0);
    });
    expect(waits[2]).toBeGreaterThan(200);
  });

  it("the retry option overrides the settings", async () => {
    const f = fakeFetch(() => problem(500, "internal_error"));
    const { retry, waits } = recordedRetry({
      maxAttempts: 2,
      baseMs: 10,
      capMs: 10,
    });
    const api = createApiClient({ baseUrl: BASE, fetch: f.fetch, retry });
    await expect(unwrap(getRun(api), "getRun")).rejects.toBeInstanceOf(
      ServerError,
    );
    expect(f.seen).toHaveLength(2);
    expect(waits).toEqual([10]);
  });

  it("repeats 409 request_in_progress but not 503 setup_required or another 4xx", async () => {
    const cases: [Response, number][] = [
      [problem(409, "request_in_progress"), 2],
      [problem(503, "setup_required"), 1],
      [problem(404, "not_found"), 1],
      [problem(409, "already_started"), 1],
    ];
    for (const [first, expected] of cases) {
      const f = fakeFetch((_, i) => (i === 0 ? first.clone() : ok()));
      const { retry } = recordedRetry();
      const api = createApiClient({ baseUrl: BASE, fetch: f.fetch, retry });
      await unwrap(getRun(api), "getRun").catch(() => undefined);
      expect(f.seen).toHaveLength(expected);
    }
  });

  it("repeats network errors and surfaces a TransportError at the end", async () => {
    let calls = 0;
    const { retry } = recordedRetry({ maxAttempts: 3 });
    const api = createApiClient({
      baseUrl: BASE,
      fetch: () => {
        calls++;
        return Promise.reject(new TypeError("fetch failed"));
      },
      retry,
    });
    const err: unknown = await unwrap(getRun(api), "getRun").catch(
      (e: unknown) => e,
    );
    expect(err).toBeInstanceOf(TransportError);
    expect((err as TransportError).operation).toBe("GET /runs/{run_id}");
    expect(calls).toBe(3);
    calls = 0;
    const post = api.POST("/runs/{run_id}/cancel", {
      params: { path: { run_id: RUN } },
      body: {},
    });
    await expect(post).rejects.toBeInstanceOf(TransportError);
    expect(calls).toBe(1);
  });

  it("limits every attempt with sdk.http_timeout", async () => {
    let calls = 0;
    const { retry } = recordedRetry({ maxAttempts: 2 });
    const api = createApiClient({
      baseUrl: BASE,
      timeoutMs: 20,
      retry,
      fetch: (request: Request) => {
        calls++;
        // Like the real fetch: a signal that expired before the call (a slow machine can spend
        // the 20 ms in preparing the attempt) rejects at once, a later abort rejects then.
        if (request.signal.aborted)
          return Promise.reject(request.signal.reason as Error);
        return new Promise<Response>((_, reject) => {
          request.signal.addEventListener("abort", () => {
            reject(request.signal.reason as Error);
          });
        });
      },
    });
    const err: unknown = await getRun(api).catch((e: unknown) => e);
    expect(err).toBeInstanceOf(TimeoutError);
    expect((err as TimeoutError).timeoutMs).toBe(20);
    expect(calls).toBe(2);
    const fromSettings = createApiClient({
      baseUrl: BASE,
      settings: { httpTimeoutMs: 4_500 },
    });
    expect(fromSettings.timeoutMs).toBe(4_500);
  });

  it("ends an attempt after sdk.http_timeout even when the fetch ignores the signal", async () => {
    // The class behind a hang under load: the client must not depend on a custom fetch
    // seeing the abort through request.signal (docs/decisions.d/M2-30.md D-M2-30-16).
    let calls = 0;
    const { retry } = recordedRetry({ maxAttempts: 2 });
    const api = createApiClient({
      baseUrl: BASE,
      timeoutMs: 20,
      retry,
      fetch: () => {
        calls++;
        return new Promise<Response>(() => undefined);
      },
    });
    const err: unknown = await getRun(api).catch((e: unknown) => e);
    expect(err).toBeInstanceOf(TimeoutError);
    expect(calls).toBe(2);
  });

  it("an abort by the caller is not repeated", async () => {
    let calls = 0;
    const controller = new AbortController();
    const api = createApiClient({
      baseUrl: BASE,
      fetch: (request: Request) => {
        calls++;
        controller.abort(new Error("caller abort"));
        return Promise.reject(request.signal.reason as Error);
      },
    });
    await expect(
      api.GET("/runs/{run_id}", {
        params: { path: { run_id: RUN } },
        signal: controller.signal,
      }),
    ).rejects.toThrow("caller abort");
    expect(calls).toBe(1);
  });
});

describe("problems", () => {
  it("KNOWN_PROBLEM_CODES equals x-problem-codes", () => {
    expect([...KNOWN_PROBLEM_CODES].sort()).toEqual(
      spec.components.schemas.Problem["x-problem-codes"]
        .map((c) => c.code)
        .sort(),
    );
  });

  it("unwrap throws the typed error with Retry-After and operation", async () => {
    const f = fakeFetch(() =>
      problem(429, "rate_limited", { "Retry-After": "3" }),
    );
    const api = createApiClient({
      baseUrl: BASE,
      fetch: f.fetch,
      retry: { maxAttempts: 1 },
    });
    const err: unknown = await unwrap(getRun(api), "getRun").catch(
      (e: unknown) => e,
    );
    expect(err).toBeInstanceOf(RateLimitedError);
    expect((err as RateLimitedError).retryAfterMs).toBe(3_000);
    expect((err as RateLimitedError).operation).toBe("getRun");
  });

  it("an unknown code is classified by status", async () => {
    const f = fakeFetch(() => problem(409, "brand_new_code"));
    const api = createApiClient({ baseUrl: BASE, fetch: f.fetch });
    const err: unknown = await unwrap(getRun(api), "getRun").catch(
      (e: unknown) => e,
    );
    expect((err as ConflictError).constructor).toBe(ConflictError);
    expect((err as ConflictError).code).toBe("brand_new_code");
  });

  it("an undecodable or empty error body maps by status", async () => {
    const html = fakeFetch(
      () =>
        new Response("<html>bad gateway</html>", {
          status: 502,
          headers: { "content-type": "text/html" },
        }),
    );
    const api = createApiClient({
      baseUrl: BASE,
      fetch: html.fetch,
      retry: { maxAttempts: 1 },
    });
    await expect(unwrap(getRun(api), "getRun")).rejects.toBeInstanceOf(
      ServerError,
    );
    const empty = fakeFetch(() => new Response(null, { status: 404 }));
    const api2 = createApiClient({ baseUrl: BASE, fetch: empty.fetch });
    await expect(unwrap(getRun(api2), "getRun")).rejects.toBeInstanceOf(
      NotFoundError,
    );
  });

  it("unwrapDetailed returns data and the response headers", async () => {
    const f = fakeFetch(() =>
      json(200, { id: RUN }, { "Idempotent-Replay": "true", ETag: '"v3"' }),
    );
    const api = createApiClient({ baseUrl: BASE, fetch: f.fetch });
    const { data, response } = await unwrapDetailed(getRun(api), "getRun");
    expect(data).toEqual({ id: RUN });
    expect(response.headers.get("Idempotent-Replay")).toBe("true");
    expect(response.headers.get("ETag")).toBe('"v3"');
  });

  it("the tolerance middleware stays registered", async () => {
    const f = fakeFetch(() => json(200, { id: RUN, summary: null }));
    const api = createApiClient({ baseUrl: BASE, fetch: f.fetch });
    const data = (await unwrap(getRun(api), "getRun")) as Record<
      string,
      unknown
    >;
    expect(data).toEqual({ id: RUN });
  });
});

describe("deprecation", () => {
  it("warns once per operation and client through the logger", async () => {
    const f = fakeFetch(() =>
      json(
        200,
        {},
        { Deprecation: "@1767225600", Sunset: "Wed, 01 Jul 2026 00:00:00 GMT" },
      ),
    );
    const logger = recordingLogger();
    const api = createApiClient({ baseUrl: BASE, fetch: f.fetch, logger });
    for (let i = 0; i < 3; i++)
      await unwrap(getRun(api, i === 2 ? RUN_B : RUN), "getRun");
    await unwrap(api.GET("/instance/discovery"), "getDiscovery");
    const warnings = logger.lines.filter((l) => l.startsWith("warn"));
    expect(warnings).toHaveLength(2);
    expect(warnings[0]).toContain("GET /runs/{run_id}");
    expect(warnings[0]).toContain("@1767225600");
    expect(warnings[0]).toContain("Wed, 01 Jul 2026 00:00:00 GMT");
    expect(warnings[1]).toContain("GET /instance/discovery");
    const again = createApiClient({ baseUrl: BASE, fetch: f.fetch, logger });
    await unwrap(getRun(again), "getRun");
    expect(logger.lines.filter((l) => l.startsWith("warn"))).toHaveLength(3);
  });

  it("defaults to process.emitWarning with type DeprecationWarning", async () => {
    const emit = vi
      .spyOn(process, "emitWarning")
      .mockImplementation(() => undefined);
    const f = fakeFetch(() =>
      json(200, {}, { Sunset: "Wed, 01 Jul 2026 00:00:00 GMT" }),
    );
    const api = createApiClient({ baseUrl: BASE, fetch: f.fetch });
    await unwrap(getRun(api), "getRun");
    await unwrap(getRun(api), "getRun");
    expect(emit).toHaveBeenCalledTimes(1);
    const [message, options] = emit.mock.calls[0] ?? [];
    expect(String(message)).toContain("GET /runs/{run_id}");
    expect(options).toMatchObject({ type: "DeprecationWarning" });
  });
});

describe("run binding (07 SR9)", () => {
  it("the header name equals components.parameters.InvocationToken", () => {
    const param = spec.components.parameters.InvocationToken;
    expect([param?.in, param?.name]).toEqual([
      "header",
      INVOCATION_TOKEN_HEADER,
    ]);
  });

  it("sends Invocation-Token only inside bindRun, with the newest token", async () => {
    const f = fakeFetch(ok);
    const api = createApiClient({
      baseUrl: BASE,
      token: CANARY,
      fetch: f.fetch,
    });
    await unwrap(getRun(api), "getRun");
    expect(api.boundRunId()).toBeUndefined();
    await api.bindRun(RUN, new Secret("token-one"), async () => {
      expect(api.boundRunId()).toBe(RUN);
      await unwrap(getRun(api), "getRun");
      api.setRunToken(RUN, "token-two");
      await unwrap(getRun(api), "getRun");
      await api.unbound(async () => {
        expect(api.boundRunId()).toBeUndefined();
        await unwrap(getRun(api), "getRun");
      });
    });
    await unwrap(getRun(api), "getRun");
    expect(
      f.seen.map((s) => fingerprint(s.headers.get(INVOCATION_TOKEN_HEADER))),
    ).toEqual([
      undefined,
      fingerprint("token-one"),
      fingerprint("token-two"),
      undefined,
      undefined,
    ]);
    // A replacement for a run that is not bound changes nothing and binds nothing.
    api.setRunToken(RUN, "token-three");
    await unwrap(getRun(api), "getRun");
    expect(f.seen.at(-1)?.headers.get(INVOCATION_TOKEN_HEADER)).toBeNull();
  });

  it("two concurrent runs never see each other's token", async () => {
    const f = fakeFetch(ok);
    const api = createApiClient({ baseUrl: BASE, fetch: f.fetch });
    const byRun = new Map<string, (string | undefined)[]>();
    let release: () => void = () => undefined;
    const gate = new Promise<void>((r) => (release = r));
    const run = (runId: string, token: string, renewed: string) =>
      api.bindRun(runId, token, async () => {
        const seen: (string | undefined)[] = [];
        byRun.set(runId, seen);
        await unwrap(getRun(api, runId), "getRun");
        await gate;
        if (runId === RUN) api.setRunToken(runId, renewed);
        await new Promise((r) => setImmediate(r));
        await unwrap(getRun(api, runId), "getRun");
        return runId;
      });
    const both = Promise.all([
      run(RUN, "token-a1", "token-a2"),
      run(RUN_B, "token-b1", "unused"),
    ]);
    await new Promise((r) => setImmediate(r));
    release();
    expect(await both).toEqual([RUN, RUN_B]);
    const tokensFor = (runId: string) =>
      f.seen
        .filter((s) => s.url.endsWith(runId))
        .map((s) => fingerprint(s.headers.get(INVOCATION_TOKEN_HEADER)));
    expect(tokensFor(RUN)).toEqual([
      fingerprint("token-a1"),
      fingerprint("token-a2"),
    ]);
    expect(tokensFor(RUN_B)).toEqual([
      fingerprint("token-b1"),
      fingerprint("token-b1"),
    ]);
  });

  it("bindRun returns the value of fn and passes its error", async () => {
    const api = createApiClient({ baseUrl: BASE, fetch: fakeFetch(ok).fetch });
    expect(await api.bindRun(RUN, "t", () => Promise.resolve(42))).toBe(42);
    await expect(
      api.bindRun(RUN, "t", () => Promise.reject(new Error("inner"))),
    ).rejects.toThrow("inner");
    expect(api.boundRunId()).toBeUndefined();
  });
});

describe("client", () => {
  it("sends the User-Agent of 06 2", async () => {
    const f = fakeFetch(ok);
    const api = createApiClient({
      baseUrl: BASE,
      fetch: f.fetch,
      userAgent: "lotse/1.2.0",
    });
    await unwrap(getRun(api), "getRun");
    const ua = f.seen[0]?.headers.get("user-agent") ?? "";
    expect(ua).toBe(userAgentHeader("lotse/1.2.0"));
    expect(ua).toMatch(
      /^lotse\/1\.2\.0 bothy-sdk\/\S+ \(node \d+\.\d+; [a-z0-9]+\)$/,
    );
    expect(userAgentHeader()).toMatch(/^bothy-sdk\//);
    const manifest = JSON.parse(
      readFileSync(new URL("../package.json", import.meta.url), "utf8"),
    ) as { version: string };
    expect(SDK_VERSION).toBe(manifest.version);
  });

  it("exposes settings, logger and clock", () => {
    const logger = recordingLogger();
    const api = createApiClient({
      baseUrl: BASE,
      logger,
      settings: { retryMaxAttempts: 2 },
    });
    expect(api.settings.retryMaxAttempts).toBe(2);
    expect(api.logger).toBe(logger);
    expect(typeof api.clock.now()).toBe("number");
  });

  it("refuses clear text off the loopback host", () => {
    expect(() =>
      createApiClient({ baseUrl: "http://example.com/api/v1" }),
    ).toThrow(/only https and wss/);
    expect(() =>
      createApiClient({ baseUrl: "http://127.0.0.1:1/api/v1" }),
    ).not.toThrow();
  });

  it("logs and errors never carry a token, a proof or a content", async () => {
    const key = await DpopKey.generate();
    const content = "secret content of a message 31337";
    const f = fakeFetch(() =>
      problem(
        403,
        "invocation_token_invalid",
        {},
        { reason: "invalid", detail: content },
      ),
    );
    const logger = recordingLogger();
    const api = createApiClient({
      baseUrl: BASE,
      token: CANARY,
      dpopKey: key,
      fetch: f.fetch,
      logger,
    });
    const err: unknown = await api
      .bindRun(RUN, "bound-token-4242", () =>
        unwrap(
          api.POST("/runs/{run_id}/finish", {
            params: { path: { run_id: RUN } },
            body: { state: "failed", summary: content },
          }),
          "finishRun",
        ),
      )
      .catch((e: unknown) => e);
    expect(err).toBeInstanceOf(InvocationTokenInvalid);
    const proof = f.seen[0]?.headers.get("dpop") ?? "";
    expect(proof).not.toBe("");
    const text = `${logger.lines.join("\n")} ${String(err)} ${inspect(err)} ${inspect(api)}`;
    for (const secret of [CANARY, "bound-token-4242", content, proof])
      expect(text).not.toContain(secret);
    expect(
      logger.lines.some((l) => l.includes("GET") || l.includes("POST")),
    ).toBe(true);
  });
});

describe("loggers", () => {
  it("defaultLogger writes warn and error to console and drops debug and info", () => {
    const warn = vi.spyOn(console, "warn").mockImplementation(() => undefined);
    const error = vi
      .spyOn(console, "error")
      .mockImplementation(() => undefined);
    const log = vi.spyOn(console, "log").mockImplementation(() => undefined);
    for (const logger of [defaultLogger, silentLogger]) {
      logger.debug("d");
      logger.info("i", { a: 1 });
    }
    defaultLogger.warn("w");
    defaultLogger.warn("w", { operation: "GET /x" });
    defaultLogger.error("e");
    defaultLogger.error("e", { code: "x" });
    silentLogger.warn("w");
    silentLogger.error("e");
    expect(warn.mock.calls).toEqual([["w"], ["w", { operation: "GET /x" }]]);
    expect(error.mock.calls).toEqual([["e"], ["e", { code: "x" }]]);
    expect(log).not.toHaveBeenCalled();
  });
});
