// SPDX-License-Identifier: Apache-2.0
/**
 * The durable inbox store: one JSON file per entry in a private directory (M2-30 Design note 7).
 *
 * @module
 */

import { randomUUID } from "node:crypto";
import { constants } from "node:fs";
import { lstat, mkdir, open, readdir, rename, unlink } from "node:fs/promises";
import { join, resolve } from "node:path";

import { Secret } from "../auth/secret.js";
import {
  EntryState,
  InboxEntry,
  afterGrant,
  afterRedelivery,
  bySeq,
  checkNew,
  notStored,
  replaced,
  type InboxStore,
} from "./inbox.js";

const ENTRY_FILE = /^entry-([0-9a-fA-F-]{36})\.json$/;
const JTI_FILE = "jti.json";
const TMP_PREFIX = ".tmp-";
const DIRECTORY_MODE = 0o700;
const FILE_MODE = 0o600;
const OPEN_TO_OTHERS = 0o077;
const STATES: ReadonlySet<string> = new Set(Object.values(EntryState));
// The directory fsync is a durability step where the platform offers it; Windows does not.
const NO_DIRECTORY_SYNC = new Set(["EISDIR", "EPERM", "EINVAL", "EACCES"]);

/** One entry as its file holds it: the event without its token, the token and the state. */
interface EntryRecord {
  event_id: string;
  seq: number | null;
  type: string;
  run_id: string | null;
  envelope: Record<string, unknown>;
  token: string | null;
  token_iat: number | null;
  token_jti: string | null;
  state: string;
  deliveries: number;
  arrival: number;
}

interface Held {
  readonly entry: InboxEntry;
  readonly arrival: number;
}

function toRecord(entry: InboxEntry, arrival: number): EntryRecord {
  return {
    event_id: entry.eventId,
    seq: entry.seq ?? null,
    type: entry.type,
    run_id: entry.runId ?? null,
    envelope: entry.envelope as Record<string, unknown>,
    token: entry.token?.reveal() ?? null,
    token_iat: entry.tokenIat ?? null,
    token_jti: entry.tokenJti ?? null,
    state: entry.state,
    deliveries: entry.deliveries,
    arrival,
  };
}

function isNumberOrNull(value: unknown): value is number | null {
  return value === null || typeof value === "number";
}

function isTextOrNull(value: unknown): value is string | null {
  return value === null || typeof value === "string";
}

function fromRecord(value: unknown, name: string): Held {
  const r = value as Partial<EntryRecord> | null;
  if (
    typeof r !== "object" ||
    r === null ||
    typeof r.event_id !== "string" ||
    !isNumberOrNull(r.seq) ||
    typeof r.type !== "string" ||
    !isTextOrNull(r.run_id) ||
    typeof r.envelope !== "object" ||
    r.envelope === null ||
    !isTextOrNull(r.token) ||
    !isNumberOrNull(r.token_iat) ||
    !isTextOrNull(r.token_jti) ||
    typeof r.state !== "string" ||
    !STATES.has(r.state) ||
    typeof r.deliveries !== "number" ||
    typeof r.arrival !== "number"
  )
    // The message names the file only, never its content.
    throw new Error(`unreadable inbox entry ${name}`);
  return {
    entry: new InboxEntry({
      eventId: r.event_id,
      seq: r.seq ?? undefined,
      type: r.type,
      runId: r.run_id ?? undefined,
      envelope: r.envelope,
      token: r.token ? new Secret(r.token) : undefined,
      tokenIat: r.token_iat ?? undefined,
      tokenJti: r.token_jti ?? undefined,
      state: r.state as EntryState,
      deliveries: r.deliveries,
    }),
    arrival: r.arrival,
  };
}

function entryFile(eventId: string): string {
  return `entry-${eventId}.json`;
}

/**
 * The durable {@link InboxStore}: one JSON file per entry in a directory of mode `0700`, each
 * file of mode `0600`, written to a temporary file, synced and renamed into place, so an entry
 * is either the old or the new one after a crash. The `jti` store lives in the same directory.
 *
 * A file holds the event without `invocation_token`, the newest token, its `iat` and `jti`,
 * the state, the delivery count and the arrival order; nothing else. The token is stored
 * because the directory is the adapter's durable acceptance (docs/DECISIONS.md D-M2-29-9); it is
 * never logged. One store object owns a directory: it reads the files once and keeps them in
 * memory, and every write is durable when its promise resolves.
 *
 * The directory is created on first use; an existing directory or file that group or others
 * can access, or a symbolic link in its place, is refused.
 *
 * @example
 * ```ts
 * const inbox = new FileInboxStore(join(stateDir, "inbox"));
 * for (const entry of await inbox.inStates(EntryState.ACCEPTED)) await start(entry);
 * ```
 */
