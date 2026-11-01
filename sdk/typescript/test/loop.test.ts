// SPDX-License-Identifier: Apache-2.0
//
// The adapter loop against the FakeServer: acceptance order, start outcomes, resume, cancel,
// restart, commands and the run binding of `Invocation-Token` (port of
// sdk/python/tests/test_adapter.py plus the cases of the brief's "loop.test.ts" bullet). The
// few answers the FakeServer has no rule for (`agent_invocation_denied`, a startRun refused with
// `invalid`) come from a wrapped `fetch` in front of it. Tokens are compared by fingerprint
// inside the test only and never printed (SEC-LOG-2).

import { createHash } from "node:crypto";
import { mkdtemp, readdir, rm, stat } from "node:fs/promises";
import { tmpdir } from "node:os";
import { join } from "node:path";

import { afterEach, beforeEach, describe, expect, it } from "vitest";

import { Secret } from "../src/auth/secret.js";
import { INVOCATION_TOKEN_HEADER } from "../src/client.js";
import {
  Envelope,
  GatewaySource,
  SourceReady,
  type Delivery,
  type EventSource,
} from "../src/gateway/index.js";
import type { components } from "../src/generated/openapi.js";
import {
  EntryState,
  MemoryInboxStore,
  type InboxEntry,
} from "../src/invocation/index.js";
import { silentLogger } from "../src/logger.js";
import {
  Adapter,
  AgentHandler,
  RuntimeFailure,
  type ActualBilling,
  type AdapterConfig,
  type AdapterOptions,
  type Invocation,
  type RunHandle,
  type StartOptions,
} from "../src/runs/index.js";
import { FakeServer } from "../src/testing/fakeserver.js";
import { Fault } from "../src/testing/faults.js";
import {
  eventually as patiently,
  PATIENCE_MS,
  withinPatience,
} from "../src/testing/patience.js";

type S = components["schemas"];
type Log = string[];

// Real turns per waiting step; the FakeServer's clock moves one step per turn.
const STEP_MS = 50;
// The runner limit of the tests that start an adapter: the patience, never a few seconds.
const TEST_TIMEOUT = PATIENCE_MS;

/** The first 16 hex characters of the SHA-256 of a token (never the token itself). */
function fingerprint(token: Secret | string): string {
  const text = typeof token === "string" ? token : token.reveal();
  return createHash("sha256").update(text).digest("hex").slice(0, 16);
}

/** Waits until `predicate` holds, letting fake time pass between real turns. */
async function until(
  fake: FakeServer,
  predicate: () => unknown,
): Promise<void> {
  await patiently(predicate, "the condition of the test", {
    step: () => fake.passTime(STEP_MS),
  });
}

/** A gate a handler waits on until the test opens it. */
class Gate {
  private release: () => void = () => undefined;
  readonly opened = new Promise<void>((resolve) => {
    this.release = resolve;
  });

  open(): void {
    this.release();
  }
}

/** The FakeServer's world, for a step the FakeServer has no helper for. */
function worldOf(server: FakeServer): {
  followup(runId: string, text: string): string;
} {
  return (
    server as unknown as {
      world: { followup(runId: string, text: string): string };
    }
  ).world;
}

/**
 * Lets Alex post a second answer while the first answer's `resumeRun` is on its way, so the
 * server answers it `superseded`; `posted` runs right after.
 */
function answerDuringResume(
  runId: () => string,
  posted?: () => void,
): Override {
  let done = false;
  return (request) => {
    if (
      !done &&
      request.method === "POST" &&
      new URL(request.url).pathname.endsWith("/resume")
    ) {
      done = true;
      worldOf(fake).followup(runId(), "no, the second");
      posted?.();
    }
    return undefined;
  };
}

/** Holds back every `agent.followup` after the first one until `gate` opens, or drops it. */
function secondAnswer(gate: Gate | "drop"): Hold {
  let first: string | undefined;
  return (delivery) => {
    if (!(delivery instanceof Envelope) || delivery.type !== "agent.followup")
      return undefined;
    first ??= delivery.id;
    if (delivery.id === first) return undefined;
    return gate === "drop" ? "drop" : gate.opened;
  };
}

/** Records every call; can block in `prepare` or `execute`. */
class RecordingHandler extends AgentHandler {
  readonly calls: [string, string | undefined][] = [];
  readonly handles = new Map<string, RunHandle>();
  readonly invocations: Invocation[] = [];
  billing: string | undefined;
  toolClass: S["ToolClass"] = "chat";
  blockPrepare: Gate | undefined;
  blockExecute: Gate | undefined;
  options: StartOptions = { model: "model-test" };

  constructor(
    init: {
      blockPrepare?: Gate;
      blockExecute?: Gate;
      billing?: string;
      toolClass?: S["ToolClass"];
    } = {},
  ) {
    super();
    this.blockPrepare = init.blockPrepare;
    this.blockExecute = init.blockExecute;
    this.billing = init.billing;
    if (init.toolClass !== undefined) this.toolClass = init.toolClass;
  }

  record(name: string, runId: string | undefined): void {
    this.calls.push([name, runId]);
  }

  count(name: string, runId?: string): number {
    return this.calls.filter(
      ([n, r]) => n === name && (runId === undefined || r === runId),
    ).length;
  }

  override actualBilling(inv: Invocation): ActualBilling {
    return { mode: this.billing ?? inv.claims.billingMode };
  }

  override actualToolClass(): S["ToolClass"] {
    return this.toolClass;
  }

  override async prepare(
    inv: Invocation,
    run: RunHandle,
  ): Promise<StartOptions> {
    this.record("prepare", run.runId);
    this.invocations.push(inv);
    this.handles.set(run.runId, run);
    if (this.blockPrepare !== undefined) await this.blockPrepare.opened;
    return this.options;
  }

  override async execute(_inv: Invocation, run: RunHandle): Promise<void> {
    this.record("execute", run.runId);
    this.handles.set(run.runId, run);
    if (this.blockExecute !== undefined) await this.blockExecute.opened;
  }

  override onFollowup(_inv: Invocation, run: RunHandle): Promise<void> {
    this.record("onFollowup", run.runId);
    return Promise.resolve();
  }

