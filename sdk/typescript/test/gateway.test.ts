// SPDX-License-Identifier: Apache-2.0
//
// Gateway: frames, DPoP proof, heartbeat, close codes, reconnect and tolerance (port of
// sdk/python/tests/test_gateway.py plus the cases of the brief's "gateway.test.ts" bullet).
// Scripted cases run against a `ws` server on 127.0.0.1 port 0 that plays exactly the frames a
// case needs; the end-to-end cases run against the FakeServer's gateway. Tokens are compared
// inside the test only and never printed.

import { createHash, createPublicKey, verify } from "node:crypto";
import { readFileSync } from "node:fs";
import type { AddressInfo } from "node:net";

import { afterEach, describe, expect, it } from "vitest";
import { WebSocket, WebSocketServer, type RawData } from "ws";

import { DpopKey, gatewayHtu } from "../src/auth/dpop.js";
import { Secret } from "../src/auth/secret.js";
import type { Credentials } from "../src/auth/types.js";
import { createApiClient } from "../src/client.js";
import type { SdkSettings } from "../src/config.js";
import type { components } from "../src/generated/openapi.js";
import { GatewayOpError } from "../src/errors.js";
import {
  CursorAdvance,
  Envelope,
  Gateway,
  GatewaySource,
  IDENTIFY_ON,
  PROTOCOL_VERSIONS,
  PollingSource,
  RESUME_ON,
  STOP_ON,
  SourceReady,
  gatewayUrl,
  type Delivery,
  type EventSource,
  type GatewayEvent,
  type GatewayOptions,
} from "../src/gateway/index.js";
import { FakeClock } from "../src/testing/clock.js";
import { FakeServer } from "../src/testing/fakeserver.js";
import {
  eventually as patiently,
  withinPatience,
} from "../src/testing/patience.js";

type Json = Record<string, any>; // eslint-disable-line @typescript-eslint/no-explicit-any

const AGENT = "0192f4a0-7b1c-7000-8a00-0000000000a2";
const OTHER = "0192f4a0-7b1c-7000-8a00-0000000000ff";
const SESSION = "0192f4a0-7b1c-7000-8a00-0000000000b1";
const EPOCH = "0192f4a0-7b1c-7000-8a00-0000000000c1";
const TOKEN = "gateway-api-token-1";
const FAST: Partial<SdkSettings> = {
  reconnectBackoffBaseMs: 10,
  reconnectBackoffCapMs: 20,
};
// The start of a backoff wait is known to within the last clock step of a test.
const MARGIN = 50;
const ASYNCAPI = new URL("../../../docs/api/asyncapi.yaml", import.meta.url);

/** A seeded random in [0, 1) (mulberry32), so jitter is reproducible. */
function seeded(seed: number): () => number {
  let a = seed >>> 0;
  return () => {
    a = (a + 0x6d2b79f5) >>> 0;
    let t = a;
    t = Math.imul(t ^ (t >>> 15), t | 1);
    t ^= t + Math.imul(t ^ (t >>> 7), t | 61);
    return ((t ^ (t >>> 14)) >>> 0) / 4294967296;
  };
}

/** A random whose next values a test sets; it repeats the last one. */
function scriptedRandom(...values: number[]): () => number {
  let last = values[0] ?? 0;
  return () => {
    last = values.length > 0 ? (values.shift() ?? last) : last;
    return last;
  };
}

const sleep = (ms: number): Promise<void> =>
  new Promise((r) => setTimeout(r, ms));

async function until(condition: () => boolean): Promise<void> {
  await patiently(condition, "the condition of the test");
}

function hello(intervalMs = 600_000, extra: Json = {}): Json {
  return {
    op: "hello",
    d: {
      heartbeat_interval_ms: intervalMs,
      resume_window_ms: 300_000,
      protocol_versions: [1],
      dpop_nonce: "nonce-1",
      server_time: "2026-01-01T00:00:00Z",
      sync_epoch: EPOCH,
      ...extra,
    },
  };
}

function ready(principal = AGENT): Json {
  const who = {
    id: principal,
    kind: "agent",
    handle: "lotse",
    display_name: "Lotse",
  };
  const snapshot = {
    head_seq: 10,
    permission_version: 1,
    sync_epoch: EPOCH,
    principal: who,
    is_owner: false,
    spaces: [],
    channels: [],
    sidebar: [],
    unread_threads: [],
    active_runs: [],
    agents: [],
    settings: [],
    drafts: [],
    features: [],
  };
  return {
    op: "ready",
    d: {
      gateway_session_id: SESSION,
      principal: who,
      permission_version: 1,
      head_seq: 10,
      snapshot,
      sync_epoch: EPOCH,
    },
  };
}

/** One server-side socket of {@link Scripted} with a receive queue. */
class Peer {
  private readonly queue: Json[] = [];
  private readonly waiters: ((frame: Json | undefined) => void)[] = [];
  closed = false;
  closeCode: number | undefined;
  closeReason = "";

