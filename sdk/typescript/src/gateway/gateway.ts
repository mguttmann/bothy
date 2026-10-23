// SPDX-License-Identifier: Apache-2.0
/**
 * Gateway connection of an agent after docs/spec/04-sync-protocol.md 6 (port of
 * `bothy_sdk/gateway.py`).
 *
 * {@link Gateway} opens the WebSocket of `Discovery.gateway.url`, answers `hello` with
 * `identify` (API token, DPoP proof with the nonce of `hello`, consumer, protocol version) or
 * `resume` within the resume window, sends the heartbeat (the first after a random part of the
 * interval, then every interval, with the agent heartbeat as member `agent`), closes and
 * reconnects when a `heartbeat_ack` is missing for one interval, and reconnects with full jitter
 * between zero and `min(sync.reconnect_backoff_cap, sync.reconnect_backoff_base * 2^n)`. Client
 * frames are built from the generated `Gateway*` types; server frames are read with them, a
 * JSON `null` member counts as absent, unknown ops and members are skipped.
 *
 * Close codes for agents (docs/DECISIONS.md D-M2-29-5, after 04 6.4):
 *
 * - stop, no reconnect: `1000` (own close), `4004`, `4007`, `4011`, `4012`, `4015`, `4016`,
 *   `4017`, `4018` (an agent cannot renew its API token; only the owner issues one);
 * - `resume` after backoff: `1001`, `4000`, `4009`, `4013` (first try after a random part of
 *   `sync.reconnect_backoff_base`), `4014`;
 * - `identify` on `4006`;
 * - `4008`: wait the `retry_after` seconds of the close reason, else backoff;
 * - log and reconnect: `4001`, `4002`, `4003`, `4005` and unknown codes.
 *
 * Node.js only: the frames carry the API token.
 *
 * @example
 * ```ts
 * const gw = new Gateway(gatewayUrl(baseUrl, discovery.gateway.url), credentials, {
 *   consumer: "default",
 *   agentId,
 * });
 * for await (const frame of gw.frames()) if (frame.op === "dispatch") handle(frame.data);
 * ```
 *
 * @module
 */

import { randomInt } from "node:crypto";

import { createDpopProof, gatewayHtu, requireSafeUrl } from "../auth/dpop.js";
import type { Credentials } from "../auth/types.js";
import { type Clock, systemClock } from "../clock.js";
import { type SdkSettings, resolveSettings } from "../config.js";
import { GatewayOpError, TransportError } from "../errors.js";
import type { components } from "../generated/openapi.js";
import { type Logger, defaultLogger } from "../logger.js";

type S = components["schemas"];

/** Gateway protocol versions this SDK speaks (`info.x-protocol-versions` of asyncapi.yaml). */
export const PROTOCOL_VERSIONS: readonly number[] = Object.freeze([1]);

/** Close codes after which an agent stops without reconnecting. */
export const STOP_ON: ReadonlySet<number> = new Set([
  1000, 4004, 4007, 4011, 4012, 4015, 4016, 4017, 4018,
]);
/** Close codes after which the gateway resumes after backoff. */
export const RESUME_ON: ReadonlySet<number> = new Set([
  1001, 4000, 4009, 4013, 4014,
]);
/** Close codes after which the gateway identifies anew. */
export const IDENTIFY_ON: ReadonlySet<number> = new Set([4006]);

const RETRY_AFTER = /retry_after=(\d+)/;
const OWN_CLOSE = 1000;
const HEARTBEAT_TIMEOUT = 1001;
const ABNORMAL = 1006;
const FRAME_TOO_LARGE = 1009;
const SERVER_RESTART = 4013;
const RATE_LIMITED = 4008;
const PRINCIPAL_MISMATCH = "principal_mismatch";
const OWN_REASON = "closed by the adapter";
const CONNECTING = 0;
const OPEN = 1;
// randomInt needs a range below 2^48.
const RANDOM_RANGE = 2 ** 47;

/**
 * Why a gateway (or an adapter) stopped: the close code and its reason text.
 *
 * @example
 * ```ts
 * new CloseReason(4015, "consumer_replaced").permanent; // true
 * ```
 */
