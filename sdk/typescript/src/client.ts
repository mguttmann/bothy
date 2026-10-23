// SPDX-License-Identifier: Apache-2.0
/**
 * REST client of the hand layer on top of the generated core (M2-30 Design note 4).
 *
 * {@link createApiClient} builds the openapi-fetch client of M2-23 with the tolerance middleware
 * and a wrapped `fetch` that, per attempt, sets `Authorization` (`DPoP` with a fresh proof when a
 * DPoP key is configured, else `Bearer`), the run's newest `Invocation-Token` inside
 * {@link ApiClientExtras.bindRun} only (07 SR9), the `User-Agent` of docs/spec/06-api.md 2 and an
 * abort after `sdk.http_timeout` (`attemptSignal`). It keeps the newest `DPoP-Nonce` and repeats once
 * on `401 use_dpop_nonce` outside the retry count (06 3.1 point 2); it repeats idempotent requests
 * only (`./retry`) and warns once per operation on `Deprecation` or `Sunset` (11 12). Node.js only:
 * the run binding lives in an `AsyncLocalStorage` (docs/decisions.d/M2-30.md D-M2-30-3).
 *
 * Logs carry the operation (`METHOD /path/template`), status, attempt and bound run; never a
 * URL, a header value, a body, a token or a proof (SEC-LOG-2).
 *
 * @example
 * ```ts
 * const api = createApiClient({
 *   baseUrl: "https://example.com/api/v1",
 *   token: new Secret(process.env.BOTHY_TOKEN ?? ""),
 *   dpopKey: await loadDpopKey("dpop.pem"),
 *   userAgent: "lotse/1.0.0",
 * });
 * const run = await unwrap(api.GET("/runs/{run_id}", { params: { path: { run_id } } }), "getRun");
 * await api.bindRun(run_id, invocationToken, () => handleRun(api));
 * ```
 *
 * @module
 */

import { AsyncLocalStorage } from "node:async_hooks";
import { randomInt } from "node:crypto";

import createClient, {
  type Client,
  type ClientOptions,
  type Middleware,
} from "openapi-fetch";

import { createDpopProof, requireSafeUrl } from "./auth/dpop.js";
import { type Secret, asSecret } from "./auth/secret.js";
import type { DpopSigner } from "./auth/types.js";
import { type Clock, systemClock } from "./clock.js";
import { type SdkSettings, resolveSettings } from "./config.js";
import { TimeoutError, TransportError, problemError } from "./errors.js";
import type { paths } from "./generated/openapi.js";
import { type Logger, defaultLogger } from "./logger.js";
import { apiPath } from "./retry/table.js";
import {
  backoffMs,
  isRetryableRequest,
  isRetryableStatus,
  parseRetryAfter,
} from "./retry/policy.js";
import { toleranceMiddleware } from "./tolerance.js";

/**
 * The header of 07 SR9 (`components.parameters.InvocationToken` of docs/api/openapi.yaml; a test
 * holds it equal).
 */
export const INVOCATION_TOKEN_HEADER = "Invocation-Token";

/** The header of docs/spec/06-api.md 7 that makes a write repeatable. */
export const IDEMPOTENCY_KEY_HEADER = "Idempotency-Key";

/** The version of this SDK (`package.json`; a test holds both equal). */
export const SDK_VERSION = "0.0.0";

const ID_SEGMENT =
  /^(?:[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}|\d+)$/i;
// `randomInt` takes at most 2^48 - 1 as its exclusive bound.
const RANDOM_RANGE = 2 ** 48 - 1;

