// SPDX-License-Identifier: Apache-2.0
/**
 * The FakeServer's gateway for agents (private; port of `bothy_sdk/testing/_fake_gateway.py`).
 *
 * Frames are `{"op": ..., "d": ...}` (docs/api/asyncapi.yaml, channel `gateway`;
 * docs/spec/04-sync-protocol.md 6): `hello`, `identify` or `resume` with the API token and a
 * DPoP proof (`htm = GET`, `htu` the HTTPS form of the gateway URL as the server builds it from
 * its public URL, the nonce of `hello`), `ready`, the consumer's dispatches with
 * `invocation_token` and `redelivered`, `cursor`, `catchup.complete`, `heartbeat_ack`, `error`
 * frames for failing ops and the close codes of the server (`4015` for a replaced consumer,
 * `4013` on a restart). This is the only module of the SDK that imports `ws`.
 *
 * @module
 */

import type { IncomingMessage } from "node:http";
import type { Duplex } from "node:stream";

import { WebSocket, WebSocketServer, type RawData } from "ws";

import { DEFAULTS } from "../../config.js";
import type { components } from "../../generated/openapi.js";
import { uuidv7 } from "../../ids.js";
import { canonicalUuid, FakeProblem, isPlainObject } from "./base.js";
import type { Authenticator } from "./dpop.js";
import type { Close, Connection, Hub, OutFrame, Send } from "./hub.js";
import { consumerOk, type World } from "./world.js";

type S = components["schemas"];

const REGISTERED = new Set([
  "ack",
  "focus",
  "heartbeat",
  "identify",
  "reauth",
  "resume",
  "run.update",
  "typing.start",
  "typing.stop",
  "command.autocomplete_result",
]);
// Close codes after which a session stays resumable within sync.resume_window.
const RESUMABLE = new Set([
  1000, 1001, 4000, 4001, 4002, 4009, 4011, 4013, 4014,
]);
const IDENTIFY = new Set([
  "protocol_version",
  "access_token",
  "api_token",
  "dpop",
  "device_id",
  "consumer",
  "cursor",
  "sync_epoch",
  "client",
]);
const RESUME = new Set([
  "gateway_session_id",
  "access_token",
  "api_token",
  "dpop",
  "cursor",
  "sync_epoch",
]);
const UUID = /^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$/;

/** A control-flow signal: close the socket with `code`. */
class CloseSignal extends Error {
  constructor(
    readonly code: number,
    readonly reason = "",
  ) {
    super(String(code));
  }
}

/** A control-flow signal of the disconnect fault: drop the socket without a close frame. */
class Dropped extends Error {}

/** A gateway session that `resume` can continue. */
interface GatewaySession {
  /** The consumer of the session. */
  readonly consumer: string;
  /** When the owning connection ended (clock monotonic), if it did. */
  detachedAt: number | undefined;
  /** The connection that owns the session now. */
  owner: Connection | undefined;
}

/** The per-socket state of {@link Gateway.handle}. */
interface SocketState {
  conn: Connection | undefined;
  sentCode: number | undefined;
}

function isInt(value: unknown, minimum = 0): value is number {
  return (
    typeof value === "number" && Number.isInteger(value) && value >= minimum
  );
}

function hasOnly(
  d: Record<string, unknown>,
  allowed: ReadonlySet<string>,
): boolean {
  return Object.keys(d).every((k) => allowed.has(k));
}

function text(data: RawData): Buffer {
  if (Buffer.isBuffer(data)) return data;
  return Array.isArray(data) ? Buffer.concat(data) : Buffer.from(data);
}

/** The parts the gateway needs from the FakeServer. */
export interface GatewayParts {
  /** Data and rules. */
  readonly world: World;
  /** Consumers and deliveries. */
  readonly hub: Hub;
  /** The credential checks. */
  readonly auth: Authenticator;
  /** Returns the `htu` a proof must name. */
  readonly htu: () => string;
  /** Returns `true` when a fault is queued for a gateway op (it drops the connection). */
  readonly takeFault: (op: string) => boolean;
  /** Records a received client op. */
  readonly record: (op: string) => void;
}

/** Serves `/api/v1/gateway`. */
export class Gateway {
  /** The sessions `resume` can continue, by ID. */
  readonly sessions = new Map<string, GatewaySession>();
  /** Whether the server restarts (new connections are refused with `4013`). */
  draining = false;
  // sync.ws_max_frame_bytes: the largest frame the server reads.
  private readonly frameLimit = DEFAULTS.wsMaxFrameBytes;
  private readonly sockets = new Map<WebSocket, Close>();
  private readonly server = new WebSocketServer({
    noServer: true,
    maxPayload: 0,
  });

