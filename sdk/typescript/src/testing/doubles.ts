// SPDX-License-Identifier: Apache-2.0
/**
 * Test doubles of the hand layer: fingerprints and recording helpers (port of
 * `bothy_sdk/testing/doubles.py`; its `FakeClock` is `./clock.js`).
 *
 * {@link fingerprint} lets a test compare tokens without ever printing one (SEC-LOG-2);
 * {@link TamperingSource} and {@link RecordingSource} wrap an adapter's event source, and
 * {@link RecordingHandler} records the calls of the runtime side.
 *
 * @example
 * ```ts
 * const handler = new RecordingHandler();
 * const adapter = new Adapter(config, handler, { clock: fake.clock });
 * const recording = new RecordingSource(await adapter.makeSource());
 * adapter.source = recording;
 * ```
 *
 * @module
 */

import { createHash } from "node:crypto";

import { Secret } from "../auth/secret.js";
import type { Clock } from "../clock.js";
import type { AutocompleteRequest } from "../commands/index.js";
import {
  Envelope,
  type CloseReason,
  type Delivery,
  type EventSource,
} from "../gateway/index.js";
import type { components } from "../generated/openapi.js";
import {
  AgentHandler,
  type ActualBilling,
  type Invocation,
  type RunHandle,
  type StartOptions,
} from "../runs/index.js";

type S = components["schemas"];

/**
 * Returns a short, stable fingerprint of a token for assertions and logs.
 *
 * @param token - A token string or a {@link Secret}.
 * @returns The first 16 hex characters of the SHA-256 of the token.
 *
 * @example
 * ```ts
 * expect(fingerprint(first)).not.toBe(fingerprint(second));
 * ```
 */
export function fingerprint(token: Secret | string): string {
  const text = token instanceof Secret ? token.reveal() : String(token);
  return createHash("sha256").update(text).digest("hex").slice(0, 16);
}

/** Rewrite result of {@link TamperingSource}: keep the delivered token. */
export const KEEP: unique symbol = Symbol("KEEP");
/** Rewrite result of {@link TamperingSource}: remove the delivered token. */
export const REMOVE: unique symbol = Symbol("REMOVE");

/**
 * The rewrite of a {@link TamperingSource}: a new token, {@link KEEP} or {@link REMOVE}. It may
 * wait, for example until the scenario forged the token.
 */
export type Rewrite = (
  envelope: Envelope,
) => Promise<Secret | typeof KEEP | typeof REMOVE>;

/** Forwards everything but iteration to the wrapped source. */
abstract class Wrapper implements EventSource {
  /** The wrapped source. */
  readonly inner: EventSource;

  constructor(inner: EventSource) {
    this.inner = inner;
  }

  /** Why the wrapped source stopped, once it has. */
  get closeReason(): CloseReason | undefined {
    return this.inner.closeReason;
  }

  abstract [Symbol.asyncIterator](): AsyncIterator<Delivery>;

  /**
   * Forwards to the wrapped source.
   *
   * @param seq - The settled `seq`.
   */
  settle(seq: number): Promise<void> {
    return this.inner.settle(seq);
  }

  /**
   * Forwards to the wrapped source.
   *
   * @param seq - The cursor.
   */
  advance(seq: number): Promise<void> {
    return this.inner.advance(seq);
  }

  /**
   * Forwards to the wrapped source.
   *
   * @param seq - The `seq`.
   */
  ack(seq: number): Promise<void> {
    return this.inner.ack(seq);
  }

  /**
   * Forwards to the wrapped source.
   *
   * @param requestId - The request.
   * @param choices - The choices.
   */
  answerAutocomplete(
    requestId: string,
    choices: readonly S["CommandChoice"][],
  ): Promise<void> {
    return this.inner.answerAutocomplete(requestId, choices);
  }

  /** Forwards to the wrapped source. */
  close(): Promise<void> {
    return this.inner.close();
  }
}

/**
 * Rewrites or removes the `invocation_token` of chosen deliveries (AG-08, AZ-P6).
 *
 * @example
 * ```ts
 * adapter.source = new TamperingSource(await adapter.makeSource(), async () => REMOVE);
 * ```
 */
export class TamperingSource extends Wrapper {
  private readonly rewrite: Rewrite;