/** Retry options of {@link ApiClientOptions}; each default is a key of {@link SdkSettings}. */
export interface RetryOptions {
  /** Attempts in total, the first one included (default `sdk.retry_max_attempts`). */
  readonly maxAttempts?: number | undefined;
  /** Base and lower bound of the wait (default `sdk.retry_backoff_base`). */
  readonly baseMs?: number | undefined;
  /** Largest wait (default `sdk.retry_backoff_cap`). */
  readonly capMs?: number | undefined;
  /** Jitter source in [0, 1) (default from `node:crypto`). */
  readonly random?: (() => number) | undefined;
  /** Waits between attempts (default `clock.sleep`). */
  readonly sleep?: ((ms: number) => Promise<void>) | undefined;
}

/** Options of {@link createApiClient}: openapi-fetch's options plus the hand layer's. */
export interface ApiClientOptions extends ClientOptions {
  /** The API token; absent sends no `Authorization`. */
  readonly token?: Secret | string | undefined;
  /** The key of a DPoP-bound token (a `DpopKey` or a key store). */
  readonly dpopKey?: DpopSigner | undefined;
  /** Bound of one attempt in milliseconds (default `sdk.http_timeout`). */
  readonly timeoutMs?: number | undefined;
  /** Retry options. */
  readonly retry?: RetryOptions | undefined;
  /** The logger (default {@link defaultLogger}; deprecations then go to `process.emitWarning`). */
  readonly logger?: Logger | undefined;
  /** `<app>/<version>` of the caller, placed before the SDK's own part of `User-Agent`. */
  readonly userAgent?: string | undefined;
  /** Overrides of the SDK settings. */
  readonly settings?: Partial<SdkSettings> | undefined;
  /** The time source of proofs, `Retry-After` dates and waits (default `systemClock`). */
  readonly clock?: Clock | undefined;
}

/** The members {@link createApiClient} adds to the openapi-fetch client. */
export interface ApiClientExtras {
  /**
   * Runs `fn` with every request of this client inside it bound to `runId`: each carries the
   * run's newest invocation token in `Invocation-Token` (07 SR9). The binding lives in an
   * `AsyncLocalStorage`, so two concurrent runs never see each other's token.
   *
   * @param runId - The run.
   * @param token - The run's newest invocation token.
   * @param fn - The work of the run.
   * @returns The result of `fn`.
   */
  bindRun<T>(
    runId: string,
    token: Secret | string,
    fn: () => Promise<T>,
  ): Promise<T>;
  /**
   * Makes `token` the token every request bound to `runId` carries from now on (after a
   * renewal or a newer delivery); a run without an active binding is left alone.
   *
   * @param runId - The run.
   * @param token - The newer token.
   */
  setRunToken(runId: string, token: Secret | string): void;
  /**
   * Runs `fn` without any run binding (the agent's own writes inside a run).
   *
   * @param fn - The work.
   * @returns The result of `fn`.
   */
  unbound<T>(fn: () => Promise<T>): Promise<T>;
  /**
   * Returns the run the current context is bound to.
   *
   * @returns The run ID, or `undefined` outside {@link ApiClientExtras.bindRun}.
   */
  boundRunId(): string | undefined;
  /**
   * Returns the newest `DPoP-Nonce` the server sent (the gateway proof starts from it).
   *
   * @returns The nonce, or `undefined` before the first one.
   */
  dpopNonce(): string | undefined;
  /** The resolved SDK settings. */
  readonly settings: SdkSettings;
  /** The logger of the client. */
  readonly logger: Logger;
  /** The time source of the client. */
  readonly clock: Clock;
  /** The bound of one attempt in milliseconds. */
  readonly timeoutMs: number;
}

/** The openapi-fetch client of every operation of docs/api/openapi.yaml with the hand layer. */
export type ApiClient = Client<paths> & ApiClientExtras;

interface Binding {
  readonly runId: string;
  token: Secret;
  refs: number;
}

function cryptoRandom(): number {
  return randomInt(RANDOM_RANGE) / RANDOM_RANGE;
}