  override onInteraction(_inv: Invocation, run: RunHandle): Promise<void> {
    this.record("onInteraction", run.runId);
    return Promise.resolve();
  }

  override onCommand(
    _inv: Invocation,
    run: RunHandle | undefined,
  ): Promise<void> {
    this.record("onCommand", run?.runId);
    return Promise.resolve();
  }
}

/** Asks once; records the event of every answer it gets. */
class Answering extends RecordingHandler {
  readonly answered: string[] = [];

  override async execute(inv: Invocation, run: RunHandle): Promise<void> {
    await super.execute(inv, run);
    await run.awaitInput("Which file?");
  }

  override async onFollowup(inv: Invocation, run: RunHandle): Promise<void> {
    await super.onFollowup(inv, run);
    this.answered.push(inv.eventId);
  }
}

/** Calls `watch` once, after the first `forRun` that follows its setting. */
class WatchedInbox extends MemoryInboxStore {
  watch: (() => void) | undefined;
  private watched = false;

  override async forRun(runId: string): Promise<InboxEntry[]> {
    const entries = await super.forRun(runId);
    if (this.watch !== undefined && !this.watched) {
      this.watched = true;
      this.watch();
    }
    return entries;
  }
}

/** Logs `prepare` and `execute` into a shared log. */
class LoggingHandler extends RecordingHandler {
  constructor(readonly log: Log) {
    super();
  }

  override async prepare(
    inv: Invocation,
    run: RunHandle,
  ): Promise<StartOptions> {
    this.log.push("prepare");
    return super.prepare(inv, run);
  }

  override async execute(inv: Invocation, run: RunHandle): Promise<void> {
    this.log.push("execute");
    await super.execute(inv, run);
  }
}

type Rewrite = (env: Envelope) => Secret | null | undefined;

/**
 * Returns a promise a delivery waits on before the adapter sees it, `"drop"` to never pass it
 * on (a delivery the server skips), or `undefined`.
 */
type Hold = (delivery: Delivery) => Promise<void> | "drop" | undefined;

/** Records every delivery; may rewrite (`Secret`) or remove (`null`) a token, or hold it back. */
class RecordingSource implements EventSource {
  readonly deliveries: Delivery[] = [];
  /** Opens on `close`: a held delivery is then dropped and the iteration ends. */
  private readonly closing = new Gate();

  constructor(
    readonly inner: EventSource,
    private readonly rewrite?: Rewrite,
    private readonly log?: Log,
    /** Never acknowledge, so `FakeServer.redeliver` brings every event again. */
    private readonly holdAcks = false,
    private readonly hold?: Hold,
  ) {}

  get closeReason(): EventSource["closeReason"] {
    return this.inner.closeReason;
  }

  async *[Symbol.asyncIterator](): AsyncGenerator<Delivery, void, undefined> {
    for await (const delivery of this.inner) {
      let out = delivery;
      if (
        this.rewrite !== undefined &&
        delivery instanceof Envelope &&
        delivery.token !== undefined
      ) {
        const token = this.rewrite(delivery);
        if (token !== undefined)
          out = new Envelope({ ...delivery, token: token ?? undefined });
      }
      const held = this.hold?.(out);
      if (held === "drop") continue;
      if (held !== undefined) {
        let dropped = false;
        await Promise.race([
          held,
          this.closing.opened.then(() => {
            dropped = true;
          }),
        ]);
        if (dropped) return;
      }
      this.deliveries.push(out);
      yield out;
    }
  }

  envelopes(type?: string): Envelope[] {
    return this.deliveries.filter(
      (d): d is Envelope =>
        d instanceof Envelope && (type === undefined || d.type === type),
    );
  }

  get ready(): boolean {
    return this.deliveries.some((d) => d instanceof SourceReady);
  }

  async settle(seq: number): Promise<void> {
    if (this.holdAcks) return;
    if (this.log !== undefined && this.inner instanceof GatewaySource) {
      // Route through this wrapper so the acknowledgement is visible in the log.
      const point = this.inner.window.settle(seq);
      if (point !== undefined) {
        this.log.push("ack");
        await this.inner.ack(point);
      }
      return;
    }
    await this.inner.settle(seq);
  }

  advance(seq: number): Promise<void> {
    return this.inner.advance(seq);
  }

  ack(seq: number): Promise<void> {
    if (this.holdAcks) return Promise.resolve();
    return this.inner.ack(seq);
  }

  answerAutocomplete(
    requestId: string,
    choices: readonly S["CommandChoice"][],
  ): Promise<void> {
    return this.inner.answerAutocomplete(requestId, choices);
  }

  close(): Promise<void> {
    this.closing.open();
    return this.inner.close();
  }
}

interface Seen {
  readonly method: string;
  readonly path: string;
  readonly status: number;
  /** Fingerprint of `Invocation-Token`, if the request carried one. */
  readonly bound: string | undefined;
}

type Override = (request: Request) => Response | undefined;

/** A `fetch` in front of the FakeServer: records requests, may answer some itself. */
class RecordingFetch {
  readonly seen: Seen[] = [];
  readonly renewed: string[] = [];
  override: Override | undefined;

  constructor(private readonly log?: Log) {}

  readonly fetch = async (request: Request): Promise<Response> => {
    const header = request.headers.get(INVOCATION_TOKEN_HEADER);
    const path = new URL(request.url).pathname;
    const response =
      this.override?.(request) ?? (await globalThis.fetch(request));
    this.seen.push({
      method: request.method,
      path,
      status: response.status,
      bound: header === null ? undefined : fingerprint(header),
    });
    const name = path.split("/").at(-1) ?? "";
    if (
      request.method === "POST" &&
      ["start", "resume", "finish", "invocation-token"].includes(name)
    )
      this.log?.push(`${name}:${response.status}`);
    if (name === "invocation-token" && response.ok) {
      const grant = (await response
        .clone()
        .json()) as S["InvocationTokenGrant"];
      this.renewed.push(fingerprint(grant.invocation_token));
    }
    return response;
  };

  count(method: string, suffix: string): number {
    return this.seen.filter(
      (s) => s.method === method && s.path.endsWith(suffix),
    ).length;
  }
}

