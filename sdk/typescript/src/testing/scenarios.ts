// SPDX-License-Identifier: Apache-2.0
/**
 * Scenario functions that run unchanged against the FakeServer and the real server (port of
 * `bothy_sdk/testing/scenarios.py`).
 *
 * {@link ScenarioServer} names only what the scenarios need from a server; `FakeServer` and the
 * `RealServer` of `test/live/sdkserver.ts` implement it (ADR 0026 "Confirmation": the same
 * scenarios against both). Every scenario connects with its own consumer names, so a run one
 * scenario leaves `queued` never reaches another: a new consumer starts at `head_seq` (04 15.1
 * point 2).
 *
 * Scenarios (docs/spec/09-agents.md 16.2, 07 AZ-P6, 09 R-AG-19, sdk/testdata/scenarios/README.md):
 * {@link ag08InvalidTokens}, {@link ag09DoubleDelivery}, {@link ag12PatchRules},
 * {@link ag28SlowStartAndRestart}, {@link azp6AdapterNeverStarts},
 * {@link rag19AllowlistFromServer}. A failed check throws
 * {@link ScenarioFailed}; its message names the check and never contains a token.
 *
 * @example
 * ```ts
 * const fake = new FakeServer();
 * await fake.start();
 * await ag09DoubleDelivery(fake);
 * await fake.stop();
 * ```
 *
 * @module
 */

import { performance } from "node:perf_hooks";

import type { Secret } from "../auth/secret.js";
import { type ApiClient, createApiClient, unwrap } from "../client.js";
import {
  BaseVersionConflict,
  GatewayOpError,
  InvocationTokenInvalid,
  ProblemError,
  ValidationFailed,
} from "../errors.js";
import {
  Gateway,
  GatewaySource,
  SourceReady,
  gatewayUrl,
  type CloseReason,
  type Envelope,
  type EventSource,
} from "../gateway/index.js";
import type { components } from "../generated/openapi.js";
import { EntryState } from "../invocation/index.js";
import {
  Adapter,
  PatchPathError,
  type ActualBilling,
  type AdapterConfig,
  type Invocation,
} from "../runs/index.js";
import {
  KEEP,
  REMOVE,
  RecordingHandler,
  RecordingSource,
  TamperingSource,
} from "./doubles.js";
import {
  FORGED_VARIANTS,
  type ForgedVariant,
  type ScenarioServer,
} from "./scenario-server.js";
import { PATIENCE_MS } from "./patience.js";

export {
  FORGED_VARIANTS,
  type ForgedVariant,
  type RunView,
  type ScenarioServer,
  type Triggered,
} from "./scenario-server.js";

type S = components["schemas"];

const MISSING = "missing";
const START_EVENT = "agent.invocation";
// The waits of `until`: how long a condition may take and how far time moves per round, in
// milliseconds of the server's clock (the values of the Python scenarios).
const UNTIL_LIMIT_MS = 60_000;
const UNTIL_STEP_MS = 50;
// The real pause per round, so the adapter's tasks run between two moves of a fake clock.
const UNTIL_REAL_PAUSE_MS = 10;
// The waits of AG-28 that span a restart of the real server.
const RESTART_LIMIT_MS = 120_000;
// One second beyond a configured window, so the window has surely passed.
const PAST_MS = 1_000;
// How long a stopped adapter may take to return from `run`.
const STOP_WAIT_MS = 30_000;

/** A scenario check failed; the message names the check, never a token. */
export class ScenarioFailed extends Error {
  /**
   * Creates the error.
   *
   * @param message - The failed check.
   */
  constructor(message: string) {
    super(message);
    this.name = "ScenarioFailed";
  }
}

const realPause = (ms: number): Promise<void> =>
  new Promise((resolve) => setTimeout(resolve, ms));

function check(condition: boolean, message: string): void {
  if (!condition) throw new ScenarioFailed(message);
}

/**
 * Waits until `predicate` holds, letting time pass on the server's clock.
 *
 * It fails only after `limit` of server time and {@link PATIENCE_MS} of real time both passed,
 * so a loaded machine that needs more rounds per server second never fails a correct run.
 */