/**
 * Returns the `User-Agent` of docs/spec/06-api.md 2 ("Client identifier").
 *
 * @param app - `<app>/<version>` of the caller, if any.
 * @returns `<app> bothy-sdk/<version> (node <major.minor>; <platform>)`.
 *
 * @example
 * ```ts
 * userAgentHeader("lotse/1.2.0"); // "lotse/1.2.0 bothy-sdk/0.0.0 (node 24.11; linux)"
 * ```
 */
export function userAgentHeader(app?: string): string {
  const node = process.versions.node.split(".").slice(0, 2).join(".");
  const own = `bothy-sdk/${SDK_VERSION} (node ${node}; ${process.platform})`;
  return app ? `${app} ${own}` : own;
}

/** Returns the path below the API prefix with every UUID or number segment as `{id}`. */
function fallbackTemplate(url: string): string {
  return apiPath(url)
    .split("/")
    .map((s) => (ID_SEGMENT.test(s) ? "{id}" : s))
    .join("/");
}

async function problemCode(response: Response): Promise<string | undefined> {
  try {
    const body: unknown = await response.clone().json();
    if (typeof body === "object" && body !== null && "code" in body) {
      const { code } = body as { code: unknown };
      return typeof code === "string" ? code : undefined;
    }
  } catch {
    // Not JSON: no code; the status decides.
  }
  return undefined;
}

/** The wrapped `fetch` of one client with its state (nonce, bindings, warned operations). */

/**
 * The signal of one attempt: aborted by the caller's signal or after `timeoutMs`.
 *
 * The attempt's `Request` carries it, and {@link untilAborted} also races the `fetch` against
 * it: in Node.js the signal of a `Request` follows the given signal only through a weak
 * reference, so after a garbage collection a custom `fetch` that waits on `request.signal`
 * waited past the timeout (docs/decisions.d/M2-30.md D-M2-30-16). The caller's listener goes
 * when the timer fires.
 */
function attemptSignal(
  caller: AbortSignal,
  timeoutMs: number,
): { readonly signal: AbortSignal; expired(): boolean } {
  const controller = new AbortController();
  let expired = false;
  const onCaller = (): void => controller.abort(caller.reason);
  const timer = setTimeout(() => {
    expired = true;
    caller.removeEventListener("abort", onCaller);
    controller.abort(
      new DOMException("The operation timed out.", "TimeoutError"),
    );
  }, timeoutMs);
  timer.unref();
  if (caller.aborted) controller.abort(caller.reason);
  else caller.addEventListener("abort", onCaller, { once: true });
  return { signal: controller.signal, expired: () => expired };
}

/**
 * Settles with `pending`, or rejects with the reason of `signal` once it aborts first; a
 * response that arrives after the abort is cancelled.
 */
function untilAborted(
  pending: Promise<Response>,
  signal: AbortSignal,
): Promise<Response> {
  if (signal.aborted) {
    void pending.then(
      (late) => late.body?.cancel(),
      () => undefined,
    );
    return Promise.reject(signal.reason as Error);
  }
  return new Promise<Response>((resolve, reject) => {
    const onAbort = (): void => {
      void pending.then(
        (late) => late.body?.cancel(),
        () => undefined,
      );
      reject(signal.reason as Error);
    };
    signal.addEventListener("abort", onAbort, { once: true });
    pending.then(
      (response) => {
        signal.removeEventListener("abort", onAbort);
        resolve(response);
      },
      (cause: unknown) => {
        signal.removeEventListener("abort", onAbort);
        reject(cause as Error);
      },
    );
  });
}

class Transport {
  readonly bindings = new Map<string, Binding>();
  readonly storage = new AsyncLocalStorage<Binding | undefined>();
  readonly templates = new WeakMap<Request, string>();
  readonly warned = new Set<string>();
  nonce: string | undefined;

  readonly middleware: Middleware = {
    onRequest: ({ request, schemaPath }) => {
      this.templates.set(request, schemaPath);
      return undefined;
    },
  };