  constructor(
    readonly ws: WebSocket,
    private readonly log: Json[],
  ) {
    ws.on("message", (data: RawData) => {
      const frame = JSON.parse(
        Buffer.from(data as Buffer).toString("utf8"),
      ) as Json;
      this.log.push(frame);
      const waiter = this.waiters.shift();
      if (waiter) waiter(frame);
      else this.queue.push(frame);
    });
    ws.on("close", (code: number, reason: Buffer) => {
      this.closed = true;
      this.closeCode = code;
      this.closeReason = reason.toString();
      for (const waiter of this.waiters.splice(0)) waiter(undefined);
    });
  }

  send(frame: Json | string): void {
    this.ws.send(typeof frame === "string" ? frame : JSON.stringify(frame));
  }

  /** The next frame of the client; rejects when the socket closed first. */
  recv(): Promise<Json> {
    const frame = this.queue.shift();
    if (frame !== undefined) return Promise.resolve(frame);
    if (this.closed) return Promise.reject(new Error("closed"));
    return new Promise((resolve, reject) =>
      this.waiters.push((f) => (f ? resolve(f) : reject(new Error("closed")))),
    );
  }

  close(code: number, reason = "reason"): void {
    this.ws.close(code, reason);
  }

  waitClosed(): Promise<void> {
    return new Promise((resolve) => {
      if (this.closed) resolve();
      else this.ws.once("close", () => resolve());
    });
  }
}

type Script = (peer: Peer, n: number) => Promise<void>;

/** A scripted gateway: each connection runs `script(peer, n)`; the frames it got are recorded. */
class Scripted {
  connections = 0;
  readonly frames: Json[] = [];
  readonly peers: Peer[] = [];
  url = "";
  private readonly server: WebSocketServer;

  private constructor(script: Script) {
    this.server = new WebSocketServer({ host: "127.0.0.1", port: 0 });
    this.server.on("connection", (ws) => {
      this.connections++;
      const peer = new Peer(ws, this.frames);
      this.peers.push(peer);
      script(peer, this.connections).catch(() => undefined);
    });
  }

  static async start(script: Script): Promise<Scripted> {
    const scripted = new Scripted(script);
    await new Promise<void>((resolve) =>
      scripted.server.once("listening", () => resolve()),
    );
    const { port } = scripted.server.address() as AddressInfo;
    scripted.url = `ws://127.0.0.1:${port}/api/v1/gateway`;
    return scripted;
  }

  async stop(): Promise<void> {
    for (const client of this.server.clients) client.terminate();
    await new Promise<void>((resolve) => this.server.close(() => resolve()));
  }
}

/** The ops the scripted server received, without heartbeats. */
function ops(server: Scripted): string[] {
  return server.frames
    .map((f) => String(f.op))
    .filter((op) => op !== "heartbeat");
}

/** Waits until the server saw connection `n` close and the gateway sleeps before the next. */
async function closedAndWaiting(
  server: Scripted,
  clock: FakeClock,
  n: number,
): Promise<void> {
  await until(() => server.peers[n - 1]?.closed === true);
  await until(() => clock.sleepers >= 1);
}

const servers: { stop(): Promise<void> }[] = [];
const gateways: Gateway[] = [];

afterEach(async () => {
  for (const gw of gateways.splice(0)) await gw.close();
  for (const server of servers.splice(0)) await server.stop();
});

async function scripted(script: Script): Promise<Scripted> {
  const server = await Scripted.start(script);
  servers.push(server);
  return server;
}

function gateway(
  url: string,
  options: Partial<GatewayOptions> & { credentials?: Credentials } = {},
): Gateway {
  const { credentials, ...rest } = options;
  const gw = new Gateway(url, credentials ?? { apiToken: new Secret(TOKEN) }, {
    consumer: "alpha",
    agentId: AGENT,
    settings: FAST,
    random: seeded(1),
    ...rest,
  });
  gateways.push(gw);
  return gw;
}

/** Iterates `frames()` in the background; `until` closes the gateway. */
class Collector {
  readonly seen: GatewayEvent[] = [];
  readonly done: Promise<void>;

  constructor(
    readonly gw: Gateway,
    stopWhen: (seen: GatewayEvent[]) => boolean = () => false,
  ) {
    this.done = (async () => {
      for await (const frame of gw.frames()) {
        this.seen.push(frame);
        if (stopWhen(this.seen)) await gw.close();
      }
    })();
  }

  /** Waits for the iterator to end (a stop, a close or `stopWhen`), with the patience only. */
  async finish(): Promise<GatewayEvent[]> {
    await withinPatience(this.done, "the end of frames()");
    return this.seen;
  }
}

function claims(jws: string): Json {
  return JSON.parse(
    Buffer.from(jws.split(".")[1] ?? "", "base64url").toString("utf8"),
  ) as Json;
}

function header(jws: string): Json {
  return JSON.parse(
    Buffer.from(jws.split(".")[0] ?? "", "base64url").toString("utf8"),
  ) as Json;
}