  /**
   * Wraps `inner`.
   *
   * @param inner - The real source.
   * @param rewrite - Called for every envelope that carries a token; returns a new token,
   *   {@link KEEP} or {@link REMOVE}.
   */
  constructor(inner: EventSource, rewrite: Rewrite) {
    super(inner);
    this.rewrite = rewrite;
  }

  /**
   * Yields the inner deliveries with rewritten tokens.
   *
   * @returns The deliveries.
   */
  async *[Symbol.asyncIterator](): AsyncGenerator<Delivery, void, undefined> {
    for await (const delivery of this.inner) {
      let out = delivery;
      if (delivery instanceof Envelope && delivery.token !== undefined) {
        const result = await this.rewrite(delivery);
        if (result === REMOVE)
          out = new Envelope({ ...delivery, token: undefined });
        else if (result instanceof Secret)
          out = new Envelope({ ...delivery, token: result });
      }
      yield out;
    }
  }
}

/**
 * Records every delivery (tokens stay in memory as {@link Secret}, never printed).
 *
 * @example
 * ```ts
 * const recording = new RecordingSource(await adapter.makeSource());
 * recording.envelopes("agent.invocation").length;
 * ```
 */
export class RecordingSource extends Wrapper {
  /** Every delivery in order. */
  readonly deliveries: Delivery[] = [];

  /**
   * Wraps `inner`.
   *
   * @param inner - The real source.
   */
  constructor(inner: EventSource) {
    super(inner);
  }

  /**
   * Yields and records the inner deliveries.
   *
   * @returns The deliveries.
   */
  async *[Symbol.asyncIterator](): AsyncGenerator<Delivery, void, undefined> {
    for await (const delivery of this.inner) {
      this.deliveries.push(delivery);
      yield delivery;
    }
  }

  /**
   * Returns the recorded envelopes, optionally of one type.
   *
   * @param eventType - The event type to keep.
   * @returns The envelopes in delivery order.
   */
  envelopes(eventType?: string): Envelope[] {
    return this.deliveries.filter(
      (d): d is Envelope =>
        d instanceof Envelope &&
        (eventType === undefined || d.type === eventType),
    );
  }
}

/** The options of {@link RecordingHandler}; every member is optional. */
export interface RecordingHandlerOptions {
  /** The actual billing mode; absent follows the token's claim. */
  readonly billing?: string | undefined;
  /** The actual tool class; default `chat`. */
  readonly toolClass?: S["ToolClass"] | undefined;
  /** When set, `prepare` waits for this promise. */
  readonly blockPrepare?: PromiseLike<void> | undefined;
  /** When set, `execute` waits for this promise. */
  readonly blockExecute?: PromiseLike<void> | undefined;
}

/**
 * An {@link AgentHandler} that records its calls and can block in `prepare` or `execute`.
 *
 * @example
 * ```ts
 * let release = (): void => undefined;
 * const gate = new Promise<void>((r) => (release = r));
 * const handler = new RecordingHandler({ blockPrepare: gate });
 * // ... later
 * release();
 * handler.count("execute"); // 1 once the run started
 * ```
 */
export class RecordingHandler extends AgentHandler {
  /** The actual billing mode; `undefined` follows the token's claim. */
  billing: string | undefined;
  /** The actual tool class. */
  toolClass: S["ToolClass"];
  /** When set, `prepare` waits for it. */
  blockPrepare: PromiseLike<void> | undefined;
  /** When set, `execute` waits for it. */
  blockExecute: PromiseLike<void> | undefined;
  /** Every call as `[name, runId]`, in order. */
  readonly calls: [string, string | undefined][] = [];
  /** The handle of each run seen in `prepare` or `execute`. */
  readonly handles = new Map<string, RunHandle>();
  /** The invocations `prepare` received, in order. */
  readonly invocations: Invocation[] = [];
  /** The envelopes `onEvent` received, in order. */
  readonly events: Envelope[] = [];
  /** The choices `onAutocomplete` returns. */
  autocompleteChoices: readonly S["CommandChoice"][] = [];
  /** The wait of `onAutocomplete` on {@link RecordingHandler.clock}, in milliseconds. */
  autocompleteDelayMs = 0;
  /** The clock of the autocomplete wait (the scenarios set the server's clock). */
  clock: Clock | undefined;
  /** `true` once any call was recorded (Python's `changed` event, as a flag). */
  changed = false;