  constructor(
    private readonly config: {
      readonly fetch: (request: Request) => Promise<Response>;
      readonly token: Secret | undefined;
      readonly dpopKey: DpopSigner | undefined;
      readonly timeoutMs: number;
      readonly maxAttempts: number;
      readonly backoff: Pick<
        SdkSettings,
        "retryBackoffBaseMs" | "retryBackoffCapMs"
      >;
      readonly random: () => number;
      readonly sleep: (ms: number) => Promise<void>;
      readonly logger: Logger;
      readonly explicitLogger: boolean;
      readonly userAgent: string;
      readonly clock: Clock;
    },
  ) {}

  boundRunId(): string | undefined {
    return this.storage.getStore()?.runId;
  }

  async send(request: Request): Promise<Response> {
    const template =
      this.templates.get(request) ?? fallbackTemplate(request.url);
    const operation = `${request.method.toUpperCase()} ${template}`;
    const binding = this.storage.getStore();
    if (this.config.token !== undefined || binding !== undefined)
      requireSafeUrl(request.url);
    const retryable = isRetryableRequest(
      request.method,
      request.url,
      request.headers,
    );
    const body =
      request.body === null ? undefined : await request.arrayBuffer();
    const { maxAttempts } = this.config;
    let nonceRepeated = false;
    for (let attempt = 1; ; attempt++) {
      const limit = attemptSignal(request.signal, this.config.timeoutMs);
      const attemptRequest = await this.prepare(
        request,
        body,
        binding,
        limit.signal,
      );
      const last = !retryable || attempt >= maxAttempts;
      let response: Response;
      try {
        response = await untilAborted(
          this.config.fetch(attemptRequest),
          limit.signal,
        );
      } catch (cause) {
        if (request.signal.aborted) throw cause;
        const error = limit.expired()
          ? new TimeoutError(operation, this.config.timeoutMs, { cause })
          : new TransportError(operation, { cause });
        this.log("debug", "request failed", operation, attempt, {
          error: error.name,
        });
        if (last) throw error;
        await this.wait(
          operation,
          attempt,
          backoffMs(attempt, this.config.backoff, this.config.random),
        );
        continue;
      }
      this.log("debug", "request", operation, attempt, {
        status: response.status,
      });
      this.nonce = response.headers.get("DPoP-Nonce") || this.nonce;
      this.warnDeprecation(operation, response.headers);
      if (
        response.status === 401 &&
        this.config.dpopKey !== undefined &&
        !nonceRepeated &&
        (await problemCode(response)) === "use_dpop_nonce"
      ) {
        // One repeat with the new nonce, outside the retry count (06 3.1 point 2).
        nonceRepeated = true;
        attempt--;
        await response.body?.cancel();
        continue;
      }
      if (last || response.status < 409) return response;
      if (!isRetryableStatus(response.status, await problemCode(response)))
        return response;
      const retryAfter = parseRetryAfter(
        response.headers.get("Retry-After"),
        this.config.clock.now(),
      );
      const jitter = backoffMs(
        attempt,
        this.config.backoff,
        this.config.random,
      );
      await response.body?.cancel();
      await this.wait(
        operation,
        attempt,
        Math.max(jitter, retryAfter ?? 0),
        response.status,
      );
    }
  }

  private async prepare(
    request: Request,
    body: ArrayBuffer | undefined,
    binding: Binding | undefined,
    signal: AbortSignal,
  ): Promise<Request> {
    const headers = new Headers(request.headers);
    if (!headers.has("User-Agent"))
      headers.set("User-Agent", this.config.userAgent);
    const { token, dpopKey } = this.config;
    if (token !== undefined) {
      if (dpopKey === undefined) {
        headers.set("Authorization", `Bearer ${token.reveal()}`);
      } else {
        headers.set("Authorization", `DPoP ${token.reveal()}`);
        headers.set(
          "DPoP",
          await createDpopProof(dpopKey, {
            htm: request.method,
            htu: request.url,
            accessToken: token,
            nonce: this.nonce,
            now: this.config.clock.now(),
          }),
        );
      }
    }
    if (binding !== undefined)
      headers.set(INVOCATION_TOKEN_HEADER, binding.token.reveal());
    return new Request(
      request,
      body === undefined ? { headers, signal } : { headers, signal, body },
    );
  }

