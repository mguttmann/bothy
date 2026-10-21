// SPDX-License-Identifier: Apache-2.0
//
// The inbox stores (M2-29 Design note 9, M2-30 Design note 7): token replacement by iat, the
// jti store and the durable FileInboxStore, ported from `sdk/python/tests/test_inbox.py`. Every
// behaviour test runs against `FileInboxStore` and `MemoryInboxStore`. Tokens are compared with
// `Secret.equals`, never printed.

import {
  chmod,
  lstat,
  mkdir,
  mkdtemp,
  readFile,
  readdir,
  rm,
  stat,
  symlink,
  writeFile,
} from "node:fs/promises";
import { tmpdir } from "node:os";
import { join } from "node:path";
import { inspect } from "node:util";

import { afterEach, beforeEach, describe, expect, it } from "vitest";

import { Secret } from "../src/auth/secret.js";
import { uuidv7 } from "../src/ids.js";
import {
  EntryState,
  FileInboxStore,
  InboxEntry,
  MemoryInboxStore,
  type InboxStore,
} from "../src/invocation/index.js";

const NOW = Date.UTC(2026, 9, 20, 9, 0);
const S = 1000;

let dir: string;

beforeEach(async () => {
  dir = await mkdtemp(join(tmpdir(), "bothy-inbox-"));
});

afterEach(async () => {
  await rm(dir, { recursive: true, force: true });
});

interface EntryOptions {
  seq?: number | undefined;
  runId?: string | undefined;
  tok?: string | undefined;
  iat?: number | undefined;
  state?: EntryState;
  envelope?: Record<string, unknown>;
}

/** A test entry; a member given as `undefined` stays absent (unlike a default parameter). */
function entry(o: EntryOptions = {}): InboxEntry {
  const pick = <K extends keyof EntryOptions>(
    name: K,
    fallback: EntryOptions[K],
  ) => (Object.hasOwn(o, name) ? o[name] : fallback);
  const eventId = uuidv7();
  const seq = pick("seq", 7);
  const tok = pick("tok", "token-a");
  return new InboxEntry({
    eventId,
    seq,
    type: "agent.invocation",
    runId: pick("runId", uuidv7()),
    envelope: o.envelope ?? { id: eventId, seq },
    token: tok === undefined ? undefined : new Secret(tok),
    tokenIat: pick("iat", 100),
    tokenJti:
      tok === undefined ? undefined : `jti-${tok.replace(/^token-/, "")}`,
    state: o.state ?? EntryState.ACCEPTED,
    deliveries: 1,
  });
}

function mode(path: string): Promise<number> {
  return stat(path).then((s) => s.mode & 0o777);
}

const kinds = {
  file: () => new FileInboxStore(join(dir, "inbox")),
  memory: () => new MemoryInboxStore(),
};

