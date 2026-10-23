// SPDX-License-Identifier: Apache-2.0
/**
 * Event sources of an agent adapter: the gateway or REST polling (docs/spec/09-agents.md 4.2;
 * port of `bothy_sdk/sources.py`).
 *
 * A source yields deliveries: {@link Envelope} (one event with its delivery fields),
 * {@link CursorAdvance} (the cursor moves without an event) and {@link SourceReady} (a
 * connection is ready; the adapter flushes its outbox and reconciles its commands). The adapter
 * settles each delivered `seq` with {@link EventSource.settle}; the source acknowledges the
 * highest `seq` up to which every delivered event is settled (docs/DECISIONS.md D-M2-29-6).
 * Because `ack` is cumulative (04 15.2 point 1), one unsettled event blocks the acknowledgement
 * of every later one. A source never acknowledges by itself.
 *
 * @example
 * ```ts
 * const window = new AckWindow();
 * window.delivered(5);
 * window.delivered(6);
 * window.settle(6); // undefined
 * window.settle(5); // 6
 * ```
 *
 * @module
 */

import { randomInt } from "node:crypto";

import { Secret } from "../auth/secret.js";
import { type ApiClient, unwrap } from "../client.js";
import type { Clock } from "../clock.js";
import type { SdkSettings } from "../config.js";
import {
  ForbiddenError,
  GoneError,
  ProblemError,
  TransportError,
  UnauthenticatedError,
} from "../errors.js";
import type { components } from "../generated/openapi.js";
import { type Logger } from "../logger.js";
import {
  type KnownEvent,
  type UnknownEvent,
  decodeEvent,
} from "../tolerance.js";
import { CloseReason, type Gateway, type GatewayEvent } from "./gateway.js";

type S = components["schemas"];

const AUTOCOMPLETE = "command.autocomplete_requested";
const TOKEN_FIELD = "invocation_token";
const UUID = /^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$/i;
// randomInt needs a range below 2^48.
const RANDOM_RANGE = 2 ** 47;

/**
 * The ephemeral `command.autocomplete_requested` event, which is not part of the durable
 * `Event` union of the core (no `seq`, no inbox, no acknowledgement).
 */
export interface EphemeralEvent {
  /** Always `ephemeral`. */
  readonly kind: "ephemeral";
  /** The event with its generated type. */
  readonly event: S["CommandAutocompleteRequestedEvent"];
}

/** The members of one {@link Envelope}. */
export interface EnvelopeFields {
  /** The envelope as received, without `invocation_token`. */
  readonly raw: Readonly<Record<string, unknown>>;
  /** The event decoded with the tolerance layer. */
  readonly event: KnownEvent | UnknownEvent | EphemeralEvent;
  /** The envelope `id`. */
  readonly id: string;
  /** The event type, for example `agent.invocation`. */
  readonly type: string;
  /** The `seq`; `undefined` for an ephemeral event. */
  readonly seq: number | undefined;
  /** The delivery field `invocation_token`, if any. */
  readonly token: Secret | undefined;
  /** The delivery field `redelivered`. */
  readonly redelivered: boolean;
}

/** One delivered event; its token never renders (it is a {@link Secret}). */
export class Envelope implements EnvelopeFields {
  /** The envelope as received, without `invocation_token`. */
  readonly raw: Readonly<Record<string, unknown>>;
  /**
   * The event decoded with the tolerance layer: `known` with its generated type, `unknown`
   * (still acknowledgeable) or `ephemeral` for an autocomplete request.
   */
  readonly event: KnownEvent | UnknownEvent | EphemeralEvent;
  /** The envelope `id`. */
  readonly id: string;
  /** The event type, for example `agent.invocation`. */
  readonly type: string;
  /** The `seq`; `undefined` for an ephemeral event. */
  readonly seq: number | undefined;
  /** The delivery field `invocation_token`, if any. */
  readonly token: Secret | undefined;
  /** The delivery field `redelivered`. */
  readonly redelivered: boolean;

