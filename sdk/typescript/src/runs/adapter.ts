// SPDX-License-Identifier: Apache-2.0
/**
 * The acceptance and run loop of an agent adapter (docs/spec/09-agents.md 4.3, 4.5, 4.9, 15;
 * port of `bothy_sdk/adapter.py`).
 *
 * {@link Adapter} connects as a consumer (gateway or REST polling), checks every invocation
 * token itself, stores an accepted event with its token in the durable inbox and only then
 * acknowledges it; a run starts later from the inbox, at most once and with the newest token,
 * renewing an expired one (R-AG-24, R-AG-25). The runtime ({@link AgentHandler.execute}) starts
 * only after `200` on `startRun` (R-AG-07); nothing is retried after the runtime died (R-AG-17).
 * Every request of a run's handler is bound to the run's newest token (07 SR9,
 * `ApiClient.bindRun`).
 *
 * Refusals (docs/DECISIONS.md D-M2-29-8): a token that fails the check on an event that would
 * start or resume a run ends the run with `finishRun` (`failed` and `invalid_invocation_token`,
 * `billing_mode_mismatch` or `tool_class_mismatch`); on any other event it is logged and
 * skipped and the run is not ended, because a forged event must not end a running run. An
 * unknown key leaves the event unsettled (the server delivers again with a current key); a
 * replayed `jti` under a new event ID is ignored and not settled. The handler never sees a
 * refused event.
 *
 * Node.js only: it handles tokens.
 *
 * @example
 * ```ts
 * class Lotse extends AgentHandler {
 *   override actualBilling(): ActualBilling {
 *     return { mode: "organization" };
 *   }
 *   override async prepare(): Promise<StartOptions> {
 *     return { model: "model-large" };
 *   }
 *   override async execute(inv: Invocation, run: RunHandle): Promise<void> {
 *     await run.send("Hello Alex");
 *   }
 * }
 * const adapter = new Adapter(
 *   { baseUrl: "https://example.com/api/v1", instanceId, agentId, credentials, consumer: "default" },
 *   new Lotse(),
 * );
 * const why = await adapter.run(); // for example 4015 when another process took the consumer
 * ```
 *
 * @module
 */

import { join } from "node:path";

import { Secret } from "../auth/secret.js";
import type { Credentials } from "../auth/types.js";
import { type ApiClient, createApiClient, unwrap } from "../client.js";
import { type Clock, systemClock } from "../clock.js";
import {
  autocompleteRequest,
  guardedAutocomplete,
  reconcileCommands,
  type AutocompleteRequest,
  type CommandSpec,
} from "../commands/index.js";
import { type SdkSettings, resolveSettings } from "../config.js";
import {
  AgentInvocationDenied,
  AlreadyStarted,
  BillingModeMismatch,
  GatewayOpError,
  InvocationTokenInvalid,
  ProblemError,
  ToolClassMismatch,
  TransportError,
} from "../errors.js";
import {
  CloseReason,
  CursorAdvance,
  Envelope,
  Gateway,
  GatewaySource,
  PollingSource,
  SourceReady,
  gatewayUrl,
  type EventSource,
  type GatewayEvent,
  type GatewaySocketConstructor,
} from "../gateway/index.js";
import type { components } from "../generated/openapi.js";
import { buildHeartbeat } from "../heartbeat/index.js";
import {
  EntryState,
  FileInboxStore,
  InboxEntry,
  InvocationVerifier,
  MemoryInboxStore,
  TOKEN_EVENT_TYPES,
  TokenVerdict,
  compareActual,
  expectationFor,
  jwksFetcher,
  storedClaims,
  triggererLoader,
  type Actual,
  type InboxStore,
  type InvocationClaims,
} from "../invocation/index.js";
import type { Logger } from "../logger.js";
import { FileOutboxStore, Outbox, type OutboxStore } from "../outbox/index.js";
import { RunHandle } from "./handle.js";

type S = components["schemas"];
type Payload = Readonly<Record<string, unknown>>;

const START_EVENT = "agent.invocation";
const FOLLOWUP = "agent.followup";
const INTERACTION = "interaction.created";
const COMMAND = "command.invoked";
const AUTOCOMPLETE = "command.autocomplete_requested";
const CANCEL = "run.cancel_requested";
const STATE_CHANGED = "run.state_changed";
const ACCESS_CHANGED = "access.changed";
const REFUSAL_REASON: ReadonlyMap<TokenVerdict, string> = new Map([
  [TokenVerdict.INVALID, "invalid_invocation_token"],
  [TokenVerdict.BILLING_MODE_MISMATCH, "billing_mode_mismatch"],
  [TokenVerdict.TOOL_CLASS_MISMATCH, "tool_class_mismatch"],
]);
const SERVER_ERROR = 500;
// A refusal answered like this was not stored; the event stays unsettled and comes again.
const NOT_STORED: ReadonlySet<number> = new Set([401, 429]);
const UUID = /^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$/i;

/** Configuration of an agent adapter. */
export interface AdapterConfig {
  /** The API base, for example `https://example.com/api/v1`. */
  readonly baseUrl: string;
  /** The configured instance (`iss` of every token, 09 4.5 point 4); never from discovery. */
  readonly instanceId: string;
  /** The agent's principal ID (`aud`). */
  readonly agentId: string;
  /** The agent's API token and optional DPoP key. */
  readonly credentials: Credentials;
  /** The consumer name. */
  readonly consumer: string;
  /** `gateway` (default) or `polling`. */
  readonly transport?: "gateway" | "polling" | undefined;
  /**
   * The directory of the inbox (`inbox/`) and the outbox (`outbox.json`); absent keeps both in
   * memory (tests only, not durable).
   */
  readonly stateDir?: string | undefined;
  /** `true` to keep the allow list of 17.7 (`getAgentTriggerers`). */
  readonly allowlist?: boolean | undefined;
  /** The adapter's name (heartbeat, `User-Agent`); default `bothy-adapter`. */
  readonly adapterName?: string | undefined;
  /** The adapter's version; default `0.0.0`. */
  readonly adapterVersion?: string | undefined;
  /** The runtime's name (heartbeat); default `runtime`. */
  readonly runtimeName?: string | undefined;
  /** The runtime's version, if known. */
  readonly runtimeVersion?: string | undefined;
  /** The heartbeat `capabilities` (09 4.8). */
  readonly capabilities?: Readonly<Record<string, unknown>> | undefined;
  /** Declared commands, reconciled after every connect; absent leaves commands alone. */
  readonly commands?: readonly CommandSpec[] | undefined;
  /** Overrides of the SDK settings. */
  readonly settings?: Partial<SdkSettings> | undefined;
}

