// SPDX-License-Identifier: Apache-2.0
/**
 * The adapter's inbox: accepted events, their newest tokens and the `jti` store.
 *
 * An event with an invocation token is acknowledged only after it passed the check and is stored
 * here (docs/spec/09-agents.md 4.3, docs/spec/04-sync-protocol.md 15.2); the start of its run
 * follows from the inbox. Per event the store keeps the envelope without `invocation_token`, the
 * newest token with its `iat` and `jti`, a state and a delivery count; besides that the `jti`
 * values of passed tokens until `exp` plus `adapter.invocation_token_leeway` (09 4.5 point 7).
 *
 * States move `accepted` (stored, not started), `started` (`run.start` answered `200`),
 * `waiting` (`awaitRunInput`) and `closed` (ended, refused or handed over to the server).
 *
 * A redelivery of a stored event (same `id`) only counts; while the entry is `accepted` its token
 * replaces the stored one when the new `iat` is not older, and an equal `iat` takes the later
 * arrival (04 15.2 point 4, docs/DECISIONS.md D-M2-29-9).
 *
 * `FileInboxStore` is durable: its directory is the adapter's acceptance and holds tokens that
 * expire after `agent.invocation_token_ttl`. {@link MemoryInboxStore} behaves alike for tests.
 *
 * @example
 * ```ts
 * const store = new MemoryInboxStore();
 * await store.add(new InboxEntry({
 *   eventId, seq: 1, type: "agent.invocation", runId: undefined, envelope: { id: eventId },
 *   token: undefined, tokenIat: undefined, tokenJti: undefined,
 *   state: EntryState.ACCEPTED, deliveries: 1,
 * }));
 * (await store.redelivered(eventId, undefined, undefined, undefined)).deliveries; // 2
 * ```
 *
 * @module
 */

import type { Secret } from "../auth/secret.js";
import type { ReplayStore } from "./verifier.js";

const TOKEN_MEMBER = "invocation_token";
const UUID = /^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$/i;

/** The state of an inbox entry. */
export const EntryState = Object.freeze({
  /** Stored after the check; the run has not started. */
  ACCEPTED: "accepted",
  /** `run.start` (or `run.resume`) answered `200`. */
  STARTED: "started",
  /** The run waits for input (`awaitRunInput`). */
  WAITING: "waiting",
  /** Ended, refused or left to the server; nothing more happens with it. */
  CLOSED: "closed",
} as const);

/** One of the values of {@link EntryState}. */
export type EntryState = (typeof EntryState)[keyof typeof EntryState];

/** The members of an {@link InboxEntry}. */
export interface InboxEntryFields {
  /** The envelope `id`. */
  readonly eventId: string;
  /** The envelope `seq`; `undefined` for an ephemeral event. */
  readonly seq: number | undefined;
  /** The event type, for example `agent.invocation`. */
  readonly type: string;
  /** The run the event belongs to, when it has one. */
  readonly runId: string | undefined;
  /** The envelope as received, without `invocation_token`. */
  readonly envelope: Readonly<Record<string, unknown>>;
  /** The newest invocation token of the event. */
  readonly token: Secret | undefined;
  /** `iat` of `token`. */
  readonly tokenIat: number | undefined;
  /** `jti` of `token`. */
  readonly tokenJti: string | undefined;
  /** The entry's state. */
  readonly state: EntryState;
  /** How often the event was delivered. */
  readonly deliveries: number;
}

/**
 * One stored event; immutable. `inspect`, `String` and `JSON.stringify` show neither the token
 * nor the envelope (SEC-LOG-2).
 */
export class InboxEntry implements InboxEntryFields {
  /** The envelope `id`. */
  readonly eventId: string;
  /** The envelope `seq`; `undefined` for an ephemeral event. */
  readonly seq: number | undefined;
  /** The event type, for example `agent.invocation`. */
  readonly type: string;
  /** The run the event belongs to, when it has one. */
  readonly runId: string | undefined;
  /** The envelope as received, without `invocation_token`. */
  readonly envelope: Readonly<Record<string, unknown>>;
  /** The newest invocation token of the event. */
  readonly token: Secret | undefined;
  /** `iat` of `token`. */
  readonly tokenIat: number | undefined;
  /** `jti` of `token`. */
  readonly tokenJti: string | undefined;
  /** The entry's state. */
  readonly state: EntryState;
  /** How often the event was delivered. */
  readonly deliveries: number;