  /**
   * Creates the envelope; use {@link envelopeFrom} for a received one.
   *
   * @param fields - Its members.
   */
  constructor(fields: EnvelopeFields) {
    this.raw = fields.raw;
    this.event = fields.event;
    this.id = fields.id;
    this.type = fields.type;
    this.seq = fields.seq;
    this.token = fields.token;
    this.redelivered = fields.redelivered;
  }

  /** True for an event without `seq` (no inbox, no acknowledgement). */
  get ephemeral(): boolean {
    return this.seq === undefined;
  }
}

/** The cursor moved to `seq` without an event (a `cursor` frame or a poll cursor). */
export class CursorAdvance {
  /** The cursor. */
  readonly seq: number;

  /**
   * Creates the delivery.
   *
   * @param seq - The cursor.
   */
  constructor(seq: number) {
    this.seq = seq;
  }
}

/** A connection is ready (`ready` or `resumed` of the gateway, the first poll). */
export class SourceReady {
  /** True after a `resume`. */
  readonly resumed: boolean;
  /** True after `resync_required` (inbox entries keep their state). */
  readonly resync: boolean;

  /**
   * Creates the delivery.
   *
   * @param options - `resumed` and `resync`, both default `false`.
   */
  constructor(options: { resumed?: boolean; resync?: boolean } = {}) {
    this.resumed = options.resumed ?? false;
    this.resync = options.resync ?? false;
  }
}

/** What a source yields. */
export type Delivery = Envelope | CursorAdvance | SourceReady;

/**
 * Builds an {@link Envelope} from a received envelope, taking the token out of it.
 *
 * @param data - The envelope (`d` of a dispatch or an item of `EventPage.events`).
 * @returns The envelope.
 * @throws `TypeError` when `id` or `type` is missing or malformed.
 */
export function envelopeFrom(
  data: Readonly<Record<string, unknown>>,
): Envelope {
  const raw: Record<string, unknown> = { ...data };
  const tokenText = raw[TOKEN_FIELD];
  delete raw[TOKEN_FIELD];
  // An empty token counts as missing; the adapter refuses a token-bearing event without one.
  const token =
    typeof tokenText === "string" && tokenText
      ? new Secret(tokenText)
      : undefined;
  const { type, id, seq } = raw;
  if (typeof type !== "string" || typeof id !== "string" || !UUID.test(id))
    throw new TypeError("envelope without id or type");
  const event: KnownEvent | UnknownEvent | EphemeralEvent =
    type === AUTOCOMPLETE
      ? {
          kind: "ephemeral",
          event: raw as S["CommandAutocompleteRequestedEvent"],
        }
      : decodeEvent(raw);
  return new Envelope({
    raw: Object.freeze(raw),
    event,
    id: id.toLowerCase(),
    type,
    seq: typeof seq === "number" && Number.isSafeInteger(seq) ? seq : undefined,
    token,
    redelivered: Boolean(raw.redelivered),
  });
}

/** Tracks delivered and settled `seq` values and computes the acknowledgement point. */
export class AckWindow {
  private readonly open = new Set<number>();
  private settled = new Set<number>();
  private point = 0;

  /** The highest acknowledged `seq`. */
  get acked(): number {
    return this.point;
  }

  /**
   * Notes a delivered event.
   *
   * @param seq - Its `seq`.
   */
  delivered(seq: number): void {
    if (seq > this.point) this.open.add(seq);
  }

  /**
   * Notes a settled event or cursor and returns a new acknowledgement point, if any.
   *
   * @param seq - The settled `seq` (a cursor counts as delivered and settled).
   * @returns The new highest `seq` up to which everything delivered is settled, or `undefined`
   *   when the point did not move.
   */
  settle(seq: number): number | undefined {
    if (seq <= this.point) return undefined;
    this.open.delete(seq);
    this.settled.add(seq);
    const limit = this.open.size > 0 ? Math.min(...this.open) : undefined;
    const candidates = [...this.settled].filter(
      (s) => limit === undefined || s < limit,
    );
    if (candidates.length === 0) return undefined;
    const point = Math.max(...candidates);
    this.settled = new Set([...this.settled].filter((s) => s > point));
    this.point = point;
    return point;
  }

