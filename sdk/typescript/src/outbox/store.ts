// SPDX-License-Identifier: Apache-2.0
/**
 * Storage of outbox entries (port of the stores of `bothy_sdk/outbox.py`).
 *
 * The {@link OutboxStore} is replaceable: {@link MemoryOutboxStore} for tests and
 * {@link FileOutboxStore}, a JSON file of mode `0600` in a directory of mode `0700`, written to
 * a temporary file, flushed to disk and renamed over the old one, so a crash leaves either the
 * old or the new state and never half of one. A sent entry is deleted; a failed one stays
 * visible in {@link OutboxStore.failed} (04 13.2 point 5: nothing is dropped silently).
 *
 * @module
 */

import { randomBytes } from "node:crypto";
import {
  constants,
  lstat,
  mkdir,
  open,
  readFile,
  rename,
  unlink,
} from "node:fs/promises";
import { dirname } from "node:path";

/** State of an outbox entry. */
export const OutboxState = Object.freeze({
  /** Stored and not yet accepted; sent again by `Outbox.flush`. */
  PENDING: "pending",
  /** Accepted by the server (`2xx`, also `Idempotent-Replay: true`); the store deletes it. */
  SENT: "sent",
  /** Refused for good; kept with its problem code. */
  FAILED: "failed",
} as const);

/** One of the values of {@link OutboxState}. */
export type OutboxState = (typeof OutboxState)[keyof typeof OutboxState];

/** The kinds of entries: `message` (a reply) or `call` (another write). */
export type OutboxEntryKind = "message" | "call";

/** One stored write. */
export interface OutboxEntry {
  /** The order of the entry (ascending per store; `0` before it is stored). */
  readonly entryId: number;
  /** The thread whose order it keeps (FIFO). */
  readonly threadId: string;
  /** `message` (a reply) or `call` (another write). */
  readonly kind: OutboxEntryKind;
  /** The `client_msg_id` of a reply or the `Idempotency-Key` of a call. */
  readonly key: string;
  /**
   * The stored request: the `MessageCreate` body of a reply, or method, path, parameters and
   * body of a call. Plain JSON.
   */
  readonly payload: Readonly<Record<string, unknown>>;
  /** Pending, sent or failed. */
  readonly state: OutboxState;
  /** The problem code of a failed entry. */
  readonly problemCode?: string | undefined;
  /**
   * The run whose binding the write goes out under (07 SR9): the run passed by the caller or,
   * when none was passed, the run the client was bound to when the entry was stored; absent for
   * a write made outside any run.
   */
  readonly runId?: string | undefined;
}

/** Durable storage of outbox entries. */
export interface OutboxStore {
  /**
   * Stores a new entry durably before it is sent.
   *
   * @param entry - The entry; its `entryId` is ignored.
   * @returns The entry with its `entryId`.
   * @throws `Error` when an entry with the same key is already stored.
   */
  add(entry: OutboxEntry): Promise<OutboxEntry>;
  /**
   * Returns every pending entry.
   *
   * @returns The pending entries in `entryId` order.
   */
  pending(): Promise<OutboxEntry[]>;
  /**
   * Changes the state of an entry; `sent` deletes it.
   *
   * @param entryId - The entry.
   * @param state - The new state.
   * @param problemCode - The problem code of a failed entry.
   */
  mark(
    entryId: number,
    state: OutboxState,
    problemCode?: string,
  ): Promise<void>;
  /**
   * Returns every failed entry.
   *
   * @returns The failed entries in `entryId` order.
   */
  failed(): Promise<OutboxEntry[]>;
  /** Releases the storage after the operations in flight. */
  close(): Promise<void>;
}

/** The entries of a store with the next `entryId`. */
interface Contents {
  next: number;
  entries: Map<number, OutboxEntry>;
}

function addTo(contents: Contents, entry: OutboxEntry): OutboxEntry {
  for (const stored of contents.entries.values())
    if (stored.key === entry.key)
      throw new Error(`outbox entry ${stored.entryId}: key already stored`);
  const stored = { ...entry, entryId: contents.next };
  contents.entries.set(stored.entryId, stored);
  contents.next += 1;
  return stored;
}

