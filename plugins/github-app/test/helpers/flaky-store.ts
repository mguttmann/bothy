// SPDX-License-Identifier: Apache-2.0
// A MemoryStore whose writes fail on chosen calls, as a full or broken disk would.

import type { Clock } from "bothy-sdk";

import { MemoryBacking, MemoryStore } from "../../src/store/memory-store.js";
import type { JobRecord, StateData } from "../../src/store/store.js";

/** The error a full disk gives. */
export function diskFull(): Error {
  return Object.assign(new Error("no space left on device"), {
    code: "ENOSPC",
  });
}

/** A {@link MemoryStore} that throws {@link diskFull} on the numbered write calls (from 1). */
export class FlakyStore extends MemoryStore {
  /** Calls of `writeJob` that fail. */
  readonly failJobWrites = new Set<number>();
  /** Calls of `writeState` that fail. */
  readonly failStateWrites = new Set<number>();
  #jobWrites = 0;
  #stateWrites = 0;

  /**
   * Creates the store.
   *
   * @param clock - Source of `now`.
   * @param windowMs - `github_app.dedupe_window`.
   */
  constructor(clock: Pick<Clock, "now">, windowMs: number) {
    super(new MemoryBacking(), clock, windowMs);
  }

  protected override writeJob(job: JobRecord): Promise<void> {
    this.#jobWrites += 1;
    if (this.failJobWrites.has(this.#jobWrites))
      return Promise.reject(diskFull());
    return super.writeJob(job);
  }

  protected override writeState(state: StateData): Promise<void> {
    this.#stateWrites += 1;
    if (this.failStateWrites.has(this.#stateWrites))
      return Promise.reject(diskFull());
    return super.writeState(state);
  }
}