type S = components["schemas"];

// The members of the client frames as the generated core declares them; `satisfies` lets tsc
// refuse a listed key the generated type does not have.
const IDENTIFY_KEYS = new Set<string>([
  "protocol_version",
  "access_token",
  "api_token",
  "dpop",
  "device_id",
  "consumer",
  "cursor",
  "sync_epoch",
  "client",
] satisfies (keyof S["GatewayIdentify"])[]);
const RESUME_KEYS = new Set<string>([
  "gateway_session_id",
  "access_token",
  "api_token",
  "dpop",
  "cursor",
  "sync_epoch",
] satisfies (keyof S["GatewayResume"])[]);

describe("protocol version", () => {
  it("equals info.x-protocol-versions of docs/api/asyncapi.yaml", () => {
    const text = readFileSync(ASYNCAPI, "utf8");
    const match = /^info:[\s\S]*?^ {2}x-protocol-versions: \[([\d, ]+)\]/m.exec(
      text,
    );
    expect(match).not.toBeNull();
    const versions = (match?.[1] ?? "").split(",").map((v) => Number(v.trim()));
    expect([...PROTOCOL_VERSIONS]).toEqual(versions);
  });
});

describe("gatewayUrl", () => {
  it("resolves the discovery URL against the API base as ws or wss", () => {
    expect(gatewayUrl("https://example.com/api/v1", "/api/v1/gateway")).toBe(
      "wss://example.com/api/v1/gateway",
    );
    expect(gatewayUrl("http://127.0.0.1:8099/api/v1", "/api/v1/gateway")).toBe(
      "ws://127.0.0.1:8099/api/v1/gateway",
    );
    expect(
      gatewayUrl("https://example.com/api/v1", "wss://gw.example.com/g#x"),
    ).toBe("wss://gw.example.com/g");
  });

  it("refuses clear text off the loopback host", () => {
    expect(() =>
      gatewayUrl("http://example.com/api/v1", "/api/v1/gateway"),
    ).toThrow(TypeError);
    expect(
      () =>
        new Gateway(
          "ws://example.com/api/v1/gateway",
          { apiToken: new Secret(TOKEN) },
          { consumer: "alpha", agentId: AGENT },
        ),
    ).toThrow(TypeError);
  });
});

describe("identify and resume", () => {
  it("sends identify with the DPoP proof of the hello nonce", async () => {
    const key = await DpopKey.generate({ alg: "EdDSA" });
    const server = await scripted(async (peer) => {
      peer.send(hello());
      await peer.recv();
      peer.send(ready());
      await peer.waitClosed();
    });
    const gw = gateway(server.url, {
      credentials: { apiToken: new Secret(TOKEN), dpop: key },
    });
    await new Collector(gw, (s) => s.length >= 1).finish();
    const identify = server.frames[0] ?? {};
    expect(identify.op).toBe("identify");
    const d = identify.d as Json;
    expect(Object.keys(d).sort()).toEqual(
      ["api_token", "consumer", "dpop", "protocol_version"].sort(),
    );
    expect(d.protocol_version).toBe(1);
    expect(d.consumer).toBe("alpha");
    expect(d.api_token === TOKEN).toBe(true);
    const proof = String(d.dpop);
    expect(header(proof)).toMatchObject({ typ: "dpop+jwt", alg: "EdDSA" });
    const c = claims(proof);
    expect(c.htm).toBe("GET");
    expect(c.htu).toBe(gatewayHtu(server.url));
    expect(String(c.htu).startsWith("https://127.0.0.1:")).toBe(true);
    expect(c.nonce).toBe("nonce-1");
    expect(c.ath).toBe(createHash("sha256").update(TOKEN).digest("base64url"));
    const [h, p, s] = proof.split(".");
    const publicKey = createPublicKey({
      key: header(proof).jwk as never,
      format: "jwk",
    });
    expect(
      verify(
        null,
        Buffer.from(`${h}.${p}`),
        publicKey,
        Buffer.from(s ?? "", "base64url"),
      ),
    ).toBe(true);
  });

  it("sends no proof for a bearer token and names the nonce to the sink", async () => {
    const nonces: (string | undefined)[] = [];
    const server = await scripted(async (peer) => {
      peer.send(hello());
      await peer.recv();
      peer.send(ready());
      await peer.waitClosed();
    });
    const gw = gateway(server.url, { nonceSink: (n) => nonces.push(n) });
    await new Collector(gw, (s) => s.length >= 1).finish();
    expect(server.frames[0]?.d).not.toHaveProperty("dpop");
    expect(nonces).toEqual(["nonce-1"]);
  });

  it("resumes within resume_window_ms", async () => {
    const server = await scripted(async (peer, n) => {
      peer.send(hello());
      await peer.recv();
      peer.send(ready());
      if (n === 1) {
        peer.close(1001, "going away");
        return;
      }
      await peer.waitClosed();
    });
    const gw = gateway(server.url);
    await new Collector(
      gw,
      () => server.connections >= 2 && server.frames.length >= 2,
    ).finish();
    const [identify, resume] = server.frames;
    expect(identify?.op).toBe("identify");
    expect(resume?.op).toBe("resume");
    const r = resume?.d as Json;
    for (const k of Object.keys(r)) expect(RESUME_KEYS.has(k)).toBe(true);
    for (const k of Object.keys(identify?.d as Json))
      expect(IDENTIFY_KEYS.has(k)).toBe(true);
    expect(r.gateway_session_id).toBe(SESSION);
    expect(r.cursor).toBe(0);
    expect(r.api_token === TOKEN).toBe(true);
  });

  it("identifies after the resume window and after 4006", async () => {
    const clock = new FakeClock();
    const server = await scripted(async (peer, n) => {
      peer.send(hello(1_000_000_000));
      const first = await peer.recv();
      if (n === 2 && first.op === "resume") {
        peer.close(4006, "session_invalid");
        return;
      }
      peer.send(ready());
      if (n === 1 || n === 3) peer.close(1001, "going away");
      await peer.waitClosed();
    });
    // The random values in order: first heartbeat, backoff after 1001 (a short wait, so the
    // second connection resumes), first heartbeat, backoff after 4006, first heartbeat, then a
    // backoff longer than the resume window, so the fourth connection identifies.
    const gw = gateway(server.url, {
      clock,
      random: scriptedRandom(0.5, 0.0001, 0.5, 0, 0.5, 0.999),
      settings: {
        reconnectBackoffBaseMs: 400_000,
        reconnectBackoffCapMs: 400_000,
      },
    });
    const collector = new Collector(gw);
    const step = async (connections: number): Promise<void> => {
      await patiently(
        () => server.connections >= connections,
        `${connections} connections`,
        { step: () => clock.advance(1000), pauseMs: 2 },
      );
    };
    await until(() => ops(server).length >= 1);
    await step(2);
    await until(() => ops(server).length >= 2);
    await step(3);
    await until(() => ops(server).length >= 3);
    await step(4);
    await until(() => ops(server).length >= 4);
    expect(ops(server)).toEqual(["identify", "resume", "identify", "identify"]);
    await gw.close();
    await collector.finish();
  });
});