function markIn(
  contents: Contents,
  entryId: number,
  state: OutboxState,
  problemCode: string | undefined,
): void {
  const entry = contents.entries.get(entryId);
  if (entry === undefined) return;
  if (state === OutboxState.SENT) {
    contents.entries.delete(entryId);
    return;
  }
  const changed: OutboxEntry = { ...entry, state };
  contents.entries.set(
    entryId,
    problemCode === undefined
      ? withoutProblem(changed)
      : { ...changed, problemCode },
  );
}

function withoutProblem(entry: OutboxEntry): OutboxEntry {
  const copy: { -readonly [K in keyof OutboxEntry]?: OutboxEntry[K] } = {
    ...entry,
  };
  delete copy.problemCode;
  return copy as OutboxEntry;
}

function inState(contents: Contents, state: OutboxState): OutboxEntry[] {
  return [...contents.entries.values()]
    .filter((e) => e.state === state)
    .sort((a, b) => a.entryId - b.entryId);
}

/** An outbox store in memory (tests; not durable). */
export class MemoryOutboxStore implements OutboxStore {
  private readonly contents: Contents = { next: 1, entries: new Map() };

  /**
   * Stores `entry` with the next `entryId`.
   *
   * @param entry - The entry.
   * @returns The stored entry.
   * @throws `Error` when the key is already stored.
   */
  add(entry: OutboxEntry): Promise<OutboxEntry> {
    try {
      return Promise.resolve(addTo(this.contents, entry));
    } catch (error) {
      return Promise.reject(error as Error);
    }
  }

  /**
   * Returns the pending entries in order.
   *
   * @returns The entries.
   */
  pending(): Promise<OutboxEntry[]> {
    return Promise.resolve(inState(this.contents, OutboxState.PENDING));
  }

  /**
   * Changes the state of an entry; sent deletes it.
   *
   * @param entryId - The entry.
   * @param state - The new state.
   * @param problemCode - The problem code of a failed entry.
   */
  mark(
    entryId: number,
    state: OutboxState,
    problemCode?: string,
  ): Promise<void> {
    markIn(this.contents, entryId, state, problemCode);
    return Promise.resolve();
  }

  /**
   * Returns the failed entries in order.
   *
   * @returns The entries.
   */
  failed(): Promise<OutboxEntry[]> {
    return Promise.resolve(inState(this.contents, OutboxState.FAILED));
  }

  /** Nothing to release. */
  close(): Promise<void> {
    return Promise.resolve();
  }
}

const FILE_VERSION = 1;
const PRIVATE_FILE = 0o600;
const PRIVATE_DIR = 0o700;
// Group and other bits: a store file must be readable by its owner only.
const SHARED_BITS = 0o077;

/**
 * The durable outbox store: one JSON file of mode `0600`, usually `<state_dir>/outbox.json`.
 *
 * Its directory is created with mode `0700` when missing. Every change writes a temporary file
 * of mode `0600` next to it, flushes it to disk, renames it over the store and flushes the
 * directory, so `add` returns only after the entry survives a crash. An existing file must be a
 * regular file (no symbolic link) without group or other permissions. Operations run one after
 * another; one process uses one store file.
 *
 * @example
 * ```ts
 * const outbox = new Outbox(api, { store: new FileOutboxStore("/var/lib/lotse/outbox.json") });
 * await outbox.flush(); // at the start: what a previous process left pending goes out now
 * ```
 */
export class FileOutboxStore implements OutboxStore {
  /** The store file. */
  readonly path: string;
  private contents: Contents | undefined;
  private queue: Promise<unknown> = Promise.resolve();

  /**
   * Creates the store; the file is read on first use.
   *
   * @param path - The store file.
   */
  constructor(path: string) {
    this.path = path;
  }

