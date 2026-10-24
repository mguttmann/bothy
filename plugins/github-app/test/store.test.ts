// SPDX-License-Identifier: Apache-2.0
// Contract of the Store port for FileStore and MemoryStore (brief M2-49 Design note 8).

import { spawn } from "node:child_process";
import { mkdtemp, readdir, rm, stat, writeFile } from "node:fs/promises";
import { tmpdir } from "node:os";
import { join } from "node:path";

import { FakeClock } from "bothy-sdk/testing";
import { afterEach, beforeEach, describe, expect, it } from "vitest";

import { createLogger } from "../src/log.js";
import {
  FileStore,
  processAlive,
  writeAtomic,
} from "../src/store/file-store.js";
import { MemoryBacking, MemoryStore } from "../src/store/memory-store.js";
import {
  StoreLockedError,
  type NewJob,
  type Store,
} from "../src/store/store.js";

const WINDOW = 60_000;

let dir = "";
let lines: string[] = [];
const logger = createLogger({ write: (l) => lines.push(l) });

beforeEach(async () => {
  dir = await mkdtemp(join(tmpdir(), "github-app-store-"));
  lines = [];
});

afterEach(async () => {
  await rm(dir, { recursive: true, force: true });
});

function job(id: string, created = 0): NewJob {
  return {
    id,
    kind: "github_event",
    created_at: created,
    data: { n: id },
    progress: {},
  };
}

type Factory = (clock: FakeClock) => { open: () => Store };

const factories: [string, Factory][] = [
  [
    "FileStore",
    (clock) => ({
      open: () => new FileStore({ dir, clock, windowMs: WINDOW, logger }),
    }),
  ],
  [
    "MemoryStore",
    (clock) => {
      const backing = new MemoryBacking();
      return { open: () => new MemoryStore(backing, clock, WINDOW) };
    },
  ],
];

describe.each(factories)("%s", (_name, factory) => {
  it("keeps jobs in creation order and survives a reopen", async () => {
    const clock = new FakeClock();
    const f = factory(clock);
    const store = f.open();
    await store.open();
    for (const id of ["c", "a", "b"])
      expect(
        await store.admit({
          seenKey: `k-${id}`,
          status: 202,
          job: job(id),
          maxJobs: 10,
        }),
      ).toEqual({ kind: "accepted" });
    expect(store.jobs().map((j) => j.id)).toEqual(["c", "a", "b"]);
    const first = store.jobs()[0];
    if (first === undefined) throw new Error("no job");
    first.progress.step = "done";
    await store.saveJob(first);
    await store.removeJob("a");
    await store.update((s) => {
      s.threads["k#1"] = { thread_id: "t", channel_id: "c" };
    });
    await store.close();
    const again = f.open();
    await again.open();
    expect(again.jobs().map((j) => [j.id, j.progress])).toEqual([
      ["c", { step: "done" }],
      ["b", {}],
    ]);
    expect(again.state().threads["k#1"]).toEqual({
      thread_id: "t",
      channel_id: "c",
    });
    expect(
      await again.admit({
        seenKey: "k-c",
        status: 202,
        job: job("d"),
        maxJobs: 10,
      }),
    ).toEqual({
      kind: "duplicate",
      status: 202,
    });
    expect(
      (
        await again.admit({
          seenKey: "k-new",
          status: 202,
          job: job("e"),
          maxJobs: 10,
        })
      ).kind,
    ).toBe("accepted");
    expect(again.jobs().map((j) => j.id)).toEqual(["c", "b", "e"]);
    await again.close();
  });

  it("refuses a job above max_jobs", async () => {
    const store = factory(new FakeClock()).open();
    await store.open();
    await store.admit({ seenKey: "1", status: 202, job: job("1"), maxJobs: 1 });
    expect(
      await store.admit({
        seenKey: "2",
        status: 202,
        job: job("2"),
        maxJobs: 1,
      }),
    ).toEqual({ kind: "full" });
    expect(
      await store.admit({ seenKey: "3", status: 204, maxJobs: 1 }),
    ).toEqual({ kind: "accepted" });
    await store.close();
  });

  it("prunes seen identifiers, created objects and drafts after the window, never the thread map", async () => {
    const clock = new FakeClock();
    const store = factory(clock).open();
    await store.open();
    await store.admit({ seenKey: "old", status: 204, maxJobs: 10 });
    await store.update((s) => {
      s.created["issue:x:1"] = clock.now();
      s.drafts.d = {
        id: "d",
        thread_id: "t",
        channel_id: "c",
        card_message_id: "m",
        title: "x",
        repositories: ["x"],
        created_at: clock.now(),
      };
      s.threads["x#1"] = { thread_id: "t", channel_id: "c" };
      s.thread_keys.t = "x#1";
    });
    await clock.advance(WINDOW + 1);
    // A known identifier past the window is no duplicate any more.
    expect(
      (await store.admit({ seenKey: "old", status: 204, maxJobs: 10 })).kind,
    ).toBe("accepted");
    await store.update(() => undefined);
    const state = store.state();
    expect(state.created).toEqual({});
    expect(state.drafts).toEqual({});
    expect(state.threads).toEqual({
      "x#1": { thread_id: "t", channel_id: "c" },
    });
    expect(state.thread_keys).toEqual({ t: "x#1" });
    await store.close();
  });

  it("restores the seen key of a job whose state write was lost", async () => {
    const clock = new FakeClock();
    const f = factory(clock);
    const store = f.open();
    await store.open();
    await store.admit({
      seenKey: "first",
      status: 202,
      job: job("1"),
      maxJobs: 10,
    });
    await store.update((s) => {
      delete s.seen.first;
    });
    await store.close();
    const again = f.open();
    await again.open();
    expect(again.state().seen.first).toEqual({ at: 0, status: 202 });
    await again.close();
  });
});