/** The test seams and replaceable parts of an {@link Adapter}. */
export interface AdapterOptions {
  /** The time source (tests pass a `FakeClock`); default `systemClock`. */
  readonly clock?: Clock | undefined;
  /** An event source; default from `transport`. */
  readonly source?: EventSource | undefined;
  /** The inbox; default `FileInboxStore` in `stateDir/inbox` (memory without `stateDir`). */
  readonly inbox?: InboxStore | undefined;
  /** The outbox store; default `FileOutboxStore` in `stateDir/outbox.json`. */
  readonly outboxStore?: OutboxStore | undefined;
  /** The `fetch` of the REST client (tests). */
  readonly fetch?: ((request: Request) => Promise<Response>) | undefined;
  /** The WebSocket constructor of the gateway (default the global `WebSocket`). */
  readonly WebSocket?: GatewaySocketConstructor | undefined;
  /** The logger of the client and the loop. */
  readonly logger?: Logger | undefined;
}

/** The billing mode the runtime actually uses for a run (I3, 10 7). */
export interface ActualBilling {
  /** `organization`, `personal` or `unknown` (`unknown` never matches). */
  readonly mode: string;
  /** The evidence sent with `startRun`, if any. */
  readonly evidence?: S["BillingEvidence"] | undefined;
}

/** What {@link AgentHandler.prepare} adds to `startRun`. */
export interface StartOptions {
  /** The model the runtime will use. */
  readonly model?: string | undefined;
  /** The runtime's session reference, if it exists before the run. */
  readonly externalSessionRef?: string | undefined;
  /** Evidence of the billing mode (overrides {@link ActualBilling.evidence}). */
  readonly billingEvidence?: S["BillingEvidence"] | undefined;
}

/** One accepted event as a handler sees it; it never carries the token. */
export interface Invocation {
  /** The envelope `id`. */
  readonly eventId: string;
  /** The envelope `seq` (`undefined` for an ephemeral event). */
  readonly seq: number | undefined;
  /** The event type. */
  readonly type: string;
  /** The event's `payload` as received. */
  readonly payload: Readonly<Record<string, unknown>>;
  /** The verified claims of the event's token. */
  readonly claims: InvocationClaims;
}

/** Thrown by a handler when the runtime failed; the run ends `failed` with `runtime_error`. */
export class RuntimeFailure extends Error {
  /**
   * Creates the error.
   *
   * @param message - What failed; never a token or message content.
   * @param options - The cause.
   */
  constructor(message: string, options?: ErrorOptions) {
    super(message, options);
    this.name = "RuntimeFailure";
  }
}

/**
 * The runtime side of an adapter; extend it and override {@link AgentHandler.prepare} and
 * {@link AgentHandler.execute}.
 *
 * Every method that belongs to a run runs inside the run's SR9 binding: each request of the
 * adapter's client carries the run's newest invocation token.
 */
export class AgentHandler {
  /**
   * Returns the billing mode the runtime uses for this run (`unknown` when unsure).
   *
   * @param inv - The invocation.
   * @returns The actual mode; the default `unknown` refuses every run (fail closed).
   */
  actualBilling(inv: Invocation): ActualBilling {
    void inv;
    return { mode: "unknown" };
  }

  /**
   * Returns the tool class the runtime actually loaded (07 2.11).
   *
   * @returns The tool class; the default is `chat`.
   */
  actualToolClass(): S["ToolClass"] {
    return "chat";
  }

  /**
   * Gets ready for a run before it starts (load context, pick a model); no runtime yet.
   *
   * @param inv - The invocation.
   * @param run - The run (not started: no patches yet).
   * @returns The options of `startRun`.
   * @throws `Error` unless overridden.
   */
  prepare(inv: Invocation, run: RunHandle): Promise<StartOptions> {
    void inv;
    void run;
    return Promise.reject(new Error("AgentHandler.prepare is not implemented"));
  }

  /**
   * Runs the runtime; called only after `200` on `startRun`.
   *
   * Returning ends the run `completed` unless the handler finished it or waits for input;
   * {@link RuntimeFailure} ends it `failed` with `runtime_error`, any other error with
   * `adapter_error`. After `run.cancel_requested` the adapter ends the run `cancelled` without
   * waiting for this promise; watch `run.cancelled` to stop the runtime.
   *
   * @param inv - The invocation.
   * @param run - The started run.
   * @returns A promise that resolves when the runtime is done.
   * @throws `Error` unless overridden.
   */
  execute(inv: Invocation, run: RunHandle): Promise<void> {
    void inv;
    void run;
    return Promise.reject(new Error("AgentHandler.execute is not implemented"));
  }

  /**
   * Handles a follow-up message of a running or resumed run.
   *
   * @param inv - The `agent.followup` invocation.
   * @param run - The run.
   * @returns A promise that resolves when handled.
   */
  onFollowup(inv: Invocation, run: RunHandle): Promise<void> {
    void inv;
    void run;
    return Promise.resolve();
  }

  /**
   * Handles an interaction with a component of a running or resumed run.
   *
   * @param inv - The `interaction.created` invocation.
   * @param run - The run.
   * @returns A promise that resolves when handled.
   */
  onInteraction(inv: Invocation, run: RunHandle): Promise<void> {
    void inv;
    void run;
    return Promise.resolve();
  }

  /**
   * Handles a command invocation (`command.invoked`); it never starts, resumes or ends a run.
   *
   * @param inv - The invocation.
   * @param run - The adapter's run the command names, if it is running here.
   * @returns A promise that resolves when handled.
   */
  onCommand(inv: Invocation, run: RunHandle | undefined): Promise<void> {
    void inv;
    void run;
    return Promise.resolve();
  }

  /**
   * Returns choices for an autocomplete request before it expires.
   *
   * Never call a model here (R-AG-14): typing must cost nothing and never hang.
   *
   * @param request - The request.
   * @param signal - Aborted when the request expires.
   * @returns At most `command.autocomplete_max_choices` choices.
   */
  onAutocomplete(
    request: AutocompleteRequest,
    signal: AbortSignal,
  ): Promise<readonly S["CommandChoice"][]> {
    void request;
    void signal;
    return Promise.resolve([]);
  }

  /**
   * Handles an event without an invocation token (for example `agent.config_changed`).
   *
   * @param envelope - The event.
   * @returns A promise that resolves when handled.
   */
  onEvent(envelope: Envelope): Promise<void> {
    void envelope;
    return Promise.resolve();
  }
}

/** One spawned piece of work of the loop. */
interface Task {
  done: boolean;
}

/** Runs one critical section at a time (the `asyncio.Lock` of the Python adapter). */
class Mutex {
  private tail: Promise<void> = Promise.resolve();

  async run<T>(fn: () => Promise<T>): Promise<T> {
    const previous = this.tail;
    let release: () => void = () => undefined;
    this.tail = new Promise<void>((resolve) => {
      release = resolve;
    });
    await previous;
    try {
      return await fn();
    } finally {
      release();
    }
  }
}