describe.each(Object.entries(kinds))("%s inbox store", (_name, make) => {
  let store: InboxStore;

  beforeEach(() => {
    store = make();
  });

  afterEach(async () => {
    await store.close();
  });

  it("replaces the token of a redelivery by iat while accepted", async () => {
    const e = entry({ tok: "token-a", iat: 100 });
    await store.add(e);
    let got = await store.redelivered(
      e.eventId,
      new Secret("token-b"),
      110,
      "jti-b",
    );
    expect([got.tokenJti, got.tokenIat, got.deliveries]).toEqual([
      "jti-b",
      110,
      2,
    ]);
    expect(got.token?.equals(new Secret("token-b"))).toBe(true);
    // An equal iat takes the later arrival.
    got = await store.redelivered(
      e.eventId,
      new Secret("token-c"),
      110,
      "jti-c",
    );
    expect([got.tokenJti, got.tokenIat, got.deliveries]).toEqual([
      "jti-c",
      110,
      3,
    ]);
    const stored = await store.get(e.eventId);
    expect(stored?.tokenJti).toBe("jti-c");
    expect(stored?.token?.equals(new Secret("token-c"))).toBe(true);
    expect(stored?.state).toBe(EntryState.ACCEPTED);
    expect(stored?.deliveries).toBe(3);
  });

  it("keeps the stored token against an older one", async () => {
    const e = entry({ tok: "token-a", iat: 100 });
    await store.add(e);
    let got = await store.redelivered(
      e.eventId,
      new Secret("token-old"),
      90,
      "jti-old",
    );
    expect([got.tokenJti, got.tokenIat, got.deliveries]).toEqual([
      "jti-a",
      100,
      2,
    ]);
    expect(got.token?.equals(new Secret("token-a"))).toBe(true);
    // A redelivery without token only counts.
    got = await store.redelivered(e.eventId, undefined, undefined, undefined);
    expect([got.tokenJti, got.tokenIat, got.deliveries]).toEqual([
      "jti-a",
      100,
      3,
    ]);
  });

  it("only counts a duplicate of a started entry", async () => {
    for (const state of [
      EntryState.STARTED,
      EntryState.WAITING,
      EntryState.CLOSED,
    ]) {
      const e = entry({ tok: "token-a", iat: 100 });
      await store.add(e);
      await store.setState(e.eventId, state);
      const got = await store.redelivered(
        e.eventId,
        new Secret("token-new"),
        200,
        "jti-new",
      );
      expect([got.state, got.tokenJti, got.tokenIat, got.deliveries]).toEqual([
        state,
        "jti-a",
        100,
        2,
      ]);
      expect(got.token?.equals(new Secret("token-a"))).toBe(true);
    }
  });

  it("replaces the token unconditionally after a renewal", async () => {
    const e = entry({ tok: "token-a", iat: 100 });
    await store.add(e);
    await store.setState(e.eventId, EntryState.STARTED);
    await store.replaceToken(
      e.eventId,
      new Secret("token-grant"),
      50,
      "jti-grant",
    );
    const got = await store.get(e.eventId);
    expect([got?.tokenJti, got?.tokenIat, got?.deliveries, got?.state]).toEqual(
      ["jti-grant", 50, 1, EntryState.STARTED],
    );
    expect(got?.token?.equals(new Secret("token-grant"))).toBe(true);
  });

  it("refuses unknown and duplicate IDs and an envelope with a token", async () => {
    const e = entry();
    expect(await store.get(e.eventId)).toBeUndefined();
    await expect(
      store.redelivered(e.eventId, undefined, undefined, undefined),
    ).rejects.toThrow(RangeError);
    await expect(store.setState(e.eventId, EntryState.CLOSED)).rejects.toThrow(
      RangeError,
    );
    await expect(
      store.replaceToken(e.eventId, new Secret("t"), 1, "j"),
    ).rejects.toThrow(RangeError);
    await store.add(e);
    await expect(store.add(e)).rejects.toThrow(/already/);
    await expect(
      store.add(entry({ envelope: { invocation_token: "x" } })),
    ).rejects.toThrow(/invocation_token/);
  });

  it("lists a run's entries and entries by state in seq order, ephemeral last", async () => {
    const run = uuidv7();
    const later = entry({ seq: 9, runId: run });
    const first = entry({ seq: 3, runId: run });
    const other = entry({ seq: 5 });
    const ephemeral = entry({
      seq: undefined,
      runId: run,
      tok: undefined,
      iat: undefined,
    });
    for (const e of [later, first, other, ephemeral]) await store.add(e);
    expect((await store.forRun(run)).map((e) => e.eventId)).toEqual([
      first.eventId,
      later.eventId,
      ephemeral.eventId,
    ]);
    await store.setState(first.eventId, EntryState.STARTED);
    await store.setState(other.eventId, EntryState.WAITING);
    const got = await store.inStates(EntryState.STARTED, EntryState.WAITING);
    expect(got.map((e) => e.eventId)).toEqual([first.eventId, other.eventId]);
    expect(await store.inStates(EntryState.CLOSED)).toEqual([]);
    expect(await store.inStates()).toEqual([]);
    expect(await store.inStates(EntryState.ACCEPTED)).toHaveLength(2);
  });

  it("keeps a jti until (not at) its end; prune drops jti rows only", async () => {
    const e = entry();
    await store.add(e);
    await store.rememberJti("j1", NOW + 10 * S);
    await store.rememberJti("j2", NOW + 100 * S);
    expect(await store.jtiSeen("j1", NOW)).toBe(true);
    expect(await store.jtiSeen("j1", NOW + 9 * S)).toBe(true);
    expect(await store.jtiSeen("j1", NOW + 10 * S)).toBe(false);
    await store.prune(NOW + 50 * S);
    expect(await store.jtiSeen("j1", NOW)).toBe(false);
    expect(await store.jtiSeen("j2", NOW + 50 * S)).toBe(true);
    expect(await store.get(e.eventId)).toBeDefined();
    // Remembering again keeps the later end.
    await store.rememberJti("j2", NOW + 20 * S);
    expect(await store.jtiSeen("j2", NOW + 60 * S)).toBe(true);
  });

  it("stores a copy of the envelope", async () => {
    const envelope: Record<string, unknown> = {
      id: "x",
      payload: { nested: [1, "two"] },
    };
    const e = entry({ envelope });
    await store.add(e);
    (envelope.payload as { nested: unknown[] }).nested.push(3);
    const got = await store.get(e.eventId);
    expect(got?.envelope).toEqual({ id: "x", payload: { nested: [1, "two"] } });
  });
});

