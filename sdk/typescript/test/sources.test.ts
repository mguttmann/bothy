// SPDX-License-Identifier: Apache-2.0
//
// Event sources: envelopes, the ack window, cursor frames, polling waits and the heartbeat (port
// of sdk/python/tests/test_sources.py). docs/spec/09-agents.md 4.2 and docs/DECISIONS.md
// D-M2-29-6: the acknowledged `seq` is the highest one up to which every delivered event is
// settled; a cursor counts as delivered and settled; the polling source polls again at once
// after a non-empty page and waits `sdk.poll_interval` after an empty one; it sends
// `sendHeartbeat` every `HeartbeatAck.interval_ms`. Tokens are compared by value inside the
// test only and never printed.

import { inspect } from "node:util";

import { describe, expect, it } from "vitest";

import { Secret } from "../src/auth/secret.js";
import { createApiClient, type ApiClient } from "../src/client.js";
import type { SdkSettings } from "../src/config.js";
import type { components } from "../src/generated/openapi.js";
import {
  AckWindow,
  CloseReason,
  CursorAdvance,
  Envelope,
  GatewayEvent,
  GatewaySource,
  PollingSource,
  SourceReady,
  envelopeFrom,
  type Delivery,
  type EventSource,
} from "../src/gateway/index.js";
import { FakeClock } from "../src/testing/clock.js";
import { FIXTURES } from "../src/testing/fake/fixtures.js";
import { eventually as patiently } from "../src/testing/patience.js";

type Json = Record<string, unknown>;
type Heartbeat = components["schemas"]["Heartbeat"];

const BASE = "https://example.com/api/v1";
const CONSUMER = "alpha";
const POLL_PATH = `/api/v1/agents/me/consumers/${CONSUMER}/events`;
const ACK_PATH = `/api/v1/agents/me/consumers/${CONSUMER}/ack`;
const HEARTBEAT_PATH = "/api/v1/agents/me/heartbeat";
const AUTOCOMPLETE_PATH = "/api/v1/command-autocompletions/";
const SYNC_EPOCH = "01900000-0000-7000-8000-0000000000e1";
// A step that adds exactly, so "just before" plus EPS lands on the deadline itself.
const EPS = 250;
// A placeholder token: the value carries no meaning.
const TOKEN = "example-invocation-token-for-tests";

/** The recorded `agent.invocation` envelope with its own `id` and `seq`. */
function envelope(seq: number | undefined, extra: Json = {}): Json {
  const data: Json = structuredClone(FIXTURES["agent.invocation"]);
  data.id = `01900000-0000-7000-8000-${String(seq ?? 0).padStart(12, "0")}`;
  if (seq === undefined) delete data.seq;
  else data.seq = seq;
  return { ...data, ...extra };
}

function autocomplete(): Json {
  return structuredClone(FIXTURES["command.autocomplete_requested"]);
}

const turn = (): Promise<void> => new Promise((r) => setImmediate(r));

/** Lets the other tasks run (a request through the fake fetch takes a few turns). */
async function turns(n = 50): Promise<void> {
  for (let i = 0; i < n; i++) await turn();
}

async function until(condition: () => boolean): Promise<void> {
  await patiently(condition, "the condition of the test", { pauseMs: 0 });
}

function json(status: number, body: unknown): Response {
  return new Response(JSON.stringify(body), {
    status,
    headers: { "content-type": "application/json" },
  });
}

/** A fake `fetch` of the polling operations that records every request. */
class Responder {
  readonly polls: number[] = [];
  readonly acks: number[] = [];
  readonly heartbeats: number[] = [];
  readonly answers: Json[] = [];
  intervalsMs: number[] = [3000];

  constructor(
    readonly clock: FakeClock,
    readonly pages: Json[],
  ) {}