/** What the adapter keeps in memory about one run of this process. */
interface RunState {
  readonly runId: string;
  readonly eventId: string;
  readonly handle: RunHandle;
  invocation: Invocation;
  task: Task | undefined;
  readonly renewing: Mutex;
  readonly resuming: Mutex;
  waitingForToken: boolean;
  cancelReason: string | undefined;
  /** Aborted by `run.cancel_requested`: handler work of the run stops being awaited. */
  readonly interrupt: AbortController;
}

const CANCELLED: unique symbol = Symbol("cancelled");
const STOPPED: unique symbol = Symbol("stopped");
type Interrupted = typeof CANCELLED | typeof STOPPED;

function isRecord(value: unknown): value is Record<string, unknown> {
  return typeof value === "object" && value !== null && !Array.isArray(value);
}

function payloadOf(envelope: Readonly<Record<string, unknown>>): Payload {
  return isRecord(envelope.payload) ? envelope.payload : {};
}

function uuidOf(value: unknown): string | undefined {
  return typeof value === "string" && UUID.test(value)
    ? value.toLowerCase()
    : undefined;
}

function errorName(error: unknown): string {
  return error instanceof Error ? error.name : typeof error;
}

/** Awaits `call` and drops a {@link ProblemError} (the server refused; nothing more to do). */
async function ignoreProblem(call: () => Promise<unknown>): Promise<void> {
  try {
    await call();
  } catch (error) {
    if (!(error instanceof ProblemError)) throw error;
  }
}

/**
 * Connects an {@link AgentHandler} to the server.
 *
 * @example
 * ```ts
 * const adapter = new Adapter(config, new Lotse(), { clock: fake.clock });
 * adapter.source = new RecordingSource(await adapter.makeSource());
 * const done = adapter.run();
 * await adapter.stop();
 * await done;
 * ```
 */
export class Adapter {
  /** The adapter configuration. */
  readonly config: AdapterConfig;
  /** The runtime side. */
  readonly handler: AgentHandler;
  /** The time source. */
  readonly clock: Clock;
  /** The resolved SDK settings. */
  readonly settings: SdkSettings;
  /** The REST client; every request of a run's handler is bound to the run (07 SR9). */
  readonly client: ApiClient;
  /** The inbox of accepted events (and the `jti` store of the verifier). */
  readonly inbox: InboxStore;
  /** The outbox of replies and writes. */
  readonly outbox: Outbox;
  /** The event source; set it before {@link Adapter.run} to wrap the default one. */
  source: EventSource | undefined;
  /** The token check, created by {@link Adapter.run}. */
  verifier: InvocationVerifier | undefined;

  private readonly logger: Logger;
  private readonly webSocket: GatewaySocketConstructor | undefined;
  private readonly runs = new Map<string, RunState>();
  // The newest token per run of this process, kept after the end, so a late outbox entry of a
  // run goes out bound to it and the server answers run_inactive (07 SR9).
  private readonly runTokens = new Map<string, Secret>();
  private readonly tasks = new Set<Promise<unknown>>();
  private readonly stopper = new AbortController();
  private stopping = false;
  private readonly degraded: string[] = [];

  /**
   * Creates the adapter; nothing connects before {@link Adapter.run}.
   *
   * @param config - The adapter configuration.
   * @param handler - The runtime side.
   * @param options - Clock, source, inbox, outbox store, `fetch`, WebSocket and logger.
   * @throws `TypeError` when `baseUrl` uses clear text off the loopback host.
   */
  constructor(
    config: AdapterConfig,
    handler: AgentHandler,
    options: AdapterOptions = {},
  ) {
    this.config = config;
    this.handler = handler;
    this.clock = options.clock ?? systemClock;
    this.settings = resolveSettings(config.settings);
    this.client = createApiClient({
      baseUrl: config.baseUrl,
      token: config.credentials.apiToken,
      dpopKey: config.credentials.dpop,
      settings: this.settings,
      clock: this.clock,
      userAgent: this.userAgent,
      logger: options.logger,
      ...(options.fetch === undefined ? {} : { fetch: options.fetch }),
    });
    this.logger = this.client.logger;
    this.webSocket = options.WebSocket;
    const state = config.stateDir;
    this.inbox =
      options.inbox ??
      (state === undefined
        ? new MemoryInboxStore()
        : new FileInboxStore(join(state, "inbox")));
    const store =
      options.outboxStore ??
      (state === undefined
        ? undefined
        : new FileOutboxStore(join(state, "outbox.json")));
    this.outbox = new Outbox(this.client, {
      store,
      tokenFor: async (runId) => (await this.tokenForRun(runId))?.reveal(),
    });
    this.source = options.source;
    this.verifier = undefined;
  }

  private get userAgent(): string {
    return `${this.config.adapterName ?? "bothy-adapter"}/${this.config.adapterVersion ?? "0.0.0"}`;
  }

  // ---- lifecycle ------------------------------------------------------------------------

  /**
   * Runs until the source stops or {@link Adapter.stop} is called.
   *
   * @returns Why the adapter stopped (for example `4015` when another process took the
   *   consumer over, or `1000` after {@link Adapter.stop}).
   * @throws {@link ProblemError} when the discovery or the first JWKS fetch failed.
   */
  async run(): Promise<CloseReason> {
    const discovery = await this.discover();
    if (
      discovery.instance_id.toLowerCase() !==
      this.config.instanceId.toLowerCase()
    ) {
      this.logger.error(
        "the server is another instance than configured; stopping",
      );
      await this.close();
      return new CloseReason(1000, "instance_mismatch");
    }
    this.verifier = new InvocationVerifier({
      instanceId: this.config.instanceId.toLowerCase(),
      agentId: this.config.agentId.toLowerCase(),
      jwks: jwksFetcher(this.client),
      replay: this.inbox,
      settings: this.settings,
      clock: this.clock,
      allowlist: this.config.allowlist
        ? triggererLoader(this.client)
        : undefined,
      logger: this.logger,
    });
    await this.verifier.refresh();
    this.source ??= this.defaultSource(discovery.gateway.url);
    const source = this.source;
    await this.recover();
    await this.outbox.flush();
    try {
      for await (const delivery of source) {
        if (delivery instanceof SourceReady) this.spawn(() => this.onReady());
        else if (delivery instanceof CursorAdvance)
          await source.advance(delivery.seq);
        else await this.onEnvelope(delivery);
        if (this.stopping) break;
      }
    } finally {
      await this.close();
    }
    return source.closeReason ?? new CloseReason(1000, "stopped");
  }

  /**
   * Stops the source; {@link Adapter.run} returns after the current delivery. Handler work
   * stops being awaited; an entry whose run has not started stays in the inbox for the next
   * process (09 4.9).
   *
   * @returns A promise that resolves once the source is closing.
   */
  async stop(): Promise<void> {
    this.stopping = true;
    this.stopper.abort();
    if (this.source !== undefined) await this.source.close();
  }