describe("FileInboxStore", () => {
  it("survives a restart with mode 0700 for the directory and 0600 for every file", async () => {
    const path = join(dir, "state", "inbox");
    const old = process.umask(0);
    let accepted: InboxEntry;
    let started: InboxEntry;
    let ephemeral: InboxEntry;
    try {
      const s = new FileInboxStore(path);
      accepted = entry({
        envelope: { id: "x", payload: { run_id: "r", nested: [1, "two"] } },
      });
      started = entry({ seq: 3 });
      ephemeral = entry({ seq: undefined, tok: undefined, iat: undefined });
      await s.add(accepted);
      await s.add(started);
      await s.add(ephemeral);
      await s.setState(started.eventId, EntryState.STARTED);
      await s.redelivered(
        accepted.eventId,
        new Secret("token-b"),
        110,
        "jti-b",
      );
      await s.rememberJti("jti-seen", NOW + 90 * S);
      await s.close();
    } finally {
      process.umask(old);
    }
    expect(await mode(path)).toBe(0o700);
    const files = await readdir(path);
    expect(files.length).toBeGreaterThanOrEqual(3);
    for (const name of files)
      expect([name, await mode(join(path, name))]).toEqual([name, 0o600]);

    // A new store on the same directory lists the entries that have not started.
    const s = new FileInboxStore(path);
    try {
      const open = await s.inStates(EntryState.ACCEPTED);
      expect(open.map((e) => e.eventId)).toEqual([
        accepted.eventId,
        ephemeral.eventId,
      ]);
      const got = open[0];
      expect(got?.token?.equals(new Secret("token-b"))).toBe(true);
      expect([got?.tokenIat, got?.tokenJti, got?.deliveries, got?.seq]).toEqual(
        [110, "jti-b", 2, 7],
      );
      expect(got?.envelope).toEqual(accepted.envelope);
      expect(got?.runId).toBe(accepted.runId);
      expect(open[1]?.token).toBeUndefined();
      expect(open[1]?.seq).toBeUndefined();
      expect((await s.get(started.eventId))?.state).toBe(EntryState.STARTED);
      expect(await s.jtiSeen("jti-seen", NOW)).toBe(true);
      expect(await s.jtiSeen("jti-other", NOW)).toBe(false);
      // A new entry after the restart keeps the arrival order behind the old ones.
      const fresh = entry({ seq: undefined, tok: undefined, iat: undefined });
      await s.add(fresh);
      expect(
        (await s.inStates(EntryState.ACCEPTED)).map((e) => e.eventId),
      ).toEqual([accepted.eventId, ephemeral.eventId, fresh.eventId]);
    } finally {
      await s.close();
    }
  });

  it("stores only the event, its token and the state fields", async () => {
    const path = join(dir, "inbox");
    const s = new FileInboxStore(path);
    const e = entry();
    await s.add(e);
    await s.close();
    const names = (await readdir(path)).filter((n) => n.includes(e.eventId));
    expect(names).toHaveLength(1);
    const stored = JSON.parse(
      await readFile(join(path, names[0] ?? ""), "utf8"),
    ) as Record<string, unknown>;
    expect(Object.keys(stored).sort()).toEqual(
      [
        "arrival",
        "deliveries",
        "envelope",
        "event_id",
        "run_id",
        "seq",
        "state",
        "token",
        "token_iat",
        "token_jti",
        "type",
      ].sort(),
    );
  });

  it("refuses a directory open to group or others, a symbolic link and a wide file", async () => {
    const wide = join(dir, "wide");
    await mkdir(wide, { mode: 0o755 });
    await chmod(wide, 0o755);
    await expect(new FileInboxStore(wide).get(uuidv7())).rejects.toThrow(
      /0700/,
    );

    const target = join(dir, "target");
    const s = new FileInboxStore(target);
    const e = entry();
    await s.add(e);
    await s.close();
    const link = join(dir, "link");
    await symlink(target, link);
    await expect(new FileInboxStore(link).get(uuidv7())).rejects.toThrow(
      /directory/,
    );

    const name =
      (await readdir(target)).find((n) => n.includes(e.eventId)) ?? "";
    await chmod(join(target, name), 0o644);
    await expect(new FileInboxStore(target).get(e.eventId)).rejects.toThrow(
      /0600/,
    );
  });

  it("ignores an interrupted write and refuses an event ID that is not a UUID", async () => {
    const path = join(dir, "inbox");
    const s = new FileInboxStore(path);
    const e = entry();
    await s.add(e);
    await s.close();
    // A temporary file of a write that never reached its rename.
    await writeFile(join(path, `.tmp-${uuidv7()}.json`), "{", { mode: 0o600 });
    const again = new FileInboxStore(path);
    try {
      expect(
        (await again.inStates(EntryState.ACCEPTED)).map((x) => x.eventId),
      ).toEqual([e.eventId]);
      expect((await readdir(path)).some((n) => n.startsWith(".tmp-"))).toBe(
        false,
      );
      const bad = new InboxEntry({ ...entry(), eventId: "../escape" });
      await expect(again.add(bad)).rejects.toThrow(RangeError);
      expect((await lstat(dir)).isDirectory()).toBe(true);
      expect(await readdir(dir)).toEqual(["inbox"]);
    } finally {
      await again.close();
    }
  });

  it("refuses calls after close", async () => {
    const s = new FileInboxStore(join(dir, "inbox"));
    await s.close();
    await s.close();
    await expect(s.get(uuidv7())).rejects.toThrow(/closed/);
  });
});

describe("InboxEntry", () => {
  it("hides the token and the envelope from inspect and JSON", () => {
    const e = entry({
      tok: "token-canary",
      envelope: { payload: { text: "content-canary" } },
    });
    const text = inspect(e) + String(e) + JSON.stringify(e);
    expect(text).not.toContain("token-canary");
    expect(text).not.toContain("content-canary");
    expect(text).toContain(e.eventId);
  });
});