  /**
   * Creates an entry.
   *
   * @param fields - The members.
   */
  constructor(fields: InboxEntryFields) {
    this.eventId = fields.eventId;
    this.seq = fields.seq;
    this.type = fields.type;
    this.runId = fields.runId;
    this.envelope = fields.envelope;
    this.token = fields.token;
    this.tokenIat = fields.tokenIat;
    this.tokenJti = fields.tokenJti;
    this.state = fields.state;
    this.deliveries = fields.deliveries;
    Object.freeze(this);
  }

  /** Returns the members without token and envelope, for logs and `JSON.stringify`. */
  toJSON(): Record<string, unknown> {
    return {
      eventId: this.eventId,
      seq: this.seq,
      type: this.type,
      runId: this.runId,
      tokenIat: this.tokenIat,
      tokenJti: this.tokenJti,
      state: this.state,
      deliveries: this.deliveries,
    };
  }

  /** Returns `InboxEntry(<event id>, <state>)`. */
  toString(): string {
    return `InboxEntry(${this.eventId}, ${this.state})`;
  }

  /** Returns the members of {@link InboxEntry.toJSON} for `util.inspect` and `console.log`. */
  [Symbol.for("nodejs.util.inspect.custom")](): string {
    return `InboxEntry ${JSON.stringify(this.toJSON())}`;
  }
}

/**
 * Storage of the inbox and of the `jti` store; every write is durable when it returns. Times
 * are Unix milliseconds.
 */
export interface InboxStore extends ReplayStore {
  /**
   * Returns the entry of `eventId`.
   *
   * @param eventId - The envelope `id`.
   * @returns The entry, or `undefined` when it is not stored.
   */
  get(eventId: string): Promise<InboxEntry | undefined>;
  /**
   * Stores a new entry.
   *
   * @param entry - The entry; its envelope must not carry `invocation_token`.
   * @throws `RangeError` when the event is already stored, its ID is not a UUID or the
   *   envelope carries a token.
   */
  add(entry: InboxEntry): Promise<void>;
  /**
   * Counts a redelivery and keeps the newer token while the entry is `accepted`.
   *
   * The token replaces the stored one when the entry is `accepted` and `iat` is not older than
   * the stored `iat` (an equal `iat` takes this later arrival).
   *
   * @param eventId - The envelope `id`.
   * @param token - The token of the redelivery, if any.
   * @param iat - Its `iat`.
   * @param jti - Its `jti`.
   * @returns The entry after the update.
   * @throws `RangeError` when the event is not stored.
   */
  redelivered(
    eventId: string,
    token: Secret | undefined,
    iat: number | undefined,
    jti: string | undefined,
  ): Promise<InboxEntry>;
  /**
   * Replaces the token (after a renewal grant).
   *
   * @param eventId - The envelope `id`.
   * @param token - The granted token.
   * @param iat - Its `iat`.
   * @param jti - Its `jti`.
   * @param replaces - The renewed token; when given, the token is replaced only while the
   *   stored token is still this one (a redelivery during the renewal stored a newer token,
   *   which stays).
   * @returns The entry after the call.
   * @throws `RangeError` when the event is not stored.
   */
  replaceToken(
    eventId: string,
    token: Secret,
    iat: number,
    jti: string,
    replaces?: Secret,
  ): Promise<InboxEntry>;
  /**
   * Sets the state of an entry.
   *
   * @param eventId - The envelope `id`.
   * @param state - The new state.
   * @throws `RangeError` when the event is not stored.
   */
  setState(eventId: string, state: EntryState): Promise<void>;
  /**
   * Returns the entries of a run in `seq` order (ephemeral entries last, in arrival order).
   *
   * @param runId - The run.
   */
  forRun(runId: string): Promise<InboxEntry[]>;
  /**
   * Returns the entries in one of `states` in `seq` order (ephemeral entries last, in arrival
   * order).
   *
   * @param states - The states asked for.
   */
  inStates(...states: EntryState[]): Promise<InboxEntry[]>;
  /**
   * Drops the `jti` rows that expired at `nowMs`; inbox entries stay.
   *
   * @param nowMs - The current wall time.
   */
  prune(nowMs: number): Promise<void>;
  /** Releases the store; closing twice is harmless. */
  close(): Promise<void>;
}

/**
 * Returns a copy of `entry` with `changes` applied.
 *
 * @param entry - The entry.
 * @param changes - The members to replace.
 * @returns The new entry.
 */