  /**
   * Builds the source `transport` names, for a test to wrap before {@link Adapter.run}.
   *
   * @returns The gateway or polling source of this adapter.
   * @throws {@link ProblemError} when the discovery failed.
   *
   * @example
   * ```ts
   * adapter.source = new TamperingSource(await adapter.makeSource(), rewrite);
   * ```
   */
  async makeSource(): Promise<EventSource> {
    const discovery = await this.discover();
    return this.defaultSource(discovery.gateway.url);
  }

  /**
   * Returns the agent heartbeat of this process (09 4.8), `active_runs` included.
   *
   * @returns The heartbeat (generated type).
   * @throws `RangeError` when a configured name or version breaks the heartbeat forms.
   */
  heartbeat(): S["Heartbeat"] {
    const details: Record<string, unknown> = {
      active_runs: [...this.runs.values()]
        .filter((s) => s.handle.tokenJti !== undefined)
        .map((s) => s.runId),
      adapter: {
        name: this.config.adapterName ?? "bothy-adapter",
        version: this.config.adapterVersion ?? "0.0.0",
      },
      runtime: { name: this.config.runtimeName ?? "runtime" },
      protocol_version: 1,
    };
    const capabilities = this.config.capabilities;
    if (capabilities !== undefined && Object.keys(capabilities).length > 0)
      details.capabilities = { ...capabilities };
    if (this.degraded.length > 0) details.degraded_reasons = [...this.degraded];
    return buildHeartbeat(this.degraded.length > 0 ? "degraded" : "online", {
      runtimeVersion: this.config.runtimeVersion,
      reportedToolClass: this.handler.actualToolClass(),
      details,
    });
  }

  private discover(): Promise<S["Discovery"]> {
    return unwrap(this.client.GET("/instance/discovery"), "getDiscovery");
  }

  /** The newest token of a run for its outbox entries: this process's, else the inbox's. */
  private async tokenForRun(runId: string): Promise<Secret | undefined> {
    const token = this.runTokens.get(runId);
    if (token !== undefined) return token;
    const entries = (await this.inbox.forRun(runId)).filter(
      (e) => e.token !== undefined,
    );
    entries.sort((a, b) => (a.tokenIat ?? 0) - (b.tokenIat ?? 0));
    return entries.at(-1)?.token;
  }

  private defaultSource(gatewayPath: string): EventSource {
    if (this.config.transport === "polling")
      return new PollingSource(this.client, this.config.consumer, {
        settings: this.settings,
        clock: this.clock,
        heartbeat: () => this.heartbeat(),
        logger: this.logger,
      });
    const gateway = new Gateway(
      gatewayUrl(this.config.baseUrl, gatewayPath),
      this.config.credentials,
      {
        consumer: this.config.consumer,
        agentId: this.config.agentId,
        settings: this.settings,
        clock: this.clock,
        heartbeat: () => this.heartbeat(),
        userAgent: this.userAgent,
        WebSocket: this.webSocket,
        logger: this.logger,
      },
    );
    const source = new GatewaySource(gateway);
    source.onError = (frame) => this.onGatewayError(frame);
    return source;
  }

  private onGatewayError(frame: GatewayEvent): void {
    const ref = frame.data.ref;
    if (typeof ref !== "string" || !ref.includes(":")) return;
    const runId = uuidOf(ref.split(":", 1)[0]);
    const patcher =
      runId === undefined
        ? undefined
        : this.runs.get(runId)?.handle.startedPatcher;
    const error = frame.model;
    if (patcher !== undefined && error instanceof GatewayOpError)
      this.spawn(() => patcher.onGatewayError(error));
  }

  private async close(): Promise<void> {
    this.stopper.abort();
    await Promise.allSettled([...this.tasks]);
    await this.inbox.close();
    await this.outbox.store.close();
  }

  private spawn(work: () => Promise<unknown>): Task {
    const task: Task = { done: false };
    const promise = Promise.resolve()
      .then(work)
      .catch((error: unknown) => {
        this.logger.error("adapter task failed", { error: errorName(error) });
      })
      .finally(() => {
        task.done = true;
        this.tasks.delete(promise);
      });
    this.tasks.add(promise);
    return task;
  }

  /**
   * Awaits handler work until it settles, the run is cancelled or the adapter stops; work that
   * is no longer awaited keeps running and its outcome is dropped.
   */
  private async interruptible<T>(
    state: RunState | undefined,
    work: () => Promise<T>,
  ): Promise<{ readonly value: T } | Interrupted> {
    const signals = [this.stopper.signal];
    if (state !== undefined) signals.push(state.interrupt.signal);
    const which = (): Interrupted =>
      state?.interrupt.signal.aborted ? CANCELLED : STOPPED;
    if (signals.some((s) => s.aborted)) return which();
    let detach: () => void = () => undefined;
    const interrupted = new Promise<Interrupted>((resolve) => {
      const on = (): void => resolve(which());
      for (const s of signals) s.addEventListener("abort", on, { once: true });
      detach = () => {
        for (const s of signals) s.removeEventListener("abort", on);
      };
    });
    const running = Promise.resolve().then(work);
    try {
      const outcome = await Promise.race([
        running.then((value) => ({ value })),
        interrupted,
      ]);
      if (outcome === CANCELLED || outcome === STOPPED)
        running.catch(() => undefined);
      return outcome;
    } finally {
      detach();
    }
  }

  private async onReady(): Promise<void> {
    await this.outbox.flush();
    if (this.config.commands === undefined) return;
    try {
      await reconcileCommands(
        this.client,
        this.config.agentId,
        this.config.commands,
        { settings: this.settings, logger: this.logger },
      );
    } catch (error) {
      if (!(error instanceof ProblemError)) throw error;
      this.logger.warn("command reconciliation failed", { code: error.code });
    }
  }

  // ---- restart --------------------------------------------------------------------------

  /** Adapter restart (09 4.9): accepted runs start again, started ones are closed. */
  private async recover(): Promise<void> {
    for (const entry of await this.inbox.inStates(
      EntryState.STARTED,
      EntryState.WAITING,
    )) {
      // The runtime of this run died with the previous process; the server ends the run
      // interrupted and nothing is retried (R-AG-17).
      await this.inbox.setState(entry.eventId, EntryState.CLOSED);
    }
    for (const entry of await this.inbox.inStates(EntryState.ACCEPTED)) {
      if (
        entry.type === START_EVENT &&
        entry.token !== undefined &&
        entry.runId !== undefined
      ) {
        const state = this.stateFor(entry, storedClaims(entry.token));
        state.task = this.spawn(() => this.start(state));
      } else {
        await this.inbox.setState(entry.eventId, EntryState.CLOSED);
      }
    }
  }