export class CloseReason {
  /** The WebSocket close code (`1000` for an own stop). */
  readonly code: number;
  /** The close reason the server sent, or an own reason. */
  readonly reason: string;

  /**
   * Creates the reason.
   *
   * @param code - The close code.
   * @param reason - The reason text.
   */
  constructor(code: number, reason = "") {
    this.code = code;
    this.reason = reason;
  }

  /** Whether an agent reconnects after this code. */
  get reconnects(): boolean {
    return !STOP_ON.has(this.code);
  }

  /** Whether the stop is final: the code is in {@link STOP_ON} (the brief's `permanent`). */
  get permanent(): boolean {
    return STOP_ON.has(this.code);
  }
}

/** One server frame that matters to an event source. */
export class GatewayEvent {
  /** `dispatch`, `cursor`, `ready`, `resumed`, `resync_required` or `error`. */
  readonly op: string;
  /** The `d` member without `null` members (the envelope of a dispatch). */
  readonly data: Readonly<Record<string, unknown>>;
  /**
   * The frame read as its generated type (`GatewayReady`, `GatewayResumed`, `GatewayCursor`,
   * `GatewayResyncRequired`), a {@link GatewayOpError} for `error`, `undefined` for `dispatch`.
   */
  readonly model: unknown;

  /**
   * Creates the event.
   *
   * @param op - The op.
   * @param data - The `d` member.
   * @param model - The typed reading of the frame, if any.
   */
  constructor(
    op: string,
    data: Readonly<Record<string, unknown>>,
    model?: unknown,
  ) {
    this.op = op;
    this.data = data;
    this.model = model;
  }
}

/**
 * The part of a WebSocket the gateway uses: the WHATWG interface of the global `WebSocket` of
 * Node.js, which the `ws` package offers too.
 */
export interface GatewaySocket {
  /** `0` connecting, `1` open, `2` closing, `3` closed. */
  readonly readyState: number;
  /**
   * Sends a text frame.
   *
   * @param data - The frame.
   */
  send(data: string): void;
  /**
   * Starts the closing handshake.
   *
   * @param code - `1000` or a code from `3000` to `4999` (the WHATWG rule).
   * @param reason - The reason text.
   */
  close(code?: number, reason?: string): void;
  /**
   * Registers a listener for `open`, `message`, `close` or `error`.
   *
   * @param type - The event type.
   * @param listener - Receives the event (`data` of a message, `code` and `reason` of a close).
   */
  addEventListener(type: string, listener: (event: unknown) => void): void;
}

/** A constructor of {@link GatewaySocket}, for example the global `WebSocket` or `ws`. */
export type GatewaySocketConstructor = new (
  url: string,
  init?: { headers: Record<string, string> },
) => GatewaySocket;

/** The options of {@link Gateway}. */
export interface GatewayOptions {
  /** The consumer name (`^[a-z0-9][a-z0-9_-]*$`). */
  readonly consumer: string;
  /** The agent's principal ID; `ready` for another principal stops the gateway. */
  readonly agentId: string;
  /** Overrides of the SDK settings (reconnect backoff, frame size, connect timeout). */
  readonly settings?: Partial<SdkSettings> | undefined;
  /** The time source of heartbeats and waits (default `systemClock`). */
  readonly clock?: Clock | undefined;
  /** The jitter source in [0, 1) (default from `node:crypto`). */
  readonly random?: (() => number) | undefined;
  /** Returns the agent heartbeat for the member `agent` (or `undefined` for none). */
  readonly heartbeat?: (() => S["Heartbeat"] | undefined) | undefined;
  /** The `User-Agent` of the upgrade request; absent leaves the WebSocket default. */
  readonly userAgent?: string | undefined;
  /** Receives each DPoP nonce of `hello` (the REST client may share it). */
  readonly nonceSink?: ((nonce: string | undefined) => void) | undefined;
  /** The WebSocket constructor (default the global `WebSocket`). */
  readonly WebSocket?: GatewaySocketConstructor | undefined;
  /** The logger (default {@link defaultLogger}). */
  readonly logger?: Logger | undefined;
}