  /**
   * Notes a cursor (a `cursor` frame or a poll cursor) and returns a new point, if any.
   *
   * A cursor says that nothing undelivered lies at or below `seq`; it never settles an event
   * that was delivered and is still open, so a cursor equal to the `seq` of such an event
   * changes nothing.
   *
   * @param seq - The cursor.
   * @returns The new acknowledgement point, or `undefined`.
   */
  cursor(seq: number): number | undefined {
    if (this.open.has(seq)) return undefined;
    return this.settle(seq);
  }

  /**
   * Starts over after a (re)connect.
   *
   * The server delivers again from its stored cursor, which a server restart may have set back
   * (04 15.2 point 4), so the window forgets its own point and acknowledges again.
   */
  reset(): void {
    this.open.clear();
    this.settled.clear();
    this.point = 0;
  }
}

/** A transport of events for one consumer: an async iterable of deliveries plus `ack`. */
export interface EventSource extends AsyncIterable<Delivery> {
  /** Why the source stopped, once it has. */
  readonly closeReason: CloseReason | undefined;
  /**
   * Marks `seq` settled and acknowledges when the acknowledgement point moves.
   *
   * @param seq - The settled `seq`.
   */
  settle(seq: number): Promise<void>;
  /**
   * Passes on a cursor; it never settles an open delivered event.
   *
   * @param seq - The cursor.
   */
  advance(seq: number): Promise<void>;
  /**
   * Acknowledges every event up to `seq`.
   *
   * @param seq - The `seq`.
   */
  ack(seq: number): Promise<void>;
  /**
   * Sends the choices of an autocomplete request.
   *
   * @param requestId - The request.
   * @param choices - The choices.
   */
  answerAutocomplete(
    requestId: string,
    choices: readonly S["CommandChoice"][],
  ): Promise<void>;
  /** Stops the source; the iterator ends. */
  close(): Promise<void>;
}

/** Runs the settle steps of one source one after another (the lock of the Python source). */
class Serial {
  private tail: Promise<void> = Promise.resolve();

  run(step: () => Promise<void>): Promise<void> {
    const next = this.tail.then(step);
    this.tail = next.catch(() => undefined);
    return next;
  }
}

async function settleWith(
  serial: Serial,
  move: () => number | undefined,
  ack: (seq: number) => Promise<void>,
): Promise<void> {
  await serial.run(async () => {
    const point = move();
    if (point !== undefined) await ack(point);
  });
}

/** The members of a {@link Gateway} a {@link GatewaySource} uses. */
export type GatewayLike = Pick<
  Gateway,
  "frames" | "ack" | "sendAutocompleteResult" | "close" | "closeReason"
>;

/** Events over the gateway; acknowledged with the op `ack`. */
export class GatewaySource implements EventSource {
  /** The gateway session. */
  readonly gateway: GatewayLike;
  /** The acknowledgement window. */
  readonly window = new AckWindow();
  /** Why the source stopped, once it has. */
  closeReason: CloseReason | undefined;
  /** Receives every `error` frame (a failed op, with a `GatewayOpError` as model). */
  onError: ((frame: GatewayEvent) => void) | undefined;
  private readonly serial = new Serial();

  /**
   * Wraps a gateway.
   *
   * @param gateway - The gateway session.
   */
  constructor(gateway: GatewayLike) {
    this.gateway = gateway;
  }