  // ---- deliveries -----------------------------------------------------------------------

  private async settle(env: Envelope): Promise<void> {
    if (env.seq !== undefined && this.source !== undefined)
      await this.source.settle(env.seq);
  }

  private async onEnvelope(env: Envelope): Promise<void> {
    if (TOKEN_EVENT_TYPES.has(env.type)) {
      if (env.type === AUTOCOMPLETE) {
        this.spawn(() => this.autocomplete(env));
        return;
      }
      await this.accept(env);
      return;
    }
    if (env.type === CANCEL) await this.onCancel(env);
    else if (env.type === STATE_CHANGED) await this.onStateChanged(env);
    else if (env.type === ACCESS_CHANGED) this.verifier?.accessChanged();
    if (env.event.kind !== "unknown") {
      try {
        await this.handler.onEvent(env);
      } catch (error) {
        // A handler failure must not block the acknowledgement.
        this.logger.error(`onEvent failed for event ${env.id}`, {
          error: errorName(error),
        });
      }
    }
    await this.settle(env);
  }

  private invocation(
    eventId: string,
    seq: number | undefined,
    type: string,
    envelope: Readonly<Record<string, unknown>>,
    claims: InvocationClaims,
  ): Invocation {
    return Object.freeze({
      eventId,
      seq,
      type,
      payload: payloadOf(envelope),
      claims,
    });
  }

  private actual(env: Envelope): (claims: InvocationClaims) => Actual {
    return (claims) => {
      const inv = this.invocation(env.id, env.seq, env.type, env.raw, claims);
      return {
        billingMode: this.handler.actualBilling(inv).mode,
        toolClass: this.handler.actualToolClass(),
      };
    };
  }

  private async check(
    env: Envelope,
  ): Promise<[TokenVerdict, InvocationClaims | undefined]> {
    if (env.token === undefined || this.verifier === undefined)
      return [TokenVerdict.INVALID, undefined];
    let expect;
    try {
      expect = expectationFor(env.type, env.raw);
    } catch {
      return [TokenVerdict.INVALID, undefined];
    }
    const result = await this.verifier.check(
      env.token,
      expect,
      this.actual(env),
    );
    return [result.verdict, result.claims];
  }

  private runIdOf(
    env: Envelope,
    claims: InvocationClaims | undefined,
  ): string | undefined {
    const payload = payloadOf(env.raw);
    const interaction = isRecord(payload.interaction)
      ? payload.interaction
      : {};
    return (
      uuidOf(payload.run_id) ?? uuidOf(interaction.run_id) ?? claims?.runId
    );
  }

  private async accept(env: Envelope): Promise<void> {
    const stored = await this.inbox.get(env.id);
    if (stored !== undefined) {
      await this.redelivery(env, stored);
      return;
    }
    let verdict: TokenVerdict;
    let claims: InvocationClaims | undefined;
    try {
      [verdict, claims] = await this.check(env);
    } catch (error) {
      // A failed JWKS or allow-list fetch: neither accept nor refuse.
      this.logger.warn(`token check of event ${env.id} failed; not settled`, {
        error: errorName(error),
      });
      return;
    }
    const runId = this.runIdOf(env, claims);
    if (verdict === TokenVerdict.UNKNOWN_KEY) {
      this.logger.info(`event ${env.id}: unknown key; not settled`);
      return;
    }
    if (verdict === TokenVerdict.REPLAY) {
      this.logger.warn(`event ${env.id}: replayed token ignored`);
      return;
    }
    const reason = REFUSAL_REASON.get(verdict);
    if (reason !== undefined) {
      await this.refuse(env, reason, runId);
      return;
    }
    // VALID and EXPIRED always carry claims.
    if (claims === undefined || env.token === undefined) return;
    const entry = new InboxEntry({
      eventId: env.id,
      seq: env.seq,
      type: env.type,
      runId,
      envelope: env.raw,
      token: env.token,
      tokenIat: claims.iat,
      tokenJti: claims.jti,
      state: EntryState.ACCEPTED,
      deliveries: 1,
    });
    await this.inbox.add(entry);
    await this.settle(env);
    await this.dispatch(entry, env, claims);
  }

  private async refuse(
    env: Envelope,
    reason: string,
    runId: string | undefined,
  ): Promise<void> {
    const state = runId === undefined ? undefined : this.runs.get(runId);
    const startsRun =
      env.type === START_EVENT ||
      ((env.type === FOLLOWUP || env.type === INTERACTION) &&
        state?.handle.waiting === true);
    if (!startsRun || runId === undefined) {
      this.logger.warn(`event ${env.id} refused (${reason}); run not ended`);
      await this.settle(env);
      return;
    }
    try {
      await unwrap(
        this.client.POST("/runs/{run_id}/finish", {
          params: { path: { run_id: runId } },
          body: { state: "failed", reason },
        }),
        "finishRun",
      );
    } catch (error) {
      if (error instanceof ProblemError) {
        if (error.status >= SERVER_ERROR || NOT_STORED.has(error.status)) {
          this.logger.warn(
            `refusal of run ${runId} not stored (${error.code}); not settled`,
          );
          return;
        }
        this.logger.info(`refusal of run ${runId} answered ${error.code}`);
      } else if (error instanceof TransportError) {
        this.logger.warn(`refusal of run ${runId} not sent; not settled`);
        return;
      } else {
        throw error;
      }
    }
    await this.inbox.add(
      new InboxEntry({
        eventId: env.id,
        seq: env.seq,
        type: env.type,
        runId,
        envelope: env.raw,
        token: undefined,
        tokenIat: undefined,
        tokenJti: undefined,
        state: EntryState.CLOSED,
        deliveries: 1,
      }),
    );
    await this.settle(env);
    this.logger.warn(`run ${runId} refused: ${reason}`);
  }

  private async redelivery(env: Envelope, stored: InboxEntry): Promise<void> {
    let token: Secret | undefined;
    let iat: number | undefined;
    let jti: string | undefined;
    if (env.token !== undefined && stored.state === EntryState.ACCEPTED) {
      let verdict: TokenVerdict;
      let claims: InvocationClaims | undefined;
      try {
        [verdict, claims] = await this.check(env);
      } catch {
        [verdict, claims] = [TokenVerdict.INVALID, undefined];
      }
      if (
        (verdict === TokenVerdict.VALID || verdict === TokenVerdict.EXPIRED) &&
        claims !== undefined
      )
        [token, iat, jti] = [env.token, claims.iat, claims.jti];
    }
    const updated = await this.inbox.redelivered(env.id, token, iat, jti);
    await this.settle(env);
    const state =
      updated.runId === undefined ? undefined : this.runs.get(updated.runId);
    if (
      state !== undefined &&
      state.waitingForToken &&
      updated.state === EntryState.ACCEPTED &&
      updated.tokenJti !== state.invocation.claims.jti
    ) {
      state.waitingForToken = false;
      state.task = this.spawn(() => this.start(state));
    }
  }

