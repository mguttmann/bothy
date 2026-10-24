// SPDX-License-Identifier: Apache-2.0
/**
 * The worker of the queue (brief M2-49 Design note 8): one job at a time, strictly in creation
 * order, so events of one issue keep their order and a `/issue` job records its issue before
 * GitHub's `opened` event of that issue is handled. A finished job is deleted; a job that met a
 * transient failure stays queued and the worker pauses `sdk.retry_backoff_cap` before it tries
 * again. {@link Worker.stop} lets the running step finish.
 *
 * @example
 * ```ts
 * const worker = new Worker({ store, run: (job) => runner.run(job), clock, pauseMs, logger });
 * worker.start();
 * worker.wake(); // after a job was queued
 * await worker.stop();
 * ```
 *
 * @module
 */

import type { Clock, Logger } from "bothy-sdk";

import type { JobRecord, Store } from "../store/store.js";
import type { JobOutcome } from "./jobs.js";

/** Options of {@link Worker}. */
export interface WorkerOptions {
  /** State and queue. */
  readonly store: Store;
  /** Runs one job. */
  readonly run: (job: JobRecord) => Promise<JobOutcome>;
  /** Source of the pause after a transient failure. */
  readonly clock: Pick<Clock, "sleep">;
  /** `sdk.retry_backoff_cap` in milliseconds. */
  readonly pauseMs: number;
  /** The app's logger. */
  readonly logger: Logger;
}

/** Works the queue of a store. */
export class Worker {
  readonly #options: WorkerOptions;
  readonly #stopping = new AbortController();
  #loop: Promise<void> | undefined;
  #wakeUp: (() => void) | undefined;
  #pending = false;
  #idleWaiters: (() => void)[] = [];

  /**
   * Creates the worker; nothing runs before {@link Worker.start}.
   *
   * @param options - Store, job function, clock, pause and logger.
   */
  constructor(options: WorkerOptions) {
    this.#options = options;
  }

  /** Starts the loop. */
  start(): void {
    this.#loop ??= this.#work();
  }

  /** Tells the worker that a job was queued. */
  wake(): void {
    this.#pending = true;
    const wake = this.#wakeUp;
    this.#wakeUp = undefined;
    wake?.();
  }

  /**
   * Resolves once the queue is empty and the worker waits for work (tests and shutdown).
   *
   * @returns A promise of the next idle moment.
   */
  idle(): Promise<void> {
    if (this.#loop === undefined) return Promise.resolve();
    return new Promise((resolve) => {
      this.#idleWaiters.push(resolve);
      // A worker that already waits is idle now.
      if (this.#wakeUp !== undefined && !this.#pending) this.#releaseIdle();
    });
  }

  /** Stops the loop after the running step. */
  async stop(): Promise<void> {
    this.#stopping.abort();
    this.wake();
    await this.#loop;
    this.#releaseIdle();
  }

  #releaseIdle(): void {
    for (const resolve of this.#idleWaiters.splice(0)) resolve();
  }

  async #work(): Promise<void> {
    const { store, run, clock, pauseMs, logger } = this.#options;
    const signal = this.#stopping.signal;
    while (!signal.aborted) {
      this.#pending = false;
      const job = store.jobs()[0];
      if (job === undefined) {
        await new Promise<void>((resolve) => {
          this.#wakeUp = resolve;
          if (this.#pending || signal.aborted) {
            this.#wakeUp = undefined;
            resolve();
            return;
          }
          this.#releaseIdle();
        });
        continue;
      }
      let outcome: JobOutcome;
      try {
        outcome = await run(job);
      } catch {
        // run classifies every failure; anything left is a fault of the store.
        logger.error("worker.failed", {
          job_id: job.id,
          job_kind: job.kind,
          reason: "store",
        });
        outcome = "retry";
      }
      if (outcome === "done") {
        try {
          await store.removeJob(job.id);
        } catch {
          logger.error("worker.failed", {
            job_id: job.id,
            job_kind: job.kind,
            reason: "store",
          });
          outcome = "retry";
        }
      }
      if (outcome === "retry" && !signal.aborted) {
        logger.info("worker.paused", {
          job_id: job.id,
          job_kind: job.kind,
          reason: "transient",
        });
        await clock.sleep(pauseMs, signal).catch(() => undefined);
      }
    }
  }
}