  /**
   * Yields the deliveries of the gateway until it stops.
   *
   * @returns The deliveries.
   */
  async *[Symbol.asyncIterator](): AsyncGenerator<Delivery, void, undefined> {
    try {
      for await (const frame of this.gateway.frames()) {
        if (frame.op === "ready" || frame.op === "resumed") {
          this.window.reset();
          yield new SourceReady({ resumed: frame.op === "resumed" });
        } else if (frame.op === "resync_required") {
          this.window.reset();
          yield new SourceReady({ resync: true });
        } else if (frame.op === "dispatch") {
          let envelope: Envelope;
          try {
            envelope = envelopeFrom(frame.data);
          } catch {
            continue; // a dispatch without id or type
          }
          if (envelope.seq !== undefined) this.window.delivered(envelope.seq);
          yield envelope;
        } else if (frame.op === "cursor") {
          const seq = frame.data.seq;
          yield new CursorAdvance(typeof seq === "number" ? seq : 0);
        } else if (frame.op === "error") {
          this.onError?.(frame);
        }
      }
    } finally {
      this.closeReason = this.gateway.closeReason;
    }
  }

  /**
   * Marks `seq` settled and sends `ack` when the acknowledgement point moves.
   *
   * @param seq - The settled `seq`.
   * @returns A promise that resolves after a needed `ack`.
   */
  settle(seq: number): Promise<void> {
    return settleWith(
      this.serial,
      () => this.window.settle(seq),
      (s) => this.ack(s),
    );
  }

  /**
   * Passes on a cursor; it never settles an open delivered event.
   *
   * @param seq - The cursor.
   * @returns A promise that resolves after a needed `ack`.
   */
  advance(seq: number): Promise<void> {
    return settleWith(
      this.serial,
      () => this.window.cursor(seq),
      (s) => this.ack(s),
    );
  }

  /**
   * Sends the op `ack`.
   *
   * @param seq - The highest settled `seq`.
   * @returns A promise that resolves once the frame is handed to the socket.
   */
  ack(seq: number): Promise<void> {
    return this.gateway.ack(seq);
  }

  /**
   * Sends the op `command.autocomplete_result`.
   *
   * @param requestId - The request.
   * @param choices - The choices.
   * @returns A promise that resolves once the frame is handed to the socket.
   * @throws `TransportError` when the gateway is not connected.
   */
  answerAutocomplete(
    requestId: string,
    choices: readonly S["CommandChoice"][],
  ): Promise<void> {
    return this.gateway.sendAutocompleteResult(requestId, choices);
  }

  /**
   * Closes the gateway with `1000`.
   *
   * @returns A promise that resolves once the close is started.
   */
  close(): Promise<void> {
    return this.gateway.close();
  }
}

function pageEnvelopes(events: unknown): Envelope[] {
  const out: Envelope[] = [];
  for (const item of Array.isArray(events) ? (events as unknown[]) : []) {
    if (typeof item !== "object" || item === null || Array.isArray(item))
      continue;
    try {
      out.push(envelopeFrom(item as Record<string, unknown>));
    } catch {
      // An item without id or type is skipped; the page cursor still moves past it.
    }
  }
  return out;
}

function cryptoRandom(): number {
  return randomInt(RANDOM_RANGE) / RANDOM_RANGE;
}

/** The options of {@link PollingSource}. */
export interface PollingSourceOptions {
  /** The SDK settings (`sdk.poll_interval`, the reconnect backoff); default the client's. */
  readonly settings?: SdkSettings | Partial<SdkSettings> | undefined;
  /** The time source (default the client's). */
  readonly clock?: Clock | undefined;
  /** Returns the agent heartbeat to send, or `undefined` for none. */
  readonly heartbeat?: (() => S["Heartbeat"] | undefined) | undefined;
  /** The page size asked for (the server caps it). */
  readonly limit?: number | undefined;
  /** The jitter source in [0, 1) of the retry wait (default from `node:crypto`). */
  readonly random?: (() => number) | undefined;
  /** The logger (default the client's). */
  readonly logger?: Logger | undefined;
}

type PollOutcome =
  | { readonly kind: "page"; readonly page: Readonly<Record<string, unknown>> }
  | { readonly kind: "resync" }
  | { readonly kind: "retry" }
  | { readonly kind: "stop" };