  /**
   * Creates the gateway.
   *
   * @param parts - World, hub, checks and hooks.
   */
  constructor(private readonly parts: GatewayParts) {}

  /**
   * Takes over an HTTP upgrade request of the gateway path.
   *
   * @param request - The upgrade request.
   * @param socket - Its socket.
   * @param head - The first bytes after the headers.
   */
  upgrade(request: IncomingMessage, socket: Duplex, head: Buffer): void {
    this.server.handleUpgrade(request, socket, head, (ws) => this.handle(ws));
  }

  /**
   * Serves one WebSocket connection from `hello` to its close.
   *
   * @param ws - The accepted socket.
   */
  handle(ws: WebSocket): void {
    const state: SocketState = { conn: undefined, sentCode: undefined };
    const send: Send = (frame: OutFrame) =>
      new Promise<void>((resolve) => {
        if (ws.readyState !== WebSocket.OPEN) {
          resolve();
          return;
        }
        ws.send(JSON.stringify(frame), () => resolve());
      });
    // The code the server sent decides whether the session stays resumable; the code the peer
    // echoes back may differ.
    const close: Close = (code: number, reason: string) => {
      state.sentCode ??= code;
      if (
        ws.readyState === WebSocket.OPEN ||
        ws.readyState === WebSocket.CONNECTING
      )
        ws.close(code, reason);
      return Promise.resolve();
    };
    this.sockets.set(ws, close);
    let chain = Promise.resolve();
    let stopped = false;
    const fail = async (error: unknown): Promise<void> => {
      if (stopped) return;
      stopped = true;
      if (error instanceof Dropped) {
        this.end(state, 1006);
        ws.terminate();
        return;
      }
      const signal =
        error instanceof CloseSignal
          ? error
          : new CloseSignal(1011, "internal_error");
      state.sentCode = signal.code;
      this.end(state, signal.code);
      await close(signal.code, signal.reason || "closed");
    };
    ws.on("message", (data: RawData, isBinary: boolean) => {
      chain = chain
        .then(async () => {
          if (stopped) return;
          if (isBinary) throw new CloseSignal(4002, "binary_frame");
          const raw = text(data);
          if (raw.length > this.frameLimit)
            throw new CloseSignal(4002, "frame_too_large");
          await this.frame(raw.toString("utf8"), state, send, close);
        })
        .catch(fail);
    });
    ws.on("close", (code: number) => {
      stopped = true;
      chain = chain
        // A close frame without a status (1005) counts as a drop, as `close_code or 1006` in Python.
        .then(() =>
          this.end(state, state.sentCode ?? (code === 1005 ? 1006 : code)),
        )
        .finally(() => this.sockets.delete(ws));
    });
    ws.on("error", () => undefined);
    if (this.draining) {
      void fail(new CloseSignal(4013, "server_restart"));
      return;
    }
    const hello: S["GatewayHello"] = {
      heartbeat_interval_ms: Math.trunc(
        this.parts.world.timings["sync.heartbeat_interval"],
      ),
      resume_window_ms: Math.trunc(
        this.parts.world.timings["sync.resume_window"],
      ),
      protocol_versions: [1],
      dpop_nonce: this.parts.auth.nonces.current(),
      server_time: this.parts.world.nowIso(),
      sync_epoch: this.parts.world.syncEpoch,
    };
    void send({ op: "hello", d: hello });
  }

  private end(state: SocketState, code: number): void {
    const conn = state.conn;
    if (conn === undefined) return;
    state.conn = undefined;
    this.parts.hub.detach(conn);
    const session = this.sessions.get(conn.sessionId);
    if (session !== undefined && session.owner === conn) {
      session.owner = undefined;
      if (RESUMABLE.has(code) || code === 1006)
        session.detachedAt = this.parts.world.clock.monotonic();
      else this.sessions.delete(conn.sessionId);
    }
  }