/** A problem answer of the server. */
function problem(status: number, code: string, reason?: string): Response {
  return new Response(
    JSON.stringify({
      type: "about:blank",
      title: code,
      status,
      code,
      ...(reason === undefined ? {} : { reason }),
    }),
    { status, headers: { "Content-Type": "application/problem+json" } },
  );
}

/** Answers the first `times` requests of `suffix` with `answer`, then lets them through. */
function answerFirst(
  suffix: string,
  answer: () => Response,
  times = 1,
): Override {
  let left = times;
  return (request) => {
    if (
      left > 0 &&
      request.method === "POST" &&
      new URL(request.url).pathname.endsWith(suffix)
    ) {
      left--;
      return answer();
    }
    return undefined;
  };
}

class LoggingInbox extends MemoryInboxStore {
  constructor(readonly log: Log) {
    super();
  }

  override async add(entry: InboxEntry): Promise<void> {
    await super.add(entry);
    this.log.push(`store:${entry.type}`);
  }
}

let fake: FakeServer;
const dirs: string[] = [];

beforeEach(async () => {
  fake = new FakeServer();
  await fake.start();
});

afterEach(async () => {
  await fake.stop();
  for (const dir of dirs.splice(0))
    await rm(dir, { recursive: true, force: true });
});

function makeAdapter(
  handler: AgentHandler,
  consumer: string,
  extra: Partial<AdapterConfig> = {},
  options: AdapterOptions = {},
): Adapter {
  const config: AdapterConfig = {
    baseUrl: fake.baseUrl,
    instanceId: fake.instanceId,
    agentId: fake.agentId,
    credentials: fake.credentials,
    consumer,
    settings: fake.sdkSettings,
    ...extra,
  };
  return new Adapter(config, handler, {
    clock: fake.clock,
    logger: silentLogger,
    ...options,
  });
}

interface Running {
  readonly source: RecordingSource;
  readonly done: Promise<unknown>;
}

/** Whether the FakeServer holds a cursor of `consumer`, so it delivers events after now. */
function knowsConsumer(consumer: string): boolean {
  try {
    fake.consumerState(consumer);
    return true;
  } catch {
    return false;
  }
}

/**
 * Runs `adapter` with a recording source until `body` returns, then stops it.
 *
 * `body` starts once the source is ready and the server holds the consumer's cursor: the first
 * poll creates the cursor at the head (09 4.2), and a polling source is ready before that poll,
 * so an event of the body before it would never be delivered.
 */
async function running<T>(
  adapter: Adapter,
  body: (run: Running) => Promise<T>,
  wrap: { rewrite?: Rewrite; log?: Log; holdAcks?: boolean; hold?: Hold } = {},
): Promise<T> {
  const source = new RecordingSource(
    await adapter.makeSource(),
    wrap.rewrite,
    wrap.log,
    wrap.holdAcks,
    wrap.hold,
  );
  adapter.source = source;
  const done = adapter.run();
  done.catch(() => undefined);
  try {
    await until(
      fake,
      () => source.ready && knowsConsumer(adapter.config.consumer),
    );
    return await body({ source, done });
  } finally {
    await adapter.stop();
    await withinPatience(
      done.catch(() => undefined),
      "the end of the adapter loop",
    );
  }
}

/** The adapter's private run table, read the way the Python tests read `_runs`. */
interface Internals {
  readonly runs: Map<string, { readonly waitingForToken: boolean }>;
  renew(state: unknown, token: Secret): Promise<Secret>;
}

function internals(adapter: Adapter): Internals {
  return adapter as unknown as Internals;
}

function waitingForToken(adapter: Adapter, runId: string): boolean {
  return internals(adapter).runs.get(runId)?.waitingForToken ?? false;
}

async function stateOf(runId: string): Promise<string> {
  return (await fake.runState(runId)).state;
}

async function closed(adapter: Adapter, runId: string): Promise<boolean> {
  const entries = await adapter.inbox.forRun(runId);
  return (
    entries.length > 0 && entries.every((e) => e.state === EntryState.CLOSED)
  );
}