describe("heartbeat", () => {
  it("sends the first heartbeat after the seeded random fraction", async () => {
    const clock = new FakeClock();
    let beat: Json | undefined;
    const server = await scripted(async (peer) => {
      peer.send(hello(10_000));
      await peer.recv();
      peer.send(ready());
      beat = await peer.recv();
      await peer.waitClosed();
    });
    const expected = seeded(5)() * 10_000;
    const gw = gateway(server.url, { clock, random: seeded(5) });
    const collector = new Collector(gw);
    await until(() => collector.seen.length >= 1);
    await clock.advance(Math.max(0, expected - 10));
    await sleep(50);
    expect(beat).toBeUndefined();
    await clock.advance(20);
    await until(() => beat !== undefined);
    expect(beat).toEqual({ op: "heartbeat", d: { last_seq: 0 } });
    await gw.close();
    await collector.finish();
  });

  it("carries last_seq and the agent heartbeat", async () => {
    const clock = new FakeClock();
    const beats: Json[] = [];
    const server = await scripted(async (peer) => {
      peer.send(hello(1000));
      await peer.recv();
      peer.send(ready());
      for (;;) {
        const frame = await peer.recv();
        if (frame.op === "heartbeat") {
          beats.push(frame);
          peer.send({
            op: "heartbeat_ack",
            d: { server_time: "2026-01-01T00:00:00Z" },
          });
        }
      }
    });
    const gw = gateway(server.url, {
      clock,
      random: scriptedRandom(0.5),
      heartbeat: () => ({ state: "online" }),
    });
    const collector = new Collector(gw);
    await until(() => collector.seen.length >= 1);
    await gw.ack(7);
    await clock.advance(500);
    await until(() => beats.length >= 1);
    expect(beats[0]).toEqual({
      op: "heartbeat",
      d: { last_seq: 7, agent: { state: "online" } },
    });
    expect(gw.lastSeq).toBe(7);
    expect(server.frames).toContainEqual({ op: "ack", d: { seq: 7 } });
    await gw.close();
    await collector.finish();
  });

  it("closes and reconnects when heartbeat_ack is missing", async () => {
    const clock = new FakeClock();
    const server = await scripted(async (peer) => {
      peer.send(hello(1000));
      await peer.recv();
      peer.send(ready());
      for (;;) await peer.recv(); // heartbeats, never acknowledged
    });
    const gw = gateway(server.url, {
      clock,
      settings: { reconnectBackoffBaseMs: 10, reconnectBackoffCapMs: 10 },
    });
    const collector = new Collector(gw);
    await until(() => collector.seen.length >= 1);
    await patiently(() => server.connections >= 2, "a second connection", {
      step: () => clock.advance(500),
    });
    expect(server.connections).toBeGreaterThanOrEqual(2);
    expect(server.frames.some((f) => f.op === "heartbeat")).toBe(true);
    expect(server.peers[0]?.closeReason).toBe("heartbeat_ack missing");
    // The second connection tries resume first: the first one ended like a 1001.
    await until(
      () => server.frames.filter((f) => f.op !== "heartbeat").length >= 2,
    );
    expect(server.frames.filter((f) => f.op !== "heartbeat")[1]?.op).toBe(
      "resume",
    );
    await gw.close();
    await collector.finish();
  });
});

