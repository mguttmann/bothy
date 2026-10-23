// SPDX-License-Identifier: Apache-2.0
//
// Outbox: the ID is stored before the send, never twice, FIFO per thread, failures kept; port of
// `sdk/python/tests/test_outbox.py`. docs/spec/04-sync-protocol.md 13.2: a reply gets its
// `client_msg_id` and a write its `Idempotency-Key` stored before the first send, every repeat
// sends the same one; `400`, `404`, `409`, `410`, `413` and `422` mark the entry failed with its
// problem code; a transient failure leaves it pending for `Outbox.flush`. The faults
// `stored_then_502` and `stored_late` of the FakeServer prove that a resend gets the stored
// message instead of a second one.

import { mkdtempSync, rmSync, statSync } from "node:fs";
import { tmpdir } from "node:os";
import { join } from "node:path";

import { afterEach, beforeEach, describe, expect, it } from "vitest";

import {
  createApiClient,
  IDEMPOTENCY_KEY_HEADER,
  type ApiClient,
} from "../src/client.js";
import { approvalCard } from "../src/components/index.js";
import type { SdkSettings } from "../src/config.js";
import type { components } from "../src/generated/openapi.js";
import { silentLogger } from "../src/logger.js";
import {
  FileOutboxStore,
  MemoryOutboxStore,
  Outbox,
  OutboxState,
  PERMANENT_STATUSES,
  type OutboxEntry,
  type OutboxStore,
} from "../src/outbox/index.js";
import { FakeClock, settle } from "../src/testing/clock.js";
import { FakeServer } from "../src/testing/fakeserver.js";
import { Fault } from "../src/testing/faults.js";

type S = components["schemas"];
type Json = Record<string, unknown>;

