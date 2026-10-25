// SPDX-License-Identifier: Apache-2.0
/**
 * `FileStore`: the `Store` in `github_app.state_dir` (brief M2-49 Design note 8).
 *
 * `state.json` holds the state and `jobs/<seq>.json` one job each. Every write goes to a
 * temporary file, is synced, renamed over the target and followed by a sync of the directory;
 * files have mode `0600`, directories `0700`. The file `lock` keeps a second store away: it is
 * created atomically with the PID inside (a temporary file linked to the name, so it is never
 * seen empty), a second store of the same process is refused before any file is touched, and a
 * lock whose PID is not alive is taken over with a Warn line only under the takeover mutex
 * `lock.takeover`, so a live lock is never removed (docs/decisions.d/M2-49.md D-M2-49-7).
 *
 * @example
 * ```ts
 * const store = new FileStore({ dir, clock, windowMs, logger });
 * await store.open(); // StoreLockedError when another process runs on dir
 * ```
 *
 * @module
 */

import { randomUUID } from "node:crypto";
import {
  link,
  mkdir,
  open,
  readFile,
  readdir,
  rename,
  unlink,
} from "node:fs/promises";
import { join } from "node:path";

import type { Clock, Logger } from "bothy-sdk";

import {
  StoreBase,
  StoreLockedError,
  type JobRecord,
  type Loaded,
  type StateData,
} from "./store.js";

const STATE_FILE = "state.json";
const JOBS_DIR = "jobs";
const LOCK_FILE = "lock";
const TEMP_MARK = ".tmp-";
// The lock files the stores of this process hold, so a lock with the own PID is told apart
// from one an earlier process with the same PID left behind.
const OWN_LOCKS = new Set<string>();
const TAKEOVER_SUFFIX = ".takeover";

/** Options of {@link FileStore}. */
export interface FileStoreOptions {
  /** The state directory (created and checked by `prepareStateDir`). */
  readonly dir: string;
  /** Source of `now`. */
  readonly clock: Pick<Clock, "now">;
  /** `github_app.dedupe_window` in milliseconds. */
  readonly windowMs: number;
  /** Receives the Warn line of a lock take-over. */
  readonly logger: Logger;
}

/**
 * Reports whether a process with this PID runs (a signal 0 reaches it or is refused for lack of
 * rights).
 *
 * @param pid - The PID.
 * @returns `true` when the process is alive.
 */
export function processAlive(pid: number): boolean {
  if (!Number.isSafeInteger(pid) || pid <= 0) return false;
  try {
    process.kill(pid, 0);
    return true;
  } catch (err) {
    return (err as NodeJS.ErrnoException).code === "EPERM";
  }
}

function ignoreMissing(err: unknown): void {
  if ((err as NodeJS.ErrnoException).code !== "ENOENT") throw err;
}

/**
 * Reads the PID of a lock file.
 *
 * @param path - The file.
 * @returns The PID, `NaN` for unreadable content, `undefined` when the file is gone.
 */
async function readPid(path: string): Promise<number | undefined> {
  try {
    return Number((await readFile(path, "utf8")).trim());
  } catch (err) {
    ignoreMissing(err);
    return undefined;
  }
}

async function syncDir(dir: string): Promise<void> {
  const handle = await open(dir, "r");
  try {
    await handle.sync();
  } finally {
    await handle.close();
  }
}

/**
 * Writes `content` to `file` atomically: temporary file, sync, rename, sync of the directory.
 *
 * @param dir - The directory of the file.
 * @param file - The target path.
 * @param content - The new content.
 */
export async function writeAtomic(
  dir: string,
  file: string,
  content: string,
): Promise<void> {
  const temp = `${file}${TEMP_MARK}${randomUUID()}`;
  const handle = await open(temp, "wx", 0o600);
  try {
    await handle.writeFile(content, "utf8");
    await handle.sync();
  } finally {
    await handle.close();
  }
  await rename(temp, file);
  await syncDir(dir);
}

function jobFile(seq: number): string {
  return `${String(seq).padStart(12, "0")}.json`;
}

/** A `Store` in the state directory. */
export class FileStore extends StoreBase {
  readonly #dir: string;
  readonly #jobs: string;
  readonly #logger: Logger;
  #locked = false;

  /**
   * Creates the store; nothing is read or locked before {@link FileStore.open}.
   *
   * @param options - Directory, clock, window and logger.
   */
  constructor(options: FileStoreOptions) {
    super(options.clock, options.windowMs);
    this.#dir = options.dir;
    this.#jobs = join(options.dir, JOBS_DIR);
    this.#logger = options.logger;
  }