describe("FileStore", () => {
  it("writes files with mode 0600 in directories with mode 0700", async () => {
    const store = new FileStore({
      dir,
      clock: new FakeClock(),
      windowMs: WINDOW,
      logger,
    });
    await store.open();
    await store.admit({
      seenKey: "1",
      status: 202,
      job: job("1"),
      maxJobs: 10,
    });
    expect((await stat(join(dir, "state.json"))).mode & 0o777).toBe(0o600);
    expect((await stat(join(dir, "lock"))).mode & 0o777).toBe(0o600);
    expect((await stat(join(dir, "jobs"))).mode & 0o777).toBe(0o700);
    for (const name of await readdir(join(dir, "jobs")))
      expect((await stat(join(dir, "jobs", name))).mode & 0o777).toBe(0o600);
    await store.close();
  });

  it("writes atomically: no temporary file stays, an interrupted one is dropped at open", async () => {
    await writeAtomic(dir, join(dir, "x.json"), "{}");
    expect(await readdir(dir)).toEqual(["x.json"]);
    const store = new FileStore({
      dir,
      clock: new FakeClock(),
      windowMs: WINDOW,
      logger,
    });
    await store.open();
    await writeFile(join(dir, "jobs", "000000000001.json.tmp-x"), "{", {
      mode: 0o600,
    });
    await store.close();
    await store.open();
    expect(await readdir(join(dir, "jobs"))).toEqual([]);
    await store.close();
  });

  it("refuses a second store on a directory locked by a live process", async () => {
    const first = new FileStore({
      dir,
      clock: new FakeClock(),
      windowMs: WINDOW,
      logger,
    });
    await first.open();
    const second = new FileStore({
      dir,
      clock: new FakeClock(),
      windowMs: WINDOW,
      logger,
    });
    await expect(second.open()).rejects.toBeInstanceOf(StoreLockedError);
    await first.close();
    await second.open();
    await second.close();
  });

  it("takes over a lock whose process is gone with a Warn line", async () => {
    // The PID of a process that ran and ended: no longer alive.
    const child = spawn(process.execPath, ["-e", ""], { stdio: "ignore" });
    const pid = child.pid ?? 0;
    await new Promise((resolve) => child.once("exit", resolve));
    expect(processAlive(pid)).toBe(false);
    expect(processAlive(process.pid)).toBe(true);
    expect(processAlive(-1)).toBe(false);
    await writeFile(join(dir, "lock"), String(pid), { mode: 0o600 });
    const store = new FileStore({
      dir,
      clock: new FakeClock(),
      windowMs: WINDOW,
      logger,
    });
    await store.open();
    expect(lines.filter((l) => l.includes("stale_lock"))).toHaveLength(1);
    await store.close();
  });
});