describe("acceptance and start", () => {
  it(
    "stores, acknowledges, prepares, starts and only then executes",
    async () => {
      const log: Log = [];
      const recording = new RecordingFetch(log);
      const adapter = makeAdapter(
        new LoggingHandler(log),
        "order",
        {},
        { inbox: new LoggingInbox(log), fetch: recording.fetch },
      );
      await running(
        adapter,
        async () => {
          const triggered = await fake.trigger();
          await until(fake, () => log.includes("execute"));
          await until(
            fake,
            async () => (await stateOf(triggered.runId)) === "completed",
          );
        },
        { log },
      );
      const steps = [
        "store:agent.invocation",
        "ack",
        "prepare",
        "start:200",
        "execute",
      ];
      const first = steps.map((s) => log.indexOf(s));
      expect(first.every((i) => i >= 0)).toBe(true);
      expect(first).toEqual([...first].sort((a, b) => a - b));
    },
    TEST_TIMEOUT,
  );

  it(
    "executes only after 200 on startRun",
    async () => {
      const log: Log = [];
      const recording = new RecordingFetch(log);
      const adapter = makeAdapter(
        new LoggingHandler(log),
        "after200",
        {},
        {
          fetch: recording.fetch,
        },
      );
      fake.inject("startRun", Fault.rateLimited(1));
      await running(adapter, async () => {
        await fake.trigger();
        await until(fake, () => log.includes("execute"));
      });
      const starts = log.filter((e) => e.startsWith("start:"));
      expect([["start:429", "start:200"], ["start:200"]]).toContainEqual(
        starts,
      );
      expect(log.indexOf("execute")).toBeGreaterThan(log.indexOf("start:200"));
    },
    TEST_TIMEOUT,
  );

  it(
    "takes an Idempotent-Replay 200 after a lost answer as the start",
    async () => {
      const log: Log = [];
      const handler = new LoggingHandler(log);
      const adapter = makeAdapter(
        handler,
        "replay200",
        {},
        {
          fetch: new RecordingFetch(log).fetch,
        },
      );
      fake.inject("startRun", Fault.storedThen502());
      await running(adapter, async () => {
        const triggered = await fake.trigger();
        await until(
          fake,
          async () => (await stateOf(triggered.runId)) === "completed",
        );
        expect((await fake.runState(triggered.runId)).starts).toBe(1);
      });
      expect(log.filter((e) => e.startsWith("start:"))).toEqual([
        "start:502",
        "start:200",
      ]);
      expect(handler.count("execute")).toBe(1);
    },
    TEST_TIMEOUT,
  );

  it(
    "finishes the run failed with invalid_invocation_token on reason invalid",
    async () => {
      const handler = new RecordingHandler();
      const recording = new RecordingFetch();
      recording.override = answerFirst("/start", () =>
        problem(403, "invocation_token_invalid", "invalid"),
      );
      const adapter = makeAdapter(
        handler,
        "start-invalid",
        {},
        {
          fetch: recording.fetch,
        },
      );
      await running(adapter, async () => {
        const triggered = await fake.trigger();
        await until(
          fake,
          async () => (await stateOf(triggered.runId)) === "failed",
        );
        const view = await fake.runState(triggered.runId);
        expect(view.stateReason).toBe("invalid_invocation_token");
        expect(view.starts).toBe(0);
        await until(fake, () => closed(adapter, triggered.runId));
      });
      expect(handler.count("execute")).toBe(0);
    },
    TEST_TIMEOUT,
  );

  it.each([
    [409, "already_started"],
    [409, "billing_mode_mismatch"],
    [409, "tool_class_mismatch"],
    [403, "agent_invocation_denied"],
  ])(
    "drops the entry on %i %s and starts nothing",
    async (status, code) => {
      const handler = new RecordingHandler();
      const recording = new RecordingFetch();
      recording.override = answerFirst("/start", () => problem(status, code));
      const adapter = makeAdapter(
        handler,
        `drop-${code.replaceAll("_", "-")}`,
        {},
        {
          fetch: recording.fetch,
        },
      );
      await running(adapter, async () => {
        const triggered = await fake.trigger();
        await until(fake, () => closed(adapter, triggered.runId));
        await fake.passTime(1_000);
        // The server already decided: no finishRun, no second start.
        expect(recording.count("POST", "/finish")).toBe(0);
        expect(recording.count("POST", "/start")).toBe(1);
        expect(await stateOf(triggered.runId)).toBe("queued");
      });
      expect(handler.count("execute")).toBe(0);
    },
    TEST_TIMEOUT,
  );

  it(
    "drops the entry when the server itself answers 409 billing_mode_mismatch",
    async () => {
      const handler = new RecordingHandler();
      handler.options = {
        model: "model-test",
        billingEvidence: { key_type: "unknown" },
      };
      const recording = new RecordingFetch();
      const adapter = makeAdapter(
        handler,
        "server-mismatch",
        {},
        {
          fetch: recording.fetch,
        },
      );
      await running(adapter, async () => {
        const triggered = await fake.trigger();
        await until(fake, () => closed(adapter, triggered.runId));
        const view = await fake.runState(triggered.runId);
        expect([view.state, view.stateReason]).toEqual([
          "failed",
          "billing_mode_mismatch",
        ]);
        expect(recording.count("POST", "/finish")).toBe(0);
      });
      expect(handler.count("execute")).toBe(0);
    },
    TEST_TIMEOUT,
  );

  it(
    "renews on reason expired and starts once more, then gives up",
    async () => {
      const handler = new RecordingHandler();
      const recording = new RecordingFetch();
      recording.override = answerFirst(
        "/start",
        () => problem(403, "invocation_token_invalid", "expired"),
        2,
      );
      const adapter = makeAdapter(
        handler,
        "start-expired",
        {},
        {
          fetch: recording.fetch,
        },
      );
      await running(adapter, async () => {
        const triggered = await fake.trigger();
        await until(
          fake,
          async () => (await stateOf(triggered.runId)) === "failed",
        );
        expect((await fake.runState(triggered.runId)).stateReason).toBe(
          "invalid_invocation_token",
        );
      });
      expect(recording.count("POST", "/invocation-token")).toBe(1);
      expect(recording.count("POST", "/start")).toBe(2);
      expect(handler.count("execute")).toBe(0);
    },
    TEST_TIMEOUT,
  );

  it(
    "never acknowledges an event whose key is unknown and accepts its redelivery",
    async () => {
      const handler = new RecordingHandler();
      let tamper = true;
      const rewrite: Rewrite = (env) => {
        if (
          !tamper ||
          env.type !== "agent.invocation" ||
          env.token === undefined
        )
          return undefined;
        const [head, body, sig] = env.token.reveal().split(".");
        const header = JSON.parse(
          Buffer.from(head ?? "", "base64url").toString("utf8"),
        ) as Record<string, unknown>;
        header.kid = "unknown-key-of-example-com";
        const forged = Buffer.from(JSON.stringify(header)).toString(
          "base64url",
        );
        return new Secret(`${forged}.${body ?? ""}.${sig ?? ""}`);
      };
      const adapter = makeAdapter(handler, "unknown-kid");
      await running(
        adapter,
        async ({ source }) => {
          const triggered = await fake.trigger();
          await until(
            fake,
            () => source.envelopes("agent.invocation").length === 1,
          );
          const seq = source.envelopes("agent.invocation")[0]?.seq ?? 0;
          await fake.passTime(fake.timings["agent.ack_persist_interval"] * 2);
          expect(fake.consumerState("unknown-kid").ackedSeq).toBeLessThan(seq);
          expect(await adapter.inbox.forRun(triggered.runId)).toEqual([]);
          expect(handler.count("prepare")).toBe(0);
          // The server delivers again after agent.ack_timeout with a current key.
          tamper = false;
          await fake.passTime(fake.timings["agent.ack_timeout"]);
          await until(
            fake,
            () => handler.count("execute", triggered.runId) === 1,
          );
        },
        { rewrite },
      );
    },
    TEST_TIMEOUT,
  );
});