export function replaced(
  entry: InboxEntry,
  changes: Partial<InboxEntryFields>,
): InboxEntry {
  return new InboxEntry({ ...entry, ...changes });
}

/**
 * Refuses an entry no store may hold: an event ID that is not a UUID or an envelope with a
 * token (the token lives in its own field only).
 *
 * @param entry - The new entry.
 * @throws `RangeError` naming the rule.
 */
export function checkNew(entry: InboxEntry): void {
  if (!UUID.test(entry.eventId))
    throw new RangeError("the event ID must be a UUID");
  if (Object.hasOwn(entry.envelope, TOKEN_MEMBER))
    throw new RangeError(`the envelope must be stored without ${TOKEN_MEMBER}`);
}

/**
 * Returns the entry after a redelivery: one more delivery, and the newer token while the entry
 * is `accepted`.
 *
 * @param entry - The stored entry.
 * @param token - The token of the redelivery, if any.
 * @param iat - Its `iat`.
 * @param jti - Its `jti`.
 * @returns The updated entry.
 */
export function afterRedelivery(
  entry: InboxEntry,
  token: Secret | undefined,
  iat: number | undefined,
  jti: string | undefined,
): InboxEntry {
  const newer =
    entry.state === EntryState.ACCEPTED &&
    token !== undefined &&
    iat !== undefined &&
    jti !== undefined &&
    (entry.tokenIat === undefined || iat >= entry.tokenIat);
  return newer
    ? replaced(entry, {
        token,
        tokenIat: iat,
        tokenJti: jti,
        deliveries: entry.deliveries + 1,
      })
    : replaced(entry, { deliveries: entry.deliveries + 1 });
}

/**
 * The entry after a renewal grant: the granted token, unless a redelivery stored a newer token
 * than `replaces` while the renewal was on its way.
 *
 * @param entry - The stored entry.
 * @param token - The granted token.
 * @param iat - Its `iat`.
 * @param jti - Its `jti`.
 * @param replaces - The renewed token, if the replacement depends on it.
 * @returns The updated entry, or `entry` itself when it keeps its token.
 */
export function afterGrant(
  entry: InboxEntry,
  token: Secret,
  iat: number,
  jti: string,
  replaces: Secret | undefined,
): InboxEntry {
  if (
    replaces !== undefined &&
    entry.token !== undefined &&
    !entry.token.equals(replaces)
  )
    return entry;
  return replaced(entry, { token, tokenIat: iat, tokenJti: jti });
}

/**
 * Orders entries with a `seq` first in `seq` order; a stable sort keeps the arrival order of
 * the rest.
 *
 * @param a - One entry.
 * @param b - The other entry.
 * @returns The comparison for `Array.prototype.sort`.
 */
export function bySeq(a: InboxEntry, b: InboxEntry): number {
  const ephemeral = Number(a.seq === undefined) - Number(b.seq === undefined);
  return ephemeral !== 0 ? ephemeral : (a.seq ?? 0) - (b.seq ?? 0);
}

/**
 * The error of an event that is not stored; it names the event ID only, never a token.
 *
 * @param eventId - The envelope `id`.
 * @returns The error.
 */
export function notStored(eventId: string): RangeError {
  return new RangeError(`event not stored: ${eventId}`);
}

/**
 * An in-memory {@link InboxStore} for tests; it behaves like `FileInboxStore` but is lost on
 * exit. Entries are kept in arrival order, so entries without `seq` keep their arrival order.
 *
 * @example
 * ```ts
 * const verifier = new InvocationVerifier({ instanceId, agentId, jwks, replay: new MemoryInboxStore() });
 * ```
 */
export class MemoryInboxStore implements InboxStore {
  private readonly entries = new Map<string, InboxEntry>();
  private jti = new Map<string, number>();

  /**
   * Returns whether `jti` is remembered beyond `nowMs`.
   *
   * @param jti - The token ID.
   * @param nowMs - The current wall time.
   * @returns Whether the `jti` is still remembered.
   */
  jtiSeen(jti: string, nowMs: number): Promise<boolean> {
    const until = this.jti.get(jti);
    return Promise.resolve(until !== undefined && nowMs < until);
  }