async function until(
  server: ScenarioServer,
  predicate: () => unknown,
  what: string,
  limit: number = UNTIL_LIMIT_MS,
  step: number = UNTIL_STEP_MS,
): Promise<void> {
  let waited = 0;
  const deadline = performance.now() + PATIENCE_MS;
  for (;;) {
    if (await predicate()) return;
    if (waited >= limit && performance.now() > deadline)
      throw new ScenarioFailed(`timed out waiting for ${what}`);
    await server.passTime(step);
    await realPause(UNTIL_REAL_PAUSE_MS);
    waited += step;
  }
}

/**
 * An adapter whose heartbeat leaves the tool class out.
 *
 * The server keeps the reported tool class of the last heartbeat (09 4.8); a reported `host`
 * makes it refuse the triggers of a person who is not the owner (SR2, `tool_class_owner_only`)
 * before the adapter sees a token. The scenarios test the adapter's own check of point 10, so
 * their heartbeats must not race the trigger with the class.
 */
class ScenarioAdapter extends Adapter {
  override heartbeat(): S["Heartbeat"] {
    const beat = super.heartbeat();
    delete beat.reported_tool_class;
    return beat;
  }
}

function adapterFor(
  server: ScenarioServer,
  handler: RecordingHandler,
  consumer: string,
  fetch?: (request: Request) => Promise<Response>,
  allowlist = false,
): Adapter {
  const config: AdapterConfig = {
    baseUrl: server.baseUrl,
    instanceId: server.instanceId,
    agentId: server.agentId,
    credentials: server.credentials,
    consumer,
    allowlist,
    adapterName: "scenario-adapter",
    adapterVersion: "1.0.0",
    runtimeName: "scenario-runtime",
    settings: server.sdkSettings,
  };
  handler.clock = server.clock;
  return new ScenarioAdapter(config, handler, {
    clock: server.clock,
    ...(fetch === undefined ? {} : { fetch }),
  });
}

/** One request as {@link Counting} saw it; the body is kept in memory only. */
interface Seen {
  readonly method: string;
  readonly path: string;
  readonly body: unknown;
}

/** Counts requests by method and path suffix (renewals and starts of AG-28 and AZ-P6). */
class Counting {
  readonly seen: Seen[] = [];

  readonly fetch = async (request: Request): Promise<Response> => {
    let body: unknown;
    try {
      const text = await request.clone().text();
      body = text ? JSON.parse(text) : undefined;
    } catch {
      body = undefined;
    }
    this.seen.push({
      method: request.method,
      path: new URL(request.url).pathname,
      body,
    });
    return globalThis.fetch(request);
  };

  count(method: string, suffix: string): number {
    return this.seen.filter(
      (s) => s.method === method && s.path.endsWith(suffix),
    ).length;
  }
}

/** The task of a running adapter: its promise and, once it returned, its result. */
interface AdapterTask {
  readonly promise: Promise<CloseReason>;
  done: boolean;
  result: CloseReason | undefined;
}

/** Runs an adapter with an optional wrapped source until `body` returns, then stops it. */
async function running<T>(
  adapter: Adapter,
  body: (recording: RecordingSource, task: AdapterTask) => Promise<T>,
  wrap?: (recording: RecordingSource) => EventSource,
): Promise<T> {
  const source = await adapter.makeSource();
  const recording = new RecordingSource(source);
  adapter.source = wrap === undefined ? recording : wrap(recording);
  const promise = adapter.run();
  const task: AdapterTask = { promise, done: false, result: undefined };
  promise.then(
    (reason) => {
      task.result = reason;
      task.done = true;
    },
    () => {
      task.done = true;
    },
  );
  try {
    return await body(recording, task);
  } finally {
    await adapter.stop();
    let timer: NodeJS.Timeout | undefined;
    await Promise.race([
      promise.catch(() => undefined),
      new Promise((resolve) => {
        timer = setTimeout(resolve, STOP_WAIT_MS);
      }),
    ]);
    clearTimeout(timer);
  }
}

function ready(recording: RecordingSource): () => boolean {
  return () => recording.deliveries.some((d) => d instanceof SourceReady);
}

function runIdOf(env: Envelope): string | undefined {
  const payload = env.raw.payload;
  if (typeof payload !== "object" || payload === null) return undefined;
  const runId = (payload as Record<string, unknown>).run_id;
  return typeof runId === "string" ? runId.toLowerCase() : undefined;
}