/**
 * Resolves `Discovery.gateway.url` against the API base and turns it into a WebSocket URL.
 *
 * @param baseUrl - The API base, for example `https://example.com/api/v1`.
 * @param url - The gateway URL of the discovery (absolute, or relative to the base).
 * @returns The `ws` or `wss` URL without fragment.
 * @throws `TypeError` when the result would use clear text off the loopback host.
 *
 * @example
 * ```ts
 * gatewayUrl("https://example.com/api/v1", "/api/v1/gateway"); // "wss://example.com/api/v1/gateway"
 * ```
 */
export function gatewayUrl(baseUrl: string, url: string): string {
  const target = new URL(url, baseUrl);
  if (target.protocol === "http:") target.protocol = "ws:";
  else if (target.protocol === "https:") target.protocol = "wss:";
  target.hash = "";
  const out = target.toString();
  requireSafeUrl(out);
  return out;
}

function cryptoRandom(): number {
  return randomInt(RANDOM_RANGE) / RANDOM_RANGE;
}

function isObject(value: unknown): value is Record<string, unknown> {
  return typeof value === "object" && value !== null && !Array.isArray(value);
}

// JSON.parse reviver: a member whose value is null is dropped (returning undefined deletes it);
// null items of an array stay, because only members can be absent (04 15.1).
function withoutNullMembers(
  this: unknown,
  _key: string,
  value: unknown,
): unknown {
  return value === null && !Array.isArray(this) ? undefined : value;
}

/** Reads one text frame into its op and `d`; anything else is skipped. */
function decode(
  raw: unknown,
): [string, Readonly<Record<string, unknown>>] | undefined {
  if (typeof raw !== "string") return undefined;
  let frame: unknown;
  try {
    frame = JSON.parse(raw, withoutNullMembers);
  } catch {
    return undefined;
  }
  if (!isObject(frame) || typeof frame.op !== "string") return undefined;
  return [frame.op, isObject(frame.d) ? frame.d : {}];
}

function isHello(d: Readonly<Record<string, unknown>>): boolean {
  return (
    typeof d.heartbeat_interval_ms === "number" &&
    d.heartbeat_interval_ms > 0 &&
    typeof d.resume_window_ms === "number" &&
    Array.isArray(d.protocol_versions)
  );
}

/** One connection: its socket, the events read from it and how it ended. */
class Link {
  readonly beats = new AbortController();
  private readonly events: GatewayEvent[] = [];
  private wake: (() => void) | undefined;
  private ended = false;
  /** The pending frame work, in arrival order. */
  chain: Promise<void> = Promise.resolve();
  code: number | undefined;
  reason = "";
  /** The code an own close stands for (`1001` heartbeat timeout, `1006`, `1009`). */
  localCode: number | undefined;
  opened = false;

  private closeTimer: ReturnType<typeof setTimeout> | undefined;

  /**
   * @param socket - The socket.
   * @param closeTimeoutMs - How long an own close waits for the peer's answer
   *   (`sdk.http_timeout`); a half-open connection never answers.
   */
  constructor(
    readonly socket: GatewaySocket,
    private readonly closeTimeoutMs: number,
  ) {}

  push(event: GatewayEvent): void {
    this.events.push(event);
    this.notify();
  }

  end(code: number | undefined, reason: string): void {
    if (this.ended) return;
    clearTimeout(this.closeTimer);
    this.code = code;
    this.reason = reason;
    this.ended = true;
    this.beats.abort();
    this.notify();
  }

  /** The next event, or `undefined` once the connection ended and every event was read. */
  async next(): Promise<GatewayEvent | undefined> {
    for (;;) {
      const event = this.events.shift();
      if (event !== undefined) return event;
      if (this.ended) return undefined;
      await new Promise<void>((resolve) => (this.wake = resolve));
    }
  }

  /** Closes with `1000` (the only code below `3000` a WHATWG WebSocket may send). */
  close(reason: string, localCode?: number): void {
    if (localCode !== undefined) this.localCode ??= localCode;
    const state = this.socket.readyState;
    if (state === CONNECTING || state === OPEN)
      this.socket.close(OWN_CLOSE, reason);
    if (this.ended || this.closeTimer !== undefined) return;
    // The WHATWG WebSocket waits for the peer's close frame without a limit; end the
    // connection locally after sdk.http_timeout, so a dead peer cannot stop the reconnect.
    this.closeTimer = setTimeout(() => {
      this.localCode ??= ABNORMAL;
      this.end(undefined, reason);
    }, this.closeTimeoutMs);
    this.closeTimer.unref();
  }