  /**
   * Creates the handler.
   *
   * @param options - Billing mode, tool class and the promises to wait for.
   */
  constructor(options: RecordingHandlerOptions = {}) {
    super();
    this.billing = options.billing;
    this.toolClass = options.toolClass ?? "chat";
    this.blockPrepare = options.blockPrepare;
    this.blockExecute = options.blockExecute;
  }

  private record(name: string, runId: string | undefined): void {
    this.calls.push([name, runId]);
    this.changed = true;
  }

  /**
   * Returns how often `name` was called (for `runId` when given).
   *
   * @param name - `prepare`, `execute`, `onFollowup`, ...
   * @param runId - The run to count for.
   * @returns The count.
   */
  count(name: string, runId?: string): number {
    return this.calls.filter(
      ([n, r]) => n === name && (runId === undefined || r === runId),
    ).length;
  }

  /**
   * Returns {@link RecordingHandler.billing} or the claim's mode.
   *
   * @param inv - The invocation.
   * @returns The actual billing.
   */
  override actualBilling(inv: Invocation): ActualBilling {
    return { mode: this.billing ?? inv.claims.billingMode };
  }

  /**
   * Returns {@link RecordingHandler.toolClass}.
   *
   * @returns The tool class.
   */
  override actualToolClass(): S["ToolClass"] {
    return this.toolClass;
  }

  /**
   * Records, waits for `blockPrepare` and returns the model `model-test`.
   *
   * @param inv - The invocation.
   * @param run - The run.
   * @returns The start options.
   */
  override async prepare(
    inv: Invocation,
    run: RunHandle,
  ): Promise<StartOptions> {
    this.record("prepare", run.runId);
    this.invocations.push(inv);
    this.handles.set(run.runId, run);
    if (this.blockPrepare !== undefined) await this.blockPrepare;
    return { model: "model-test" };
  }

  /**
   * Records and waits for `blockExecute`.
   *
   * @param inv - The invocation.
   * @param run - The started run.
   */
  override async execute(inv: Invocation, run: RunHandle): Promise<void> {
    void inv;
    this.record("execute", run.runId);
    this.handles.set(run.runId, run);
    if (this.blockExecute !== undefined) await this.blockExecute;
  }

  /**
   * Records.
   *
   * @param inv - The invocation.
   * @param run - The run.
   */
  override onFollowup(inv: Invocation, run: RunHandle): Promise<void> {
    void inv;
    this.record("onFollowup", run.runId);
    return Promise.resolve();
  }

  /**
   * Records.
   *
   * @param inv - The invocation.
   * @param run - The run.
   */
  override onInteraction(inv: Invocation, run: RunHandle): Promise<void> {
    void inv;
    this.record("onInteraction", run.runId);
    return Promise.resolve();
  }

  /**
   * Records.
   *
   * @param inv - The invocation.
   * @param run - The run, if it is running here.
   */
  override onCommand(
    inv: Invocation,
    run: RunHandle | undefined,
  ): Promise<void> {
    void inv;
    this.record("onCommand", run?.runId);
    return Promise.resolve();
  }

  /**
   * Records and returns {@link RecordingHandler.autocompleteChoices} after
   * {@link RecordingHandler.autocompleteDelayMs}.
   *
   * @param request - The request.
   * @param signal - Aborted when the request expires.
   * @returns The choices.
   */
  override async onAutocomplete(
    request: AutocompleteRequest,
    signal: AbortSignal,
  ): Promise<readonly S["CommandChoice"][]> {
    void request;
    this.record("onAutocomplete", undefined);
    if (this.autocompleteDelayMs > 0 && this.clock !== undefined)
      await this.clock.sleep(this.autocompleteDelayMs, signal);
    return this.autocompleteChoices;
  }

  /**
   * Records.
   *
   * @param envelope - The event.
   */
  override onEvent(envelope: Envelope): Promise<void> {
    this.events.push(envelope);
    this.record("onEvent", undefined);
    return Promise.resolve();
  }
}