  readonly fetch = async (request: Request): Promise<Response> => {
    const path = new URL(request.url).pathname;
    if (request.method === "GET" && path === POLL_PATH) return this.poll();
    if (request.method === "POST" && path === ACK_PATH) {
      const body = (await request.json()) as { seq: number };
      this.acks.push(body.seq);
      return json(200, {
        consumer: CONSUMER,
        acked_seq: body.seq,
        head_seq: body.seq,
      });
    }
    if (request.method === "POST" && path === HEARTBEAT_PATH) {
      this.heartbeats.push(this.clock.monotonic());
      // Each answer takes the next interval; the last one repeats.
      const interval =
        this.intervalsMs.length > 1
          ? (this.intervalsMs.shift() ?? 0)
          : (this.intervalsMs[0] ?? 0);
      return json(200, {
        presence: {
          state: "online",
          last_heartbeat_at: "2026-01-01T00:00:00Z",
          billing_mode_matches: true,
        },
        interval_ms: interval,
      });
    }
    if (request.method === "POST" && path.startsWith(AUTOCOMPLETE_PATH)) {
      this.answers.push((await request.json()) as Json);
      return new Response(null, { status: 204 });
    }
    return json(404, { status: 404, code: "not_found", title: "x" });
  };

  protected poll(): Response {
    this.polls.push(this.clock.monotonic());
    const page = this.pages.shift() ?? { events: [], next_cursor: 0 };
    return json(200, { head_seq: 0, sync_epoch: SYNC_EPOCH, ...page });
  }
}

function client(
  clock: FakeClock,
  responder: Responder,
  settings: Partial<SdkSettings>,
): ApiClient {
  return createApiClient({
    baseUrl: BASE,
    token: new Secret("example-api-token"),
    settings,
    clock,
    fetch: responder.fetch,
  });
}

/** Iterates a source in the background and collects its deliveries. */
class Running {
  readonly deliveries: Delivery[] = [];
  readonly done: Promise<void>;
  finished = false;

  constructor(readonly source: EventSource) {
    this.done = (async () => {
      for await (const delivery of source) this.deliveries.push(delivery);
      this.finished = true;
    })();
  }

  envelopes(): Envelope[] {
    return this.deliveries.filter((d): d is Envelope => d instanceof Envelope);
  }

  async stop(clock: FakeClock): Promise<void> {
    await this.source.close();
    await clock.advance(0);
    await turns();
    await this.done;
  }
}

describe("envelopeFrom", () => {
  it("takes the token out of raw", () => {
    const env = envelopeFrom(
      envelope(5, { invocation_token: TOKEN, redelivered: true }),
    );
    expect(env.raw).not.toHaveProperty("invocation_token");
    expect(env.token).toBeInstanceOf(Secret);
    expect(env.token?.reveal() === TOKEN).toBe(true);
    expect(inspect(env, { depth: 10 })).not.toContain(TOKEN);
    expect(JSON.stringify(env)).not.toContain(TOKEN);
    expect(env.event.kind).toBe("known");
    expect(env.type).toBe("agent.invocation");
    expect(env.seq).toBe(5);
    expect(env.id).toBe("01900000-0000-7000-8000-000000000005");
    expect(env.redelivered).toBe(true);
    expect(env.ephemeral).toBe(false);
  });

  it("counts an empty token as missing", () => {
    const env = envelopeFrom(envelope(6, { invocation_token: "" }));
    expect(env.token).toBeUndefined();
    expect(env.raw).not.toHaveProperty("invocation_token");
    expect(envelopeFrom(envelope(7)).token).toBeUndefined();
    expect(envelopeFrom(envelope(8)).redelivered).toBe(false);
  });

  it("decodes autocomplete as an ephemeral event", () => {
    const env = envelopeFrom({ ...autocomplete(), invocation_token: TOKEN });
    expect(env.event.kind).toBe("ephemeral");
    if (env.event.kind === "ephemeral")
      expect(env.event.event.payload.request_id).toBe(
        "01900000-0000-7000-8000-000000000602",
      );
    expect(env.seq).toBeUndefined();
    expect(env.ephemeral).toBe(true);
    expect(env.token).toBeDefined();
  });

  it("keeps an unknown type acknowledgeable", () => {
    const env = envelopeFrom({
      id: "01900000-0000-7000-8000-000000000011",
      type: "brand.new_event",
      seq: 11,
    });
    expect(env.event.kind).toBe("unknown");
    expect(env.seq).toBe(11);
  });

  it.each(["id", "type"])("refuses an envelope without %s", (missing) => {
    const data = envelope(9);
    delete data[missing];
    expect(() => envelopeFrom(data)).toThrow(/id or type/);
  });

  it("refuses a malformed id", () => {
    expect(() => envelopeFrom(envelope(9, { id: "not-a-uuid" }))).toThrow(
      /id or type/,
    );
  });
});