  /**
   * Stores `entry` durably and returns it with its `entryId`.
   *
   * @param entry - The entry.
   * @returns The stored entry.
   * @throws `Error` when the key is already stored or the file cannot be read or written.
   */
  add(entry: OutboxEntry): Promise<OutboxEntry> {
    return this.serial(async (contents) => {
      const stored = addTo(contents, entry);
      try {
        await this.write(contents);
      } catch (error) {
        contents.entries.delete(stored.entryId);
        contents.next -= 1;
        throw error;
      }
      return stored;
    });
  }

  /**
   * Returns the pending entries in order.
   *
   * @returns The entries.
   * @throws `Error` when the file cannot be read or is shared with others.
   */
  pending(): Promise<OutboxEntry[]> {
    return this.serial((contents) =>
      Promise.resolve(inState(contents, OutboxState.PENDING)),
    );
  }

  /**
   * Changes the state of an entry durably; sent deletes it.
   *
   * @param entryId - The entry.
   * @param state - The new state.
   * @param problemCode - The problem code of a failed entry.
   */
  mark(
    entryId: number,
    state: OutboxState,
    problemCode?: string,
  ): Promise<void> {
    return this.serial(async (contents) => {
      if (!contents.entries.has(entryId)) return;
      markIn(contents, entryId, state, problemCode);
      await this.write(contents);
    });
  }

  /**
   * Returns the failed entries in order.
   *
   * @returns The entries.
   */
  failed(): Promise<OutboxEntry[]> {
    return this.serial((contents) =>
      Promise.resolve(inState(contents, OutboxState.FAILED)),
    );
  }

  /** Waits for the operations in flight and drops the cached state; a later call reads again. */
  async close(): Promise<void> {
    await this.queue.catch(() => undefined);
    this.contents = undefined;
  }

  private serial<T>(work: (contents: Contents) => Promise<T>): Promise<T> {
    const run = this.queue
      .catch(() => undefined)
      .then(async () => work(await this.load()));
    this.queue = run;
    return run;
  }

  private async load(): Promise<Contents> {
    if (this.contents !== undefined) return this.contents;
    let text: string | undefined;
    try {
      const info = await lstat(this.path);
      if (!info.isFile())
        throw new Error("outbox store must be a regular file");
      if (process.platform !== "win32" && (info.mode & SHARED_BITS) !== 0)
        throw new Error(
          "outbox store must not be accessible by group or others (mode 0600)",
        );
      text = await readFile(this.path, "utf8");
    } catch (error) {
      if ((error as NodeJS.ErrnoException).code !== "ENOENT") throw error;
    }
    this.contents =
      text === undefined ? { next: 1, entries: new Map() } : parse(text);
    return this.contents;
  }

  private async write(contents: Contents): Promise<void> {
    const dir = dirname(this.path);
    await mkdir(dir, { recursive: true, mode: PRIVATE_DIR });
    const data = JSON.stringify({
      version: FILE_VERSION,
      next: contents.next,
      entries: [...contents.entries.values()],
    });
    const temp = `${this.path}.${randomBytes(6).toString("hex")}.tmp`;
    const handle = await open(
      temp,
      constants.O_WRONLY | constants.O_CREAT | constants.O_EXCL,
      PRIVATE_FILE,
    );
    try {
      await handle.writeFile(data, "utf8");
      await handle.sync();
    } catch (error) {
      await handle.close();
      await unlink(temp).catch(() => undefined);
      throw error;
    }
    await handle.close();
    await rename(temp, this.path);
    await syncDir(dir);
  }
}

/** Flushes a directory entry to disk where the platform allows it. */
async function syncDir(dir: string): Promise<void> {
  if (process.platform === "win32") return;
  const handle = await open(dir, constants.O_RDONLY);
  try {
    await handle.sync();
  } finally {
    await handle.close();
  }
}

function parse(text: string): Contents {
  const data = JSON.parse(text) as {
    version?: unknown;
    next?: unknown;
    entries?: unknown;
  };
  if (
    data.version !== FILE_VERSION ||
    typeof data.next !== "number" ||
    !Array.isArray(data.entries)
  )
    throw new Error("outbox store has an unknown format");
  const entries = new Map<number, OutboxEntry>();
  for (const entry of data.entries as OutboxEntry[])
    entries.set(entry.entryId, entry);
  return { next: data.next, entries };
}