function envelopeFor(
  recording: RecordingSource,
  runId: string,
  eventType: string,
): Envelope[] {
  const wanted = runId.toLowerCase();
  return recording
    .envelopes(eventType)
    .filter((env) => runIdOf(env) === wanted);
}

function consumerState(
  client: ApiClient,
  consumer: string,
): Promise<S["ConsumerState"]> {
  return unwrap(
    client.GET("/agents/me/consumers/{consumer}", {
      params: { path: { consumer } },
    }),
    "getConsumerState",
  );
}

function clientFor(server: ScenarioServer): ApiClient {
  return createApiClient({
    baseUrl: server.baseUrl,
    token: server.credentials.apiToken,
    dpopKey: server.credentials.dpop,
    settings: server.sdkSettings,
    clock: server.clock,
  });
}

async function expectTokenInvalid(
  call: () => Promise<unknown>,
  reason: string,
  what: string,
): Promise<void> {
  try {
    await call();
  } catch (error) {
    if (error instanceof InvocationTokenInvalid) {
      check(
        error.reason === reason,
        `${what}: reason ${String(error.reason)}, expected ${reason}`,
      );
      return;
    }
    if (error instanceof ProblemError)
      throw new ScenarioFailed(
        `${what}: ${error.status} ${error.code}, expected ${reason}`,
      );
    throw error;
  }
  throw new ScenarioFailed(
    `${what}: accepted, expected invocation_token_invalid (${reason})`,
  );
}

function startRun(
  client: ApiClient,
  runId: string,
  body: S["RunStart"],
): Promise<S["Run"]> {
  return unwrap(
    client.POST("/runs/{run_id}/start", {
      params: { path: { run_id: runId } },
      body,
    }),
    "startRun",
  );
}

/** A promise with its resolver (`Promise.withResolvers` is outside the ES2023 library). */
interface Deferred<T> {
  readonly promise: Promise<T>;
  readonly resolve: (value: T) => void;
}

function deferred<T>(): Deferred<T> {
  let resolve: (value: T) => void = () => undefined;
  const promise = new Promise<T>((r) => {
    resolve = r;
  });
  return { promise, resolve };
}

// ---- AG-08 ------------------------------------------------------------------------------------

/**
 * AG-08 adapter side: no start on a missing or forged token; the run fails and is acknowledged.
 *
 * @param server - The server under test.
 * @returns A promise that resolves when every check held.
 * @throws {@link ScenarioFailed} when a check failed.
 */