  /**
   * Takes the lock, then loads the state and the queue.
   *
   * @throws `StoreLockedError` when a live process holds the lock.
   */
  override async open(): Promise<void> {
    await this.#lock();
    try {
      await mkdir(this.#jobs, { mode: 0o700, recursive: true });
      await super.open();
    } catch (err) {
      await this.close();
      throw err;
    }
  }

  /** Releases the lock. */
  override async close(): Promise<void> {
    if (!this.#locked) return;
    this.#locked = false;
    const path = join(this.#dir, LOCK_FILE);
    OWN_LOCKS.delete(path);
    await unlink(path).catch(() => undefined);
  }

  async #lock(): Promise<void> {
    const path = join(this.#dir, LOCK_FILE);
    // A second store of this process: refused at once, before any file is touched, so two
    // starts in one process never race on the files. The entry stays while the lock is held.
    if (OWN_LOCKS.has(path)) throw new StoreLockedError();
    OWN_LOCKS.add(path);
    try {
      if (await this.#create(path)) {
        this.#locked = true;
        return;
      }
      if (this.#live(await readPid(path))) throw new StoreLockedError();
      await this.#takeOver(path);
      this.#locked = true;
    } catch (err) {
      OWN_LOCKS.delete(path);
      throw err;
    }
  }

  /**
   * Replaces a stale lock under the takeover mutex. Only a holder of the mutex removes a lock,
   * and only after it read the lock again: while the lock exists nobody else can create one,
   * so the lock it removes is the stale one it read.
   */
  async #takeOver(path: string): Promise<void> {
    const mutex = `${path}${TAKEOVER_SUFFIX}`;
    if (!(await this.#create(mutex))) {
      // A mutex left by a start that died during a takeover is removed once; a live one wins.
      if (this.#live(await readPid(mutex))) throw new StoreLockedError();
      await unlink(mutex).catch(ignoreMissing);
      if (!(await this.#create(mutex))) throw new StoreLockedError();
    }
    try {
      if (this.#live(await readPid(path))) throw new StoreLockedError();
      this.#logger.warn("store.lock.taken_over", { reason: "stale_lock" });
      await unlink(path).catch(ignoreMissing);
      if (!(await this.#create(path))) throw new StoreLockedError();
    } finally {
      await unlink(mutex).catch(ignoreMissing);
    }
  }

  /**
   * Creates `path` with the own PID atomically: the PID goes into a temporary file that is
   * linked to `path`, so the file is never seen empty.
   *
   * @returns `false` when `path` exists.
   */
  async #create(path: string): Promise<boolean> {
    const temp = `${path}${TEMP_MARK}${randomUUID()}`;
    const handle = await open(temp, "wx", 0o600);
    try {
      await handle.writeFile(String(process.pid), "utf8");
      await handle.sync();
    } finally {
      await handle.close();
    }
    try {
      await link(temp, path);
      await syncDir(this.#dir);
      return true;
    } catch (err) {
      if ((err as NodeJS.ErrnoException).code === "EEXIST") return false;
      throw err;
    } finally {
      await unlink(temp).catch(ignoreMissing);
    }
  }

  /**
   * Reports whether a lock or mutex with PID `holder` belongs to a running store. A file with
   * the own PID cannot belong to a store of this process (those are refused before any file is
   * touched), so it was left by an earlier process with the same PID, for example a container
   * restarted after a kill.
   */
  #live(holder: number | undefined): boolean {
    if (holder === undefined || holder === process.pid) return false;
    return processAlive(holder);
  }

  /** Loads what was persisted (see `StoreBase.load`). */
  protected async load(): Promise<Loaded> {
    let state: StateData | undefined;
    try {
      state = JSON.parse(
        await readFile(join(this.#dir, STATE_FILE), "utf8"),
      ) as StateData;
    } catch (err) {
      if ((err as NodeJS.ErrnoException).code !== "ENOENT") throw err;
    }
    const jobs: JobRecord[] = [];
    for (const name of await readdir(this.#jobs)) {
      const path = join(this.#jobs, name);
      // A temporary file of an interrupted write was never renamed: drop it.
      if (name.includes(TEMP_MARK)) {
        await unlink(path).catch(() => undefined);
        continue;
      }
      if (!name.endsWith(".json")) continue;
      jobs.push(JSON.parse(await readFile(path, "utf8")) as JobRecord);
    }
    return { state, jobs };
  }

  /** Writes the state (see `StoreBase.writeState`). */
  protected writeState(state: StateData): Promise<void> {
    return writeAtomic(
      this.#dir,
      join(this.#dir, STATE_FILE),
      JSON.stringify(state),
    );
  }

  /** Writes one job (see `StoreBase.writeJob`). */
  protected writeJob(job: JobRecord): Promise<void> {
    return writeAtomic(
      this.#jobs,
      join(this.#jobs, jobFile(job.seq)),
      JSON.stringify(job),
    );
  }

  /** Deletes one job (see `StoreBase.deleteJob`). */
  protected async deleteJob(job: JobRecord): Promise<void> {
    await unlink(join(this.#jobs, jobFile(job.seq)));
    await syncDir(this.#jobs);
  }
}