  private async wait(
    operation: string,
    attempt: number,
    ms: number,
    status?: number,
  ): Promise<void> {
    this.log("info", "retry", operation, attempt, {
      status,
      waitMs: Math.round(ms),
    });
    await this.config.sleep(ms);
  }

  private warnDeprecation(operation: string, headers: Headers): void {
    const deprecation = headers.get("Deprecation");
    const sunset = headers.get("Sunset");
    if ((deprecation === null && sunset === null) || this.warned.has(operation))
      return;
    this.warned.add(operation);
    const parts = [
      deprecation === null ? undefined : `Deprecation: ${deprecation}`,
      sunset === null ? undefined : `Sunset: ${sunset}`,
    ].filter((p) => p !== undefined);
    const message = `the server marked ${operation} as deprecated (${parts.join("; ")})`;
    if (this.config.explicitLogger)
      this.config.logger.warn(message, { operation, deprecation, sunset });
    else
      process.emitWarning(message, {
        type: "DeprecationWarning",
        code: "BOTHY_SDK_DEPRECATED",
      });
  }

  private log(
    level: "debug" | "info",
    msg: string,
    operation: string,
    attempt: number,
    fields: Record<string, unknown>,
  ): void {
    this.config.logger[level](`${msg} ${operation}`, {
      operation,
      attempt,
      run: this.boundRunId(),
      ...fields,
    });
  }
}

/**
 * Creates the openapi-fetch client for the public API with the tolerance middleware and the
 * hand layer's wrapped `fetch` (auth, DPoP, run binding, timeout, retries, deprecation).
 *
 * @param options - openapi-fetch's options (`baseUrl` ending in `/api/v1`, `fetch`, `headers`)
 *   plus `token`, `dpopKey`, `timeoutMs`, `retry`, `logger`, `userAgent`, `settings`, `clock`.
 * @returns The client; its JSON responses never contain a `null` member.
 * @throws `TypeError` when `baseUrl` uses clear text off the loopback host.
 */
export function createApiClient(options: ApiClientOptions = {}): ApiClient {
  const {
    token,
    dpopKey,
    timeoutMs,
    retry = {},
    logger,
    userAgent,
    settings: overrides,
    clock = systemClock,
    fetch: innerFetch = (request: Request) => globalThis.fetch(request),
    ...clientOptions
  } = options;
  if (clientOptions.baseUrl) requireSafeUrl(clientOptions.baseUrl);
  const settings = resolveSettings(overrides);
  const log = logger ?? defaultLogger;
  const transport = new Transport({
    fetch: innerFetch,
    token: token === undefined ? undefined : asSecret(token),
    dpopKey,
    timeoutMs: timeoutMs ?? settings.httpTimeoutMs,
    maxAttempts: Math.max(1, retry.maxAttempts ?? settings.retryMaxAttempts),
    backoff: {
      retryBackoffBaseMs: retry.baseMs ?? settings.retryBackoffBaseMs,
      retryBackoffCapMs: retry.capMs ?? settings.retryBackoffCapMs,
    },
    random: retry.random ?? cryptoRandom,
    sleep: retry.sleep ?? ((ms: number) => clock.sleep(ms)),
    logger: log,
    explicitLogger: logger !== undefined,
    userAgent: userAgentHeader(userAgent),
    clock,
  });
  const client = createClient<paths>({
    ...clientOptions,
    fetch: (request: Request) => transport.send(request),
  });
  client.use(toleranceMiddleware, transport.middleware);
  const extras: ApiClientExtras = {
    async bindRun<T>(
      runId: string,
      runToken: Secret | string,
      fn: () => Promise<T>,
    ): Promise<T> {
      let binding = transport.bindings.get(runId);
      if (binding === undefined) {
        binding = { runId, token: asSecret(runToken), refs: 0 };
        transport.bindings.set(runId, binding);
      } else {
        binding.token = asSecret(runToken);
      }
      binding.refs++;
      const held = binding;
      try {
        return await transport.storage.run(held, fn);
      } finally {
        held.refs--;
        if (held.refs === 0) transport.bindings.delete(runId);
      }
    },
    setRunToken(runId: string, runToken: Secret | string): void {
      const binding = transport.bindings.get(runId);
      if (binding !== undefined) binding.token = asSecret(runToken);
    },
    unbound<T>(fn: () => Promise<T>): Promise<T> {
      return transport.storage.run(undefined, fn);
    },
    boundRunId: () => transport.boundRunId(),
    dpopNonce: () => transport.nonce,
    settings,
    logger: log,
    clock,
    timeoutMs: timeoutMs ?? settings.httpTimeoutMs,
  };
  return Object.assign(client, extras);
}