  private stateFor(entry: InboxEntry, claims: InvocationClaims): RunState {
    const runId = entry.runId;
    if (runId === undefined)
      throw new RangeError("a start event carries a run");
    const eventId = entry.eventId;
    const handle = new RunHandle(this.client, runId, claims.threadId, {
      outbox: this.outbox,
      settings: this.settings,
      onWaiting: () => this.inbox.setState(eventId, EntryState.WAITING),
    });
    const state: RunState = {
      runId,
      eventId,
      handle,
      invocation: this.invocation(
        eventId,
        entry.seq,
        entry.type,
        entry.envelope,
        claims,
      ),
      task: undefined,
      renewing: new Mutex(),
      resuming: new Mutex(),
      waitingForToken: false,
      cancelReason: undefined,
      interrupt: new AbortController(),
    };
    this.runs.set(runId, state);
    return state;
  }

  private async dispatch(
    entry: InboxEntry,
    env: Envelope,
    claims: InvocationClaims,
  ): Promise<void> {
    const inv = this.invocation(env.id, env.seq, env.type, env.raw, claims);
    if (env.type === START_EVENT) {
      const fresh = this.stateFor(entry, claims);
      fresh.task = this.spawn(() => this.start(fresh));
      return;
    }
    const state =
      entry.runId === undefined ? undefined : this.runs.get(entry.runId);
    if (env.type === FOLLOWUP || env.type === INTERACTION) {
      if (state === undefined) {
        this.logger.info(
          `event ${env.id} for a run this process does not hold`,
        );
        await this.inbox.setState(entry.eventId, EntryState.CLOSED);
        return;
      }
      this.spawn(() => this.continueRun(state, entry, inv));
      return;
    }
    if (env.type === COMMAND) this.spawn(() => this.command(entry, inv, state));
  }

  // ---- runs -----------------------------------------------------------------------------

  private async newest(state: RunState): Promise<InboxEntry> {
    const entry = await this.inbox.get(state.eventId);
    if (entry === undefined)
      throw new RangeError(`event not stored: ${state.eventId}`);
    return entry;
  }

  /** `renewInvocationToken` once per run at a time; the grant becomes the newest token. */
  private renew(
    state: RunState,
    token: Secret,
    eventId?: string,
  ): Promise<Secret> {
    const target = eventId ?? state.eventId;
    return state.renewing.run(async () => {
      const current = await this.inbox.get(target);
      if (
        current?.token !== undefined &&
        current.token.reveal() !== token.reveal()
      )
        return current.token; // another renewal or a redelivery won the race
      const grant = await unwrap(
        this.client.POST("/runs/{run_id}/invocation-token", {
          params: { path: { run_id: state.runId } },
          body: { invocation_token: token.reveal() },
        }),
        "renewInvocationToken",
      );
      let renewed = new Secret(grant.invocation_token);
      const claims = storedClaims(renewed);
      const stored = await this.inbox.replaceToken(
        target,
        renewed,
        claims.iat,
        claims.jti,
        token,
      );
      // A redelivery during the call replaced the run's token after the grant: start with
      // the most recently received token (09 4.5.1).
      if (stored.token !== undefined && !stored.token.equals(renewed))
        renewed = stored.token;
      this.runTokens.set(state.runId, renewed);
      this.client.setRunToken(state.runId, renewed);
      return renewed;
    });
  }

  /**
   * Renews, reacting to a refusal as the `reason` table of 09 4.5 and 4.5.1 says.
   *
   * Returns the renewed token, or `undefined` when the run must not start here (the entry is
   * handled: superseded waits, the rest is closed).
   */
  private async renewOrClose(
    state: RunState,
    token: Secret,
    eventId?: string,
  ): Promise<Secret | undefined> {
    try {
      return await this.renew(state, token, eventId);
    } catch (error) {
      if (error instanceof InvocationTokenInvalid) {
        if (error.reason === "superseded" && eventId === undefined)
          await this.superseded(state, token);
        else if (error.reason !== "superseded")
          await this.finishRefused(state, "invalid_invocation_token");
        // A superseded answer of a waiting run: a newer delivery is on its way.
        return undefined;
      }
      if (
        error instanceof AlreadyStarted ||
        error instanceof AgentInvocationDenied
      )
        this.logger.info(
          `renewal for run ${state.runId} refused: ${error.code}`,
        );
      else if (error instanceof ProblemError || error instanceof TransportError)
        this.logger.warn(
          `renewal for run ${state.runId} failed: ${error.name}`,
        );
      else throw error;
    }
    if (eventId === undefined) {
      await this.inbox.setState(state.eventId, EntryState.CLOSED);
      this.runs.delete(state.runId);
    }
    return undefined;
  }

  private expired(claims: InvocationClaims): boolean {
    return (
      this.clock.now() >=
      claims.exp * 1000 + this.settings.invocationTokenLeewayMs
    );
  }

  private async prepare(state: RunState): Promise<StartOptions | undefined> {
    const entry = await this.newest(state);
    if (entry.token === undefined) return undefined;
    let outcome: { readonly value: StartOptions } | Interrupted;
    try {
      outcome = await this.client.bindRun(state.runId, entry.token, () =>
        this.interruptible(state, () =>
          this.handler.prepare(state.invocation, state.handle),
        ),
      );
    } catch (error) {
      this.logger.error(`prepare failed for run ${state.runId}`, {
        error: errorName(error),
      });
      await this.finishRefused(state, "adapter_error");
      return undefined;
    }
    if (outcome === CANCELLED || outcome === STOPPED) {
      // A cancel request closes the entry; a stop of the adapter keeps it accepted, so the next
      // process starts the run again from the inbox (09 4.9).
      if (state.cancelReason !== undefined)
        await this.inbox.setState(state.eventId, EntryState.CLOSED);
      return undefined;
    }
    return outcome.value;
  }