export async function ag08InvalidTokens(server: ScenarioServer): Promise<void> {
  type Slot = Deferred<Secret | typeof KEEP | typeof REMOVE>;
  const forged = new Map<string, Slot>();
  const slot = (runId: string): Slot => {
    const key = runId.toLowerCase();
    let found = forged.get(key);
    if (found === undefined) {
      found = deferred();
      forged.set(key, found);
    }
    return found;
  };
  const rewrite = (
    env: Envelope,
  ): Promise<Secret | typeof KEEP | typeof REMOVE> => {
    if (env.type !== START_EVENT) return Promise.resolve(KEEP);
    const runId = runIdOf(env);
    if (runId === undefined) return Promise.resolve(KEEP);
    return slot(runId).promise;
  };
  const handler = new RecordingHandler();
  const adapter = adapterFor(server, handler, "ag08-adapter");
  const client = clientFor(server);
  await running(
    adapter,
    async (rec) => {
      await until(server, ready(rec), "the adapter to connect");
      for (const variant of [MISSING, ...FORGED_VARIANTS]) {
        const triggered = await server.trigger(`AG-08 ${variant} @Lotse`);
        // The adapter keys its runs in lower case.
        const runId = triggered.runId.toLowerCase();
        const token =
          variant === MISSING
            ? REMOVE
            : await server.forgeToken(runId, variant as ForgedVariant);
        slot(runId).resolve(token);
        await until(
          server,
          async () => (await server.runState(runId)).state === "failed",
          `run of ${variant} to fail`,
        );
        const view = await server.runState(runId);
        check(
          view.stateReason === "invalid_invocation_token",
          `${variant}: reason ${String(view.stateReason)}`,
        );
        check(view.starts === 0, `${variant}: the run started`);
        check(
          handler.count("prepare", runId) === 0,
          `${variant}: prepare was called`,
        );
        check(
          handler.count("execute", runId) === 0,
          `${variant}: execute was called`,
        );
        const envs = envelopeFor(rec, runId, START_EVENT);
        check(envs.length > 0, `${variant}: no delivery recorded`);
        const seq = envs[0]?.seq ?? 0;
        await until(
          server,
          async () =>
            (await consumerState(client, "ag08-adapter")).acked_seq >= seq,
          `the acknowledgement of ${variant}`,
        );
      }
    },
    (inner) => new TamperingSource(inner, rewrite),
  );
  // Direct calls with the same tokens for a run no adapter of this scenario receives.
  const queued = await server.trigger("AG-08 direct @Lotse");
  for (const variant of [MISSING, ...FORGED_VARIANTS]) {
    const text =
      variant === MISSING
        ? ""
        : (
            await server.forgeToken(queued.runId, variant as ForgedVariant)
          ).reveal();
    const body: S["RunStart"] = {
      invocation_token: text,
      billing_mode_used: "organization",
      tool_class_used: "chat",
    };
    await expectTokenInvalid(
      () => startRun(client, queued.runId, body),
      "invalid",
      `startRun with ${variant}`,
    );
    await expectTokenInvalid(
      () =>
        unwrap(
          client.POST("/runs/{run_id}/invocation-token", {
            params: { path: { run_id: queued.runId } },
            body: { invocation_token: text },
          }),
          "renewInvocationToken",
        ),
      "invalid",
      `renewInvocationToken with ${variant}`,
    );
  }
  const view = await server.runState(queued.runId);
  check(
    view.state === "queued",
    `direct calls changed the run to ${view.state}`,
  );
}

// ---- AZ-P6 ------------------------------------------------------------------------------------

async function refusedWith(
  server: ScenarioServer,
  handler: RecordingHandler,
  consumer: string,
  reason: string,
): Promise<void> {
  const adapter = adapterFor(server, handler, consumer);
  await running(adapter, async (rec) => {
    await until(server, ready(rec), "the adapter to connect");
    const triggered = await server.trigger(`AZ-P6 ${consumer} @Lotse`);
    await until(
      server,
      async () => (await server.runState(triggered.runId)).state === "failed",
      `the refusal ${reason}`,
    );
    const view = await server.runState(triggered.runId);
    check(
      view.stateReason === reason,
      `${consumer}: reason ${String(view.stateReason)}`,
    );
    check(view.starts === 0, `${consumer}: the run started`);
    check(handler.count("execute") === 0, `${consumer}: execute was called`);
  });
}

/** Reports the billing mode that is not the claim's. */
class OtherMode extends RecordingHandler {
  override actualBilling(inv: Invocation): ActualBilling {
    const other =
      inv.claims.billingMode === "organization" ? "personal" : "organization";
    return { mode: other };
  }
}

/** A server that can deliver a given token again under a new event ID (the FakeServer). */
interface ReplayServer extends ScenarioServer {
  deliverWithToken(envelopeId: string, token: Secret): Promise<string>;
}

function canReplay(server: ScenarioServer): server is ReplayServer {
  return (
    typeof (server as Partial<ReplayServer>).deliverWithToken === "function"
  );
}

/**
 * AZ-P6: the runtime never starts on an invalid, mismatching, replayed or expired token.
 *
 * The forged and missing tokens are AG-08 ({@link ag08InvalidTokens}); here: a billing mode
 * different from the claim and `unknown`, a tool class above the claim, a replayed `jti` under
 * a new event ID (only a server with `deliverWithToken` can send that), and an expired token
 * that is renewed before the start.
 *
 * @param server - The server under test.
 * @returns A promise that resolves when every check held.
 * @throws {@link ScenarioFailed} when a check failed.
 */
export async function azp6AdapterNeverStarts(
  server: ScenarioServer,
): Promise<void> {
  await refusedWith(
    server,
    new RecordingHandler({ billing: "unknown" }),
    "azp6-unknown",
    "billing_mode_mismatch",
  );
  await refusedWith(
    server,
    new OtherMode(),
    "azp6-mode",
    "billing_mode_mismatch",
  );
  await refusedWith(
    server,
    new RecordingHandler({ toolClass: "host" }),
    "azp6-host",
    "tool_class_mismatch",
  );
  await expiredThenRenewed(server, "azp6-expired");
  if (canReplay(server)) await replayedJti(server);
}