describe("AckWindow", () => {
  it("never skips an unsettled seq", () => {
    const window = new AckWindow();
    for (const seq of [5, 6, 7]) window.delivered(seq);
    expect(window.settle(6)).toBeUndefined();
    expect(window.settle(7)).toBeUndefined();
    expect(window.acked).toBe(0);
    expect(window.settle(5)).toBe(7);
    expect(window.acked).toBe(7);
    // Settling again or settling an acknowledged seq moves nothing.
    expect(window.settle(7)).toBeUndefined();
    expect(window.settle(4)).toBeUndefined();
  });

  it("lets cursors settle, blocked by a lower open seq", () => {
    const window = new AckWindow();
    window.delivered(3);
    // A cursor counts as delivered and settled, but an unsettled lower seq still blocks it.
    expect(window.settle(5)).toBeUndefined();
    expect(window.settle(3)).toBe(5);
    // A cursor alone moves the point when nothing below it is open.
    expect(window.settle(9)).toBe(9);
    // A cursor below an open seq moves the point up to the cursor only.
    window.delivered(12);
    expect(window.settle(10)).toBe(10);
    expect(window.settle(12)).toBe(12);
  });

  it("never settles an open event through a cursor of the same seq", () => {
    const window = new AckWindow();
    window.delivered(4);
    expect(window.cursor(4)).toBeUndefined();
    expect(window.settle(4)).toBe(4);
  });

  it("forgets its point on reset", () => {
    const window = new AckWindow();
    window.delivered(2);
    expect(window.settle(2)).toBe(2);
    window.reset();
    expect(window.acked).toBe(0);
    window.delivered(2);
    expect(window.settle(2)).toBe(2);
  });
});

describe("GatewaySource", () => {
  /** A gateway double that yields fixed frames and records acks. */
  class StubGateway {
    readonly closeReason = new CloseReason(1000, "done");
    readonly acks: number[] = [];
    readonly answers: [string, unknown][] = [];
    closed = false;

    constructor(private readonly events: GatewayEvent[]) {}

    async *frames(): AsyncGenerator<GatewayEvent> {
      for (const event of this.events) {
        await turn();
        yield event;
      }
    }

    ack(seq: number): Promise<void> {
      this.acks.push(seq);
      return Promise.resolve();
    }

    sendAutocompleteResult(requestId: string, choices: unknown): Promise<void> {
      this.answers.push([requestId, choices]);
      return Promise.resolve();
    }

    close(): Promise<void> {
      this.closed = true;
      return Promise.resolve();
    }
  }

  it("turns frames into deliveries and acknowledges the settled point", async () => {
    const gateway = new StubGateway([
      new GatewayEvent("ready", {}),
      new GatewayEvent("dispatch", envelope(3, { invocation_token: TOKEN })),
      new GatewayEvent("cursor", { seq: 5 }),
    ]);
    const source = new GatewaySource(gateway);
    const deliveries: Delivery[] = [];
    for await (const d of source) deliveries.push(d);
    expect(deliveries[0]).toBeInstanceOf(SourceReady);
    expect(deliveries[1]).toBeInstanceOf(Envelope);
    expect((deliveries[1] as Envelope).seq).toBe(3);
    expect(deliveries[2]).toEqual(new CursorAdvance(5));
    expect(source.closeReason).toEqual(new CloseReason(1000, "done"));
    await source.settle(5);
    expect(gateway.acks).toEqual([]);
    await source.settle(3);
    expect(gateway.acks).toEqual([5]);
  });

  it("marks resumed and resync, skips a dispatch without id and reports error frames", async () => {
    const errors: GatewayEvent[] = [];
    const error = new GatewayEvent("error", { op: "run.update", code: "x" });
    const gateway = new StubGateway([
      new GatewayEvent("resumed", {}),
      new GatewayEvent("dispatch", { type: "agent.invocation", seq: 1 }),
      error,
      new GatewayEvent("resync_required", {}),
    ]);
    const source = new GatewaySource(gateway);
    source.onError = (frame) => errors.push(frame);
    const deliveries: Delivery[] = [];
    for await (const d of source) deliveries.push(d);
    expect(deliveries).toEqual([
      new SourceReady({ resumed: true }),
      new SourceReady({ resync: true }),
    ]);
    expect(errors).toEqual([error]);
    await source.answerAutocomplete("01900000-0000-7000-8000-000000000602", [
      { name: "Alex", value: "alex" },
    ]);
    expect(gateway.answers).toHaveLength(1);
    await source.close();
    expect(gateway.closed).toBe(true);
  });
});