  private notify(): void {
    const wake = this.wake;
    this.wake = undefined;
    wake?.();
  }
}

/**
 * One agent's gateway session with reconnects, as an async iterator of server frames.
 *
 * Nothing connects before {@link Gateway.frames} is iterated. Control frames (`hello`,
 * `heartbeat_ack`) are handled when they arrive; the other frames wait for the consumer in
 * arrival order.
 */
export class Gateway {
  /** The WebSocket URL ({@link gatewayUrl}). */
  readonly url: string;
  /** The consumer name. */
  readonly consumer: string;
  /** The agent's principal ID. */
  readonly agentId: string;
  /** The resolved SDK settings. */
  readonly settings: SdkSettings;
  /** The time source. */
  readonly clock: Clock;
  /** Why the gateway stopped, once it has. */
  closeReason: CloseReason | undefined;
  /** The number of opened connections. */
  connects = 0;
  private readonly credentials: Credentials;
  private readonly random: () => number;
  private readonly heartbeat: (() => S["Heartbeat"] | undefined) | undefined;
  private readonly userAgent: string | undefined;
  private readonly nonceSink: ((nonce: string | undefined) => void) | undefined;
  private readonly socketConstructor: GatewaySocketConstructor;
  private readonly logger: Logger;
  private readonly stop = new AbortController();
  private link: Link | undefined;
  private session: string | undefined;
  private resumeWindowMs = 0;
  private disconnectedAt: number | undefined;
  private identifyNext = false;
  private seq = 0;
  private ackPending = false;
  private closing = false;
  private attempt = 0;

  /**
   * Creates the gateway; nothing connects before {@link Gateway.frames} is iterated.
   *
   * @param url - The WebSocket URL ({@link gatewayUrl}).
   * @param credentials - The agent's API token and optional DPoP key.
   * @param options - Consumer, agent ID, settings, clock, random, heartbeat source, User-Agent,
   *   nonce sink, WebSocket constructor and logger.
   * @throws `TypeError` when `url` uses clear text off the loopback host.
   */
  constructor(url: string, credentials: Credentials, options: GatewayOptions) {
    requireSafeUrl(url);
    this.url = url;
    this.credentials = credentials;
    this.consumer = options.consumer;
    this.agentId = options.agentId.toLowerCase();
    this.settings = resolveSettings(options.settings);
    this.clock = options.clock ?? systemClock;
    this.random = options.random ?? cryptoRandom;
    this.heartbeat = options.heartbeat;
    this.userAgent = options.userAgent;
    this.nonceSink = options.nonceSink;
    this.socketConstructor =
      options.WebSocket ??
      (globalThis.WebSocket as unknown as GatewaySocketConstructor);
    this.logger = options.logger ?? defaultLogger;
  }

  /** The gateway session of the last `ready`, if any. */
  get sessionId(): string | undefined {
    return this.session;
  }

  /** The highest acknowledged `seq` (the cursor of a `resume`). */
  get lastSeq(): number {
    return this.seq;
  }

  /**
   * Acknowledges every event up to `seq` (op `ack`, cumulative); without a connection the
   * acknowledgement is dropped, the next one covers it.
   *
   * @param seq - The highest settled `seq`.
   * @returns A promise that resolves once the frame is handed to the socket.
   */
  ack(seq: number): Promise<void> {
    // No local deduplication: after a reconnect the server's cursor may lie below ours, and an
    // ack at or below the server's cursor changes nothing there (04 15.2).
    this.seq = Math.max(this.seq, seq);
    try {
      this.send("ack", { seq } satisfies S["GatewayAck"]);
    } catch {
      // Not connected: the server delivers again after a reconnect and the inbox settles it.
    }
    return Promise.resolve();
  }