describe("runs in progress", () => {
  it(
    "resumes a waiting run on a follow-up and on an interaction",
    async () => {
      class Waiting extends RecordingHandler {
        override async execute(inv: Invocation, run: RunHandle): Promise<void> {
          await super.execute(inv, run);
          await run.awaitInput("Which file?");
        }

        override async onFollowup(
          inv: Invocation,
          run: RunHandle,
        ): Promise<void> {
          await super.onFollowup(inv, run);
          await run.awaitInput("Approve?");
        }
      }
      const handler = new Waiting();
      const adapter = makeAdapter(handler, "resume");
      let runId = "";
      await running(adapter, async () => {
        runId = (await fake.trigger()).runId;
        await until(
          fake,
          async () => (await stateOf(runId)) === "waiting_input",
        );
        await fake.deliverFollowup(runId);
        await until(fake, () => handler.count("onFollowup") === 1);
        await until(
          fake,
          async () => (await stateOf(runId)) === "waiting_input",
        );
        await fake.deliverInteraction(runId);
        await until(fake, () => handler.count("onInteraction") === 1);
        await until(fake, async () => (await stateOf(runId)) === "completed");
      });
      expect((await fake.runState(runId)).starts).toBe(3); // the start and two resumes
    },
    TEST_TIMEOUT,
  );

  it(
    "hands follow-ups and interactions of a running run to the handle",
    async () => {
      const gate = new Gate();
      const handler = new RecordingHandler({ blockExecute: gate });
      const recording = new RecordingFetch();
      const adapter = makeAdapter(
        handler,
        "running-followup",
        {},
        {
          fetch: recording.fetch,
        },
      );
      await running(adapter, async () => {
        const { runId } = await fake.trigger();
        await until(fake, () => handler.count("execute") === 1);
        await fake.deliverFollowup(runId);
        await fake.deliverInteraction(runId);
        await until(
          fake,
          () =>
            handler.count("onFollowup", runId) === 1 &&
            handler.count("onInteraction", runId) === 1,
        );
        expect(recording.count("POST", "/resume")).toBe(0);
        expect((await fake.runState(runId)).starts).toBe(1);
        gate.open();
        await until(fake, async () => (await stateOf(runId)) === "completed");
      });
    },
    TEST_TIMEOUT,
  );

  it(
    "reaches the handler with both answers to one question",
    async () => {
      class TwoAnswers extends RecordingHandler {
        readonly both = new Gate();

        override async execute(inv: Invocation, run: RunHandle): Promise<void> {
          await super.execute(inv, run);
          await run.awaitInput("Which file?");
        }

        override async onFollowup(
          inv: Invocation,
          run: RunHandle,
        ): Promise<void> {
          await super.onFollowup(inv, run);
          if (this.count("onFollowup") === 2) this.both.open();
          await this.both.opened;
        }
      }
      const handler = new TwoAnswers();
      const adapter = makeAdapter(handler, "two-answers");
      let runId = "";
      await running(adapter, async () => {
        runId = (await fake.trigger()).runId;
        await until(
          fake,
          async () => (await stateOf(runId)) === "waiting_input",
        );
        await fake.deliverFollowup(runId, "the first file");
        await fake.deliverFollowup(runId, "no, the second");
        await until(fake, () => handler.count("onFollowup") === 2);
        await until(fake, async () => (await stateOf(runId)) === "completed");
      });
      expect((await fake.runState(runId)).starts).toBe(2); // the start and one resume
    },
    TEST_TIMEOUT,
  );

  it(
    "hands both answers over in order when the first resume is superseded before the second arrives",
    async () => {
      const handler = new Answering();
      const inbox = new WatchedInbox();
      const second = new Gate();
      const recording = new RecordingFetch();
      const adapter = makeAdapter(
        handler,
        "superseded-answer",
        {},
        { inbox, fetch: recording.fetch },
      );
      let runId = "";
      // The second answer is posted while the first one's resumeRun is on its way, and the
      // adapter sees it only after it looked for a newer answer in the inbox and found none.
      recording.override = answerDuringResume(
        () => runId,
        () => {
          inbox.watch = () => second.open();
        },
      );
      await running(
        adapter,
        async ({ source }) => {
          runId = (await fake.trigger()).runId;
          await until(
            fake,
            async () => (await stateOf(runId)) === "waiting_input",
          );
          await fake.deliverFollowup(runId, "the first file");
          await until(fake, () => handler.answered.length === 2);
          await until(fake, async () => (await stateOf(runId)) === "completed");
          expect(handler.answered).toEqual(
            source.envelopes("agent.followup").map((e) => e.id),
          );
        },
        { hold: secondAnswer(second) },
      );
      expect(recording.count("POST", "/resume")).toBe(2); // superseded, then the newer token
      expect((await fake.runState(runId)).starts).toBe(2); // the start and one resume
    },
    TEST_TIMEOUT,
  );

  it(
    "stops while a superseded answer waits for the newer one",
    async () => {
      const handler = new Answering();
      const recording = new RecordingFetch();
      const adapter = makeAdapter(
        handler,
        "superseded-stop",
        {},
        { fetch: recording.fetch },
      );
      let runId = "";
      recording.override = answerDuringResume(() => runId);
      await running(
        adapter,
        async () => {
          runId = (await fake.trigger()).runId;
          await until(
            fake,
            async () => (await stateOf(runId)) === "waiting_input",
          );
          await fake.deliverFollowup(runId, "the first file");
          await until(fake, () => recording.count("POST", "/resume") === 1);
        },
        // The second answer never reaches the adapter.
        { hold: secondAnswer("drop") },
      );
      expect(handler.answered).toEqual([]);
      expect((await fake.runState(runId)).state).toBe("waiting_input");
    },
    TEST_TIMEOUT,
  );

  it(
    "closes a superseded answer waiting for the newer one when the run is cancelled",
    async () => {
      const handler = new Answering();
      const recording = new RecordingFetch();
      const adapter = makeAdapter(
        handler,
        "superseded-cancel",
        {},
        { fetch: recording.fetch },
      );
      let runId = "";
      recording.override = answerDuringResume(() => runId);
      await running(
        adapter,
        async () => {
          runId = (await fake.trigger()).runId;
          await until(
            fake,
            async () => (await stateOf(runId)) === "waiting_input",
          );
          await fake.deliverFollowup(runId, "the first file");
          await until(fake, () => recording.count("POST", "/resume") === 1);
          await fake.cancel(runId);
          await until(fake, async () => (await stateOf(runId)) === "cancelled");
          await until(fake, () => closed(adapter, runId));
          expect(handler.answered).toEqual([]);
        },
        { hold: secondAnswer("drop") },
      );
    },
    TEST_TIMEOUT,
  );

  it(
    "calls the cancel callbacks on run.cancel_requested and ends the run cancelled",
    async () => {
      const gate = new Gate();
      let callbacks = 0;
      class Cancellable extends RecordingHandler {
        override async execute(inv: Invocation, run: RunHandle): Promise<void> {
          run.onCancel(() => callbacks++);
          await super.execute(inv, run);
        }
      }
      const handler = new Cancellable({ blockExecute: gate });
      const adapter = makeAdapter(handler, "cancel");
      await running(adapter, async () => {
        const { runId } = await fake.trigger();
        await until(fake, () => handler.count("execute") === 1);
        await fake.cancel(runId);
        await until(fake, async () => (await stateOf(runId)) === "cancelled");
        expect((await fake.runState(runId)).stateReason).toBe("user_cancelled");
        expect(handler.handles.get(runId)?.cancelled.aborted).toBe(true);
        await until(fake, () => closed(adapter, runId));
      });
      expect(callbacks).toBe(1);
    },
    TEST_TIMEOUT,
  );

  it.each([
    [new RuntimeFailure("model gone"), "runtime_error"],
    [new TypeError("bug"), "adapter_error"],
  ])(
    "ends the run failed when the handler throws %s",
    async (error, reason) => {
      class Failing extends RecordingHandler {
        override async execute(inv: Invocation, run: RunHandle): Promise<void> {
          await super.execute(inv, run);
          throw error;
        }
      }
      const handler = new Failing();
      const adapter = makeAdapter(handler, `fail-${reason.replace("_", "-")}`);
      await running(adapter, async () => {
        const { runId } = await fake.trigger();
        await until(fake, async () => (await stateOf(runId)) === "failed");
        expect((await fake.runState(runId)).stateReason).toBe(reason);
      });
      expect(handler.count("execute")).toBe(1); // never retried
    },
    TEST_TIMEOUT,
  );

  it(
    "gives command.invoked to the command handler and never starts a run for it",
    async () => {
      const gate = new Gate();
      const handler = new RecordingHandler({ blockExecute: gate });
      const recording = new RecordingFetch();
      const adapter = makeAdapter(
        handler,
        "commands",
        {},
        {
          fetch: recording.fetch,
        },
      );
      await running(adapter, async () => {
        const { runId } = await fake.trigger();
        await until(fake, () => handler.count("execute") === 1);
        await fake.invokeCommand("status", { runId });
        await until(fake, () => handler.count("onCommand", runId) === 1);
        await fake.invokeCommand("status");
        await until(fake, () => handler.count("onCommand") === 2);
        expect(
          handler.calls.filter(
            ([n, r]) => n === "onCommand" && r === undefined,
          ),
        ).toHaveLength(1);
        expect(handler.count("prepare")).toBe(1);
        expect(recording.count("POST", "/start")).toBe(1);
        expect(recording.count("POST", "/resume")).toBe(0);
        expect((await fake.runState(runId)).state).toBe("running");
        gate.open();
      });
    },
    TEST_TIMEOUT,
  );

  it(
    "reports the active runs in the heartbeat",
    async () => {
      const gate = new Gate();
      const handler = new RecordingHandler({ blockExecute: gate });
      const adapter = makeAdapter(handler, "beat");
      await running(adapter, async () => {
        const { runId } = await fake.trigger();
        await until(fake, () => handler.count("execute") === 1);
        await fake.passTime(fake.timings["sync.heartbeat_interval"] * 2);
        await until(fake, () =>
          fake.heartbeats.some((h) => {
            const details = h.details as { active_runs?: string[] } | undefined;
            return details?.active_runs?.includes(runId) ?? false;
          }),
        );
        gate.open();
      });
      const beat = fake.heartbeats.at(-1) as S["Heartbeat"];
      expect((beat.details as Record<string, unknown>).adapter).toEqual({
        name: "bothy-adapter",
        version: "0.0.0",
      });
      expect(beat.state).toBe("online");
    },
    TEST_TIMEOUT,
  );

  it(
    "runs over REST polling",
    async () => {
      const handler = new RecordingHandler();
      // The first poll creates the cursor at the head; it reaches the server only after the
      // trigger unless the test waits for the cursor, which moves the fake time meanwhile.
      let triggered = false;
      let firstPoll = true;
      const fetch = async (request: Request): Promise<Response> => {
        if (
          firstPoll &&
          request.method === "GET" &&
          new URL(request.url).pathname.endsWith("/consumers/polling/events")
        ) {
          firstPoll = false;
          const from = fake.clock.monotonic();
          await patiently(
            () => triggered || fake.clock.monotonic() >= from + 3 * STEP_MS,
            "the trigger or the wait for the cursor",
          );
        }
        return globalThis.fetch(request);
      };
      const adapter = makeAdapter(
        handler,
        "polling",
        { transport: "polling" },
        { fetch },
      );
      await running(adapter, async () => {
        const trigger = fake.trigger();
        triggered = true;
        const { runId } = await trigger;
        await until(fake, async () => (await stateOf(runId)) === "completed");
      });
      expect(handler.count("execute")).toBe(1);
      const paths = fake.requests().map((r) => `${r.method} ${r.path}`);
      expect(
        paths.some(
          (p) => p.startsWith("GET") && p.endsWith("/consumers/polling/events"),
        ),
      ).toBe(true);
      expect(
        paths.some(
          (p) => p.startsWith("POST") && p.endsWith("/consumers/polling/ack"),
        ),
      ).toBe(true);
    },
    TEST_TIMEOUT,
  );
});