async function expiredThenRenewed(
  server: ScenarioServer,
  consumer: string,
): Promise<void> {
  const release = deferred<void>();
  const handler = new RecordingHandler({ blockPrepare: release.promise });
  const counting = new Counting();
  const adapter = adapterFor(server, handler, consumer, counting.fetch);
  await running(adapter, async (rec) => {
    await until(server, ready(rec), "the adapter to connect");
    const triggered = await server.trigger(`AZ-P6 expired ${consumer} @Lotse`);
    await until(server, () => handler.count("prepare") === 1, "prepare");
    const first = handler.invocations[0]?.claims;
    const expire =
      (server.timings["agent.invocation_token_ttl"] ?? 0) +
      server.sdkSettings.invocationTokenLeewayMs +
      PAST_MS;
    await server.passTime(expire);
    release.resolve();
    await until(
      server,
      () => handler.count("execute") === 1,
      "execute after renewal",
    );
    const handle = handler.handles.get(triggered.runId.toLowerCase());
    check(
      counting.count("POST", "/invocation-token") === 1,
      "not exactly one renewal",
    );
    check(counting.count("POST", "/start") === 1, "not exactly one start");
    check(
      handle?.tokenJti !== undefined && handle.tokenJti !== first?.jti,
      "the run started with the expired token",
    );
    const view = await server.runState(triggered.runId);
    check(view.starts === 1, `starts ${view.starts}`);
  });
}

async function replayedJti(server: ReplayServer): Promise<void> {
  const handler = new RecordingHandler();
  const adapter = adapterFor(server, handler, "azp6-replay");
  const client = clientFor(server);
  await running(adapter, async (rec) => {
    await until(server, ready(rec), "the adapter to connect");
    const triggered = await server.trigger("AZ-P6 replay @Lotse");
    await until(server, () => handler.count("execute") === 1, "the first run");
    const first = envelopeFor(rec, triggered.runId, START_EVENT)[0];
    if (first?.token === undefined)
      throw new ScenarioFailed("the delivery had no token");
    const replayId = await server.deliverWithToken(first.id, first.token);
    await until(
      server,
      () => rec.envelopes().some((e) => e.id === replayId),
      "the replayed delivery",
    );
    const replay = rec.envelopes().find((e) => e.id === replayId);
    await server.passTime(PAST_MS);
    const state = await consumerState(client, "azp6-replay");
    check(
      state.acked_seq < (replay?.seq ?? 0),
      "the replayed event was acknowledged",
    );
    check(handler.count("prepare") === 1, "the replayed token reached prepare");
    check(handler.count("execute") === 1, "the replayed token reached execute");
  });
}

// ---- AG-09 ------------------------------------------------------------------------------------

/**
 * AG-09: two adapter processes of one agent; exactly one start, the other closes its entry.
 *
 * Variant: a second adapter with the same consumer makes the first stop with `4015`.
 *
 * @param server - The server under test.
 * @returns A promise that resolves when every check held.
 * @throws {@link ScenarioFailed} when a check failed.
 */