  /**
   * Sends the op `run.update` (no success reply; a failure comes as an `error` frame).
   *
   * @param runId - The run.
   * @param baseVersion - The version the patch is based on.
   * @param ops - The RFC 6902 operations.
   * @param ref - A reference the `error` frame repeats.
   * @returns A promise that resolves once the frame is handed to the socket.
   * @throws `TransportError` when the gateway is not connected.
   */
  sendRunUpdate(
    runId: string,
    baseVersion: number,
    ops: readonly S["JsonPatchOperation"][],
    ref?: string,
  ): Promise<void> {
    const frame: S["GatewayRunUpdate"] = {
      run_id: runId,
      base_version: baseVersion,
      ops: [...ops],
    };
    if (ref !== undefined) frame.ref = ref;
    return this.sendAsync("run.update", frame);
  }

  /**
   * Sends the op `command.autocomplete_result`.
   *
   * @param requestId - The request of the `command.autocomplete_requested` event.
   * @param choices - The choices.
   * @param ref - A reference the `error` frame repeats.
   * @returns A promise that resolves once the frame is handed to the socket.
   * @throws `TransportError` when the gateway is not connected.
   */
  sendAutocompleteResult(
    requestId: string,
    choices: readonly S["CommandChoice"][],
    ref?: string,
  ): Promise<void> {
    const frame: S["GatewayAutocompleteResult"] = {
      request_id: requestId,
      choices: [...choices],
    };
    if (ref !== undefined) frame.ref = ref;
    return this.sendAsync("command.autocomplete_result", frame);
  }

  /**
   * Closes with `1000`; {@link Gateway.frames} ends without reconnecting.
   *
   * @returns A promise that resolves once the close is started.
   */
  close(): Promise<void> {
    this.closing = true;
    this.closeReason ??= new CloseReason(OWN_CLOSE, OWN_REASON);
    this.stop.abort();
    this.link?.close(OWN_REASON);
    return Promise.resolve();
  }

  /**
   * Returns the wait before reconnect attempt `attempt`: uniform in
   * [0, min(`sync.reconnect_backoff_cap`, `sync.reconnect_backoff_base` * 2^attempt)].
   *
   * @param attempt - The attempt, from zero; `ready` and `resumed` reset it.
   * @returns The wait in milliseconds.
   */
  backoff(attempt: number): number {
    const cap = Math.min(
      this.settings.reconnectBackoffCapMs,
      this.settings.reconnectBackoffBaseMs * 2 ** attempt,
    );
    return this.random() * cap;
  }

  /**
   * Connects, keeps the session alive and yields every relevant server frame.
   *
   * The iterator reconnects after every close code that allows it and ends when the gateway
   * stops; {@link Gateway.closeReason} then tells why. Breaking out of the loop closes the
   * gateway.
   *
   * @returns The server frames `ready`, `resumed`, `dispatch`, `cursor`, `resync_required` and
   *   `error` (with a {@link GatewayOpError} as model).
   */
  async *frames(): AsyncGenerator<GatewayEvent, void, undefined> {
    this.attempt = 0;
    try {
      while (!this.closing) {
        const link = this.open();
        if (link !== undefined) {
          for (;;) {
            const event = await link.next();
            if (event === undefined) break;
            yield event;
            if (this.closing) break;
          }
          this.endConnection(link);
        }
        if (this.closing) break;
        const code = link?.localCode ?? link?.code ?? ABNORMAL;
        const wait = this.afterClose(code, link?.reason ?? "", this.attempt);
        if (wait === undefined) break;
        this.attempt++;
        try {
          await this.clock.sleep(wait, this.stop.signal);
        } catch {
          break;
        }
      }
    } finally {
      // A consumer that leaves the loop stops the gateway like an own close.
      await this.close();
      const link = this.link;
      if (link !== undefined) this.endConnection(link);
    }
  }