export class FileInboxStore implements InboxStore {
  private readonly directory: string;
  private readonly entries = new Map<string, Held>();
  private jti = new Map<string, number>();
  private nextArrival = 1;
  private ready: Promise<void> | undefined;
  private tail: Promise<unknown> = Promise.resolve();
  private closed = false;

  /**
   * Creates the store; the directory is opened on the first call.
   *
   * @param directory - The inbox directory, usually `inbox` in the adapter's state directory.
   */
  constructor(directory: string) {
    this.directory = resolve(directory);
  }

  /**
   * Returns whether `jti` is remembered beyond `nowMs`.
   *
   * @param jti - The token ID.
   * @param nowMs - The current wall time.
   * @returns Whether the `jti` is still remembered.
   * @throws `Error` when the directory cannot be opened or the store is closed.
   */
  jtiSeen(jti: string, nowMs: number): Promise<boolean> {
    return this.run(() => {
      const until = this.jti.get(jti);
      return Promise.resolve(until !== undefined && nowMs < until);
    });
  }

  /**
   * Remembers `jti` until `untilMs` (an earlier end never shortens a later one); durable when
   * this resolves.
   *
   * @param jti - The token ID.
   * @param untilMs - When the token cannot pass anymore (`exp` plus the leeway).
   * @returns A promise that resolves when written.
   * @throws `Error` when the write fails or the store is closed.
   */
  rememberJti(jti: string, untilMs: number): Promise<void> {
    return this.run(async () => {
      const next = new Map(this.jti);
      const old = next.get(jti);
      next.set(jti, old === undefined || untilMs > old ? untilMs : old);
      await this.writeJti(next);
    });
  }

  /**
   * Returns the entry of `eventId`.
   *
   * @param eventId - The envelope `id`.
   * @returns The entry, or `undefined`.
   * @throws `Error` when the directory cannot be opened or the store is closed.
   */
  get(eventId: string): Promise<InboxEntry | undefined> {
    return this.run(() => Promise.resolve(this.entries.get(eventId)?.entry));
  }

  /**
   * Stores a new entry; durable when this resolves.
   *
   * @param entry - The entry.
   * @returns A promise that resolves when written.
   * @throws `RangeError` when the event is already stored, its ID is not a UUID or the
   *   envelope carries a token; `Error` when the write fails.
   */
  add(entry: InboxEntry): Promise<void> {
    return this.run(async () => {
      checkNew(entry);
      if (this.entries.has(entry.eventId))
        throw new RangeError("event already stored");
      const held = {
        entry: replaced(entry, { envelope: structuredClone(entry.envelope) }),
        arrival: this.nextArrival,
      };
      await this.writeEntry(held);
      this.nextArrival++;
    });
  }

  /**
   * Counts a redelivery; see {@link InboxStore.redelivered}. Durable when this resolves.
   *
   * @param eventId - The envelope `id`.
   * @param token - The token of the redelivery, if any.
   * @param iat - Its `iat`.
   * @param jti - Its `jti`.
   * @returns The entry after the update.
   * @throws `RangeError` when the event is not stored; `Error` when the write fails.
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
   * Replaces the token; durable when this resolves; see {@link InboxStore.replaceToken}.
   *
   * @param eventId - The envelope `id`.
   * @param token - The granted token.
   * @param iat - Its `iat`.
   * @param jti - Its `jti`.
   * @param replaces - The renewed token, if the replacement depends on it.
   * @returns The entry after the call.
   * @throws `RangeError` when the event is not stored; `Error` when the write fails.
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
   * Sets the state of an entry; durable when this resolves.
   *
   * @param eventId - The envelope `id`.
   * @param state - The new state.
   * @returns A promise that resolves when written.
   * @throws `RangeError` when the event is not stored; `Error` when the write fails.
   */
  async setState(eventId: string, state: EntryState): Promise<void> {
    await this.update(eventId, (e) => replaced(e, { state }));
  }

  /**
   * Returns the entries of a run in `seq` order (ephemeral entries last, in arrival order).
   *
   * @param runId - The run.
   * @returns The entries.
   * @throws `Error` when the directory cannot be opened or the store is closed.
   */
  forRun(runId: string): Promise<InboxEntry[]> {
    return this.list((e) => e.runId === runId);
  }

  /**
   * Returns the entries in one of `states` in `seq` order (ephemeral entries last, in arrival
   * order).
   *
   * @param states - The states asked for.
   * @returns The entries.
   * @throws `Error` when the directory cannot be opened or the store is closed.
   */
  inStates(...states: EntryState[]): Promise<InboxEntry[]> {
    return this.list((e) => states.includes(e.state));
  }

  /**
   * Drops the `jti` rows that expired at `nowMs`; inbox entries stay.
   *
   * @param nowMs - The current wall time.
   * @returns A promise that resolves when written.
   * @throws `Error` when the write fails or the store is closed.
   */
  prune(nowMs: number): Promise<void> {
    return this.run(async () => {
      const next = new Map([...this.jti].filter(([, until]) => nowMs < until));
      if (next.size !== this.jti.size) await this.writeJti(next);
    });
  }