  /** Starts a run from the inbox: prepare, compare again, renew if expired, start. */
  private async start(state: RunState): Promise<void> {
    const options = await this.prepare(state);
    if (options === undefined) return;
    const entry = await this.newest(state);
    if (entry.state !== EntryState.ACCEPTED || entry.token === undefined)
      return;
    const claims = storedClaims(entry.token);
    const inv: Invocation = Object.freeze({ ...state.invocation, claims });
    state.invocation = inv;
    const billing = this.handler.actualBilling(inv);
    const toolClass = this.handler.actualToolClass();
    const mismatch = compareActual(claims, {
      billingMode: billing.mode,
      toolClass,
    });
    const refusal = REFUSAL_REASON.get(mismatch);
    if (refusal !== undefined) {
      await this.finishRefused(state, refusal);
      return;
    }
    let token = entry.token;
    if (this.expired(claims)) {
      const renewed = await this.renewOrClose(state, token);
      if (renewed === undefined) return;
      token = renewed;
    }
    if (this.stopping || state.cancelReason !== undefined) return;
    const body: S["RunStart"] = {
      invocation_token: "",
      billing_mode_used: billing.mode as S["BillingMode"],
      tool_class_used: toolClass,
    };
    if (options.model !== undefined) body.model = options.model;
    if (options.externalSessionRef !== undefined)
      body.external_session_ref = options.externalSessionRef;
    const evidence = options.billingEvidence ?? billing.evidence;
    if (evidence !== undefined) body.billing_evidence = evidence;
    const started = await this.startCall(state, token, body);
    if (started === undefined) return;
    // A stop during startRun leaves the entry accepted: the runtime never ran here, so the
    // next process starts it again and the server answers with the idempotent replay (09 4.9).
    if (this.stopping) return;
    const [run, used] = started;
    await this.inbox.setState(state.eventId, EntryState.STARTED);
    state.handle.started(run, storedClaims(used).jti);
    this.client.setRunToken(state.runId, used);
    this.runTokens.set(state.runId, used);
    await this.executeRun(
      state,
      used,
      (i, r) => this.handler.execute(i, r),
      inv,
    );
  }

  /** `startRun` with the reactions of the `reason` table of 09 4.5. */
  private async startCall(
    state: RunState,
    first: Secret,
    body: S["RunStart"],
  ): Promise<[S["Run"], Secret] | undefined> {
    let token = first;
    for (let attempt = 0; attempt < 2; attempt++) {
      body.invocation_token = token.reveal();
      try {
        const run = await unwrap(
          this.client.POST("/runs/{run_id}/start", {
            params: { path: { run_id: state.runId } },
            body,
          }),
          "startRun",
        );
        return [run, token];
      } catch (error) {
        if (error instanceof InvocationTokenInvalid) {
          if (error.reason === "expired" && attempt === 0) {
            const renewed = await this.renewOrClose(state, token);
            if (renewed === undefined) return undefined;
            token = renewed;
            continue;
          }
          if (error.reason === "superseded")
            await this.superseded(state, token);
          else await this.finishRefused(state, "invalid_invocation_token");
          return undefined;
        }
        if (
          error instanceof AlreadyStarted ||
          error instanceof BillingModeMismatch ||
          error instanceof ToolClassMismatch ||
          error instanceof AgentInvocationDenied
        ) {
          // The server already ended or holds the run: close the entry, start nothing.
          await this.inbox.setState(state.eventId, EntryState.CLOSED);
          this.runs.delete(state.runId);
          return undefined;
        }
        throw error;
      }
    }
    return undefined;
  }

  /** Runs handler work of a started run and ends the run as 09 4.9 says. */
  private async executeRun(
    state: RunState,
    token: Secret,
    work: (inv: Invocation, run: RunHandle) => Promise<void>,
    inv: Invocation,
  ): Promise<void> {
    const handle = state.handle;
    try {
      await this.client.bindRun(state.runId, token, async () => {
        const outcome = await this.interruptible(state, () =>
          work(inv, handle),
        );
        // A stop leaves the run to the server; the next process closes the entry (09 4.9).
        if (outcome === STOPPED) return;
        if (outcome === CANCELLED || state.cancelReason !== undefined) {
          if (!handle.finished)
            await ignoreProblem(() =>
              handle.finish("cancelled", {
                reason: state.cancelReason ?? "user_cancelled",
              }),
            );
          return;
        }
        if (!handle.finished && !handle.waiting)
          await handle.finish("completed");
      });
    } catch (error) {
      const failure = error instanceof RuntimeFailure;
      if (!failure)
        this.logger.error(`handler failed for run ${state.runId}`, {
          error: errorName(error),
        });
      await this.client.bindRun(state.runId, token, () =>
        ignoreProblem(() =>
          handle.finish("failed", {
            reason: failure ? "runtime_error" : "adapter_error",
          }),
        ),
      );
    } finally {
      if (handle.finished || handle.cancelled.aborted) {
        try {
          await this.inbox.setState(state.eventId, EntryState.CLOSED);
        } catch {
          // The store may be closed already when the adapter stops.
        }
        this.runs.delete(state.runId);
      }
    }
  }

  private async finishRefused(state: RunState, reason: string): Promise<void> {
    try {
      await unwrap(
        this.client.POST("/runs/{run_id}/finish", {
          params: { path: { run_id: state.runId } },
          body: { state: "failed", reason },
        }),
        "finishRun",
      );
    } catch (error) {
      if (!(error instanceof ProblemError || error instanceof TransportError))
        throw error;
    }
    await this.inbox.setState(state.eventId, EntryState.CLOSED);
    this.runs.delete(state.runId);
  }

  /** `superseded`: start nothing; close when the run left `queued`, else wait. */
  private async superseded(state: RunState, used: Secret): Promise<void> {
    let run: S["Run"] | undefined;
    try {
      run = await unwrap(
        this.client.GET("/runs/{run_id}", {
          params: { path: { run_id: state.runId } },
        }),
        "getRun",
      );
    } catch (error) {
      if (!(error instanceof ProblemError || error instanceof TransportError))
        throw error;
      run = undefined;
    }
    if (run !== undefined && run.state !== "queued") {
      await this.inbox.setState(state.eventId, EntryState.CLOSED);
      this.runs.delete(state.runId);
      return;
    }
    state.waitingForToken = true;
    // A redelivery may have stored a newer token while startRun was on its way.
    const entry = await this.inbox.get(state.eventId);
    // A redelivery may also have restarted the run during the read: start at most once.
    if (
      state.waitingForToken &&
      entry?.token !== undefined &&
      entry.token.reveal() !== used.reveal()
    ) {
      state.waitingForToken = false;
      state.task = this.spawn(() => this.start(state));
    }
  }

  /**
   * A follow-up or interaction: resume a waiting run, or hand it to a running one.
   *
   * Resumes are serialised per run: of two answers to one question, the first resumes the run
   * and the second reaches the handler as a follow-up of the running run.
   */
  private async continueRun(
    state: RunState,
    entry: InboxEntry,
    inv: Invocation,
  ): Promise<void> {
    if (entry.token === undefined) return; // accepted entries always carry their token
    const work = (i: Invocation, r: RunHandle): Promise<void> =>
      entry.type === FOLLOWUP
        ? this.handler.onFollowup(i, r)
        : this.handler.onInteraction(i, r);
    const [token, runningHere] = await state.resuming.run(async () => {
      const resumed = state.handle.waiting
        ? await this.resume(state, entry, inv)
        : undefined;
      return [
        resumed,
        state.handle.tokenJti !== undefined && !state.handle.waiting,
      ] as const;
    });
    await this.inbox.setState(entry.eventId, EntryState.CLOSED);
    if (token !== undefined) {
      const task: Task = { done: false };
      state.task = task;
      try {
        await this.executeRun(state, token, work, inv);
      } finally {
        task.done = true;
      }
      return;
    }
    if (runningHere && !state.handle.finished) {
      const bound = this.runTokens.get(state.runId) ?? entry.token;
      await this.client.bindRun(state.runId, bound, () =>
        this.interruptible(undefined, () => work(inv, state.handle)),
      );
    }
  }