  private open(): Link | undefined {
    let socket: GatewaySocket;
    try {
      socket =
        this.userAgent === undefined
          ? new this.socketConstructor(this.url)
          : new this.socketConstructor(this.url, {
              headers: { "User-Agent": this.userAgent },
            });
    } catch (error) {
      this.logger.info("gateway connect failed", {
        error: error instanceof Error ? error.name : "unknown",
      });
      return undefined;
    }
    const link = new Link(socket, this.settings.httpTimeoutMs);
    this.link = link;
    // sdk.http_timeout bounds the opening handshake; a real-time timer, like a socket timeout.
    const timer = setTimeout(() => {
      if (!link.opened) link.close("open timeout", ABNORMAL);
    }, this.settings.httpTimeoutMs);
    timer.unref();
    socket.addEventListener("open", () => {
      link.opened = true;
      clearTimeout(timer);
      this.connects++;
    });
    socket.addEventListener("message", (event: unknown) => {
      const data = (event as { data?: unknown }).data;
      link.chain = link.chain
        .then(() => this.handle(link, data))
        .catch((error: unknown) => {
          this.logger.info("gateway frame failed", {
            error: error instanceof Error ? error.name : "unknown",
          });
        });
    });
    socket.addEventListener("close", (event: unknown) => {
      clearTimeout(timer);
      const { code, reason } = event as { code?: unknown; reason?: unknown };
      link.chain = link.chain.then(() =>
        link.end(
          typeof code === "number" ? code : undefined,
          typeof reason === "string" ? reason : "",
        ),
      );
    });
    // A failed connection also fires `close` (code 1006); the error itself carries nothing more.
    socket.addEventListener("error", () => undefined);
    return link;
  }

  private endConnection(link: Link): void {
    link.beats.abort();
    link.close(OWN_REASON);
    if (this.link === link) {
      this.link = undefined;
      this.ackPending = false;
      this.disconnectedAt = this.clock.monotonic();
    }
  }

  /** Returns the wait before the next connect, or `undefined` to stop (D-M2-29-5). */
  private afterClose(
    code: number,
    reason: string,
    attempt: number,
  ): number | undefined {
    this.logger.info("gateway closed", { code });
    if (STOP_ON.has(code)) {
      this.closeReason = new CloseReason(code, reason);
      return undefined;
    }
    if (IDENTIFY_ON.has(code)) this.identifyNext = true;
    if (code === SERVER_RESTART && attempt === 0)
      return this.random() * this.settings.reconnectBackoffBaseMs;
    if (code === RATE_LIMITED) {
      const match = RETRY_AFTER.exec(reason);
      if (match !== null) return Number(match[1]) * 1_000;
    }
    return this.backoff(attempt);
  }

  private send(op: string, data: object): void {
    const link = this.link;
    if (link === undefined || link.socket.readyState !== OPEN)
      throw new TransportError(`gateway op ${op}`, {}, "gateway not connected");
    link.socket.send(JSON.stringify({ op, d: data }));
  }

  private sendAsync(op: string, data: object): Promise<void> {
    try {
      this.send(op, data);
      return Promise.resolve();
    } catch (error) {
      return Promise.reject(error as Error);
    }
  }

  private async handle(link: Link, raw: unknown): Promise<void> {
    if (
      typeof raw === "string" &&
      Buffer.byteLength(raw, "utf8") > this.settings.wsMaxFrameBytes
    ) {
      // sync.ws_max_frame_bytes: a larger frame ends the connection like a protocol error.
      link.close("frame_too_large", FRAME_TOO_LARGE);
      return;
    }
    const frame = decode(raw);
    if (frame === undefined) {
      this.logger.info("gateway: undecodable frame skipped");
      return;
    }
    const [op, data] = frame;
    switch (op) {
      case "hello":
        await this.onHello(link, data);
        return;
      case "ready":
        this.onReady(link, data);
        return;
      case "heartbeat_ack":
        this.ackPending = false;
        return;
      case "resync_required":
        this.identifyNext = true;
        this.session = undefined;
        link.push(
          new GatewayEvent(op, data, data as S["GatewayResyncRequired"]),
        );
        return;
      case "error": {
        const detail =
          typeof data.detail === "string" ? data.detail : undefined;
        const ref = typeof data.ref === "string" ? data.ref : undefined;
        const error = new GatewayOpError(
          typeof data.op === "string" ? data.op : "",
          typeof data.code === "string" ? data.code : "",
          { detail, ref },
        );
        link.push(new GatewayEvent(op, data, error));
        return;
      }
      case "dispatch":
        link.push(new GatewayEvent(op, data));
        return;
      case "resumed":
        this.attempt = 0;
        link.push(new GatewayEvent(op, data, data as S["GatewayResumed"]));
        return;
      case "cursor":
        link.push(new GatewayEvent(op, data, data as S["GatewayCursor"]));
        return;
      case "catchup.complete":
        return;
      default:
        this.logger.info("gateway: unknown op skipped", { op });
    }
  }