/**
 * Events by REST polling (`pollAgentEvents`), acknowledged with `ackAgentEvents`.
 *
 * It polls again at once after a non-empty page and waits `sdk.poll_interval` after an empty
 * one; it sends the agent heartbeat with `sendHeartbeat` every `HeartbeatAck.interval_ms` (the
 * first at once, `sdk.poll_interval` until an answer names the interval). A failed poll waits
 * with full jitter between zero and `min(sync.reconnect_backoff_cap,
 * sync.reconnect_backoff_base * 2^n)`; `401` and `403` stop it for good.
 */
export class PollingSource implements EventSource {
  /** The REST client. */
  readonly client: ApiClient;
  /** The consumer name. */
  readonly consumer: string;
  /** The SDK settings. */
  readonly settings: SdkSettings;
  /** The time source. */
  readonly clock: Clock;
  /** The acknowledgement window. */
  readonly window = new AckWindow();
  /** Why the source stopped, once it has. */
  closeReason: CloseReason | undefined;
  private readonly heartbeat: (() => S["Heartbeat"] | undefined) | undefined;
  private readonly limit: number | undefined;
  private readonly random: () => number;
  private readonly logger: Logger;
  private readonly stopped = new AbortController();
  private readonly serial = new Serial();

  /**
   * Creates the source; nothing is sent before iterating.
   *
   * @param client - The REST client of the agent.
   * @param consumer - The consumer name.
   * @param options - Settings, clock, heartbeat source, page size, random and logger.
   */
  constructor(
    client: ApiClient,
    consumer: string,
    options: PollingSourceOptions = {},
  ) {
    this.client = client;
    this.consumer = consumer;
    this.settings = { ...client.settings, ...options.settings };
    this.clock = options.clock ?? client.clock;
    this.heartbeat = options.heartbeat;
    this.limit = options.limit;
    this.random = options.random ?? cryptoRandom;
    this.logger = options.logger ?? client.logger;
  }

  /**
   * Polls until {@link PollingSource.close} is called.
   *
   * @returns The deliveries.
   */
  async *[Symbol.asyncIterator](): AsyncGenerator<Delivery, void, undefined> {
    const beats = this.heartbeat === undefined ? undefined : this.beat();
    let failures = 0;
    try {
      yield new SourceReady();
      while (!this.stopped.signal.aborted) {
        const outcome = await this.poll();
        if (outcome.kind === "stop") return;
        if (outcome.kind === "resync") {
          this.window.reset();
          yield new SourceReady({ resync: true });
          continue;
        }
        if (outcome.kind === "retry") {
          await this.wait(this.backoff(failures));
          failures++;
          continue;
        }
        failures = 0;
        const events = outcome.page.events;
        for (const envelope of pageEnvelopes(events)) {
          if (envelope.seq !== undefined) this.window.delivered(envelope.seq);
          yield envelope;
        }
        const cursor = outcome.page.next_cursor;
        if (typeof cursor === "number" && Number.isSafeInteger(cursor))
          yield new CursorAdvance(cursor);
        if (!Array.isArray(events) || events.length === 0)
          await this.wait(this.settings.pollIntervalMs);
      }
    } finally {
      this.stopped.abort();
      await beats;
      this.closeReason ??= new CloseReason(1000, "polling stopped");
    }
  }

  /** One `pollAgentEvents`: a page, `resync`, `retry` or `stop`. */
  private async poll(): Promise<PollOutcome> {
    try {
      const page = await unwrap(
        this.client.GET("/agents/me/consumers/{consumer}/events", {
          params: {
            path: { consumer: this.consumer },
            ...(this.limit === undefined
              ? {}
              : { query: { limit: this.limit } }),
          },
        }),
        "pollAgentEvents",
      );
      return typeof page === "object" && page !== null
        ? { kind: "page", page: page as Readonly<Record<string, unknown>> }
        : { kind: "retry" };
    } catch (error) {
      if (error instanceof GoneError) {
        // 410 resync_required: the server moved the cursor to head_seq.
        return { kind: "resync" };
      }
      if (
        error instanceof UnauthenticatedError ||
        error instanceof ForbiddenError
      ) {
        // The API token is invalid or revoked: only the owner can fix that.
        this.logger.error("polling stopped", { code: error.code });
        this.closeReason = new CloseReason(4004, error.code);
        return { kind: "stop" };
      }
      // A server restart or an outage outlasted the retries: wait with full jitter between zero
      // and sync.reconnect_backoff_cap, then poll again.
      this.logger.info("poll failed; polling again", {
        error: error instanceof Error ? error.name : "unknown",
      });
      return { kind: "retry" };
    }
  }

