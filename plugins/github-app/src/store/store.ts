// SPDX-License-Identifier: Apache-2.0
/**
 * The port `Store` of the app's state and queue (brief M2-49 Design note 8).
 *
 * The state holds the thread map in both directions, the drafts of `/issue`, the GitHub objects
 * the app created and the seen delivery identifiers with their time; the queue holds one record
 * per job. Every change goes through one in-process mutex, and every write prunes seen
 * identifiers, created objects and drafts older than `github_app.dedupe_window`; the thread map
 * stays. {@link StoreBase} implements the rules once; `FileStore` and `MemoryStore` only persist.
 *
 * @example
 * ```ts
 * const admitted = await store.admit({ seenKey: "gh:<delivery>", status: 202, job, maxJobs });
 * if (admitted.kind === "duplicate") return reply(admitted.status);
 * ```
 *
 * @module
 */

import type { Clock } from "bothy-sdk";

/** JSON values a job may hold. */
export type JsonValue =
  | string
  | number
  | boolean
  | readonly JsonValue[]
  | { readonly [key: string]: JsonValue };

/** The thread of an issue or pull request. */
export interface ThreadRef {
  /** The thread. */
  readonly thread_id: string;
  /** Its forum. */
  readonly channel_id: string;
}

/** An open form of `/issue` (Design note 11). */
export interface Draft {
  /** The draft ID, part of the form's `custom_id`. */
  readonly id: string;
  /** The forum post of the draft. */
  readonly thread_id: string;
  /** The forum of the post. */
  readonly channel_id: string;
  /** The message of the current form card. */
  readonly card_message_id: string;
  /** The title from the command option, cleaned. */
  readonly title: string;
  /** The repositories the form offers. */
  readonly repositories: readonly string[];
  /** Creation time, Unix milliseconds; drafts older than the window are pruned. */
  readonly created_at: number;
}

/** A remembered delivery identifier and the status it was answered with. */
export interface Seen {
  /** Unix milliseconds of the first receipt. */
  readonly at: number;
  /** The HTTP status of the first answer. */
  readonly status: number;
}

/** The persistent state besides the queue. */
export interface StateData {
  /** Key of an issue or pull request to its thread. */
  threads: Record<string, ThreadRef>;
  /** Thread to key. */
  thread_keys: Record<string, string>;
  /** Draft ID to draft. */
  drafts: Record<string, Draft>;
  /** Key of a GitHub object the app created (`createdKey`) to its creation time. */
  created: Record<string, number>;
  /** `gh:<X-GitHub-Delivery>` or `wh:<webhook-id>` to its first receipt. */
  seen: Record<string, Seen>;
  /** The sequence number of the next job. */
  next_seq: number;
}

/** One queued job. */
export interface JobRecord {
  /** The job ID (a UUID). */
  readonly id: string;
  /** Creation order; the worker takes the lowest first. */
  readonly seq: number;
  /** The kind (`worker/jobs.ts`). */
  readonly kind: string;
  /** Creation time, Unix milliseconds. */
  readonly created_at: number;
  /** The identifier that admitted the job; restored into `seen` after a crash. */
  readonly seen_key?: string | undefined;
  /** The input of the job. */
  readonly data: JsonValue;
  /** The progress per step, written before and after each step. */
  progress: Record<string, JsonValue>;
}

/** A job before it is admitted: everything but `seq`. */
export type NewJob = Omit<JobRecord, "seq">;

/** The input of {@link Store.admit}. */
export interface AdmitInput {
  /** The identifier to remember; a repeat is a duplicate. */
  readonly seenKey: string;
  /** The status the first receipt answers with. */
  readonly status: number;
  /** The job to queue, if any. */
  readonly job?: NewJob | undefined;
  /** `github_app.max_queued_jobs`. */
  readonly maxJobs: number;
}

/** What {@link Store.admit} did. */
export type AdmitResult =
  | {
      /** The identifier was remembered and the job queued. */
      readonly kind: "accepted";
    }
  | {
      /** The identifier is known. */
      readonly kind: "duplicate";
      /** The status of the first answer. */
      readonly status: number;
    }
  | {
      /** The queue holds `github_app.max_queued_jobs` jobs. */
      readonly kind: "full";
    };

/** What a store loads at `open`. */
export interface Loaded {
  /** The written state, `undefined` before the first write. */
  readonly state: StateData | undefined;
  /** The queued jobs. */
  readonly jobs: JobRecord[];
}

/** The state and queue of the app. */
export interface Store {
  /**
   * Opens the store: takes the lock (`FileStore`) and loads the state and the queue.
   *
   * @throws `StoreLockedError` when another live process holds the lock.
   */
  open(): Promise<void>;
  /** Releases the lock; the store is unusable afterwards. */
  close(): Promise<void>;
  /**
   * Returns the current state. Treat it as read-only; change it through
   * {@link Store.update}.
   *
   * @returns The state.
   */
  state(): Readonly<StateData>;
  /**
   * Changes the state under the mutex and persists it.
   *
   * @param change - Mutates the state and returns a value.
   * @returns The value of `change`.
   */
  update<T>(change: (state: StateData) => T): Promise<T>;
  /**
   * Remembers `seenKey` and queues `job` in one write, unless the key is known (duplicate) or
   * the queue is full.
   *
   * @param input - Key, status, job and bound.
   * @returns The outcome.
   */
  admit(input: AdmitInput): Promise<AdmitResult>;
  /**
   * Returns the queued jobs in creation order.
   *
   * @returns Copies of the records.
   */
  jobs(): JobRecord[];
  /**
   * Persists the progress of a queued job.
   *
   * @param job - The job with its new progress.
   */
  saveJob(job: JobRecord): Promise<void>;
  /**
   * Removes a finished job.
   *
   * @param id - The job ID.
   */
  removeJob(id: string): Promise<void>;
}