describe("renewal and superseded tokens", () => {
  it(
    "waits on superseded and closes the entry when the run started elsewhere",
    async () => {
      const gates = { alpha: new Gate(), beta: new Gate() };
      const alpha = new RecordingHandler({ blockPrepare: gates.alpha });
      const beta = new RecordingHandler({ blockPrepare: gates.beta });
      const a = makeAdapter(alpha, "sup-alpha");
      const b = makeAdapter(beta, "sup-beta");
      let runId = "";
      await running(a, () =>
        running(b, async () => {
          runId = (await fake.trigger()).runId;
          await until(
            fake,
            () => alpha.count("prepare") === 1 && beta.count("prepare") === 1,
          );
          const world = (
            fake as unknown as {
              world: { runs: Map<string, { invocationJti: string }> };
            }
          ).world;
          const current = world.runs.get(runId)?.invocationJti;
          const holder =
            alpha.invocations[0]?.claims.jti === current ? "alpha" : "beta";
          const other = holder === "alpha" ? "beta" : "alpha";
          const handlers = { alpha, beta };
          const otherAdapter = other === "alpha" ? a : b;
          gates[other].open(); // the stale token starts first: superseded, the run is queued
          await until(fake, () => waitingForToken(otherAdapter, runId));
          gates[holder].open();
          await until(fake, () => handlers[holder].count("execute") === 1);
          await until(fake, () => closed(otherAdapter, runId));
          expect(handlers[other].count("execute")).toBe(0);
        }),
      );
      expect((await fake.runState(runId)).starts).toBe(1);
    },
    TEST_TIMEOUT,
  );

  it(
    "closes the entry when the renewal is refused because the run started elsewhere",
    async () => {
      const gate = new Gate();
      const late = new RecordingHandler({ blockPrepare: gate });
      const quick = new RecordingHandler();
      const a = makeAdapter(late, "renew-late");
      const b = makeAdapter(quick, "renew-quick");
      let runId = "";
      await running(a, () =>
        running(b, async () => {
          runId = (await fake.trigger()).runId;
          await until(fake, () => late.count("prepare") === 1);
          await until(fake, () => quick.count("execute") === 1);
          await fake.passTime(
            fake.timings["agent.invocation_token_ttl"] +
              fake.timings["auth.clock_leeway"] +
              1_000,
          );
          gate.open(); // the late side's token expired; the renewal answers already_started
          await until(fake, () => closed(a, runId));
          expect(internals(a).runs.has(runId)).toBe(false);
        }),
      );
      expect(late.count("execute")).toBe(0);
      expect((await fake.runState(runId)).starts).toBe(1);
    },
    TEST_TIMEOUT,
  );

  it(
    "starts with the newest token when a redelivery arrives during a renewal",
    async () => {
      // The server granted the renewal and then redelivered the event with a newer token,
      // which the adapter stored before the grant arrived: the grant must not overwrite it
      // (09 4.5.1).
      await fake.stop();
      fake = new FakeServer({ ackTimeoutMs: 100_000_000 });
      await fake.start();
      const runs = (
        fake as unknown as {
          world: {
            runs: Map<
              string,
              { invocationJti: string; startedJti: string | undefined }
            >;
          };
        }
      ).world.runs;
      const gate = new Gate();
      const handler = new RecordingHandler({ blockPrepare: gate });
      let hook: (() => Promise<void>) | undefined;
      const afterRenewal = async (request: Request): Promise<Response> => {
        const renewal =
          request.method === "POST" &&
          new URL(request.url).pathname.endsWith("/invocation-token");
        const response = await globalThis.fetch(request);
        if (renewal && response.ok && hook !== undefined) {
          const run = hook;
          hook = undefined;
          const body = await response.text();
          await run();
          return new Response(body, {
            status: response.status,
            headers: response.headers,
          });
        }
        return response;
      };
      const adapter = makeAdapter(
        handler,
        "renew-redelivery",
        {},
        { fetch: afterRenewal },
      );
      let runId = "";
      const newest: string[] = [];
      await running(
        adapter,
        async () => {
          runId = (await fake.trigger()).runId;
          await until(fake, () => handler.count("prepare") === 1);
          const eventId = (await adapter.inbox.forRun(runId))[0]?.eventId;
          expect(eventId).toBeDefined();
          hook = async () => {
            await fake.redeliver();
            newest.push(runs.get(runId)?.invocationJti ?? "");
            await until(
              fake,
              async () =>
                (await adapter.inbox.get(eventId as string))?.tokenJti ===
                newest[0],
            );
          };
          await fake.passTime(
            fake.timings["agent.invocation_token_ttl"] +
              fake.timings["auth.clock_leeway"] +
              1_000,
          );
          gate.open();
          await until(fake, async () => (await stateOf(runId)) === "completed");
        },
        { holdAcks: true },
      );
      expect(newest).toHaveLength(1);
      expect(runs.get(runId)?.startedJti).toBe(newest[0]);
      expect(handler.count("execute")).toBe(1);
      expect((await fake.runState(runId)).starts).toBe(1);
    },
    TEST_TIMEOUT,
  );

  it(
    "runs at most one renewal per run at a time",
    async () => {
      const gate = new Gate();
      const handler = new RecordingHandler({ blockPrepare: gate });
      const recording = new RecordingFetch();
      const adapter = makeAdapter(
        handler,
        "renew-once",
        {},
        {
          fetch: recording.fetch,
        },
      );
      await running(adapter, async () => {
        const { runId } = await fake.trigger();
        await until(fake, () => handler.count("prepare") === 1);
        const entry = (await adapter.inbox.forRun(runId))[0];
        const token = entry?.token;
        expect(token).toBeDefined();
        fake.inject("renewInvocationToken", Fault.storedLate(1_000));
        const state = internals(adapter).runs.get(runId);
        const both = Promise.all([
          internals(adapter).renew(state, token as Secret),
          internals(adapter).renew(state, token as Secret),
        ]);
        let settled = false;
        void both.finally(() => {
          settled = true;
        });
        await until(fake, () => settled);
        const [first, second] = await both;
        expect(recording.count("POST", "/invocation-token")).toBe(1);
        expect(first && fingerprint(first)).toBe(second && fingerprint(second));
        expect(first && fingerprint(first)).not.toBe(
          fingerprint(token as Secret),
        );
        gate.open();
        await until(fake, () => handler.count("execute") === 1);
      });
    },
    TEST_TIMEOUT,
  );
});

