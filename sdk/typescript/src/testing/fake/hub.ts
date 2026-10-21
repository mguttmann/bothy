// SPDX-License-Identifier: Apache-2.0
/**
 * Consumers of the FakeServer: the event log, cursors, REST windows and gateway runners
 * (private; port of `bothy_sdk/testing/_fake_hub.py`).
 *
 * One holder per consumer (newest wins, `4015` for a replaced gateway connection), a cursor
 * created at the head by the first takeover, cumulative acknowledgements, and redelivery after
 * `agent.ack_timeout` through background loops on the shared FakeClock
 * (docs/spec/04-sync-protokoll.md 15.1 and 15.2, docs/spec/09-agenten.md 4.3).
 *
 * @module
 */

import type { components } from "../../generated/openapi.js";
import type { FakeClock } from "../clock.js";
import { FakeProblem, fieldError, type Timings } from "./base.js";
import {
  fill,
  Window,
  type ConsumerCursor,
  type Envelope,
  type Hook,
  type LogEntry,
} from "./events.js";

/** A gateway frame the FakeServer sends. */
export interface OutFrame {
  /** The op. */
  readonly op: string;
  /** The data. */
  readonly d: unknown;
}

/** Sends one frame on a connection. */
export type Send = (frame: OutFrame) => Promise<void>;
/** Closes a connection with a code and a reason. */
export type Close = (code: number, reason: string) => Promise<void>;

/** A flag a loop can wait for (the `asyncio.Event` of the Python fake). */
export class Signal {
  private flag = false;
  private waiters: (() => void)[] = [];

  /** Sets the flag and wakes every waiter. */
  set(): void {
    this.flag = true;
    for (const wake of this.waiters.splice(0)) wake();
  }

  /** Clears the flag. */
  clear(): void {
    this.flag = false;
  }

  /**
   * Waits until the flag is set.
   *
   * @returns A promise that resolves once the flag is set.
   */
  wait(): Promise<void> {
    if (this.flag) return Promise.resolve();
    return new Promise<void>((resolve) => this.waiters.push(resolve));
  }
}

/** One identified gateway connection of a consumer. */
export class Connection {
  /** Whether the backlog was sent and `catchup.complete` followed. */
  live = false;
  /** The highest `seq` named in a frame. */
  lastFrame = 0;
  /** Wakes the delivery loop. */
  readonly wake = new Signal();
  /** Wakes the redelivery timer. */
  readonly armed = new Signal();
  /** Stops the loops. */
  readonly stop = new AbortController();
  /** Whether the connection ended. */
  closed = false;

  /**
   * Creates the connection.
   *
   * @param consumer - The consumer name.
   * @param sessionId - The gateway session.
   * @param send - Sends a frame.
   * @param close - Closes the socket.
   * @param window - The delivery window.
   * @param readyHead - The head at `ready` or `resumed`.
   */
  constructor(
    readonly consumer: string,
    readonly sessionId: string,
    readonly send: Send,
    readonly close: Close,
    readonly window: Window,
    readonly readyHead: number,
  ) {}
}

/** The answer of `pollAgentEvents` before the FakeServer adds `sync_epoch`. */
export type PollPage = Omit<
  components["schemas"]["EventPage"],
  "sync_epoch" | "events"
> & {
  events: Envelope[];
};

/** Log, cursors and holders. */
export class Hub {
  /** The event log. */
  readonly log: LogEntry[] = [];
  /** The stored cursor per consumer. */
  readonly cursors = new Map<string, ConsumerCursor>();
  /** The REST window per consumer. */
  readonly rest = new Map<string, Window>();
  /** The gateway connection per consumer. */
  readonly connections = new Map<string, Connection>();
  /** The current holder per consumer: a gateway session ID or `"rest"`. */
  readonly holders = new Map<string, string>();

  /**
   * Creates the hub.
   *
   * @param clock - The shared FakeClock.
   * @param timings - The server keys the FakeServer plays.
   * @param hook - The delivery hook of the run rules.
   */
  constructor(
    readonly clock: FakeClock,
    readonly timings: Timings,
    readonly hook: Hook,
  ) {}

  /** The highest `seq` in the log. */
  get head(): number {
    return this.log.at(-1)?.seq ?? 0;
  }