/** The result of an openapi-fetch call. */
export interface FetchResult {
  /** The parsed body of a `2xx` answer. */
  readonly data?: unknown;
  /** The parsed body of a failed answer. */
  readonly error?: unknown;
  /** The raw response. */
  readonly response: Response;
}

/** What {@link unwrapDetailed} resolves to. */
export interface Unwrapped<D> {
  /** The parsed body of the `2xx` answer. */
  readonly data: D;
  /** The raw response, for its headers. */
  readonly response: Response;
}

/** The `data` type of the success branch of an openapi-fetch result. */
export type SuccessData<R> = R extends DataOf<infer D> ? D : never;

/** The member `data` of a result; matched by {@link SuccessData}. */
export interface DataOf<D> {
  /** The parsed body of the `2xx` answer. */
  data: D;
}

/**
 * Resolves an openapi-fetch call to its `data` and the response, or throws the typed
 * {@link ProblemError} of a failed answer.
 *
 * Use it where a header of the answer matters (`Idempotent-Replay`, `Cache-Control`, `ETag`).
 *
 * @param call - The pending call, for example `api.GET("/runs/{run_id}", init)`.
 * @param operation - The name of the operation for the error message (its `operationId`).
 * @returns The data of the `2xx` answer and the response.
 * @throws {@link ProblemError} (the subclass follows code and status) for a non-`2xx` answer,
 *   {@link TransportError} or {@link TimeoutError} when no answer arrived.
 */
export async function unwrapDetailed<R extends FetchResult>(
  call: Promise<R>,
  operation: string,
): Promise<Unwrapped<SuccessData<R>>> {
  const result = await call;
  const { response } = result;
  if (response.ok) return { data: result.data as SuccessData<R>, response };
  const retryAfterMs = parseRetryAfter(
    response.headers.get("Retry-After"),
    Date.now(),
  );
  throw problemError(response.status, result.error, {
    retryAfterMs,
    operation,
  });
}

/**
 * Resolves an openapi-fetch call to its `data`, or throws the typed {@link ProblemError}.
 *
 * @param call - The pending call.
 * @param operation - The name of the operation for the error message (its `operationId`).
 * @returns The data of the `2xx` answer (`undefined` for an answer without body).
 * @throws {@link ProblemError} for a non-`2xx` answer, {@link TransportError} or
 *   {@link TimeoutError} when no answer arrived.
 *
 * @example
 * ```ts
 * const run = await unwrap(api.GET("/runs/{run_id}", { params: { path: { run_id } } }), "getRun");
 * ```
 */
export async function unwrap<R extends FetchResult>(
  call: Promise<R>,
  operation: string,
): Promise<SuccessData<R>> {
  return (await unwrapDetailed(call, operation)).data;
}