  private async frame(
    raw: string,
    state: SocketState,
    send: Send,
    close: Close,
  ): Promise<void> {
    let frame: unknown;
    try {
      frame = JSON.parse(raw);
    } catch {
      throw new CloseSignal(4002, "decode_error");
    }
    if (!isPlainObject(frame) || !hasOnly(frame, new Set(["op", "d"])))
      throw new CloseSignal(4002, "decode_error");
    const op = frame.op;
    if (typeof op !== "string" || !op)
      throw new CloseSignal(4002, "decode_error");
    this.parts.record(op);
    if (!REGISTERED.has(op)) throw new CloseSignal(4001, "unknown_op");
    const identified = state.conn !== undefined;
    if (!identified && op !== "identify" && op !== "resume")
      throw new CloseSignal(4003, "not_identified");
    if (identified && (op === "identify" || op === "resume"))
      throw new CloseSignal(4005, "already_identified");
    const d = frame.d;
    if (!isPlainObject(d)) throw new CloseSignal(4002, "decode_error");
    if (this.parts.takeFault(op)) throw new Dropped();
    if (op === "identify") await this.identify(d, state, send, close);
    else if (op === "resume") await this.resume(d, state, send, close);
    else if (op === "heartbeat") await this.heartbeat(d, send);
    else if (op === "ack") {
      if (!hasOnly(d, new Set(["seq"])) || !isInt(d.seq))
        throw new CloseSignal(4002, "decode_error");
      if (
        state.conn !== undefined &&
        !this.parts.hub.gatewayAck(state.conn, d.seq)
      )
        await send({
          op: "error",
          d: {
            op: "ack",
            code: "validation_failed",
          } satisfies S["GatewayError"],
        });
    } else if (op === "run.update") await this.runUpdate(d, send);
    else if (op === "command.autocomplete_result") this.autocomplete(d);
  }

  private async authenticate(d: Record<string, unknown>): Promise<void> {
    const tokens = ["access_token", "api_token"].filter((n) => n in d);
    if (tokens.length !== 1 || ("cursor" in d && !isInt(d.cursor)))
      throw new CloseSignal(4002, "decode_error");
    if (tokens[0] !== "api_token" || typeof d.api_token !== "string")
      throw new CloseSignal(4004, "authentication_failed");
    const scheme = "dpop" in d ? "DPoP" : "Bearer";
    try {
      await this.parts.auth.authenticate({
        method: "GET",
        htu: this.parts.htu(),
        authorization: [`${scheme} ${d.api_token}`],
        proofs: typeof d.dpop === "string" ? [d.dpop] : [],
      });
    } catch (error) {
      if (error instanceof FakeProblem)
        throw new CloseSignal(4004, "authentication_failed");
      throw error;
    }
  }

  private async identify(
    d: Record<string, unknown>,
    state: SocketState,
    send: Send,
    close: Close,
  ): Promise<void> {
    const consumer = "consumer" in d ? d.consumer : "default";
    if (
      !hasOnly(d, IDENTIFY) ||
      typeof consumer !== "string" ||
      !consumerOk(consumer)
    )
      throw new CloseSignal(4002, "decode_error");
    if (!isInt(d.protocol_version, 0))
      throw new CloseSignal(4002, "decode_error");
    if (d.protocol_version !== 1)
      throw new CloseSignal(4007, "protocol_version_unsupported");
    await this.authenticate(d);
    const sessionId = uuidv7();
    const session: GatewaySession = {
      consumer,
      detachedAt: undefined,
      owner: undefined,
    };
    this.sessions.set(sessionId, session);
    const world = this.parts.world;
    const principal = world.lotse.ref();
    const head = this.parts.hub.head;
    const snapshot: S["Snapshot"] = {
      head_seq: head,
      permission_version: world.permissionVersion,
      sync_epoch: world.syncEpoch,
      principal,
      is_owner: false,
      spaces: [],
      channels: [],
      sidebar: [],
      unread_threads: [],
      active_runs: [],
      active_threads: [],
      agents: [],
      settings: [],
      drafts: [],
      features: [],
    };
    const ready: S["GatewayReady"] = {
      gateway_session_id: sessionId,
      principal,
      permission_version: world.permissionVersion,
      head_seq: head,
      snapshot,
      sync_epoch: world.syncEpoch,
    };
    await send({ op: "ready", d: ready });
    state.conn = await this.parts.hub.attach(consumer, sessionId, send, close);
    session.owner = state.conn;
  }