  /**
   * Waits for pending writes and refuses later calls; closing twice is harmless.
   *
   * @returns A promise that resolves when every pending write ended.
   */
  async close(): Promise<void> {
    this.closed = true;
    await this.tail.catch(() => undefined);
  }

  private run<T>(fn: () => Promise<T>): Promise<T> {
    if (this.closed)
      return Promise.reject(new Error("the inbox store is closed"));
    const work = async (): Promise<T> => {
      this.ready ??= this.open();
      await this.ready;
      return fn();
    };
    const result = this.tail.then(work, work);
    this.tail = result.catch(() => undefined);
    return result;
  }

  private async update(
    eventId: string,
    change: (entry: InboxEntry) => InboxEntry,
  ): Promise<InboxEntry> {
    return this.run(async () => {
      const current = this.entries.get(eventId);
      if (current === undefined) throw notStored(eventId);
      const held = { entry: change(current.entry), arrival: current.arrival };
      await this.writeEntry(held);
      return held.entry;
    });
  }

  private list(keep: (entry: InboxEntry) => boolean): Promise<InboxEntry[]> {
    return this.run(() =>
      Promise.resolve(
        [...this.entries.values()]
          .sort((a, b) => a.arrival - b.arrival)
          .map((h) => h.entry)
          .filter(keep)
          .sort(bySeq),
      ),
    );
  }

  private async open(): Promise<void> {
    await mkdir(this.directory, { recursive: true, mode: DIRECTORY_MODE });
    const info = await lstat(this.directory);
    if (!info.isDirectory())
      throw new Error(`${this.directory} is not a directory`);
    if (process.platform !== "win32" && info.mode & OPEN_TO_OTHERS)
      throw new Error(
        `${this.directory} must not be accessible by group or others (mode 0700)`,
      );
    for (const name of await readdir(this.directory)) {
      const path = join(this.directory, name);
      if (name.startsWith(TMP_PREFIX)) {
        // Left by a write that never reached its rename; the old file is still in place.
        await unlink(path);
      } else if (ENTRY_FILE.test(name)) {
        const held = fromRecord(JSON.parse(await this.readPrivate(path)), name);
        this.entries.set(held.entry.eventId, held);
        this.nextArrival = Math.max(this.nextArrival, held.arrival + 1);
      } else if (name === JTI_FILE) {
        const stored = JSON.parse(await this.readPrivate(path)) as unknown;
        if (typeof stored !== "object" || stored === null)
          throw new Error(`unreadable ${JTI_FILE}`);
        for (const [jti, until] of Object.entries(stored))
          if (typeof until === "number") this.jti.set(jti, until);
      }
    }
  }

  private async readPrivate(path: string): Promise<string> {
    const handle = await open(
      path,
      constants.O_RDONLY | (constants.O_NOFOLLOW ?? 0),
    );
    try {
      const info = await handle.stat();
      if (!info.isFile()) throw new Error(`${path} is not a regular file`);
      if (process.platform !== "win32" && info.mode & OPEN_TO_OTHERS)
        throw new Error(
          `${path} must not be accessible by group or others (mode 0600)`,
        );
      return await handle.readFile("utf8");
    } finally {
      await handle.close();
    }
  }

  private async writeEntry(held: Held): Promise<void> {
    await this.writePrivate(
      entryFile(held.entry.eventId),
      JSON.stringify(toRecord(held.entry, held.arrival)),
    );
    this.entries.set(held.entry.eventId, held);
  }

  private async writeJti(next: Map<string, number>): Promise<void> {
    await this.writePrivate(JTI_FILE, JSON.stringify(Object.fromEntries(next)));
    this.jti = next;
  }

  private async writePrivate(name: string, text: string): Promise<void> {
    const temporary = join(this.directory, `${TMP_PREFIX}${randomUUID()}.json`);
    const handle = await open(
      temporary,
      constants.O_WRONLY |
        constants.O_CREAT |
        constants.O_EXCL |
        (constants.O_NOFOLLOW ?? 0),
      FILE_MODE,
    );
    try {
      try {
        await handle.writeFile(text, "utf8");
        await handle.sync();
      } finally {
        await handle.close();
      }
      await rename(temporary, join(this.directory, name));
    } catch (error) {
      await unlink(temporary).catch(() => undefined);
      throw error;
    }
    await this.syncDirectory();
  }

  private async syncDirectory(): Promise<void> {
    let handle;
    try {
      handle = await open(this.directory, constants.O_RDONLY);
      await handle.sync();
    } catch (error) {
      const code = (error as NodeJS.ErrnoException).code ?? "";
      if (!NO_DIRECTORY_SYNC.has(code)) throw error;
    } finally {
      await handle?.close();
    }
  }
}
