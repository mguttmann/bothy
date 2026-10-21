// SPDX-License-Identifier: Apache-2.0
/**
 * Event log, consumer cursors and delivery windows of the FakeServer (private; port of
 * `bothy_sdk/testing/_fake_events.py`).
 *
 * The rules are those of docs/spec/04-sync-protokoll.md 15.1 and 15.2 and
 * docs/spec/09-agenten.md 4.3 as the server's consumer engine builds them: a cursor per consumer
 * name created at the head by the first takeover, delivery from `acked_seq + 1` in `seq` order,
 * at most `agent.max_inflight_events` unacknowledged, and after `agent.ack_timeout` (counted
 * from the oldest unacknowledged delivery) a new delivery from `acked_seq + 1` with
 * `redelivered`.
 *
 * @module
 */

/** An event envelope as the FakeServer stores and sends it. */
export type Envelope = Record<string, unknown> & { readonly id: string };

/** One durable event: its envelope without delivery fields and the run it concerns. */
export interface LogEntry {
  /** The position in the log. */
  readonly seq: number;
  /** The event type. */
  readonly type: string;
  /** The stored envelope (with `seq`). */
  readonly envelope: Envelope;
  /** The run the event concerns, if any. */
  readonly runId: string | undefined;
}

/** The stored acknowledgement of one consumer. */
export interface ConsumerCursor {
  /** The acknowledged `seq`. */
  acked: number;
}

/** What one holder (a gateway connection or the REST window) has on the way. */
export class Window {
  /** The acknowledged `seq` known to the window. */
  acked: number;
  /** The highest `seq` the window checked. */
  read: number;
  /** Unacknowledged `seq` values with their delivery time (clock monotonic, milliseconds). */
  inflight = new Map<number, number>();
  /** Deliveries up to this `seq` carry `redelivered`. */
  redeliverUpTo = 0;
  /** The highest `seq` ever delivered by this window. */
  highestSent = 0;

  /**
   * Starts reading after the acknowledgement.
   *
   * @param acked - The acknowledged `seq`.
   */
  constructor(acked: number) {
    this.acked = acked;
    this.read = acked;
  }

  /**
   * Applies a cumulative, monotone acknowledgement.
   *
   * @param seq - The acknowledged `seq`.
   */
  ack(seq: number): void {
    if (seq <= this.acked) return;
    this.acked = seq;
    for (const s of [...this.inflight.keys()])
      if (s <= seq) this.inflight.delete(s);
    this.read = Math.max(this.read, seq);
  }

  /**
   * Returns the monotonic time at which the oldest delivery times out, if any.
   *
   * @param ackTimeout - `agent.ack_timeout` in milliseconds.
   * @returns The deadline, or `undefined` when nothing is on the way.
   */
  deadline(ackTimeout: number): number | undefined {
    return this.inflight.size
      ? Math.min(...this.inflight.values()) + ackTimeout
      : undefined;
  }

  /**
   * Starts a new delivery from `acked + 1` when the oldest delivery timed out.
   *
   * @param now - The clock's monotonic time.
   * @param ackTimeout - `agent.ack_timeout` in milliseconds.
   * @returns Whether the window expired.
   */
  expire(now: number, ackTimeout: number): boolean {
    const due = this.deadline(ackTimeout);
    if (due === undefined || now < due) return false;
    this.restartDelivery();
    return true;
  }

  /** Forgets what is on the way and delivers again from `acked + 1` with `redelivered`. */
  restartDelivery(): void {
    this.redeliverUpTo = Math.max(this.redeliverUpTo, this.highestSent);
    this.inflight.clear();
    this.read = this.acked;
  }

  /**
   * Returns how many more deliveries may be on the way.
   *
   * @param maxInflight - `agent.max_inflight_events`.
   * @returns The remaining budget.
   */
  budget(maxInflight: number): number {
    return Math.max(0, maxInflight - this.inflight.size);
  }

  /**
   * Records a delivery of `seq`.
   *
   * @param seq - The delivered `seq`.
   * @param now - The clock's monotonic time.
   * @returns The delivery field `redelivered`.
   */
  sent(seq: number, now: number): boolean {
    this.inflight.set(seq, now);
    this.read = seq;
    this.highestSent = Math.max(this.highestSent, seq);
    return seq <= this.redeliverUpTo;
  }
}

/**
 * A delivery hook decides per event: `undefined` skips it (it counts as checked), else the
 * envelope with its delivery fields.
 */
export type Hook = (entry: LogEntry) => Envelope | undefined;

/** The inputs of one {@link fill} pass besides window, log and hook. */
export interface FillOptions {
  /** The clock's monotonic time. */
  readonly now: number;
  /** `agent.max_inflight_events`. */
  readonly maxInflight: number;
  /** At most this many envelopes. */
  readonly limit?: number;
}

/**
 * Delivers from `window.read + 1` while the budget lasts.
 *
 * @param window - The holder's window.
 * @param log - The event log.
 * @param hook - The delivery hook.
 * @param options - Time, budget and limit.
 * @returns The envelopes to send, in `seq` order.
 */
export function fill(
  window: Window,
  log: readonly LogEntry[],
  hook: Hook,
  options: FillOptions,
): Envelope[] {
  const out: Envelope[] = [];
  for (const entry of log) {
    if (entry.seq <= window.read) continue;
    if (entry.seq <= window.acked) {
      window.read = entry.seq;
      continue;
    }
    if (
      window.budget(options.maxInflight) === 0 ||
      (options.limit !== undefined && out.length >= options.limit)
    )
      break;
    const envelope = hook(entry);
    if (envelope === undefined) {
      window.read = entry.seq;
      continue;
    }
    if (window.sent(entry.seq, options.now)) envelope.redelivered = true;
    out.push(envelope);
  }
  return out;
}