  private backoff(attempt: number): number {
    const cap = Math.min(
      this.settings.reconnectBackoffCapMs,
      this.settings.reconnectBackoffBaseMs * 2 ** attempt,
    );
    return this.random() * cap;
  }

  /** Sleeps `ms` or until the source stops. */
  private async wait(ms: number): Promise<void> {
    try {
      await this.clock.sleep(ms, this.stopped.signal);
    } catch {
      // Stopped.
    }
  }

  private async beat(): Promise<void> {
    let interval = this.settings.pollIntervalMs;
    while (!this.stopped.signal.aborted) {
      const body = this.heartbeat?.();
      if (body !== undefined) {
        try {
          const ack = await unwrap(
            this.client.POST("/agents/me/heartbeat", { body }),
            "sendHeartbeat",
          );
          interval = ack.interval_ms;
        } catch {
          // A failed heartbeat is retried in the next interval.
          this.logger.info("heartbeat failed; retrying in the next interval");
        }
      }
      await this.wait(interval);
    }
  }

  /**
   * Marks `seq` settled and calls `ackAgentEvents` when the acknowledgement point moves.
   *
   * @param seq - The settled `seq`.
   * @returns A promise that resolves after a needed acknowledgement.
   */
  settle(seq: number): Promise<void> {
    return settleWith(
      this.serial,
      () => this.window.settle(seq),
      (s) => this.ack(s),
    );
  }

  /**
   * Passes on a cursor; it never settles an open delivered event.
   *
   * @param seq - The cursor.
   * @returns A promise that resolves after a needed acknowledgement.
   */
  advance(seq: number): Promise<void> {
    return settleWith(
      this.serial,
      () => this.window.cursor(seq),
      (s) => this.ack(s),
    );
  }

  /**
   * Calls `ackAgentEvents`; a failure is logged, the next acknowledgement covers it.
   *
   * @param seq - The highest settled `seq`.
   * @returns A promise that resolves after the call, also when it failed.
   */
  async ack(seq: number): Promise<void> {
    try {
      await unwrap(
        this.client.POST("/agents/me/consumers/{consumer}/ack", {
          params: { path: { consumer: this.consumer } },
          body: { seq },
        }),
        "ackAgentEvents",
      );
    } catch (error) {
      if (!(error instanceof ProblemError || error instanceof TransportError))
        throw error;
      // ack is cumulative: a later one covers this seq; without one the server delivers again
      // after agent.ack_timeout and the inbox settles the duplicate.
      this.logger.info("ack failed", { seq, error: error.name });
    }
  }

  /**
   * Calls `answerAutocomplete`.
   *
   * @param requestId - The request.
   * @param choices - The choices.
   * @returns A promise that resolves after the answer.
   * @throws `ProblemError` or `TransportError` when the call fails.
   */
  async answerAutocomplete(
    requestId: string,
    choices: readonly S["CommandChoice"][],
  ): Promise<void> {
    await unwrap(
      this.client.POST("/command-autocompletions/{request_id}/answer", {
        params: { path: { request_id: requestId } },
        body: { choices: [...choices] },
      }),
      "answerAutocomplete",
    );
  }

  /**
   * Stops polling after the current request.
   *
   * @returns A resolved promise.
   */
  close(): Promise<void> {
    this.stopped.abort();
    return Promise.resolve();
  }
}