export async function ag09DoubleDelivery(
  server: ScenarioServer,
): Promise<void> {
  const alpha = new RecordingHandler();
  const beta = new RecordingHandler();
  const a = adapterFor(server, alpha, "ag09-alpha");
  const b = adapterFor(server, beta, "ag09-beta");
  await running(a, (recA) =>
    running(b, async (recB) => {
      await until(server, ready(recA), "alpha to connect");
      await until(server, ready(recB), "beta to connect");
      const triggered = await server.trigger("AG-09 @Lotse");
      const runId = triggered.runId;
      await until(
        server,
        () =>
          envelopeFor(recA, runId, START_EVENT).length > 0 &&
          envelopeFor(recB, runId, START_EVENT).length > 0,
        "both deliveries",
      );
      const ta = envelopeFor(recA, runId, START_EVENT)[0]?.token;
      const tb = envelopeFor(recB, runId, START_EVENT)[0]?.token;
      check(
        ta !== undefined && tb !== undefined && !ta.equals(tb),
        "the tokens are not different",
      );
      const settled = async (): Promise<boolean> => {
        if (alpha.count("execute") + beta.count("execute") !== 1) return false;
        for (const adapter of [a, b]) {
          const entries = await adapter.inbox.forRun(runId.toLowerCase());
          if (entries.some((e) => e.state === EntryState.ACCEPTED))
            return false;
        }
        return true;
      };
      await until(server, settled, "one execute and no pending entry");
      const view = await server.runState(runId);
      check(view.starts === 1, `starts ${view.starts}`);
    }),
  );
  // Variant: the same consumer twice.
  const first = adapterFor(server, new RecordingHandler(), "ag09-same");
  const second = adapterFor(server, new RecordingHandler(), "ag09-same");
  await running(first, async (rec1, task1) => {
    await until(server, ready(rec1), "the first adapter to connect");
    await running(second, async (rec2) => {
      await until(server, ready(rec2), "the second adapter to connect");
      await until(server, () => task1.done, "the first adapter to stop");
      const code = task1.result?.code;
      check(code === 4015, `close code ${String(code)}, expected 4015`);
      const recording = first.source;
      const inner =
        recording instanceof RecordingSource ? recording.inner : undefined;
      const gateway =
        inner instanceof GatewaySource ? inner.gateway : undefined;
      if (gateway instanceof Gateway)
        check(
          gateway.connects === 1,
          "the first adapter reconnected after 4015",
        );
    });
  });
}

// ---- AG-12 ------------------------------------------------------------------------------------

function op(
  name: S["JsonPatchOperation"]["op"],
  path: string,
  value?: unknown,
): S["JsonPatchOperation"] {
  const item: S["JsonPatchOperation"] = { op: name, path };
  if (name !== "remove") item.value = value;
  return item;
}

function patchRun(
  client: ApiClient,
  runId: string,
  body: S["RunPatch"],
): Promise<S["RunPatchResult"]> {
  return unwrap(
    client.PATCH("/runs/{run_id}", {
      params: { path: { run_id: runId } },
      body,
    }),
    "patchRun",
  );
}

/**
 * AG-12: forbidden paths and stale versions over REST and the gateway; the patcher rebases.
 *
 * @param server - The server under test.
 * @returns A promise that resolves when every check held.
 * @throws {@link ScenarioFailed} when a check failed.
 */
export async function ag12PatchRules(server: ScenarioServer): Promise<void> {
  const release = deferred<void>();
  const handler = new RecordingHandler({ blockExecute: release.promise });
  const adapter = adapterFor(server, handler, "ag12-adapter");
  const client = clientFor(server);
  await running(adapter, async (rec) => {
    await until(server, ready(rec), "the adapter to connect");
    const triggered = await server.trigger("AG-12 @Lotse");
    const runId = triggered.runId;
    await until(
      server,
      () => handler.count("execute") === 1,
      "the run to start",
    );
    const handle = handler.handles.get(runId.toLowerCase());
    if (handle === undefined) throw new ScenarioFailed("no handle of the run");
    let refused = false;
    try {
      await handle.patch.sendOps([op("replace", "/state", "completed")]);
    } catch (error) {
      if (!(error instanceof PatchPathError)) throw error;
      refused = true;
    }
    check(refused, "sendOps accepted /state");
    const version = (await server.runState(runId)).version;
    try {
      await patchRun(client, runId, {
        base_version: version,
        ops: [op("replace", "/state", "completed")],
      });
      throw new ScenarioFailed("patchRun accepted /state");
    } catch (error) {
      if (!(error instanceof ValidationFailed)) throw error;
      const first = error.errors[0];
      check(first !== undefined, "no field errors");
      check(
        first?.code === "patch_path_forbidden",
        `code ${String(first?.code)}`,
      );
      check(first?.pointer === "/ops/0", `pointer ${String(first?.pointer)}`);
    }
    try {
      await patchRun(client, runId, {
        base_version: version - 1,
        ops: [op("replace", "/model", "m")],
      });
      throw new ScenarioFailed("patchRun accepted a stale base_version");
    } catch (error) {
      if (!(error instanceof BaseVersionConflict)) throw error;
      const current = error.run;
      const serverVersion = (await server.runState(runId)).version;
      if (current === undefined)
        throw new ScenarioFailed("no current document");
      check(
        current.version === serverVersion,
        "current is not the server's version",
      );
    }
    // A concurrent patch moves the version; the patcher rebases.
    const stale = handle.patch.baseVersion;
    await patchRun(client, runId, {
      base_version: (await server.runState(runId)).version,
      ops: [op("replace", "/status_line", "external change")],
    });
    check(
      handle.patch.baseVersion === stale,
      "the patcher saw the change too early",
    );
    handle.patch.setModel("model-rebased");
    await handle.patch.flush();
    const run = await unwrap(
      client.GET("/runs/{run_id}", { params: { path: { run_id: runId } } }),
      "getRun",
    );
    check(
      run.status_line === "external change",
      "the concurrent change is lost",
    );
    check(run.model === "model-rebased", "the rebased change is missing");
    await gatewayCases(server, client, runId);
    release.resolve();
  });
}