  private async resume(
    d: Record<string, unknown>,
    state: SocketState,
    send: Send,
    close: Close,
  ): Promise<void> {
    const sessionId = d.gateway_session_id;
    if (
      !hasOnly(d, RESUME) ||
      typeof sessionId !== "string" ||
      !isInt(d.cursor)
    )
      throw new CloseSignal(4002, "decode_error");
    await this.authenticate(d);
    const session = this.sessions.get(sessionId);
    const window = this.parts.world.timings["sync.resume_window"];
    if (
      session === undefined ||
      (session.detachedAt !== undefined &&
        this.parts.world.clock.monotonic() - session.detachedAt >= window)
    ) {
      this.sessions.delete(sessionId);
      throw new CloseSignal(4006, "session_invalid");
    }
    const old = session.owner;
    if (old !== undefined) {
      session.owner = undefined;
      this.parts.hub.detach(old);
      await old.close(4015, "consumer_replaced").catch(() => undefined);
    }
    session.detachedAt = undefined;
    const resumed: S["GatewayResumed"] = {
      gateway_session_id: sessionId,
      head_seq: this.parts.hub.head,
      sync_epoch: this.parts.world.syncEpoch,
    };
    await send({ op: "resumed", d: resumed });
    const holder = this.parts.hub.holders.get(session.consumer);
    if (holder !== undefined && holder !== sessionId) {
      // The fenced save of the resumed session loses: a newer holder took the consumer.
      this.sessions.delete(sessionId);
      throw new CloseSignal(4015, "consumer_replaced");
    }
    state.conn = await this.parts.hub.attach(
      session.consumer,
      sessionId,
      send,
      close,
    );
    session.owner = state.conn;
  }

  private async heartbeat(
    d: Record<string, unknown>,
    send: Send,
  ): Promise<void> {
    if (!hasOnly(d, new Set(["last_seq", "agent"])) || !isInt(d.last_seq))
      throw new CloseSignal(4002, "decode_error");
    const ack: S["GatewayHeartbeatAck"] = {
      server_time: this.parts.world.nowIso(),
    };
    await send({ op: "heartbeat_ack", d: ack });
    if ("agent" in d) {
      try {
        this.parts.world.heartbeat(d.agent);
      } catch (error) {
        if (!(error instanceof FakeProblem)) throw error;
        await send({
          op: "error",
          d: { op: "heartbeat", code: error.code } satisfies S["GatewayError"],
        });
      }
    }
  }

  private async runUpdate(
    d: Record<string, unknown>,
    send: Send,
  ): Promise<void> {
    if (
      !hasOnly(d, new Set(["run_id", "base_version", "ops", "ref"])) ||
      typeof d.run_id !== "string" ||
      !UUID.test(d.run_id) ||
      !isInt(d.base_version, Number.MIN_SAFE_INTEGER) ||
      !Array.isArray(d.ops) ||
      ("ref" in d && typeof d.ref !== "string")
    )
      throw new CloseSignal(4002, "decode_error");
    try {
      this.parts.world.patch(d.run_id, d.base_version, d.ops);
    } catch (error) {
      if (!(error instanceof FakeProblem)) throw error;
      const out: S["GatewayError"] = { op: "run.update", code: error.code };
      const first = error.errors?.[0];
      if (error.code === "validation_failed" && first !== undefined)
        out.detail = `${first.pointer}: ${first.code}`;
      else if (error.reason && error.code !== "base_version_conflict")
        out.detail = error.reason;
      if (typeof d.ref === "string" && d.ref) out.ref = d.ref;
      await send({ op: "error", d: out });
    }
  }

  private autocomplete(d: Record<string, unknown>): void {
    if (
      !hasOnly(d, new Set(["request_id", "choices", "ref"])) ||
      !Array.isArray(d.choices)
    )
      throw new CloseSignal(4002, "decode_error");
    const requestId = canonicalUuid(String(d.request_id));
    if (requestId === undefined) throw new CloseSignal(4002, "decode_error");
    const world = this.parts.world;
    const pending = world.autocomplete.get(requestId);
    if (pending === undefined || world.now() > pending.expires) return;
    if (!world.autocompleteAnswers.has(requestId))
      world.autocompleteAnswers.set(requestId, [
        ...(d.choices as S["CommandChoice"][]),
      ]);
  }

  /**
   * Closes every socket, identified or not.
   *
   * @param code - The close code.
   * @param reason - The close reason.
   */
  async closeAll(code: number, reason: string): Promise<void> {
    await this.parts.hub.closeAll(code, reason);
    for (const close of [...this.sockets.values()])
      await close(code, reason).catch(() => undefined);
  }

  /** Drops every session (a process restart). */
  forgetSessions(): void {
    this.sessions.clear();
  }

  /**
   * Stops accepting connections and terminates the remaining sockets.
   *
   * @returns A promise that resolves when the WebSocket server closed.
   */
  shutdown(): Promise<void> {
    for (const ws of this.sockets.keys()) ws.terminate();
    return new Promise<void>((resolve) => this.server.close(() => resolve()));
  }
}
