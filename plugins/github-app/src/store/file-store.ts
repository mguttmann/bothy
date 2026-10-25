// SPDX-License-Identifier: Apache-2.0
/**
 * `FileStore`: the `Store` in `github_app.state_dir` (brief M2-49 Design note 8).
 *
 * `state.json` holds the state and `jobs/<seq>.json` one job each. Every write goes to a
 * temporary file, is synced, renamed over the target and followed by a sync of the directory;
 * files have mode `0600`, directories `0700`. The file `lock`, created exclusively with the PID,
 * keeps a second process away; a lock whose PID is not alive is taken over with a Warn line.
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
    await unlink(join(this.#dir, LOCK_FILE)).catch(() => undefined);
  }

  async #lock(): Promise<void> {
    const path = join(this.#dir, LOCK_FILE);
    for (let attempt = 0; attempt < 3; attempt++) {
      try {
        const handle = await open(path, "wx", 0o600);
        try {
          await handle.writeFile(String(process.pid), "utf8");
          await handle.sync();
        } finally {
          await handle.close();
        }
        this.#locked = true;
        return;
      } catch (err) {
        if ((err as NodeJS.ErrnoException).code !== "EEXIST") throw err;
      }
      // Claim the existing lock by renaming it to a name of our own: only one process can
      // rename a given file, so two starts never both take over the same stale lock, and the
      // PID is read from the file that was claimed, not from whatever the path holds later.
      const claimed = `${path}.claim-${randomUUID()}`;
      try {
        await rename(path, claimed);
      } catch (err) {
        if ((err as NodeJS.ErrnoException).code === "ENOENT") continue;
        throw err;
      }
      const holder = Number((await readFile(claimed, "utf8")).trim());
      if (processAlive(holder)) {
        // A live lock: put it back (unless a new one appeared meanwhile) and give up.
        await link(claimed, path).catch(() => undefined);
        await unlink(claimed);
        throw new StoreLockedError();
      }
      this.#logger.warn("store.lock.taken_over", { reason: "stale_lock" });
      await unlink(claimed);
    }
    throw new StoreLockedError();
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