  /**
   * Appends a durable event and wakes every runner.
   *
   * @param type - The event type.
   * @param envelope - The envelope without `seq`.
   * @returns The log entry.
   */
  append(type: string, envelope: Envelope): LogEntry {
    const seq = this.head + 1;
    const payload = envelope.payload as Record<string, unknown> | undefined;
    const runId = payload?.run_id;
    const entry: LogEntry = {
      seq,
      type,
      envelope: { ...envelope, seq },
      runId: typeof runId === "string" ? runId : undefined,
    };
    this.log.push(entry);
    for (const conn of this.connections.values()) conn.wake.set();
    return entry;
  }

  private maxInflight(): number {
    return Math.trunc(this.timings["agent.max_inflight_events"]);
  }

  private ackTimeout(): number {
    return this.timings["agent.ack_timeout"];
  }

  /**
   * `pollAgentEvents`: takes the consumer over and delivers from the window.
   *
   * @param consumer - The consumer name.
   * @param limit - The `limit` query parameter.
   * @returns The page.
   */
  async poll(consumer: string, limit: number | undefined): Promise<PollPage> {
    const conn = this.connections.get(consumer);
    if (conn !== undefined) {
      this.connections.delete(consumer);
      await this.drop(conn, 4015, "consumer_replaced");
    }
    this.holders.set(consumer, "rest");
    let cursor = this.cursors.get(consumer);
    const created = cursor === undefined;
    if (cursor === undefined) {
      cursor = { acked: this.head };
      this.cursors.set(consumer, cursor);
    }
    let window = this.rest.get(consumer);
    if (window === undefined || created) {
      window = new Window(cursor.acked);
      this.rest.set(consumer, window);
    } else {
      window.ack(cursor.acked);
      window.expire(this.clock.monotonic(), this.ackTimeout());
    }
    if (created)
      return { events: [], next_cursor: this.head, head_seq: this.head };
    const earlier = window.inflight.size
      ? Math.min(...window.inflight.keys())
      : undefined;
    const cap = this.maxInflight();
    const events = fill(window, this.log, this.hook, {
      now: this.clock.monotonic(),
      maxInflight: cap,
      limit: Math.min(limit ?? cap, cap),
    });
    const nextCursor =
      earlier === undefined ? window.read : Math.min(window.read, earlier - 1);
    return { events, next_cursor: nextCursor, head_seq: this.head };
  }

  /**
   * `ackAgentEvents`: stores at once and monotonically, without a takeover.
   *
   * @param consumer - The consumer name.
   * @param seq - The acknowledged `seq`.
   * @returns The consumer state.
   * @throws {@link FakeProblem} `400` for a `seq` above the head, `404` for an unknown consumer.
   */
  ack(consumer: string, seq: number): components["schemas"]["ConsumerState"] {
    if (seq < 0 || seq > this.head)
      throw fieldError(
        "/seq",
        "invalid",
        "the sequence number is below 0 or above the head",
      );
    const cursor = this.cursors.get(consumer);
    if (cursor === undefined) throw new FakeProblem("not_found");
    cursor.acked = Math.max(cursor.acked, seq);
    this.rest.get(consumer)?.ack(cursor.acked);
    const conn = this.connections.get(consumer);
    if (conn !== undefined) {
      conn.window.ack(cursor.acked);
      conn.wake.set();
    }
    return this.state(consumer);
  }

  /**
   * `getConsumerState`.
   *
   * @param consumer - The consumer name.
   * @returns The stored acknowledgement and the head.
   * @throws {@link FakeProblem} `404` for a consumer that never connected.
   */
  state(consumer: string): components["schemas"]["ConsumerState"] {
    const cursor = this.cursors.get(consumer);
    if (cursor === undefined) throw new FakeProblem("not_found");
    return { consumer, acked_seq: cursor.acked, head_seq: this.head };
  }

  /**
   * Takes the consumer over for a gateway connection after `ready` or `resumed`.
   *
   * @param consumer - The consumer name.
   * @param sessionId - The gateway session.
   * @param send - Sends a frame.
   * @param close - Closes the socket.
   * @returns The connection, its loops running.
   */
  async attach(
    consumer: string,
    sessionId: string,
    send: Send,
    close: Close,
  ): Promise<Connection> {
    let cursor = this.cursors.get(consumer);
    if (cursor === undefined) {
      cursor = { acked: this.head };
      this.cursors.set(consumer, cursor);
    }
    const conn = new Connection(
      consumer,
      sessionId,
      send,
      close,
      new Window(cursor.acked),
      this.head,
    );
    const old = this.connections.get(consumer);
    this.connections.set(consumer, conn);
    this.holders.set(consumer, sessionId);
    this.rest.delete(consumer);
    if (old !== undefined) await this.drop(old, 4015, "consumer_replaced");
    void this.run(conn).catch(() => undefined);
    void this.timer(conn).catch(() => undefined);
    return conn;
  }