  /**
   * Remembers `jti` until `untilMs` (an earlier end never shortens a later one).
   *
   * @param jti - The token ID.
   * @param untilMs - When the token cannot pass anymore (`exp` plus the leeway).
   * @returns A promise that resolves when stored.
   */
  rememberJti(jti: string, untilMs: number): Promise<void> {
    const old = this.jti.get(jti);
    this.jti.set(jti, old === undefined || untilMs > old ? untilMs : old);
    return Promise.resolve();
  }

  /**
   * Returns the entry of `eventId`.
   *
   * @param eventId - The envelope `id`.
   * @returns The entry, or `undefined`.
   */
  get(eventId: string): Promise<InboxEntry | undefined> {
    return Promise.resolve(this.entries.get(eventId));
  }

  /**
   * Stores a new entry with a copy of its envelope.
   *
   * @param entry - The entry.
   * @returns A promise that resolves when stored.
   * @throws `RangeError` when the event is already stored, its ID is not a UUID or the
   *   envelope carries a token.
   */
  add(entry: InboxEntry): Promise<void> {
    try {
      checkNew(entry);
      if (this.entries.has(entry.eventId))
        throw new RangeError("event already stored");
    } catch (error) {
      return Promise.reject(error as Error);
    }
    this.entries.set(
      entry.eventId,
      replaced(entry, { envelope: structuredClone(entry.envelope) }),
    );
    return Promise.resolve();
  }

  /**
   * Replaces the token; see {@link InboxStore.replaceToken}.
   *
   * @param eventId - The envelope `id`.
   * @param token - The granted token.
   * @param iat - Its `iat`.
   * @param jti - Its `jti`.
   * @param replaces - The renewed token, if the replacement depends on it.
   * @returns The entry after the call.
   * @throws `RangeError` when the event is not stored.
   */
  replaceToken(
    eventId: string,
    token: Secret,
    iat: number,
    jti: string,
    replaces?: Secret,
  ): Promise<InboxEntry> {
    return this.update(eventId, (e) =>
      afterGrant(e, token, iat, jti, replaces),
    );
  }

  /**
   * Counts a redelivery; see {@link InboxStore.redelivered}.
   *
   * @param eventId - The envelope `id`.
   * @param token - The token of the redelivery, if any.
   * @param iat - Its `iat`.
   * @param jti - Its `jti`.
   * @returns The entry after the update.
   * @throws `RangeError` when the event is not stored.
   */
  redelivered(
    eventId: string,
    token: Secret | undefined,
    iat: number | undefined,
    jti: string | undefined,
  ): Promise<InboxEntry> {
    return this.update(eventId, (e) => afterRedelivery(e, token, iat, jti));
  }

  /**
   * Sets the state of an entry.
   *
   * @param eventId - The envelope `id`.
   * @param state - The new state.
   * @returns A promise that resolves when stored.
   * @throws `RangeError` when the event is not stored.
   */
  async setState(eventId: string, state: EntryState): Promise<void> {
    await this.update(eventId, (e) => replaced(e, { state }));
  }

  /**
   * Returns the entries of a run in `seq` order.
   *
   * @param runId - The run.
   * @returns The entries.
   */
  forRun(runId: string): Promise<InboxEntry[]> {
    return Promise.resolve(
      [...this.entries.values()].filter((e) => e.runId === runId).sort(bySeq),
    );
  }

  /**
   * Returns the entries in one of `states` in `seq` order.
   *
   * @param states - The states asked for.
   * @returns The entries.
   */
  inStates(...states: EntryState[]): Promise<InboxEntry[]> {
    return Promise.resolve(
      [...this.entries.values()]
        .filter((e) => states.includes(e.state))
        .sort(bySeq),
    );
  }

  /**
   * Drops the expired `jti` rows.
   *
   * @param nowMs - The current wall time.
   * @returns A promise that resolves when done.
   */
  prune(nowMs: number): Promise<void> {
    this.jti = new Map([...this.jti].filter(([, until]) => nowMs < until));
    return Promise.resolve();
  }

  /**
   * Nothing to release.
   *
   * @returns A resolved promise.
   */
  close(): Promise<void> {
    return Promise.resolve();
  }

  private update(
    eventId: string,
    change: (entry: InboxEntry) => InboxEntry,
  ): Promise<InboxEntry> {
    const current = this.entries.get(eventId);
    if (current === undefined) return Promise.reject(notStored(eventId));
    const updated = change(current);
    this.entries.set(eventId, updated);
    return Promise.resolve(updated);
  }
}