async function gatewayCases(
  server: ScenarioServer,
  client: ApiClient,
  runId: string,
): Promise<void> {
  const discovery = await unwrap(
    client.GET("/instance/discovery"),
    "getDiscovery",
  );
  const gateway = new Gateway(
    gatewayUrl(server.baseUrl, discovery.gateway.url),
    server.credentials,
    {
      consumer: "ag12-gateway",
      agentId: server.agentId,
      settings: server.sdkSettings,
      clock: server.clock,
    },
  );
  const errors: GatewayOpError[] = [];
  const reader = (async () => {
    for await (const frame of gateway.frames())
      if (frame.op === "error" && frame.model instanceof GatewayOpError)
        errors.push(frame.model);
  })();
  reader.catch(() => undefined);
  try {
    await until(
      server,
      () => gateway.sessionId !== undefined,
      "the gateway session",
    );
    const version = (await server.runState(runId)).version;
    await gateway.sendRunUpdate(
      runId,
      version,
      [op("replace", "/state", "x")],
      "g1",
    );
    await until(server, () => errors.length >= 1, "the error of /state");
    check(
      errors[0]?.code === "validation_failed",
      `gateway code ${String(errors[0]?.code)}`,
    );
    await gateway.sendRunUpdate(
      runId,
      version - 1,
      [op("replace", "/model", "g")],
      "g2",
    );
    await until(
      server,
      () => errors.length >= 2,
      "the error of the stale version",
    );
    check(
      errors[1]?.code === "base_version_conflict",
      `gateway code ${String(errors[1]?.code)}`,
    );
  } finally {
    await gateway.close();
    await reader.catch(() => undefined);
  }
}

// ---- AG-28 ------------------------------------------------------------------------------------

/**
 * AG-28: acknowledgement after acceptance; a redelivery replaces the token; one start.
 *
 * @param server - The server under test.
 * @returns A promise that resolves when every check held.
 * @throws {@link ScenarioFailed} when a check failed.
 */