describe("unanswered close", () => {
  /** A server that stops reading after `ready`, so a close of the client gets no answer. */
  async function deaf(): Promise<Scripted> {
    return scripted(async (peer, n) => {
      peer.send(hello(1000));
      await peer.recv();
      peer.send(ready());
      if (n === 1) {
        await sleep(20);
        (peer.ws as unknown as { _socket: { pause(): void } })._socket.pause();
      }
      await peer.waitClosed();
    });
  }

  it("ends frames() after sdk.http_timeout when close() gets no answer", async () => {
    const server = await deaf();
    const gw = gateway(server.url, {
      settings: { ...FAST, httpTimeoutMs: 200 },
    });
    const collector = new Collector(gw);
    await until(() => collector.seen.length >= 1);
    await sleep(50);
    await gw.close();
    await withinPatience(collector.done, "the end of frames()");
  });

  it("reconnects after a heartbeat timeout whose close gets no answer", async () => {
    const clock = new FakeClock();
    const server = await deaf();
    const gw = gateway(server.url, {
      clock,
      random: scriptedRandom(0.5),
      settings: { ...FAST, httpTimeoutMs: 200 },
    });
    const collector = new Collector(gw);
    await until(() => collector.seen.length >= 1);
    await sleep(50);
    await patiently(() => server.connections >= 2, "a second connection", {
      step: () => clock.advance(500),
      pauseMs: 30,
    });
    expect(server.connections).toBe(2);
    await gw.close();
    await collector.finish();
  });
});