describe("restart", () => {
  it(
    "resumes a not started inbox entry and closes a started one; FileInboxStore modes",
    async () => {
      const stateDir = await mkdtemp(join(tmpdir(), "bothy-loop-"));
      dirs.push(stateDir);
      const first = new RecordingHandler({ blockExecute: new Gate() });
      const adapter = makeAdapter(first, "restart", { stateDir });
      let started = "";
      let accepted = "";
      await running(adapter, async () => {
        started = (await fake.trigger("started @Lotse")).runId;
        await until(fake, () => first.count("execute") === 1);
        first.blockPrepare = new Gate(); // the second run stays in prepare
        accepted = (await fake.trigger("accepted @Lotse")).runId;
        await until(fake, () => first.count("prepare", accepted) === 1);
      });
      const inboxDir = join(stateDir, "inbox");
      expect((await stat(inboxDir)).mode & 0o777).toBe(0o700);
      const files = await readdir(inboxDir);
      expect(files.length).toBeGreaterThan(0);
      for (const file of files)
        expect((await stat(join(inboxDir, file))).mode & 0o777).toBe(0o600);

      const second = new RecordingHandler();
      const log: Log = [];
      const restarted = makeAdapter(
        second,
        "restart",
        { stateDir },
        {
          fetch: new RecordingFetch(log).fetch,
        },
      );
      await running(restarted, async () => {
        await until(fake, () => second.count("execute", accepted) === 1);
        const entries = await restarted.inbox.forRun(started);
        expect(entries.every((e) => e.state === EntryState.CLOSED)).toBe(true);
      });
      expect(second.count("prepare", started)).toBe(0);
      expect((await fake.runState(accepted)).starts).toBe(1);
      // Only the run that started in the new process ends there.
      expect(log.filter((e) => e.startsWith("finish"))).toHaveLength(1);
    },
    TEST_TIMEOUT,
  );
});