/** Another live process holds the lock of the state directory. */
export class StoreLockedError extends Error {
  /** Creates the error. */
  constructor() {
    super("the state directory is locked by another running process");
    this.name = "StoreLockedError";
  }
}

/** An empty state. */
export function emptyState(): StateData {
  return {
    threads: {},
    thread_keys: {},
    drafts: {},
    created: {},
    seen: {},
    next_seq: 1,
  };
}

function clone<T>(value: T): T {
  return structuredClone(value);
}

/**
 * The rules of {@link Store} on top of four persistence hooks: one mutex, pruning at each write,
 * the seen identifiers of queued jobs restored after a crash.
 */
export abstract class StoreBase implements Store {
  /** The current state. */
  protected data: StateData = emptyState();
  /** The queue, keyed by job ID. */
  protected queue = new Map<string, JobRecord>();
  #mutex: Promise<unknown> = Promise.resolve();

  /**
   * Creates the base.
   *
   * @param clock - Source of `now` for pruning and receipt times.
   * @param windowMs - `github_app.dedupe_window`.
   */
  constructor(
    protected readonly clock: Pick<Clock, "now">,
    protected readonly windowMs: number,
  ) {}

  /**
   * Loads what was persisted.
   *
   * @returns The state, if one was written, and the queued jobs.
   */
  protected abstract load(): Promise<Loaded>;
  /**
   * Writes the state atomically.
   *
   * @param state - The state to write.
   */
  protected abstract writeState(state: StateData): Promise<void>;
  /**
   * Writes one job atomically.
   *
   * @param job - The job.
   */
  protected abstract writeJob(job: JobRecord): Promise<void>;
  /**
   * Deletes one job.
   *
   * @param job - The job.
   */
  protected abstract deleteJob(job: JobRecord): Promise<void>;

  /** Opens the store (see `Store.open`). */
  async open(): Promise<void> {
    const loaded = await this.load();
    this.data = loaded.state ?? emptyState();
    this.queue = new Map();
    for (const job of loaded.jobs) {
      this.queue.set(job.id, job);
      // A crash between the job file and the state file leaves the job without its seen key.
      if (job.seen_key !== undefined && !(job.seen_key in this.data.seen))
        this.data.seen[job.seen_key] = { at: job.created_at, status: 202 };
      if (job.seq >= this.data.next_seq) this.data.next_seq = job.seq + 1;
    }
  }

  /** Releases the store (see `Store.close`). */
  close(): Promise<void> {
    return Promise.resolve();
  }

  /** Returns the current state (see `Store.state`). */
  state(): Readonly<StateData> {
    return this.data;
  }

  /**
   * Runs `fn` alone: one writer at a time.
   *
   * @param fn - The critical section.
   * @returns Its result.
   */
  protected exclusive<T>(fn: () => Promise<T>): Promise<T> {
    const run = this.#mutex.then(fn, fn);
    this.#mutex = run.catch(() => undefined);
    return run;
  }

  /** Removes seen identifiers, created objects and drafts older than the window. */
  protected prune(state: StateData): void {
    const oldest = this.clock.now() - this.windowMs;
    for (const [k, v] of Object.entries(state.seen))
      if (v.at < oldest) delete state.seen[k];
    for (const [k, at] of Object.entries(state.created))
      if (at < oldest) delete state.created[k];
    for (const [k, d] of Object.entries(state.drafts))
      if (d.created_at < oldest) delete state.drafts[k];
  }

  /** Changes the state under the mutex (see `Store.update`). */
  update<T>(change: (state: StateData) => T): Promise<T> {
    return this.exclusive(async () => {
      const next = clone(this.data);
      const result = change(next);
      this.prune(next);
      await this.writeState(next);
      this.data = next;
      return result;
    });
  }

  /** Remembers an identifier and queues a job in one write (see `Store.admit`). */
  admit(input: AdmitInput): Promise<AdmitResult> {
    return this.exclusive(async (): Promise<AdmitResult> => {
      const known = this.data.seen[input.seenKey];
      if (known !== undefined && known.at >= this.clock.now() - this.windowMs)
        return { kind: "duplicate", status: known.status };
      if (input.job !== undefined && this.queue.size >= input.maxJobs)
        return { kind: "full" };
      const next = clone(this.data);
      next.seen[input.seenKey] = { at: this.clock.now(), status: input.status };
      if (input.job !== undefined) {
        const job: JobRecord = {
          ...clone(input.job),
          seq: next.next_seq,
          seen_key: input.seenKey,
        };
        next.next_seq += 1;
        // The job first: a crash before the state write still restores its seen key.
        await this.writeJob(job);
        this.queue.set(job.id, job);
      }
      this.prune(next);
      await this.writeState(next);
      this.data = next;
      return { kind: "accepted" };
    });
  }

  /** Returns the queued jobs in creation order (see `Store.jobs`). */
  jobs(): JobRecord[] {
    return [...this.queue.values()]
      .sort((a, b) => a.seq - b.seq)
      .map((j) => clone(j));
  }

  /** Persists the progress of a job (see `Store.saveJob`). */
  saveJob(job: JobRecord): Promise<void> {
    return this.exclusive(async () => {
      if (!this.queue.has(job.id)) return;
      const copy = clone(job);
      await this.writeJob(copy);
      this.queue.set(job.id, copy);
    });
  }

  /** Removes a finished job (see `Store.removeJob`). */
  removeJob(id: string): Promise<void> {
    return this.exclusive(async () => {
      const job = this.queue.get(id);
      if (job === undefined) return;
      await this.deleteJob(job);
      this.queue.delete(id);
    });
  }
}