describe("close codes", () => {
  const reconnecting = [
    ...[...RESUME_ON].sort(),
    ...[...IDENTIFY_ON].sort(),
    4001,
    4002,
    4003,
    4005,
    4999,
  ];

  it.each([...STOP_ON].sort())(
    "stops without reconnecting on %i",
    async (code) => {
      const server = await scripted(async (peer) => {
        peer.send(hello());
        await peer.recv();
        peer.send(ready());
        peer.close(code);
      });
      const gw = gateway(server.url);
      await new Collector(gw).finish();
      expect(server.connections).toBe(1);
      expect(gw.closeReason?.code).toBe(code);
      expect(gw.closeReason?.permanent).toBe(true);
      expect(gw.closeReason?.reconnects).toBe(false);
    },
  );

  it.each(reconnecting)("reconnects on %i", async (code) => {
    const server = await scripted(async (peer, n) => {
      peer.send(hello());
      await peer.recv();
      peer.send(ready());
      if (n === 1) {
        peer.close(code);
        return;
      }
      await peer.waitClosed();
    });
    const gw = gateway(server.url);
    await new Collector(
      gw,
      () => server.connections >= 2 && server.frames.length >= 2,
    ).finish();
    expect(server.connections).toBeGreaterThanOrEqual(2);
    expect(server.frames[1]?.op).toBe(
      IDENTIFY_ON.has(code) ? "identify" : "resume",
    );
  });

  /** Runs a gateway whose first connection the server closes before `ready`. */
  async function firstClosed(
    code: number,
    reason: string,
    options: Partial<GatewayOptions>,
  ): Promise<{ server: Scripted; clock: FakeClock; collector: Collector }> {
    const clock = new FakeClock();
    const server = await scripted(async (peer, n) => {
      peer.send(hello());
      await peer.recv();
      if (n === 1) {
        peer.close(code, reason);
        return;
      }
      peer.send(ready());
      await peer.waitClosed();
    });
    const gw = gateway(server.url, { clock, ...options });
    const collector = new Collector(gw);
    return { server, clock, collector };
  }

  /**
   * Expects connection `n + 1` exactly `ms` after connection `n` closed: the wait starts at the
   * fake time `closedAndWaiting` sees, because the clock stands still while the test waits.
   */
  async function reconnectsAfter(
    server: Scripted,
    clock: FakeClock,
    ms: number,
    n = 1,
  ): Promise<void> {
    await closedAndWaiting(server, clock, n);
    const start = clock.monotonic();
    await clock.advance(ms - MARGIN);
    await sleep(50);
    expect(server.connections).toBe(n);
    // Time moves only while the gateway still sleeps, so a slow wake-up cannot stretch the wait.
    await patiently(() => server.connections > n, `connection ${n + 1}`, {
      step: () => (clock.sleepers >= 1 ? clock.advance(5) : undefined),
    });
    expect(server.connections).toBe(n + 1);
    const waited = clock.monotonic() - start;
    expect(waited).toBeGreaterThanOrEqual(ms);
    expect(waited).toBeLessThanOrEqual(ms + 4 * MARGIN);
  }

  it("waits the integer seconds of a 4008 reason", async () => {
    const { server, clock, collector } = await firstClosed(
      4008,
      "retry_after=30",
      {},
    );
    await reconnectsAfter(server, clock, 30_000);
    await collector.gw.close();
    await collector.finish();
  });

  it("falls back to backoff on a 4008 reason without seconds", async () => {
    const { server, clock, collector } = await firstClosed(
      4008,
      "rate_limited",
      {
        random: scriptedRandom(0.5),
        settings: {
          reconnectBackoffBaseMs: 1000,
          reconnectBackoffCapMs: 60_000,
        },
      },
    );
    await reconnectsAfter(server, clock, 500);
    await collector.gw.close();
    await collector.finish();
  });

  it("reconnects after a uniform part of sync.reconnect_backoff_base on 4013", async () => {
    const { server, clock, collector } = await firstClosed(
      4013,
      "server_restart",
      {
        random: scriptedRandom(0.25),
        settings: {
          reconnectBackoffBaseMs: 2000,
          reconnectBackoffCapMs: 60_000,
        },
      },
    );
    await reconnectsAfter(server, clock, 500);
    await collector.gw.close();
    await collector.finish();
  });

  it("grows the backoff per attempt and resets it on ready", async () => {
    const clock = new FakeClock();
    const server = await scripted(async (peer, n) => {
      peer.send(hello());
      await peer.recv();
      if (n === 3) peer.send(ready());
      peer.close(4000, "unknown_error");
    });
    const gw = gateway(server.url, {
      clock,
      random: scriptedRandom(0.999),
      settings: { reconnectBackoffBaseMs: 1000, reconnectBackoffCapMs: 60_000 },
    });
    const collector = new Collector(gw);
    // Attempts 0 and 1 wait the base times 2^n; ready on the third connection resets n.
    await reconnectsAfter(server, clock, 999, 1);
    await reconnectsAfter(server, clock, 1998, 2);
    await reconnectsAfter(server, clock, 999, 3);
    expect(collector.seen.map((f) => f.op)).toEqual(["ready"]);
    await gw.close();
    await collector.finish();
  });

  it("keeps every backoff within [0, min(cap, base * 2^n)] and spreads it", () => {
    const gw = gateway("ws://127.0.0.1:1/g", {
      settings: { reconnectBackoffBaseMs: 1000, reconnectBackoffCapMs: 8000 },
      random: seeded(9),
    });
    for (let attempt = 0; attempt < 10; attempt++) {
      const bound = Math.min(8000, 1000 * 2 ** attempt);
      const waits = Array.from({ length: 50 }, () => gw.backoff(attempt));
      for (const w of waits) {
        expect(w).toBeGreaterThanOrEqual(0);
        expect(w).toBeLessThanOrEqual(bound);
      }
      expect(Math.max(...waits)).toBeGreaterThan(bound / 4);
    }
  });

  it("stops on ready for another principal", async () => {
    const server = await scripted(async (peer) => {
      peer.send(hello());
      await peer.recv();
      peer.send(ready(OTHER));
      await peer.waitClosed();
    });
    const gw = gateway(server.url);
    const seen = await new Collector(gw).finish();
    expect(server.connections).toBe(1);
    expect(seen.some((f) => f.op === "ready")).toBe(false);
    expect(gw.closeReason?.reason).toBe("principal_mismatch");
  });
});

describe("tolerance", () => {
  it("ignores undecodable frames and unknown ops", async () => {
    const server = await scripted(async (peer) => {
      peer.send(hello());
      await peer.recv();
      peer.send(ready());
      peer.send("not json");
      peer.send({ op: "brand_new_op", d: { x: 1 } });
      peer.send({ op: "catchup.complete", d: { head_seq: 10 } });
      peer.send({ op: "dispatch", d: { id: SESSION, type: "x.y", seq: 11 } });
      await peer.waitClosed();
    });
    const gw = gateway(server.url);
    const seen = await new Collector(gw, (s) =>
      s.some((f) => f.op === "dispatch"),
    ).finish();
    expect(seen.map((f) => f.op)).toEqual(["ready", "dispatch"]);
  });

  it("drops JSON null members and ignores unknown fields", async () => {
    const server = await scripted(async (peer) => {
      peer.send(hello(600_000, { brand_new: null, extra: 1 }));
      await peer.recv();
      peer.send(ready());
      peer.send({
        op: "dispatch",
        d: {
          id: SESSION,
          type: "x.y",
          seq: 11,
          redelivered: null,
          z: { a: null },
        },
        brand_new: 1,
      });
      await peer.waitClosed();
    });
    const gw = gateway(server.url);
    const seen = await new Collector(gw, (s) =>
      s.some((f) => f.op === "dispatch"),
    ).finish();
    const dispatch = seen.find((f) => f.op === "dispatch");
    expect(dispatch?.data).toEqual({
      id: SESSION,
      type: "x.y",
      seq: 11,
      z: {},
    });
  });

  it("reports error frames with code and ref", async () => {
    const server = await scripted(async (peer) => {
      peer.send(hello());
      await peer.recv();
      peer.send(ready());
      peer.send({
        op: "error",
        d: { op: "run.update", code: "not_found", detail: null, ref: "r-1" },
      });
      await peer.waitClosed();
    });
    const gw = gateway(server.url);
    const seen = await new Collector(gw, (s) =>
      s.some((f) => f.op === "error"),
    ).finish();
    const error = seen.find((f) => f.op === "error")?.model;
    expect(error).toBeInstanceOf(GatewayOpError);
    expect((error as GatewayOpError).code).toBe("not_found");
    expect((error as GatewayOpError).ref).toBe("r-1");
    expect((error as GatewayOpError).op).toBe("run.update");
  });
});