describe("stop while startRun is in flight", () => {
  it(
    "leaves the entry for the next process, which executes the run exactly once",
    async () => {
      const stateDir = await mkdtemp(join(tmpdir(), "bothy-loop-"));
      dirs.push(stateDir);
      const hold = new Gate();
      let startSeen = false;
      const holdingFetch = async (request: Request): Promise<Response> => {
        const response = await globalThis.fetch(request);
        if (
          request.method === "POST" &&
          new URL(request.url).pathname.endsWith("/start")
        ) {
          startSeen = true;
          await hold.opened;
        }
        return response;
      };
      const first = new RecordingHandler();
      const adapter = makeAdapter(
        first,
        "stop-start",
        { stateDir },
        { fetch: holdingFetch },
      );
      const source = new RecordingSource(await adapter.makeSource());
      adapter.source = source;
      const done = adapter.run();
      done.catch(() => undefined);
      await until(fake, () => source.ready);
      const { runId } = await fake.trigger("held start @Lotse");
      await until(fake, () => startSeen);
      const stopping = adapter.stop();
      hold.open();
      await stopping;
      await withinPatience(
        done.catch(() => undefined),
        "the end of the adapter loop",
      );
      expect(first.count("execute")).toBe(0);

      const second = new RecordingHandler();
      const restarted = makeAdapter(second, "stop-start", { stateDir });
      await running(restarted, async () => {
        await until(fake, () => second.count("execute", runId) === 1);
      });
      expect(second.count("execute", runId)).toBe(1);
      expect((await fake.runState(runId)).starts).toBe(1);
    },
    TEST_TIMEOUT,
  );
});

describe("Invocation-Token of the run's requests (07 SR9)", () => {
  it(
    "binds every request of prepare and execute to the run's token, nothing else",
    async () => {
      class Context extends RecordingHandler {
        override async prepare(
          inv: Invocation,
          run: RunHandle,
        ): Promise<StartOptions> {
          await run.loadContext();
          await run.client.GET("/runs/{run_id}", {
            params: { path: { run_id: run.runId } },
          });
          return super.prepare(inv, run);
        }
      }
      const handler = new Context();
      const recording = new RecordingFetch();
      const adapter = makeAdapter(
        handler,
        "sr9",
        {},
        { fetch: recording.fetch },
      );
      let runId = "";
      const delivered = await running(adapter, async ({ source }) => {
        runId = (await fake.trigger()).runId;
        await until(fake, () => handler.count("execute") === 1);
        return source.envelopes("agent.invocation")[0]?.token;
      });
      expect(delivered).toBeDefined();
      const expected = fingerprint(delivered as Secret);
      const bound = recording.seen.filter(
        (s) => s.method === "GET" && s.path.includes(runId),
      );
      expect(bound.length).toBeGreaterThanOrEqual(2);
      for (const request of bound) expect(request.bound).toBe(expected);
      const unbound = recording.seen.filter(
        (s) => s.path.includes("/instance/") || s.path.includes("/agents/me/"),
      );
      expect(unbound.length).toBeGreaterThan(0);
      for (const request of unbound) expect(request.bound).toBeUndefined();
    },
    TEST_TIMEOUT,
  );

  it(
    "carries the renewed token after a renewal",
    async () => {
      const gate = new Gate();
      const recording = new RecordingFetch();
      let executeBound: string | undefined;
      class Reading extends RecordingHandler {
        override async execute(inv: Invocation, run: RunHandle): Promise<void> {
          await run.client.GET("/runs/{run_id}", {
            params: { path: { run_id: run.runId } },
          });
          executeBound = recording.seen.at(-1)?.bound;
          await super.execute(inv, run);
        }
      }
      const handler = new Reading({ blockPrepare: gate });
      const adapter = makeAdapter(
        handler,
        "sr9-renew",
        {},
        {
          fetch: recording.fetch,
        },
      );
      let delivered: Secret | undefined;
      await running(adapter, async ({ source }) => {
        const { runId } = await fake.trigger();
        await until(fake, () => handler.count("prepare") === 1);
        delivered = source.envelopes("agent.invocation")[0]?.token;
        await fake.passTime(
          fake.timings["agent.invocation_token_ttl"] +
            fake.sdkSettings.invocationTokenLeewayMs +
            1_000,
        );
        gate.open();
        await until(fake, () => handler.count("execute", runId) === 1);
        await until(fake, async () => (await stateOf(runId)) === "completed");
      });
      expect(recording.renewed).toHaveLength(1);
      expect(executeBound).toBe(recording.renewed[0]);
      expect(executeBound).not.toBe(fingerprint(delivered as Secret));
      const finish = recording.seen.find(
        (s) => s.method === "POST" && s.path.endsWith("/finish"),
      );
      expect(finish?.bound).toBe(recording.renewed[0]);
    },
    TEST_TIMEOUT,
  );
});