const BASE = "https://example.com/api/v1";
const THREAD_A = "01900000-0000-7000-8000-00000000a001";
const THREAD_B = "01900000-0000-7000-8000-00000000b001";
const RUN = "01900000-0000-7000-8000-000000000101";
const CHANNEL = "01900000-0000-7000-8000-000000000103";
const AGENT = "01900000-0000-7000-8000-000000000301";
const UUID7 =
  /^[0-9a-f]{8}-[0-9a-f]{4}-7[0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$/;

const CODES: Record<number, string> = {
  400: "validation_failed",
  404: "not_found",
  409: "conflict",
  410: "gone",
  413: "payload_too_large",
  422: "idempotency_key_reuse",
  429: "rate_limited",
  500: "internal_error",
  502: "bad_gateway",
  503: "unavailable",
};

function problem(status: number, inProgress = false): Response {
  const code =
    status === 409 && inProgress
      ? "request_in_progress"
      : (CODES[status] ?? "internal_error");
  return Response.json(
    { type: "about:blank", title: code, status, code },
    {
      status,
      headers: {
        "Content-Type": "application/problem+json",
        ...(status === 429 ? { "Retry-After": "1" } : {}),
      },
    },
  );
}

function message(threadId: string, body: Json): S["Message"] {
  return {
    id: "01900000-0000-7000-8000-00000000c001",
    thread_id: threadId,
    channel_id: CHANNEL,
    thread_seq: 1,
    author: {
      id: AGENT,
      kind: "agent",
      handle: "lotse",
      display_name: "Lotse",
    },
    client_msg_id: String(body.client_msg_id),
    origin: "agent",
    kind: (body.kind as S["MessageKind"] | undefined) ?? "text",
    content_md: typeof body.content_md === "string" ? body.content_md : "",
    content_format: 1,
    silent: false,
    attachments: [],
    mention_ids: [],
    reactions: [],
    pinned: false,
    components: [],
    version: 1,
    created_at: "2026-01-01T00:00:00Z",
  } as unknown as S["Message"];
}

function commandDoc(body: Json): S["Command"] {
  return {
    id: "01900000-0000-7000-8000-00000000d001",
    owner_principal_id: AGENT,
    name: String(body.name),
    description: String(body.description),
    scope_kind: "instance",
    options: [],
    required_level: "trigger",
    control: false,
    version: 1,
  };
}

type Answer = number | "network" | "in_progress";

/**
 * A fetch of `createMessage` and `registerCommand`: `answers` per thread (or `"commands"`) is
 * a queue of statuses; an empty queue answers success. Every request is recorded as (target,
 * `client_msg_id` or `Idempotency-Key`, `Invocation-Token` present).
 */
class MockServer {
  readonly answers = new Map<string, Answer[]>();
  readonly seen: { target: string; key: string; token: string | null }[] = [];
  readonly gates = new Map<string, Promise<void>>();
  observe: ((key: string) => Promise<void>) | undefined;

  readonly fetch = async (request: Request): Promise<Response> => {
    const text = await request.text();
    const body = text ? (JSON.parse(text) as Json) : {};
    const path = new URL(request.url).pathname;
    const isMessage = path.endsWith("/messages");
    const target = isMessage ? (path.split("/").at(-2) ?? "") : "commands";
    const key = isMessage
      ? String(body.client_msg_id)
      : (request.headers.get(IDEMPOTENCY_KEY_HEADER) ?? "");
    this.seen.push({
      target,
      key,
      token: request.headers.get("Invocation-Token"),
    });
    await this.observe?.(key);
    await this.gates.get(target);
    const answer = this.answers.get(target)?.shift() ?? 201;
    if (answer === "network") throw new TypeError("fetch failed");
    if (answer === "in_progress") return problem(409, true);
    if (answer !== 201) return problem(answer);
    return Response.json(isMessage ? message(target, body) : commandDoc(body), {
      status: 201,
    });
  };

  keys(target: string): string[] {
    return this.seen.filter((s) => s.target === target).map((s) => s.key);
  }
}

function mockClient(
  server: MockServer,
  settings: Partial<SdkSettings> = { retryMaxAttempts: 1 },
  clock: FakeClock = new FakeClock(),
): ApiClient {
  return createApiClient({
    baseUrl: BASE,
    token: "example-api-token",
    fetch: server.fetch,
    clock,
    settings,
    logger: silentLogger,
  });
}

/** Moves `clock` until `task` settles (retries sleep on the clock). */
async function drive<T>(
  clock: FakeClock,
  task: Promise<T>,
  stepMs = 250,
): Promise<T> {
  let done = false;
  void task.then(
    () => (done = true),
    () => (done = true),
  );
  for (let i = 0; i < 400 && !done; i++) {
    await new Promise((r) => setTimeout(r, 1));
    await clock.advance(stepMs);
  }
  if (!done) throw new Error("the outbox did not finish");
  return task;
}

// Store before send -----------------------------------------------------------------------------

describe("store before send", () => {
  it("stores the client_msg_id before the first send", async () => {
    const store = new MemoryOutboxStore();
    const server = new MockServer();
    const observed: [string, string[]][] = [];
    server.observe = async (key) => {
      observed.push([key, (await store.pending()).map((e) => e.key)]);
    };
    const entry = await new Outbox(mockClient(server), { store }).sendMessage(
      THREAD_A,
      "Hello Alex",
      { runId: RUN },
    );
    expect(entry.state).toBe(OutboxState.SENT);
    expect(observed).toEqual([[entry.key, [entry.key]]]);
    expect(entry.key).toMatch(UUID7);
    expect(entry.runId).toBe(RUN);
    expect(entry.payload.run_id).toBe(RUN);
    // Deleted after the 2xx: nothing pending, nothing failed.
    expect(await store.pending()).toEqual([]);
    expect(await store.failed()).toEqual([]);
  });

  it("stores the members of the reply", async () => {
    const server = new MockServer();
    const replyTo = "01900000-0000-7000-8000-000000000105";
    const entry = await new Outbox(mockClient(server)).sendMessage(
      THREAD_A,
      "Done",
      {
        replyToMessageId: replyTo,
      },
    );
    expect(entry.payload.reply_to_message_id).toBe(replyTo);
    expect(entry.payload.content_md).toBe("Done");
    expect(entry.payload.kind).toBe("text");
    expect(entry.payload).not.toHaveProperty("run_id");
    expect(entry.runId).toBeUndefined();
  });

  it("checks the components of a card before storing it", async () => {
    const server = new MockServer();
    const store = new MemoryOutboxStore();
    const outbox = new Outbox(
      mockClient(server, { retryMaxAttempts: 1, componentMaxPerMessage: 2 }),
      {
        store,
      },
    );
    await expect(
      outbox.sendMessage(THREAD_A, "Allow?", {
        kind: "card",
        components: approvalCard(),
      }),
    ).rejects.toThrow(/component\.max_per_message/);
    expect(server.seen).toEqual([]);
    expect(await store.pending()).toEqual([]);
    const card = await new Outbox(mockClient(server)).sendMessage(
      THREAD_A,
      "Allow?",
      {
        kind: "card",
        components: approvalCard(),
      },
    );
    expect(card.state).toBe(OutboxState.SENT);
    expect(card.payload.kind).toBe("card");
    expect(card.payload.components).toHaveLength(4);
  });
});

// FakeServer faults -----------------------------------------------------------------------------

describe("FakeServer faults", () => {
  let fake: FakeServer;

  beforeEach(async () => {
    fake = new FakeServer();
    await fake.start();
  });

  afterEach(async () => {
    await fake.stop();
  });

  /** A client of Lotse; `keys` records the `client_msg_id` and `Idempotency-Key` of every attempt. */
  function fakeClient(
    settings: Partial<SdkSettings>,
    keys: string[] = [],
    timeoutMs?: number,
  ): ApiClient {
    return createApiClient({
      baseUrl: fake.baseUrl,
      token: fake.credentials.apiToken,
      dpopKey: fake.credentials.dpop,
      clock: fake.clock,
      settings: { ...fake.sdkSettings, ...settings },
      logger: silentLogger,
      ...(timeoutMs === undefined ? {} : { timeoutMs }),
      fetch: async (request: Request) => {
        const text = await request.clone().text();
        const key = request.headers.get(IDEMPOTENCY_KEY_HEADER);
        if (key !== null) keys.push(key);
        else if (text)
          keys.push(String((JSON.parse(text) as Json).client_msg_id));
        return fetch(request);
      },
    });
  }

  /** Takes the DPoP nonce first, so no counted request meets the challenge. */
  async function warm(api: ApiClient): Promise<void> {
    await api.GET("/instance/discovery");
    await api.GET("/agents/me/channels");
  }

  it("stored_then_502: the retry gets the stored message, no duplicate", async () => {
    const triggered = await fake.trigger();
    const keys: string[] = [];
    const api = fakeClient({ retryMaxAttempts: 3 }, keys);
    await warm(api);
    fake.inject("createMessage", Fault.storedThen502());
    const before = fake.messages.length;
    const entry = await drive(
      fake.clock,
      new Outbox(api).sendMessage(triggered.threadId, "Reply", {
        runId: triggered.runId,
      }),
    );
    expect(entry.state).toBe(OutboxState.SENT);
    expect(keys).toEqual([entry.key, entry.key]);
    expect(fake.messages.slice(before).map((m) => m.client_msg_id)).toEqual([
      entry.key,
    ]);
  });

  it("stored_then_502 without retries: flush resends the same id and gets the stored message", async () => {
    const triggered = await fake.trigger();
    const keys: string[] = [];
    const api = fakeClient({ retryMaxAttempts: 1 }, keys);
    await warm(api);
    fake.inject("createMessage", Fault.storedThen502());
    const before = fake.messages.length;
    const store = new MemoryOutboxStore();
    const outbox = new Outbox(api, { store, logger: silentLogger });
    const entry = await outbox.sendMessage(triggered.threadId, "Reply");
    expect(entry.state).toBe(OutboxState.PENDING);
    expect((await store.pending()).map((e) => e.key)).toEqual([entry.key]);
    await outbox.flush();
    expect(await store.pending()).toEqual([]);
    expect(await outbox.failed()).toEqual([]);
    expect(keys).toEqual([entry.key, entry.key]);
    expect(fake.messages.slice(before).map((m) => m.client_msg_id)).toEqual([
      entry.key,
    ]);
    const statuses = fake
      .requests()
      .filter((r) => r.path.endsWith("/messages"))
      .map((r) => r.status);
    expect(statuses).toEqual([502, 200]);
  });

  it("stored_late: the repeat after a timeout gets the stored message, no duplicate", async () => {
    const triggered = await fake.trigger();
    const delayMs = 5_000;
    const keys: string[] = [];
    // A short real timeout makes the client give up on the late answer and repeat the send.
    const api = fakeClient({ retryMaxAttempts: 3 }, keys, 300);
    await warm(api);
    fake.inject("createMessage", Fault.storedLate(delayMs));
    const before = fake.messages.length;
    const task = new Outbox(api).sendMessage(triggered.threadId, "Late");
    // The server waits on its clock; the client times out and sleeps before its retry.
    for (let i = 0; i < 300 && fake.clock.sleepers < 2; i++)
      await new Promise((r) => setTimeout(r, 10));
    expect(fake.clock.sleepers).toBeGreaterThanOrEqual(2);
    const entry = await drive(fake.clock, task);
    // Let the late request finish its store as well.
    await fake.passTime(delayMs);
    await new Promise((r) => setTimeout(r, 50));
    await fake.passTime(0);
    expect(entry.state).toBe(OutboxState.SENT);
    // At least one repeat after the timeout; a slow machine may time out once more.
    expect(keys.length).toBeGreaterThanOrEqual(2);
    expect(new Set(keys)).toEqual(new Set([entry.key]));
    expect(fake.messages.slice(before).map((m) => m.client_msg_id)).toEqual([
      entry.key,
    ]);
  });

  it("call keeps its Idempotency-Key across a retry and a flush", async () => {
    const keys: string[] = [];
    const api = fakeClient({ retryMaxAttempts: 1 }, keys);
    await warm(api);
    fake.inject("registerCommand", Fault.storedThen502());
    const outbox = new Outbox(api, { logger: silentLogger });
    const entry = await outbox.call(
      "post",
      "/commands",
      { body: { name: "status", description: "Show the run status" } },
      { threadId: THREAD_A },
    );
    expect(entry.state).toBe(OutboxState.PENDING);
    await outbox.flush();
    expect(keys).toEqual([entry.key, entry.key]);
    expect([...fake.commands.values()].map((c) => c.name)).toEqual(["status"]);
    expect(await outbox.failed()).toEqual([]);
  });
});

// Order -----------------------------------------------------------------------------------------

describe("order", () => {
  it("keeps FIFO per thread", async () => {
    const server = new MockServer();
    server.answers.set(THREAD_A, [503, 503]);
    const outbox = new Outbox(mockClient(server), { logger: silentLogger });
    const a1 = await outbox.sendMessage(THREAD_A, "a1");
    expect(a1.state).toBe(OutboxState.PENDING);
    // a1 is tried again first and fails again, so a2 waits behind it and is not sent.
    const a2 = await outbox.sendMessage(THREAD_A, "a2");
    expect(a2.state).toBe(OutboxState.PENDING);
    expect(server.keys(THREAD_A)).toEqual([a1.key, a1.key]);
    // Another thread is not blocked.
    const b1 = await outbox.sendMessage(THREAD_B, "b1");
    expect(b1.state).toBe(OutboxState.SENT);
    await outbox.flush();
    expect(server.keys(THREAD_A)).toEqual([a1.key, a1.key, a1.key, a2.key]);
    const a3 = await outbox.sendMessage(THREAD_A, "a3");
    expect(a3.state).toBe(OutboxState.SENT);
    expect(server.keys(THREAD_A).at(-1)).toBe(a3.key);
  });

  it("flushes threads in parallel", async () => {
    const server = new MockServer();
    server.answers.set(THREAD_A, ["network"]);
    server.answers.set(THREAD_B, ["network"]);
    const store = new MemoryOutboxStore();
    const outbox = new Outbox(mockClient(server), {
      store,
      logger: silentLogger,
    });
    const a1 = await outbox.sendMessage(THREAD_A, "a1");
    const b1 = await outbox.sendMessage(THREAD_B, "b1");
    expect([a1.state, b1.state]).toEqual([
      OutboxState.PENDING,
      OutboxState.PENDING,
    ]);
    // Thread A hangs on the server; thread B still goes out during the same flush.
    let open: () => void = () => undefined;
    server.gates.set(THREAD_A, new Promise<void>((r) => (open = r)));
    let flushed = false;
    const flush = outbox.flush().then(() => (flushed = true));
    for (let i = 0; i < 500 && server.keys(THREAD_B).length < 2; i++)
      await settle(1);
    await settle();
    expect(server.keys(THREAD_B)).toEqual([b1.key, b1.key]);
    expect((await store.pending()).map((e) => e.key)).toEqual([a1.key]);
    expect(flushed).toBe(false);
    open();
    await flush;
    expect(await store.pending()).toEqual([]);
  });
});

// Failures --------------------------------------------------------------------------------------

describe("failures", () => {
  it("has the permanent statuses", () => {
    expect([...PERMANENT_STATUSES].sort()).toEqual([
      400, 404, 409, 410, 413, 422,
    ]);
  });

  it.each([400, 404, 409, 410, 413, 422])(
    "marks %i failed, reports it and does not retry it",
    async (status) => {
      const server = new MockServer();
      server.answers.set(THREAD_A, [status]);
      const reported: OutboxEntry[] = [];
      const outbox = new Outbox(mockClient(server, { retryMaxAttempts: 3 }), {
        onFailed: (entry) => {
          reported.push(entry);
          return Promise.resolve();
        },
        logger: silentLogger,
      });
      const entry = await outbox.sendMessage(THREAD_A, "rejected");
      expect(entry.state).toBe(OutboxState.FAILED);
      expect(entry.problemCode).toBe(CODES[status]);
      expect(reported.map((e) => [e.key, e.problemCode])).toEqual([
        [entry.key, CODES[status]],
      ]);
      expect(
        (await outbox.failed()).map((e) => [e.key, e.problemCode]),
      ).toEqual([[entry.key, CODES[status]]]);
      expect(await outbox.store.pending()).toEqual([]);
      // Not retried: one request, and a flush sends nothing again.
      await outbox.flush();
      expect(server.keys(THREAD_A)).toEqual([entry.key]);
      // A failed entry does not block the thread.
      expect((await outbox.sendMessage(THREAD_A, "later")).state).toBe(
        OutboxState.SENT,
      );
    },
  );

  it("drops an entry on 422 idempotency_key_reuse and surfaces it", async () => {
    const server = new MockServer();
    server.answers.set(THREAD_A, [422]);
    const reported: string[] = [];
    const store = new MemoryOutboxStore();
    const outbox = new Outbox(mockClient(server), {
      store,
      onFailed: (e) => {
        reported.push(`${e.key} ${e.problemCode}`);
      },
      logger: silentLogger,
    });
    const entry = await outbox.sendMessage(THREAD_A, "reused");
    expect(reported).toEqual([`${entry.key} idempotency_key_reuse`]);
    expect(await store.pending()).toEqual([]);
    await outbox.flush();
    expect(server.keys(THREAD_A)).toEqual([entry.key]);
  });

  it.each<Answer>([500, 502, 503, 429, "network"])(
    "keeps %s pending and resends it with the same id",
    async (answer) => {
      const server = new MockServer();
      const attempts = 3;
      server.answers.set(THREAD_A, Array<Answer>(attempts).fill(answer));
      const clock = new FakeClock();
      const outbox = new Outbox(
        mockClient(server, { retryMaxAttempts: attempts }, clock),
        {
          logger: silentLogger,
        },
      );
      const entry = await drive(
        clock,
        outbox.sendMessage(THREAD_A, "later"),
        1_000,
      );
      expect(entry.state).toBe(OutboxState.PENDING);
      expect(server.keys(THREAD_A)).toEqual(
        Array<string>(attempts).fill(entry.key),
      );
      expect(await outbox.failed()).toEqual([]);
      await outbox.flush();
      expect(server.keys(THREAD_A)).toEqual(
        Array<string>(attempts + 1).fill(entry.key),
      );
      expect(await outbox.store.pending()).toEqual([]);
    },
  );

  it("keeps 409 request_in_progress pending: the first request still runs", async () => {
    const server = new MockServer();
    server.answers.set(THREAD_A, ["in_progress"]);
    const reported: OutboxEntry[] = [];
    const outbox = new Outbox(mockClient(server), {
      onFailed: (e) => {
        reported.push(e);
      },
      logger: silentLogger,
    });
    const entry = await outbox.sendMessage(THREAD_A, "busy");
    expect(entry.state).toBe(OutboxState.PENDING);
    expect(reported).toEqual([]);
    await outbox.flush();
    expect(server.keys(THREAD_A)).toEqual([entry.key, entry.key]);
    expect(await outbox.store.pending()).toEqual([]);
  });

  it("keeps an entry pending when fetch fails", async () => {
    const store = new MemoryOutboxStore();
    const api = createApiClient({
      baseUrl: BASE,
      token: "example-api-token",
      fetch: () => Promise.reject(new SyntaxError("broken handler")),
      settings: { retryMaxAttempts: 1 },
      logger: silentLogger,
    });
    const entry = await new Outbox(api, {
      store,
      logger: silentLogger,
    }).sendMessage(THREAD_A, "x");
    // The client turns every fetch failure into a TransportError: the entry stays pending.
    expect(entry.state).toBe(OutboxState.PENDING);
    expect((await store.pending()).map((e) => e.key)).toEqual([entry.key]);
  });
});

// Durable store ---------------------------------------------------------------------------------

describe("FileOutboxStore", () => {
  let dir: string;

  beforeEach(() => {
    dir = mkdtempSync(join(tmpdir(), "bothy-outbox-"));
  });

  afterEach(() => {
    rmSync(dir, { recursive: true, force: true });
  });

  it("survives a restart and resends with the same id", async () => {
    const path = join(dir, "state", "outbox.json");
    const server = new MockServer();
    server.answers.set(THREAD_A, [503]);
    const store = new FileOutboxStore(path);
    const entry = await new Outbox(mockClient(server), {
      store,
      logger: silentLogger,
    }).sendMessage(THREAD_A, "survives", { runId: RUN });
    expect(entry.state).toBe(OutboxState.PENDING);
    await store.close();
    if (process.platform !== "win32") {
      expect(statSync(path).mode & 0o777).toBe(0o600);
      expect(statSync(join(dir, "state")).mode & 0o777).toBe(0o700);
    }

    const reopened = new FileOutboxStore(path);
    const [kept, ...more] = await reopened.pending();
    expect(more).toEqual([]);
    expect([kept?.key, kept?.threadId, kept?.runId]).toEqual([
      entry.key,
      THREAD_A,
      RUN,
    ]);
    await new Outbox(mockClient(server), { store: reopened }).flush();
    expect(server.keys(THREAD_A)).toEqual([entry.key, entry.key]);
    expect(await reopened.pending()).toEqual([]);
    await reopened.close();

    const third = new FileOutboxStore(path);
    expect(await third.pending()).toEqual([]);
    expect(await third.failed()).toEqual([]);
    await third.close();
  });

  it("keeps a failed entry across a reopen and numbers entries on", async () => {
    const path = join(dir, "outbox.json");
    const server = new MockServer();
    server.answers.set(THREAD_A, [409]);
    const store = new FileOutboxStore(path);
    const outbox = new Outbox(mockClient(server), {
      store,
      logger: silentLogger,
    });
    const entry = await outbox.sendMessage(THREAD_A, "conflict");
    const sent = await outbox.sendMessage(THREAD_A, "fine");
    await store.close();
    const reopened = new FileOutboxStore(path);
    expect(
      (await reopened.failed()).map((e) => [e.key, e.state, e.problemCode]),
    ).toEqual([[entry.key, OutboxState.FAILED, "conflict"]]);
    const next = await reopened.add({
      ...entry,
      entryId: 0,
      key: "other",
      state: OutboxState.PENDING,
    });
    expect(next.entryId).toBeGreaterThan(sent.entryId);
    await reopened.close();
  });

  it("refuses a store file that others can read and a key stored twice", async () => {
    const path = join(dir, "outbox.json");
    const store = new FileOutboxStore(path);
    const entry = await store.add({
      entryId: 0,
      threadId: THREAD_A,
      kind: "message",
      key: "01900000-0000-7000-8000-00000000f001",
      payload: {},
      state: OutboxState.PENDING,
    });
    await expect(store.add({ ...entry, entryId: 0 })).rejects.toThrow(
      /already stored/,
    );
    await store.close();
    if (process.platform !== "win32") {
      const { chmodSync } = await import("node:fs");
      chmodSync(path, 0o644);
      await expect(new FileOutboxStore(path).pending()).rejects.toThrow(/0600/);
    }
  });
});

// Outbox.call -----------------------------------------------------------------------------------

describe("Outbox.call", () => {
  let dir: string;

  beforeEach(() => {
    dir = mkdtempSync(join(tmpdir(), "bothy-outbox-"));
  });

  afterEach(() => {
    rmSync(dir, { recursive: true, force: true });
  });

  it.each([false, true])(
    "stores the Idempotency-Key before the first attempt and repeats it (durable %s)",
    async (durable) => {
      const server = new MockServer();
      server.answers.set("commands", [503]);
      const store: OutboxStore = durable
        ? new FileOutboxStore(join(dir, "outbox.json"))
        : new MemoryOutboxStore();
      let storedBefore: string[] = [];
      server.observe = async () => {
        storedBefore = (await store.pending()).map((e) => e.key);
      };
      const outbox = new Outbox(mockClient(server), {
        store,
        logger: silentLogger,
      });
      const entry = await outbox.call(
        "post",
        "/commands",
        { body: { name: "status", description: "Show the run status" } },
        { threadId: THREAD_A },
      );
      expect(entry.state).toBe(OutboxState.PENDING);
      expect(entry.key).toMatch(UUID7);
      expect(storedBefore).toEqual([entry.key]);
      const [stored] = await store.pending();
      expect([stored?.key, stored?.kind]).toEqual([entry.key, "call"]);
      await outbox.flush();
      expect(server.keys("commands")).toEqual([entry.key, entry.key]);
      expect(await store.pending()).toEqual([]);
      await store.close();
    },
  );

  it("repeats the key across the retries of one attempt", async () => {
    const server = new MockServer();
    server.answers.set("commands", [502, 503]);
    const clock = new FakeClock();
    const outbox = new Outbox(
      mockClient(server, { retryMaxAttempts: 3 }, clock),
    );
    const entry = await drive(
      clock,
      outbox.call(
        "post",
        "/commands",
        { body: { name: "status", description: "Show the run status" } },
        { threadId: THREAD_A },
      ),
      1_000,
    );
    expect(entry.state).toBe(OutboxState.SENT);
    expect(server.keys("commands")).toEqual([entry.key, entry.key, entry.key]);
  });

  it("refuses an argument it cannot store", async () => {
    const server = new MockServer();
    const outbox = new Outbox(mockClient(server));
    for (const body of [
      () => 1,
      new Date(0),
      Number.NaN,
      1n,
      { nested: [undefined] },
    ])
      await expect(
        outbox.call("post", "/commands", { body } as never, {
          threadId: THREAD_A,
        }),
      ).rejects.toThrow(/cannot store/);
    await expect(
      outbox.call(
        "post",
        "/commands",
        {
          body: { name: "s", description: "d" },
          params: { header: { "Idempotency-Key": "x" } },
        } as never,
        { threadId: THREAD_A },
      ),
    ).rejects.toThrow(/header/);
    expect(server.seen).toEqual([]);
    expect(await outbox.store.pending()).toEqual([]);
  });
});

// Run binding -----------------------------------------------------------------------------------

describe("run binding", () => {
  it("sends every entry bound to its run and never unbound", async () => {
    const runA = "01900000-0000-7000-8000-0000000000aa";
    const runB = "01900000-0000-7000-8000-0000000000bb";
    const server = new MockServer();
    const tokens = new Map<string, string>([[runA, "token-of-a"]]);
    const failed: (string | undefined)[] = [];
    const api = mockClient(server);
    const outbox = new Outbox(api, {
      tokenFor: (runId) => Promise.resolve(tokens.get(runId)),
      onFailed: (e) => {
        failed.push(e.problemCode);
      },
      logger: silentLogger,
    });
    tokens.set(runB, "token-of-b");
    const other = await api.bindRun(runB, "token-of-b", async () => {
      // Inside run B: an entry of run A carries A's token; a write into another thread without
      // runId still carries B's token (09 4.7: every request inside a run).
      await outbox.sendMessage(THREAD_A, "for A", { runId: runA });
      return outbox.sendMessage(THREAD_B, "into another thread");
    });
    // Outside any run: no header.
    await outbox.sendMessage(THREAD_B, "outside");
    tokens.delete(runB);
    const entry = await outbox.sendMessage(THREAD_B, "for B", { runId: runB });
    // Tokens are compared by equality only and never printed.
    expect(server.seen.map((s) => s.token)).toEqual([
      "token-of-a",
      "token-of-b",
      null,
    ]);
    expect(other.runId).toBe(runB);
    expect(entry.state).toBe(OutboxState.FAILED);
    expect(failed).toEqual(["run_token_unavailable"]);
  });

  it("sends an entry stored outside a run unbound, also inside another run", async () => {
    const server = new MockServer();
    server.answers.set(THREAD_A, [503]);
    const api = mockClient(server);
    const outbox = new Outbox(api, { logger: silentLogger });
    await outbox.sendMessage(THREAD_A, "outside");
    await api.bindRun(RUN, "token-of-run", () => outbox.flush());
    expect(server.seen.map((s) => s.token)).toEqual([null, null]);
  });
});