describe("client ops", () => {
  it("sends ack, run.update and command.autocomplete_result", async () => {
    const server = await scripted(async (peer) => {
      peer.send(hello());
      await peer.recv();
      peer.send(ready());
      await peer.waitClosed();
    });
    const gw = gateway(server.url);
    const collector = new Collector(gw);
    await until(() => collector.seen.length >= 1);
    const runId = "0192f4a0-7b1c-7000-8a00-000000000101";
    const requestId = "0192f4a0-7b1c-7000-8a00-000000000102";
    await gw.ack(3);
    await gw.sendRunUpdate(
      runId,
      2,
      [{ op: "replace", path: "/status_line", value: "x" }],
      "r-1",
    );
    await gw.sendAutocompleteResult(requestId, [
      { name: "Robin", value: "robin" },
    ]);
    await until(() => server.frames.length >= 4);
    expect(server.frames.slice(1)).toEqual([
      { op: "ack", d: { seq: 3 } },
      {
        op: "run.update",
        d: {
          run_id: runId,
          base_version: 2,
          ops: [{ op: "replace", path: "/status_line", value: "x" }],
          ref: "r-1",
        },
      },
      {
        op: "command.autocomplete_result",
        d: {
          request_id: requestId,
          choices: [{ name: "Robin", value: "robin" }],
        },
      },
    ]);
    await gw.close();
    await collector.finish();
    // Without a connection ack is dropped quietly; the other ops fail.
    await expect(gw.ack(4)).resolves.toBeUndefined();
    await expect(gw.sendRunUpdate(runId, 2, [])).rejects.toThrow(
      /not connected/,
    );
  });

  it("uses an injected WebSocket constructor and its User-Agent", async () => {
    const agents: (string | undefined)[] = [];
    const server = await scripted(async (peer) => {
      peer.send(hello());
      await peer.recv();
      peer.send(ready());
      await peer.waitClosed();
    });
    let built = 0;
    class Counting extends WebSocket {
      constructor(url: string, options?: { headers?: Record<string, string> }) {
        super(url, options);
        built++;
        agents.push(options?.headers?.["User-Agent"]);
      }
    }
    const gw = gateway(server.url, {
      WebSocket: Counting,
      userAgent: "lotse/1.0.0",
    });
    await new Collector(gw, (s) => s.length >= 1).finish();
    expect(built).toBe(1);
    expect(agents).toEqual(["lotse/1.0.0"]);
  });
});