  private async onHello(
    link: Link,
    data: Readonly<Record<string, unknown>>,
  ): Promise<void> {
    if (!isHello(data)) {
      this.logger.info("gateway: malformed hello; reconnecting");
      link.close("decode_error", ABNORMAL);
      return;
    }
    const hello = data as S["GatewayHello"];
    this.resumeWindowMs = hello.resume_window_ms;
    const nonce =
      typeof hello.dpop_nonce === "string" ? hello.dpop_nonce : undefined;
    this.nonceSink?.(nonce);
    const [name, payload] = await this.identifyFrame(hello, nonce);
    if (link !== this.link || link.socket.readyState !== OPEN) return;
    link.socket.send(JSON.stringify({ op: name, d: payload }));
    void this.beat(link, hello.heartbeat_interval_ms);
  }

  private async identifyFrame(
    hello: S["GatewayHello"],
    nonce: string | undefined,
  ): Promise<[string, S["GatewayIdentify"] | S["GatewayResume"]]> {
    const token = this.credentials.apiToken;
    const dpop = this.credentials.dpop;
    const proof =
      dpop === undefined
        ? undefined
        : await createDpopProof(dpop, {
            htm: "GET",
            htu: gatewayHtu(this.url),
            accessToken: token,
            nonce,
            now: this.clock.now(),
          });
    const common = hello.protocol_versions.filter((v) =>
      PROTOCOL_VERSIONS.includes(v),
    );
    const version =
      common.length > 0
        ? Math.max(...common)
        : (PROTOCOL_VERSIONS[PROTOCOL_VERSIONS.length - 1] ?? 1);
    const resumeOk =
      !this.identifyNext &&
      this.session !== undefined &&
      this.disconnectedAt !== undefined &&
      this.clock.monotonic() - this.disconnectedAt < this.resumeWindowMs;
    if (resumeOk && this.session !== undefined) {
      const resume: S["GatewayResume"] = {
        gateway_session_id: this.session,
        cursor: this.seq,
        api_token: token.reveal(),
      };
      if (proof !== undefined) resume.dpop = proof;
      return ["resume", resume];
    }
    const identify: S["GatewayIdentify"] = {
      protocol_version: version,
      api_token: token.reveal(),
      consumer: this.consumer,
    };
    if (proof !== undefined) identify.dpop = proof;
    return ["identify", identify];
  }

  private onReady(link: Link, data: Readonly<Record<string, unknown>>): void {
    const ready = data as S["GatewayReady"];
    const principal = isObject(ready.principal)
      ? ready.principal.id
      : undefined;
    if (
      typeof principal !== "string" ||
      principal.toLowerCase() !== this.agentId
    ) {
      this.logger.error("gateway ready for another principal; stopping");
      this.closeReason = new CloseReason(OWN_CLOSE, PRINCIPAL_MISMATCH);
      this.closing = true;
      this.stop.abort();
      link.close(PRINCIPAL_MISMATCH);
      return;
    }
    this.session =
      typeof ready.gateway_session_id === "string"
        ? ready.gateway_session_id
        : undefined;
    this.identifyNext = false;
    this.attempt = 0;
    link.push(new GatewayEvent("ready", data, ready));
  }

  /** Heartbeat loop of one connection (04 6.3): random first delay, then every interval. */
  private async beat(link: Link, intervalMs: number): Promise<void> {
    const signal = link.beats.signal;
    try {
      await this.clock.sleep(this.random() * intervalMs, signal);
      for (;;) {
        if (signal.aborted) return;
        if (this.ackPending) {
          this.logger.info("gateway: heartbeat_ack missing; reconnecting");
          link.close("heartbeat_ack missing", HEARTBEAT_TIMEOUT);
          return;
        }
        const beat: S["GatewayHeartbeat"] = { last_seq: this.seq };
        const agent = this.heartbeat?.();
        if (agent !== undefined) beat.agent = agent;
        if (link.socket.readyState !== OPEN) return;
        this.ackPending = true;
        link.socket.send(JSON.stringify({ op: "heartbeat", d: beat }));
        await this.clock.sleep(intervalMs, signal);
      }
    } catch {
      // Aborted at the end of the connection.
    }
  }
}
