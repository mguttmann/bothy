// SPDX-License-Identifier: Apache-2.0
/**
 * `MemoryStore`: the `Store` of unit tests, persisted in a {@link MemoryBacking} that
 * survives the store, so a test can open a second store on the same backing as after a restart
 * (brief M2-49 Design note 8). It holds copies, like files would.
 *
 * @example
 * ```ts
 * const backing = new MemoryBacking();
 * const first = new MemoryStore(backing, clock, windowMs);
 * await first.open();
 * ```
 *
 * @module
 */

import type { Clock } from "bothy-sdk";

import {
  StoreBase,
  type JobRecord,
  type Loaded,
  type StateData,
} from "./store.js";

/** The persisted content of a {@link MemoryStore}. */
export class MemoryBacking {
  /** The written state, if any. */
  state: StateData | undefined;
  /** The written jobs by ID. */
  readonly jobs = new Map<string, JobRecord>();
}

/** A `Store` in memory. */
export class MemoryStore extends StoreBase {
  /**
   * Creates the store; nothing is read before `open`.
   *
   * @param backing - Where the state and the jobs are kept.
   * @param clock - Source of `now`.
   * @param windowMs - `github_app.dedupe_window`.
   */
  constructor(
    readonly backing: MemoryBacking,
    clock: Pick<Clock, "now">,
    windowMs: number,
  ) {
    super(clock, windowMs);
  }

  /** Loads what was persisted (see `StoreBase.load`). */
  protected load(): Promise<Loaded> {
    return Promise.resolve({
      state: structuredClone(this.backing.state),
      jobs: [...this.backing.jobs.values()].map((j) => structuredClone(j)),
    });
  }

  /** Writes the state (see `StoreBase.writeState`). */
  protected writeState(state: StateData): Promise<void> {
    this.backing.state = structuredClone(state);
    return Promise.resolve();
  }

  /** Writes one job (see `StoreBase.writeJob`). */
  protected writeJob(job: JobRecord): Promise<void> {
    this.backing.jobs.set(job.id, structuredClone(job));
    return Promise.resolve();
  }

  /** Deletes one job (see `StoreBase.deleteJob`). */
  protected deleteJob(job: JobRecord): Promise<void> {
    this.backing.jobs.delete(job.id);
    return Promise.resolve();
  }
}