describe("against the FakeServer", () => {
  async function fakeServer(): Promise<FakeServer> {
    const fake = new FakeServer();
    await fake.start();
    servers.push(fake);
    return fake;
  }

  function fakeGateway(
    fake: FakeServer,
    options: Partial<GatewayOptions> = {},
  ): Gateway {
    const gw = new Gateway(fake.gatewayUrl, fake.credentials, {
      consumer: "default",
      agentId: fake.agentId,
      settings: fake.sdkSettings,
      clock: fake.clock,
      random: scriptedRandom(0.5),
      ...options,
    });
    gateways.push(gw);
    return gw;
  }

  async function advanceUntil(
    fake: FakeServer,
    condition: () => boolean,
    stepMs = 100,
  ): Promise<void> {
    await patiently(condition, "the condition of the test", {
      step: () => fake.passTime(stepMs),
      pauseMs: 2,
    });
    expect(condition()).toBe(true);
  }

  it("identifies with DPoP, keeps heartbeats acknowledged and resumes after 1001", async () => {
    const fake = await fakeServer();
    const gw = fakeGateway(fake, { heartbeat: () => ({ state: "online" }) });
    const collector = new Collector(gw);
    await until(() => collector.seen.some((f) => f.op === "ready"));
    expect(gw.sessionId).toBeDefined();
    // Three heartbeat intervals pass with every heartbeat acknowledged: no reconnect.
    await advanceUntil(
      fake,
      () => fake.frames().filter((f) => f.op === "heartbeat").length >= 3,
      5_000,
    );
    expect(fake.frames().filter((f) => f.op === "identify")).toHaveLength(1);
    expect(fake.heartbeats.length).toBeGreaterThanOrEqual(1);
    // A new nonce window and a 1001: the next proof takes the new nonce of hello.
    fake.rotateNonce();
    await fake.closeConnections(1001);
    await advanceUntil(fake, () =>
      collector.seen.some((f) => f.op === "resumed"),
    );
    const ops = fake
      .frames()
      .map((f) => f.op)
      .filter((op) => op !== "heartbeat");
    expect(ops).toEqual(["identify", "resume"]);
    await gw.close();
    await collector.finish();
    expect(gw.closeReason?.code).toBe(1000);
  });

  it("identifies again after a restart (4013, then 4006 on resume)", async () => {
    const fake = await fakeServer();
    const gw = fakeGateway(fake);
    const collector = new Collector(gw);
    await until(() => collector.seen.some((f) => f.op === "ready"));
    await fake.restart();
    await advanceUntil(
      fake,
      () => collector.seen.filter((f) => f.op === "ready").length >= 2,
    );
    const ops = fake
      .frames()
      .map((f) => f.op)
      .filter((op) => op !== "heartbeat");
    expect(ops).toEqual(["identify", "resume", "identify"]);
    await gw.close();
    await collector.finish();
  });

  it("stops on 4015 when another process takes the consumer", async () => {
    const fake = await fakeServer();
    const gw = fakeGateway(fake);
    const collector = new Collector(gw);
    await until(() => collector.seen.some((f) => f.op === "ready"));
    await fake.closeConnections(4015);
    await collector.finish();
    expect(gw.closeReason?.code).toBe(4015);
    expect(gw.closeReason?.permanent).toBe(true);
    expect(fake.frames().filter((f) => f.op === "identify")).toHaveLength(1);
  });

  it("delivers to the same handler over the gateway and over polling", async () => {
    /** The handler of an adapter: settles every envelope with a seq and every cursor. */
    async function handle(
      source: EventSource,
      seen: Delivery[],
      want: number,
    ): Promise<void> {
      for await (const delivery of source) {
        seen.push(delivery);
        if (delivery instanceof Envelope && delivery.seq !== undefined)
          await source.settle(delivery.seq);
        else if (delivery instanceof CursorAdvance)
          await source.advance(delivery.seq);
        if (seen.filter((d) => d instanceof Envelope).length >= want)
          await source.close();
      }
    }

    const fake = await fakeServer();
    const gatewaySeen: Delivery[] = [];
    const gw = fakeGateway(fake);
    const viaGateway = handle(new GatewaySource(gw), gatewaySeen, 1);
    await until(() => gatewaySeen.some((d) => d instanceof SourceReady));
    const first = await fake.trigger("Hello @Lotse");
    await advanceUntil(fake, () =>
      gatewaySeen.some((d) => d instanceof Envelope),
    );
    await viaGateway;
    const viaGatewayEnvelope = gatewaySeen.find((d) => d instanceof Envelope);
    expect(viaGatewayEnvelope?.type).toBe("agent.invocation");
    expect(viaGatewayEnvelope?.token).toBeDefined();
    const acked = fake.consumerState("default").ackedSeq;
    expect(acked).toBe(viaGatewayEnvelope?.seq);

    const api = createApiClient({
      baseUrl: fake.baseUrl,
      token: fake.credentials.apiToken,
      dpopKey: fake.credentials.dpop,
      clock: fake.clock,
      settings: fake.sdkSettings,
    });
    const pollingSeen: Delivery[] = [];
    const polling = new PollingSource(api, "default", {
      settings: fake.sdkSettings,
      clock: fake.clock,
    });
    const viaPolling = handle(polling, pollingSeen, 1);
    const second = await fake.trigger("Hello again @Lotse");
    await advanceUntil(
      fake,
      () => pollingSeen.some((d) => d instanceof Envelope),
      500,
    );
    await fake.passTime(fake.sdkSettings.pollIntervalMs);
    await viaPolling;
    const viaPollingEnvelope = pollingSeen.find((d) => d instanceof Envelope);
    expect(viaPollingEnvelope?.type).toBe("agent.invocation");
    expect(first.runId).not.toBe(second.runId);
    await until(
      () => fake.consumerState("default").ackedSeq === viaPollingEnvelope?.seq,
    );
  });

  it("surfaces the error frame of a failing run.update", async () => {
    const fake = await fakeServer();
    const gw = fakeGateway(fake);
    const collector = new Collector(gw, (s) => s.some((f) => f.op === "error"));
    await until(() => collector.seen.some((f) => f.op === "ready"));
    await gw.sendRunUpdate(
      "0192f4a0-7b1c-7000-8a00-000000000999",
      1,
      [{ op: "replace", path: "/status_line", value: "x" }],
      "r-9",
    );
    const seen = await collector.finish();
    const error = seen.find((f) => f.op === "error")?.model as GatewayOpError;
    expect(error).toBeInstanceOf(GatewayOpError);
    expect(error.op).toBe("run.update");
    expect(error.ref).toBe("r-9");
  });
});