describe("PollingSource", () => {
  it("never acknowledges past an unsettled seq", async () => {
    const clock = new FakeClock();
    const page = {
      events: [5, 6, 7].map((s) => envelope(s, { invocation_token: TOKEN })),
      next_cursor: 7,
    };
    const responder = new Responder(clock, [page]);
    const settings = { pollIntervalMs: 1_000_000 };
    const running = new Running(
      new PollingSource(client(clock, responder, settings), CONSUMER, {
        settings,
        clock,
      }),
    );
    await until(() => running.envelopes().length === 3);
    expect(running.envelopes().map((e) => e.seq)).toEqual([5, 6, 7]);
    await running.source.settle(6);
    await running.source.settle(7);
    expect(responder.acks).toEqual([]);
    await running.source.settle(5);
    expect(responder.acks).toEqual([7]);
    await running.stop(clock);
  });

  it("does not settle an unsettled event through the page cursor", async () => {
    // The page cursor names the last seq read, which is the seq of the page's last event. The
    // adapter settles a CursorAdvance at once; that must not settle the event itself, which
    // stays unsettled while its token check failed (D-M2-29-6, 04 15.2 point 1).
    const clock = new FakeClock();
    const page = {
      events: [5, 6, 7].map((s) => envelope(s, { invocation_token: TOKEN })),
      next_cursor: 7,
    };
    const responder = new Responder(clock, [page]);
    const settings = { pollIntervalMs: 1_000_000 };
    const running = new Running(
      new PollingSource(client(clock, responder, settings), CONSUMER, {
        settings,
        clock,
      }),
    );
    await until(() =>
      running.deliveries.some((d) => d instanceof CursorAdvance && d.seq === 7),
    );
    await running.source.settle(5);
    await running.source.settle(6);
    await running.source.advance(7);
    expect(Math.max(0, ...responder.acks)).toBeLessThanOrEqual(6);
    await running.stop(clock);
  });

  it("waits sdk.poll_interval only after an empty page", async () => {
    const clock = new FakeClock();
    const interval = 7_000;
    const pages = [
      { events: [envelope(1, { invocation_token: TOKEN })], next_cursor: 1 },
      { events: [envelope(2, { invocation_token: TOKEN })], next_cursor: 2 },
      { events: [], next_cursor: 2 },
      { events: [], next_cursor: 2 },
    ];
    const responder = new Responder(clock, pages);
    const settings = { pollIntervalMs: interval };
    const running = new Running(
      new PollingSource(client(clock, responder, settings), CONSUMER, {
        settings,
        clock,
      }),
    );
    await until(() => responder.polls.length === 3 && clock.sleepers === 1);
    // Two non-empty pages: the next poll came at once; the empty third page waits.
    expect(responder.polls).toEqual([0, 0, 0]);
    expect(running.deliveries[0]).toBeInstanceOf(SourceReady);
    expect(running.deliveries).toContainEqual(new CursorAdvance(2));
    await clock.advance(interval - EPS);
    await turns();
    expect(responder.polls).toHaveLength(3);
    await clock.advance(EPS);
    await until(() => responder.polls.length === 4 && clock.sleepers === 1);
    expect(responder.polls[3]).toBe(interval);
    // The fourth page is empty too: the next poll waits a full interval again.
    await clock.advance(interval - EPS);
    await turns();
    expect(responder.polls).toHaveLength(4);
    await clock.advance(EPS);
    await until(() => responder.polls.length === 5);
    expect(responder.polls[4]).toBe(2 * interval);
    await running.stop(clock);
    expect(running.finished).toBe(true);
    expect(running.source.closeReason).toEqual(
      new CloseReason(1000, "polling stopped"),
    );
  });

  it("sends the heartbeat every HeartbeatAck.interval_ms", async () => {
    const clock = new FakeClock();
    const responder = new Responder(clock, []);
    responder.intervalsMs = [3000, 5000];
    const settings = { pollIntervalMs: 1_000_000 };
    const beat: Heartbeat = { state: "online" };
    const running = new Running(
      new PollingSource(client(clock, responder, settings), CONSUMER, {
        settings,
        clock,
        heartbeat: () => beat,
      }),
    );
    await until(
      () => responder.heartbeats.length === 1 && clock.sleepers === 2,
    );
    expect(responder.heartbeats).toEqual([0]);
    await clock.advance(3000 - EPS);
    await turns();
    expect(responder.heartbeats).toHaveLength(1);
    await clock.advance(EPS);
    await until(
      () => responder.heartbeats.length === 2 && clock.sleepers === 2,
    );
    // The second answer names 5000 ms: the third beat follows that interval.
    await clock.advance(5000 - EPS);
    await turns();
    expect(responder.heartbeats).toHaveLength(2);
    await clock.advance(EPS);
    await until(() => responder.heartbeats.length === 3);
    expect(responder.heartbeats).toEqual([0, 3000, 8000]);
    await running.stop(clock);
  });

  it("sends no heartbeat without a heartbeat source", async () => {
    const clock = new FakeClock();
    const responder = new Responder(clock, []);
    const settings = { pollIntervalMs: 1000 };
    const running = new Running(
      new PollingSource(client(clock, responder, settings), CONSUMER, {
        settings,
        clock,
      }),
    );
    await until(() => responder.polls.length === 1);
    await clock.advance(5000);
    await turns();
    expect(responder.heartbeats).toEqual([]);
    await running.stop(clock);
  });

  it("survives a server restart", async () => {
    class Failing extends Responder {
      failures = 3;

      protected override poll(): Response {
        if (this.failures > 0) {
          this.failures--;
          this.polls.push(this.clock.monotonic());
          return new Response("<html>restarting</html>", { status: 503 });
        }
        return super.poll();
      }
    }
    const clock = new FakeClock();
    const page = {
      events: [envelope(3, { invocation_token: TOKEN })],
      next_cursor: 3,
    };
    const responder = new Failing(clock, [page]);
    const settings = { pollIntervalMs: 1_000_000, retryMaxAttempts: 1 };
    const running = new Running(
      new PollingSource(client(clock, responder, settings), CONSUMER, {
        settings,
        clock,
      }),
    );
    await patiently(() => running.envelopes().length > 0, "an envelope", {
      step: async () => {
        await clock.advance(1000);
        await turn();
      },
      pauseMs: 0,
    });
    expect(running.envelopes().map((e) => e.seq)).toEqual([3]);
    expect(responder.polls.length).toBeGreaterThanOrEqual(4);
    expect(running.finished).toBe(false);
    await running.stop(clock);
  });

  it("yields a resync on 410 and stops for good on 401", async () => {
    class Answers extends Responder {
      statuses = [410, 401];

      protected override poll(): Response {
        const status = this.statuses.shift();
        if (status === undefined) return super.poll();
        this.polls.push(this.clock.monotonic());
        const code = status === 410 ? "resync_required" : "unauthenticated";
        return new Response(JSON.stringify({ status, code, title: "x" }), {
          status,
          headers: { "content-type": "application/problem+json" },
        });
      }
    }
    const clock = new FakeClock();
    const responder = new Answers(clock, []);
    const settings = { pollIntervalMs: 1000 };
    const running = new Running(
      new PollingSource(client(clock, responder, settings), CONSUMER, {
        settings,
        clock,
      }),
    );
    await running.done;
    expect(running.deliveries).toEqual([
      new SourceReady(),
      new SourceReady({ resync: true }),
    ]);
    expect(running.source.closeReason).toEqual(
      new CloseReason(4004, "unauthenticated"),
    );
    expect(running.source.closeReason?.permanent).toBe(true);
  });

  it("answers autocomplete over REST and swallows a failed ack", async () => {
    const clock = new FakeClock();
    const responder = new Responder(clock, []);
    const failing = createApiClient({
      baseUrl: BASE,
      token: new Secret("example-api-token"),
      clock,
      settings: { retryMaxAttempts: 1 },
      fetch: (request: Request) =>
        new URL(request.url).pathname === ACK_PATH
          ? Promise.resolve(new Response("x", { status: 503 }))
          : responder.fetch(request),
    });
    const source = new PollingSource(failing, CONSUMER, { clock });
    await source.answerAutocomplete("01900000-0000-7000-8000-000000000602", [
      { name: "Sam", value: "sam" },
    ]);
    expect(responder.answers).toEqual([
      { choices: [{ name: "Sam", value: "sam" }] },
    ]);
    await expect(source.ack(4)).resolves.toBeUndefined();
  });
});