export async function ag28SlowStartAndRestart(
  server: ScenarioServer,
): Promise<void> {
  const release = deferred<void>();
  const handler = new RecordingHandler({ blockPrepare: release.promise });
  const counting = new Counting();
  const consumer = "ag28-adapter";
  const adapter = adapterFor(server, handler, consumer, counting.fetch);
  const client = clientFor(server);
  await running(adapter, async (rec) => {
    await until(server, ready(rec), "the adapter to connect");
    const triggered = await server.trigger("AG-28 @Lotse");
    const runId = triggered.runId;
    await until(server, () => handler.count("prepare") === 1, "prepare");
    const env = envelopeFor(rec, runId, START_EVENT)[0];
    if (env === undefined) throw new ScenarioFailed("no delivery recorded");
    const seq = env.seq ?? 0;
    await until(
      server,
      async () => (await consumerState(client, consumer)).acked_seq >= seq,
      "the acknowledgement while queued",
    );
    check(
      (await server.runState(runId)).state === "queued",
      "the run is not queued",
    );
    await server.passTime((server.timings["agent.ack_timeout"] ?? 0) + PAST_MS);
    check(
      envelopeFor(rec, runId, START_EVENT).length === 1,
      "the event was delivered again before the restart",
    );
    const firstToken = env.token;
    await server.restart(consumer, seq - 1);
    await until(
      server,
      () => envelopeFor(rec, runId, START_EVENT).length >= 2,
      "the redelivery after the restart",
      RESTART_LIMIT_MS,
    );
    await until(
      server,
      async () => {
        const entry = await adapter.inbox.get(env.id);
        return (
          entry !== undefined &&
          entry.deliveries >= 2 &&
          (entry.token === undefined ||
            firstToken === undefined ||
            !entry.token.equals(firstToken))
        );
      },
      "the newer token in the inbox",
    );
    check(handler.count("prepare") === 1, "the redelivery was prepared again");
    if (firstToken === undefined)
      throw new ScenarioFailed("the delivered invocation had no token");
    const billing =
      handler.invocations[0]?.claims.billingMode ?? "organization";
    await expectTokenInvalid(
      () =>
        startRun(client, runId, {
          invocation_token: firstToken.reveal(),
          billing_mode_used: billing as S["BillingMode"],
          tool_class_used: "chat",
        }),
      "superseded",
      "startRun with the first token",
    );
    await server.passTime(
      (server.timings["agent.invocation_token_ttl"] ?? 0) +
        (server.timings["auth.clock_leeway"] ?? 0) +
        PAST_MS,
    );
    const renewalsBefore = counting.count("POST", "/invocation-token");
    release.resolve();
    await until(
      server,
      () => handler.count("execute") === 1,
      "execute",
      RESTART_LIMIT_MS,
    );
    check(
      counting.count("POST", "/invocation-token") - renewalsBefore === 1,
      "not exactly one renewal",
    );
    check(counting.count("POST", "/start") === 1, "not exactly one start");
    const handle = handler.handles.get(runId.toLowerCase());
    const entry = await adapter.inbox.get(env.id);
    check(
      entry !== undefined &&
        handle !== undefined &&
        handle.tokenJti === entry.tokenJti,
      "the start did not use the renewed token",
    );
    const view = await server.runState(runId);
    check(view.starts === 1, `starts ${view.starts}`);
    check(handler.count("execute") === 1, "execute ran more than once");
  });
}

// ---- R-AG-19 ----------------------------------------------------------------------------------

/**
 * R-AG-19 (09 17.7): the adapter's allow list comes from the server and admits the trigger.
 *
 * The agent reads `getAgentTriggerers` with its own credentials; then an adapter with the allow
 * list on runs Alex's trigger, so the server's list holds the person the server let trigger, in
 * the form the SDK decodes.
 *
 * @param server - The server under test.
 * @returns A promise that resolves when every check held.
 * @throws {@link ScenarioFailed} when a check failed.
 */
export async function rag19AllowlistFromServer(
  server: ScenarioServer,
): Promise<void> {
  let allow: S["AgentTriggerers"];
  try {
    allow = await unwrap(
      clientFor(server).GET("/agents/me/triggerers"),
      "getAgentTriggerers",
    );
  } catch (error) {
    if (error instanceof ProblemError)
      throw new ScenarioFailed(
        `getAgentTriggerers: ${error.status} ${error.code}`,
      );
    throw error;
  }
  const people = allow.principal_ids.map((id) => id.toLowerCase());
  check(people.length > 0, "the allow list is empty");
  check(
    new Set(people).size === people.length,
    "the allow list repeats a person",
  );
  check(
    !people.includes(server.agentId.toLowerCase()) &&
      !people.includes(server.otherAgentId.toLowerCase()),
    "an agent is on the allow list",
  );
  check(
    allow.permission_version >= 1,
    `permission_version ${allow.permission_version}`,
  );
  const handler = new RecordingHandler();
  const counting = new Counting();
  const adapter = adapterFor(
    server,
    handler,
    "rag19-allow",
    counting.fetch,
    true,
  );
  await running(adapter, async (rec) => {
    await until(server, ready(rec), "the adapter to connect");
    const triggered = await server.trigger("R-AG-19 @Lotse");
    await until(server, () => handler.count("execute") === 1, "execute");
    check(
      counting.count("GET", "/agents/me/triggerers") >= 1,
      "the adapter did not load the allow list",
    );
    const view = await server.runState(triggered.runId);
    check(view.starts === 1, `starts ${view.starts}`);
  });
}