  /**
   * `resumeRun` with the event's token; `undefined` when the run did not resume here.
   *
   * An expired token is renewed once; a superseded one is replaced once by the token of a
   * newer answer of the same run.
   */
  private async resume(
    state: RunState,
    entry: InboxEntry,
    inv: Invocation,
  ): Promise<Secret | undefined> {
    let token = entry.token;
    for (let attempt = 0; attempt < 2; attempt++) {
      if (token === undefined) return undefined;
      const body: S["RunStart"] = {
        invocation_token: token.reveal(),
        billing_mode_used: this.handler.actualBilling(inv)
          .mode as S["BillingMode"],
        tool_class_used: this.handler.actualToolClass(),
      };
      let run: S["Run"];
      try {
        run = await unwrap(
          this.client.POST("/runs/{run_id}/resume", {
            params: { path: { run_id: state.runId } },
            body,
          }),
          "resumeRun",
        );
      } catch (error) {
        if (error instanceof InvocationTokenInvalid) {
          this.logger.info(
            `resume of run ${state.runId} refused: ${error.reason ?? error.code}`,
          );
          if (attempt === 0 && error.reason === "expired") {
            token = await this.renewOrClose(state, token, entry.eventId);
            continue;
          }
          if (attempt === 0 && error.reason === "superseded") {
            token = await this.newerAnswer(state, entry);
            continue;
          }
          if (error.reason === "invalid")
            await this.finishRefused(state, "invalid_invocation_token");
          return undefined;
        }
        if (error instanceof ProblemError || error instanceof TransportError) {
          if (!(
            error instanceof AlreadyStarted ||
            error instanceof BillingModeMismatch ||
            error instanceof ToolClassMismatch ||
            error instanceof AgentInvocationDenied
          ))
            this.logger.warn(
              `resume of run ${state.runId} failed: ${error.name}`,
            );
          return undefined;
        }
        throw error;
      }
      await this.inbox.setState(state.eventId, EntryState.STARTED);
      state.handle.started(run, storedClaims(token).jti);
      this.client.setRunToken(state.runId, token);
      this.runTokens.set(state.runId, token);
      return token;
    }
    return undefined;
  }

  /** The token of a later accepted follow-up or interaction of the same run, if any. */
  private async newerAnswer(
    state: RunState,
    entry: InboxEntry,
  ): Promise<Secret | undefined> {
    const newer = (await this.inbox.forRun(state.runId)).filter(
      (e) =>
        e.eventId !== entry.eventId &&
        (e.type === FOLLOWUP || e.type === INTERACTION) &&
        e.token !== undefined &&
        (e.tokenIat ?? 0) >= (entry.tokenIat ?? 0),
    );
    newer.sort((a, b) => (a.tokenIat ?? 0) - (b.tokenIat ?? 0));
    return newer.at(-1)?.token;
  }

  private async command(
    entry: InboxEntry,
    inv: Invocation,
    state: RunState | undefined,
  ): Promise<void> {
    const token = entry.token;
    if (token === undefined) return; // accepted entries always carry their token
    const handle =
      state !== undefined && state.handle.tokenJti !== undefined
        ? state.handle
        : undefined;
    try {
      if (handle !== undefined)
        await this.client.bindRun(handle.runId, token, () =>
          this.interruptible(undefined, () =>
            this.handler.onCommand(inv, handle),
          ),
        );
      else
        await this.interruptible(undefined, () =>
          this.handler.onCommand(inv, undefined),
        );
    } finally {
      await this.inbox.setState(entry.eventId, EntryState.CLOSED);
    }
  }

  private async onCancel(env: Envelope): Promise<void> {
    const payload = payloadOf(env.raw);
    const runId = uuidOf(payload.run_id);
    const state = runId === undefined ? undefined : this.runs.get(runId);
    if (state === undefined) return;
    const reason =
      typeof payload.reason === "string" && payload.reason
        ? payload.reason
        : "user_cancelled";
    state.cancelReason = reason;
    state.handle.setCancelled();
    if (state.handle.tokenJti === undefined) {
      // Not started yet: the server cancels a queued run itself; only close the entry.
      state.interrupt.abort();
      await this.inbox.setState(state.eventId, EntryState.CLOSED);
      this.runs.delete(state.runId);
      return;
    }
    if (state.task !== undefined && !state.task.done) {
      state.interrupt.abort();
    } else if (!state.handle.finished) {
      const token =
        this.runTokens.get(state.runId) ?? (await this.newest(state)).token;
      if (token !== undefined)
        await this.client.bindRun(state.runId, token, () =>
          ignoreProblem(() => state.handle.finish("cancelled", { reason })),
        );
      await this.inbox.setState(state.eventId, EntryState.CLOSED);
      this.runs.delete(state.runId);
    }
  }

  private async onStateChanged(env: Envelope): Promise<void> {
    const summary = payloadOf(env.raw).run;
    if (!isRecord(summary)) return;
    const runId = uuidOf(summary.id);
    const state = runId === undefined ? undefined : this.runs.get(runId);
    const next = typeof summary.state === "string" ? summary.state : undefined;
    if (state?.waitingForToken && next !== undefined && next !== "queued") {
      await this.inbox.setState(state.eventId, EntryState.CLOSED);
      this.runs.delete(state.runId);
    }
  }

  private async autocomplete(env: Envelope): Promise<void> {
    let verdict: TokenVerdict;
    try {
      [verdict] = await this.check(env);
    } catch {
      this.logger.warn(`autocomplete ${env.id}: token check failed`);
      return;
    }
    if (verdict !== TokenVerdict.VALID) {
      this.logger.warn(`autocomplete ${env.id} refused (${verdict})`);
      return;
    }
    const source = this.source;
    if (source === undefined) return;
    let request: AutocompleteRequest;
    try {
      request = autocompleteRequest(
        payloadOf(
          env.raw,
        ) as unknown as S["CommandAutocompleteRequestedPayload"],
      );
    } catch {
      this.logger.warn(`autocomplete ${env.id}: payload refused`);
      return;
    }
    await guardedAutocomplete(
      (req, signal) => this.handler.onAutocomplete(req, signal),
      request,
      (requestId, choices) => source.answerAutocomplete(requestId, choices),
      { clock: this.clock, settings: this.settings, logger: this.logger },
    );
  }
}