  /**
   * The op `ack`.
   *
   * @param conn - The connection.
   * @param seq - The acknowledged `seq`.
   * @returns `false` when the `seq` is above what may be acknowledged.
   */
  gatewayAck(conn: Connection, seq: number): boolean {
    if (seq < 0 || seq > Math.max(this.head, conn.window.read)) return false;
    const cursor = this.cursors.get(conn.consumer);
    if (cursor === undefined) return false;
    cursor.acked = Math.max(cursor.acked, seq);
    conn.window.ack(cursor.acked);
    conn.wake.set();
    return true;
  }

  /**
   * Stops a connection's loops (the connection ended).
   *
   * @param conn - The connection.
   */
  detach(conn: Connection): void {
    conn.closed = true;
    if (this.connections.get(conn.consumer) === conn)
      this.connections.delete(conn.consumer);
    conn.stop.abort();
    conn.wake.set();
    conn.armed.set();
  }

  private async drop(
    conn: Connection,
    code: number,
    reason: string,
  ): Promise<void> {
    this.detach(conn);
    try {
      await conn.close(code, reason);
    } catch {
      // The socket is gone already.
    }
  }

  /**
   * Closes every gateway connection with `code`.
   *
   * @param code - The close code.
   * @param reason - The close reason.
   */
  async closeAll(code: number, reason: string): Promise<void> {
    for (const conn of [...this.connections.values()])
      await this.drop(conn, code, reason);
  }

  /**
   * Forgets in-memory windows (the process restarted), optionally setting one cursor.
   *
   * @param consumer - The consumer whose cursor is set.
   * @param ackedSeq - The cursor value.
   */
  restart(consumer: string | undefined, ackedSeq: number | undefined): void {
    this.rest.clear();
    if (consumer !== undefined && ackedSeq !== undefined)
      this.cursors.set(consumer, { acked: ackedSeq });
  }

  /**
   * Sends an ephemeral event to one live connection.
   *
   * @param envelope - The envelope without `seq`.
   * @returns `false` when no connection is live.
   */
  async ephemeral(envelope: Envelope): Promise<boolean> {
    for (const conn of this.connections.values()) {
      if (conn.live && !conn.closed) {
        await conn.send({ op: "dispatch", d: envelope });
        return true;
      }
    }
    return false;
  }

  // The delivery loop: backlog, cursor and catchup.complete, then live dispatches.
  private async run(conn: Connection): Promise<void> {
    while (!conn.closed) {
      // Clear before reading the log, so an append during the sends below is not lost.
      conn.wake.clear();
      const events = fill(conn.window, this.log, this.hook, {
        now: this.clock.monotonic(),
        maxInflight: this.maxInflight(),
      });
      if (events.length) conn.armed.set();
      for (const envelope of events) {
        if (conn.closed) return;
        await conn.send({ op: "dispatch", d: envelope });
        conn.lastFrame = envelope.seq as number;
      }
      if (conn.closed) return;
      if (!conn.live && conn.window.read >= conn.readyHead) {
        if (conn.window.read > conn.lastFrame) {
          await conn.send({ op: "cursor", d: { seq: conn.window.read } });
          conn.lastFrame = conn.window.read;
        }
        const head = Math.max(conn.readyHead, conn.window.read);
        await conn.send({ op: "catchup.complete", d: { head_seq: head } });
        conn.live = true;
      }
      if (!events.length) await conn.wake.wait();
    }
  }

  // Redelivery after agent.ack_timeout, on the FakeClock.
  private async timer(conn: Connection): Promise<void> {
    while (!conn.closed) {
      const due = conn.window.deadline(this.ackTimeout());
      if (due === undefined) {
        conn.armed.clear();
        await conn.armed.wait();
        continue;
      }
      await this.clock.sleep(due - this.clock.monotonic(), conn.stop.signal);
      if (conn.window.expire(this.clock.monotonic(), this.ackTimeout()))
        conn.wake.set();
    }
  }
}
