// SPDX-License-Identifier: Apache-2.0
//
// The FakeServer plays the server rules the SDK meets (docs/spec/09-agents.md 16.1). Written
// with plain `fetch` and the `ws` client (not with the SDK's client modules), so the server is
// proven on its own; the cases port `sdk/python/tests/test_fakeserver.py`. Tokens are compared
// by `jti` or a fingerprint, never printed.

import {
  createHash,
  createHmac,
  createPublicKey,
  verify as verifySignature,
} from "node:crypto";
import { readFileSync } from "node:fs";
import { createServer, type IncomingMessage } from "node:http";
import type { AddressInfo } from "node:net";

import { createLocalJWKSet, decodeProtectedHeader, jwtVerify } from "jose";
import { afterEach, beforeEach, describe, expect, it } from "vitest";
import { WebSocket } from "ws";

import { Secret } from "../src/auth/secret.js";
import type { components } from "../src/generated/openapi.js";
import { uuidv7 } from "../src/ids.js";
import { FakeProblem } from "../src/testing/fake/base.js";
import { FIXTURES } from "../src/testing/fake/fixtures.js";
import { applyOps, parseOps } from "../src/testing/fake/patch.js";
import { FakeServer } from "../src/testing/fakeserver.js";
import { Fault, FaultKind } from "../src/testing/faults.js";
import type { ScenarioServer } from "../src/testing/scenario-server.js";

// The prefix is assembled at runtime so no literal matches the secret-scanning format.
const WHSEC = ["whsec", ""].join("_");

type Json = Record<string, any>; // eslint-disable-line @typescript-eslint/no-explicit-any
type RunDoc = components["schemas"]["Run"];

const ZERO_UUID = "00000000-0000-0000-0000-000000000000";
const SEVEN_UUID = "00000000-0000-0000-0000-000000000007";
const enc = new TextEncoder();

function b64url(data: Uint8Array | string): string {
  return Buffer.from(
    typeof data === "string" ? enc.encode(data) : data,
  ).toString("base64url");
}

function fingerprint(value: string): string {
  return createHash("sha256").update(value).digest("hex").slice(0, 16);
}

function claims(token: string): Json {
  return JSON.parse(
    Buffer.from(token.split(".")[1] ?? "", "base64url").toString("utf8"),
  ) as Json;
}

function jti(token: unknown): string {
  return String(claims(String(token)).jti);
}

/** A DPoP proof (RFC 9449) signed with the FakeServer's key for Lotse. */
async function proof(
  fake: FakeServer,
  htm: string,
  htu: string,
  nonce: string | undefined,
): Promise<string> {
  const key = fake.credentials.dpop;
  if (key === undefined) throw new Error("no DPoP key");
  const header = { typ: "dpop+jwt", alg: key.alg, jwk: key.publicJwk };
  const body: Json = {
    jti: uuidv7(),
    htm,
    htu,
    iat: Math.floor(fake.clock.now() / 1000),
    ath: b64url(
      createHash("sha256").update(fake.credentials.apiToken.reveal()).digest(),
    ),
  };
  if (nonce !== undefined) body.nonce = nonce;
  const input = `${b64url(JSON.stringify(header))}.${b64url(JSON.stringify(body))}`;
  return `${input}.${b64url(await key.sign(enc.encode(input)))}`;
}

interface Answer {
  status: number;
  headers: Headers;
  body: Json;
}

/** A minimal REST client: Lotse's credentials, DPoP with the newest nonce, one nonce retry. */
class Api {
  private nonce: string | undefined;

  constructor(private readonly fake: FakeServer) {}

  async call(
    method: string,
    path: string,
    json?: unknown,
    headers: Record<string, string> = {},
  ): Promise<Answer> {
    const url = this.fake.baseUrl + path;
    for (let attempt = 0; ; attempt++) {
      const h: Record<string, string> = { ...headers };
      const token = this.fake.credentials.apiToken.reveal();
      if (this.fake.credentials.dpop !== undefined) {
        h.authorization = `DPoP ${token}`;
        h.dpop = await proof(
          this.fake,
          method,
          url.split("?")[0] ?? url,
          this.nonce,
        );
      } else {
        h.authorization = `Bearer ${token}`;
      }
      if (json !== undefined) h["content-type"] = "application/json";
      const init: RequestInit = { method, headers: h };
      if (json !== undefined) init.body = JSON.stringify(json);
      const r = await fetch(url, init);
      this.nonce = r.headers.get("dpop-nonce") ?? this.nonce;
      const text = await r.text();
      const body = (text ? JSON.parse(text) : {}) as Json;
      if (r.status === 401 && body.code === "use_dpop_nonce" && attempt === 0)
        continue;
      return { status: r.status, headers: r.headers, body };
    }
  }

  get(path: string, headers?: Record<string, string>): Promise<Answer> {
    return this.call("GET", path, undefined, headers);
  }

  post(
    path: string,
    json: unknown,
    headers?: Record<string, string>,
  ): Promise<Answer> {
    return this.call("POST", path, json, headers);
  }

  patch(
    path: string,
    json: unknown,
    headers?: Record<string, string>,
  ): Promise<Answer> {
    return this.call("PATCH", path, json, headers);
  }

  put(
    path: string,
    json: unknown,
    headers?: Record<string, string>,
  ): Promise<Answer> {
    return this.call("PUT", path, json, headers);
  }

  delete(path: string): Promise<Answer> {
    return this.call("DELETE", path);
  }
}

function startBody(token: unknown, extra: Json = {}): Json {
  return {
    invocation_token: token,
    billing_mode_used: "organization",
    tool_class_used: "scoped",
    ...extra,
  };
}

async function poll(api: Api, consumer = "rest"): Promise<Json> {
  const r = await api.get(`/agents/me/consumers/${consumer}/events`);
  expect(r.status).toBe(200);
  return r.body;
}

/** Triggers a run and fetches its agent.invocation over REST: run ID and token. */
async function queuedWithToken(
  fake: FakeServer,
  api: Api,
): Promise<[string, string]> {
  await poll(api); // the first poll creates the cursor at the head
  const triggered = await fake.trigger();
  const page = await poll(api);
  const events = (page.events as Json[]).filter(
    (e) => e.type === "agent.invocation",
  );
  expect(events).toHaveLength(1);
  await api.post("/agents/me/consumers/rest/ack", {
    seq: page.next_cursor as number,
  });
  return [triggered.runId, String(events[0]?.invocation_token)];
}

interface Frame {
  op: string;
  d: Json;
}

class Closed extends Error {
  constructor(readonly code: number) {
    super(`closed ${code}`);
  }
}

/** A minimal gateway client for the tests. */
class Gw {
  readonly pending: Frame[] = [];
  sessionId = "";
  private readonly queue: Frame[] = [];
  private waiter: (() => void) | undefined;
  private closedCode: number | undefined;

  constructor(readonly ws: WebSocket) {
    ws.on("message", (data: Buffer) => {
      this.queue.push(JSON.parse(data.toString("utf8")) as Frame);
      this.waiter?.();
    });
    ws.on("close", (code: number) => {
      this.closedCode = code;
      this.waiter?.();
    });
    ws.on("error", () => undefined);
  }

  async recv(): Promise<Frame> {
    const deadline = Date.now() + 5_000;
    for (;;) {
      const frame = this.queue.shift();
      if (frame !== undefined) return frame;
      if (this.closedCode !== undefined) throw new Closed(this.closedCode);
      if (Date.now() > deadline) throw new Error("no frame within 5 s");
      await new Promise<void>((resolve) => {
        this.waiter = resolve;
        setTimeout(resolve, 50);
      });
    }
  }

  send(op: string, d: Json): void {
    this.ws.send(JSON.stringify({ op, d }));
  }

  async until(op: string): Promise<Frame> {
    const index = this.pending.findIndex((f) => f.op === op);
    if (index >= 0) return this.pending.splice(index, 1)[0] as Frame;
    for (;;) {
      const frame = await this.recv();
      if (frame.op === op) return frame;
    }
  }

  async closeCode(): Promise<number> {
    try {
      await this.until("never");
    } catch (error) {
      if (error instanceof Closed) return error.code;
      throw error;
    }
    return -1;
  }

  close(): void {
    this.ws.close(1000);
  }
}

async function open(fake: FakeServer): Promise<[Gw, string]> {
  const ws = new WebSocket(fake.gatewayUrl);
  const gw = new Gw(ws);
  const hello = await gw.recv();
  expect(hello.op).toBe("hello");
  expect(Object.keys(hello.d).sort()).toEqual(
    [
      "dpop_nonce",
      "heartbeat_interval_ms",
      "protocol_versions",
      "resume_window_ms",
      "server_time",
      "sync_epoch",
    ].sort(),
  );
  return [gw, String(hello.d.dpop_nonce)];
}

async function gatewayCredentials(
  fake: FakeServer,
  nonce: string,
): Promise<Json> {
  return {
    api_token: fake.credentials.apiToken.reveal(),
    dpop: await proof(fake, "GET", fake.gatewayHtu, nonce),
  };
}

async function connect(fake: FakeServer, consumer = "gw"): Promise<Gw> {
  const [gw, nonce] = await open(fake);
  gw.send("identify", {
    protocol_version: 1,
    consumer,
    ...(await gatewayCredentials(fake, nonce)),
  });
  const ready = await gw.recv();
  expect(ready.op).toBe("ready");
  expect((ready.d.principal as Json).id).toBe(fake.agentId);
  gw.sessionId = String(ready.d.gateway_session_id);
  for (;;) {
    const frame = await gw.recv();
    if (frame.op === "catchup.complete") break;
    gw.pending.push(frame);
  }
  return gw;
}

async function resume(fake: FakeServer, sessionId: string): Promise<Gw> {
  const [gw, nonce] = await open(fake);
  gw.send("resume", {
    gateway_session_id: sessionId,
    cursor: 0,
    ...(await gatewayCredentials(fake, nonce)),
  });
  return gw;
}

async function eventually(check: () => boolean): Promise<void> {
  const deadline = Date.now() + 5_000;
  while (!check()) {
    if (Date.now() > deadline) throw new Error("condition not met within 5 s");
    await new Promise((r) => setTimeout(r, 10));
  }
}

let fake: FakeServer;
let api: Api;

beforeEach(async () => {
  fake = new FakeServer();
  await fake.start();
  api = new Api(fake);
});

afterEach(async () => {
  await fake.stop();
});

describe("discovery, JWKS and tokens", () => {
  it("serves discovery, the JWKS and the webhook keys", async () => {
    const disco = (await (
      await fetch(`${fake.baseUrl}/instance/discovery`)
    ).json()) as Json;
    const jwks = await fetch(`${fake.baseUrl}/instance/jwks`);
    const keys = await fetch(`${fake.baseUrl}/instance/webhook-keys`);
    expect(disco.instance_id).toBe(fake.instanceId);
    expect((disco.gateway as Json).url).toBe("/api/v1/gateway");
    expect(disco.webhook_keys_uri).toBe("/api/v1/instance/webhook-keys");
    expect(jwks.headers.get("cache-control")).toBe("public, max-age=300");
    const [key, ...more] = ((await jwks.json()) as Json).keys as Json[];
    expect(more).toEqual([]);
    expect([key?.kty, key?.crv, key?.alg, key?.use]).toEqual([
      "OKP",
      "Ed25519",
      "EdDSA",
      "sig",
    ]);
    const webhook = (await keys.json()) as Json;
    expect(keys.headers.get("cache-control")).toBe("public, max-age=300");
    expect(
      (webhook.keys as Json[]).map((k): unknown => [
        k.state,
        String(k.public_key).slice(0, 5),
      ]),
    ).toEqual([["active", "whpk_"]]);
  });

  it("issues invocation tokens that verify with the JWKS", async () => {
    const [runId, token] = await queuedWithToken(fake, api);
    const jwks = (await api.get("/instance/jwks")).body;
    const header = decodeProtectedHeader(token);
    expect([header.alg, header.typ]).toEqual(["EdDSA", "agent-invocation+jwt"]);
    const { payload } = await jwtVerify(
      token,
      createLocalJWKSet(jwks as never),
      {
        algorithms: ["EdDSA"],
        audience: fake.agentId,
        issuer: fake.instanceId,
        typ: "agent-invocation+jwt",
        currentDate: new Date(fake.clock.now()),
      },
    );
    expect(payload.run_id).toBe(runId);
    expect([payload.trigger_kind, payload.level]).toEqual([
      "message",
      "trigger",
    ]);
    expect([payload.billing_mode, payload.tool_class]).toEqual([
      "organization",
      "scoped",
    ]);
    expect(((payload.exp ?? 0) - (payload.iat ?? 0)) * 1000).toBe(
      fake.timings["agent.invocation_token_ttl"],
    );
    expect(payload.iat).toBe(Math.floor(fake.clock.now() / 1000));
    expect(String(payload.jti)[14]).toBe("7");
    expect(Number.isInteger(payload.permission_version)).toBe(true);
  });
});

describe("DPoP", () => {
  it("enforces nonces, replay, binding and htu", async () => {
    const token = fake.credentials.apiToken.reveal();
    const url = `${fake.baseUrl}/runs/${ZERO_UUID}`;
    const get = (p: string, scheme = "DPoP"): Promise<Response> =>
      fetch(url, { headers: { authorization: `${scheme} ${token}`, dpop: p } });
    const first = await get(await proof(fake, "GET", url, undefined));
    expect(first.status).toBe(401);
    expect(((await first.json()) as Json).code).toBe("use_dpop_nonce");
    const nonce = first.headers.get("dpop-nonce") ?? "";
    const p = await proof(fake, "GET", url, nonce);
    const ok = await get(p);
    expect(ok.status).toBe(404); // authenticated; the run does not exist
    expect(ok.headers.get("dpop-nonce")).toBeTruthy();
    const replay = await get(p);
    expect([replay.status, ((await replay.json()) as Json).code]).toEqual([
      401,
      "invalid_dpop_proof",
    ]);
    const bearer = await fetch(url, {
      headers: { authorization: `Bearer ${token}` },
    });
    expect(((await bearer.json()) as Json).code).toBe("invalid_dpop_proof");
    // The nonce rotates: after two of its windows the old one is refused.
    await fake.passTime(2 * fake.timings["auth.dpop_nonce_ttl"]);
    const stale = await get(await proof(fake, "GET", url, nonce));
    expect(((await stale.json()) as Json).code).toBe("use_dpop_nonce");
    const fresh = stale.headers.get("dpop-nonce") ?? "";
    expect(fresh).not.toBe(nonce);
    const wrongHtu = await get(
      await proof(fake, "GET", `${fake.baseUrl}/other`, fresh),
    );
    expect(((await wrongHtu.json()) as Json).code).toBe("invalid_dpop_proof");
    expect((await api.get(`/runs/${ZERO_UUID}`)).status).toBe(404);
  });

  it("follows a rotated nonce while the previous one stays valid one window", async () => {
    const url = `${fake.baseUrl}/agents/me/channels`;
    const token = fake.credentials.apiToken.reveal();
    const first = await fetch(url, {
      headers: {
        authorization: `DPoP ${token}`,
        dpop: await proof(fake, "GET", url, undefined),
      },
    });
    const nonce = first.headers.get("dpop-nonce") ?? "";
    fake.rotateNonce();
    const previous = await fetch(url, {
      headers: {
        authorization: `DPoP ${token}`,
        dpop: await proof(fake, "GET", url, nonce),
      },
    });
    expect(previous.status).toBe(200);
    expect(previous.headers.get("dpop-nonce")).not.toBe(nonce);
  });

  it("serves bearer tokens without DPoP", async () => {
    const bearer = new FakeServer({ dpop: false });
    await bearer.start();
    try {
      expect(bearer.credentials.dpop).toBeUndefined();
      const r = await new Api(bearer).get("/agents/me/channels");
      expect(r.status).toBe(200);
      expect((r.body.items as Json[])[0]?.channel_id).toBeTruthy();
      expect(r.headers.get("dpop-nonce")).toBeNull();
      expect((await fetch(`${bearer.baseUrl}/agents/me/channels`)).status).toBe(
        401,
      );
    } finally {
      await bearer.stop();
    }
  });
});

describe("delivery", () => {
  it("delivers a trigger over the gateway and polling with a fresh token each", async () => {
    const gw = await connect(fake);
    await poll(api);
    const triggered = await fake.trigger();
    const frame = await gw.until("dispatch");
    const page = await poll(api);
    const envelope = frame.d;
    expect(envelope.type).toBe("agent.invocation");
    expect((envelope.payload as Json).run_id).toBe(triggered.runId);
    expect((envelope.payload as Json).message_id).toBe(triggered.messageId);
    expect("redelivered" in envelope).toBe(false);
    const [polled] = page.events as Json[];
    expect(polled?.id).toBe(envelope.id);
    expect(jti(polled?.invocation_token)).not.toBe(
      jti(envelope.invocation_token),
    );
    gw.send("ack", { seq: envelope.seq as number });
    await eventually(() => {
      const state = fake.consumerState("gw");
      return state.ackedSeq === envelope.seq && state.headSeq === envelope.seq;
    });
    gw.close();
  });

  it("redelivers after agent.ack_timeout with redelivered: true and new tokens", async () => {
    const gw = await connect(fake);
    await poll(api);
    const run = await fake.trigger();
    const first = (await gw.until("dispatch")).d;
    const polled = ((await poll(api)).events as Json[])[0] ?? {};
    expect((await poll(api)).events).toEqual([]); // nothing repeats within the ack timeout
    await fake.passTime(fake.timings["agent.ack_timeout"]);
    const again = (await gw.until("dispatch")).d;
    const repolled = ((await poll(api)).events as Json[])[0] ?? {};
    expect(again.id).toBe(first.id);
    expect(again.redelivered).toBe(true);
    expect(repolled.redelivered).toBe(true);
    const jtis = new Set(
      [first, polled, again, repolled].map((e) => jti(e.invocation_token)),
    );
    expect(jtis.size).toBe(4);
    // The newest delivery's token is the run's current one; an older one is superseded.
    const old = await api.post(
      `/runs/${run.runId}/start`,
      startBody(first.invocation_token),
    );
    expect([old.status, old.body.reason]).toEqual([403, "superseded"]);
    const fresh = await api.post(
      `/runs/${run.runId}/start`,
      startBody(repolled.invocation_token),
    );
    expect(fresh.status).toBe(200);
    expect((fresh.body as RunDoc).state).toBe("running");
    gw.close();
  });

  it("redeliver() repeats every unacknowledged event at once", async () => {
    const gw = await connect(fake);
    await fake.trigger();
    const first = (await gw.until("dispatch")).d;
    await fake.redeliver();
    const again = (await gw.until("dispatch")).d;
    expect([again.id, again.redelivered]).toEqual([first.id, true]);
    expect(jti(again.invocation_token)).not.toBe(jti(first.invocation_token));
    gw.close();
  });

  it("lets the newest holder win and redelivers after restart() with new tokens (AG-28)", async () => {
    const old = await connect(fake, "alpha");
    const fresh = await connect(fake, "alpha");
    expect(await old.closeCode()).toBe(4015);
    const run = await fake.trigger();
    const frame = (await fresh.until("dispatch")).d;
    fresh.send("ack", { seq: frame.seq as number });
    await eventually(() => fake.consumerState("alpha").ackedSeq === frame.seq);
    await fake.restart("alpha", Number(frame.seq) - 1);
    expect(await fresh.closeCode()).toBe(4013);
    const again = await connect(fake, "alpha");
    const redelivered = (await again.until("dispatch")).d;
    expect(redelivered.id).toBe(frame.id);
    expect(claims(String(redelivered.invocation_token)).run_id).toBe(run.runId);
    expect(jti(redelivered.invocation_token)).not.toBe(
      jti(frame.invocation_token),
    );
    expect((await fake.runState(run.runId)).state).toBe("queued");
    again.close();
  });

  it("refuses an identify while restarting and accepts one after", async () => {
    const gw = await connect(fake, "c5");
    await fake.restart("c5", 0);
    expect(await gw.closeCode()).toBe(4013);
    const again = await connect(fake, "c5");
    again.close();
  });

  it("completes a backlog beyond the inflight budget after acks", async () => {
    const small = new FakeServer({
      timings: { "agent.max_inflight_events": 1 },
    });
    await small.start();
    try {
      const first = await connect(small, "c4");
      first.close();
      await small.trigger();
      await small.trigger();
      const [gw, nonce] = await open(small);
      gw.send("identify", {
        protocol_version: 1,
        consumer: "c4",
        ...(await gatewayCredentials(small, nonce)),
      });
      expect((await gw.recv()).op).toBe("ready");
      const one = await gw.recv();
      expect([one.op, one.d.seq]).toEqual(["dispatch", 1]);
      gw.send("ack", { seq: 1 });
      const two = await gw.recv();
      expect([two.op, two.d.seq]).toEqual(["dispatch", 2]);
      expect(await gw.recv()).toEqual({
        op: "catchup.complete",
        d: { head_seq: 2 },
      });
      gw.close();
    } finally {
      await small.stop();
    }
  });

  it("delivers a token-bound event again with a given token (AZ-P6)", async () => {
    await poll(api);
    await fake.trigger();
    const [event] = (await poll(api)).events as Json[];
    const replayed = await fake.deliverWithToken(
      String(event?.id),
      new Secret(String(event?.invocation_token)),
    );
    const [again] = (await poll(api)).events as Json[];
    expect(again?.id).toBe(replayed);
    expect(fingerprint(String(again?.invocation_token))).toBe(
      fingerprint(String(event?.invocation_token)),
    );
  });
});

describe("run rules (09 4.5 and 4.5.1)", () => {
  it("answers start and renew with the verdicts of the server", async () => {
    const [runId, token] = await queuedWithToken(fake, api);
    for (const variant of [
      "bad_signature",
      "alg_hs256",
      "typ_access",
      "aud_other_agent",
    ] as const) {
      const forged = (await fake.forgeToken(runId, variant)).reveal();
      for (const path of ["start", "invocation-token"]) {
        const body =
          path === "start" ? startBody(forged) : { invocation_token: forged };
        const r = await api.post(`/runs/${runId}/${path}`, body);
        expect([r.status, r.body.code, r.body.reason]).toEqual([
          403,
          "invocation_token_invalid",
          "invalid",
        ]);
      }
    }
    expect(
      (await api.post(`/runs/${runId}/start`, startBody(""))).body.reason,
    ).toBe("invalid");
    expect((await fake.runState(runId)).state).toBe("queued");
    // Expired after agent.invocation_token_ttl plus auth.clock_leeway (strictly after).
    await fake.passTime(
      fake.timings["agent.invocation_token_ttl"] +
        fake.timings["auth.clock_leeway"] +
        1_000,
    );
    expect(
      (await api.post(`/runs/${runId}/start`, startBody(token))).body.reason,
    ).toBe("expired");
    const grant = await api.post(`/runs/${runId}/invocation-token`, {
      invocation_token: token,
    });
    expect(grant.status).toBe(200);
    expect(Object.keys(grant.body).sort()).toEqual([
      "expires_at",
      "invocation_token",
    ]);
    const renewed = grant.body.invocation_token as string;
    // The renewed-from token may renew again (a lost answer); the older grant is replaced.
    const retry = await api.post(`/runs/${runId}/invocation-token`, {
      invocation_token: token,
    });
    const renewed2 = retry.body.invocation_token as string;
    expect(
      (await api.post(`/runs/${runId}/start`, startBody(renewed))).body.reason,
    ).toBe("superseded");
    const started = await api.post(`/runs/${runId}/start`, startBody(renewed2));
    expect([started.status, started.body.state]).toEqual([200, "running"]);
    const replay = await api.post(`/runs/${runId}/start`, startBody(renewed2));
    expect([replay.status, replay.headers.get("idempotent-replay")]).toEqual([
      200,
      "true",
    ]);
    const again = await api.post(`/runs/${runId}/start`, startBody(token));
    expect([again.status, again.body.code]).toEqual([409, "already_started"]);
    const renewRunning = await api.post(`/runs/${runId}/invocation-token`, {
      invocation_token: token,
    });
    expect(renewRunning.body.code).toBe("already_started");
    const view = await fake.runState(runId);
    expect([view.state, view.starts]).toEqual(["running", 1]);
  });

  it.each([
    [{ billing_mode_used: "personal" }, "billing_mode_mismatch"],
    [{ billing_evidence: { key_type: "unknown" } }, "billing_mode_mismatch"],
    [{ tool_class_used: "host" }, "tool_class_mismatch"],
  ])("fails the run on a mismatch %j", async (extra, code) => {
    const [runId, token] = await queuedWithToken(fake, api);
    const r = await api.post(`/runs/${runId}/start`, startBody(token, extra));
    expect([r.status, r.body.code, r.body.reason]).toEqual([409, code, code]);
    const view = await fake.runState(runId);
    expect([view.state, view.stateReason, view.starts]).toEqual([
      "failed",
      code,
      0,
    ]);
  });

  it("lets a lower tool class pass", async () => {
    const [runId, token] = await queuedWithToken(fake, api);
    const r = await api.post(
      `/runs/${runId}/start`,
      startBody(token, { tool_class_used: "chat" }),
    );
    expect(r.status).toBe(200);
  });

  it("waits for input, resumes on a follow-up and finishes", async () => {
    const [runId, token] = await queuedWithToken(fake, api);
    await api.post(`/runs/${runId}/start`, startBody(token));
    const waiting = await api.post(`/runs/${runId}/await-input`, {
      status_line: "Which one?",
    });
    expect(waiting.body.state).toBe("waiting_input");
    const consumed = await api.post(`/runs/${runId}/resume`, startBody(token));
    expect(consumed.body.reason).toBe("superseded");
    const messageId = await fake.deliverFollowup(runId, "The second one");
    const followups = ((await poll(api)).events as Json[]).filter(
      (e) => e.type === "agent.followup",
    );
    expect(followups).toHaveLength(1);
    const followup = followups[0] ?? {};
    expect((followup.payload as Json).message_id).toBe(messageId);
    expect(claims(String(followup.invocation_token)).trigger_kind).toBe(
      "followup",
    );
    const resumed = await api.post(
      `/runs/${runId}/resume`,
      startBody(followup.invocation_token),
    );
    expect([resumed.status, resumed.body.state]).toEqual([200, "running"]);
    const done = await api.post(`/runs/${runId}/finish`, {
      state: "completed",
    });
    expect(done.body.state).toBe("completed");
    expect(typeof done.body.duration_ms).toBe("number");
    expect(
      (await api.post(`/runs/${runId}/finish`, { state: "completed" })).status,
    ).toBe(200);
    const costs = await api.post(`/runs/${runId}/finish`, {
      state: "completed",
      costs: [],
    });
    const error = (costs.body.errors as Json[])[0];
    expect([costs.status, error?.pointer, error?.code]).toEqual([
      400,
      "/costs",
      "not_enabled",
    ]);
    expect((await fake.runState(runId)).starts).toBe(2);
  });

  it("lets a refused run finish failed and maps an unknown reason to adapter_error", async () => {
    const [runId] = await queuedWithToken(fake, api);
    const r = await api.post(`/runs/${runId}/finish`, {
      state: "failed",
      reason: "invalid_invocation_token",
    });
    expect(r.status).toBe(200);
    const view = await fake.runState(runId);
    expect([view.state, view.stateReason, view.starts]).toEqual([
      "failed",
      "invalid_invocation_token",
      0,
    ]);
    const [other] = await queuedWithToken(fake, api);
    const mapped = await api.post(`/runs/${other}/finish`, {
      state: "failed",
      reason: "disk_full",
    });
    expect([mapped.body.state, mapped.body.state_reason]).toEqual([
      "failed",
      "adapter_error",
    ]);
    const steps = Object.values(mapped.body.steps as Json) as Json[];
    expect(
      steps.map((s): unknown => [s.name, (s.detail as Json).original_reason]),
    ).toEqual([["adapter_reason", "disk_full"]]);
  });

  it("cancels a running run after the adapter and a queued run at once", async () => {
    const [runId, token] = await queuedWithToken(fake, api);
    await api.post(`/runs/${runId}/start`, startBody(token));
    await fake.cancel(runId);
    const events = ((await poll(api)).events as Json[]).filter(
      (e) => e.type === "run.cancel_requested",
    );
    expect(events).toHaveLength(1);
    expect("invocation_token" in (events[0] ?? {})).toBe(false);
    expect((await fake.runState(runId)).state).toBe("running");
    await api.post(`/runs/${runId}/finish`, {
      state: "cancelled",
      reason: "user_cancelled",
    });
    const queued = await fake.trigger();
    await fake.cancel(queued.runId);
    expect((await fake.runState(queued.runId)).state).toBe("cancelled");
  });

  it("ends a run with cancel_timeout after agent.cancel_grace", async () => {
    const [runId, token] = await queuedWithToken(fake, api);
    await api.post(`/runs/${runId}/start`, startBody(token));
    await fake.cancel(runId);
    await fake.passTime(fake.timings["agent.cancel_grace"]);
    const view = await fake.runState(runId);
    expect([view.state, view.stateReason]).toEqual([
      "cancelled",
      "cancel_timeout",
    ]);
  });
});

describe("patches (09 5.3)", () => {
  it("checks the path table and base_version with current on a conflict", async () => {
    const [runId, token] = await queuedWithToken(fake, api);
    const base = Number(
      (await api.post(`/runs/${runId}/start`, startBody(token))).body.version,
    );
    const forbidden = await api.patch(`/runs/${runId}`, {
      base_version: base,
      ops: [{ op: "replace", path: "/state", value: "x" }],
    });
    expect([forbidden.status, forbidden.body.code]).toEqual([
      400,
      "validation_failed",
    ]);
    const error = (forbidden.body.errors as Json[])[0];
    expect([error?.code, error?.pointer]).toEqual([
      "patch_path_forbidden",
      "/ops/0",
    ]);
    const ok = await api.patch(`/runs/${runId}`, {
      base_version: base,
      ops: [{ op: "replace", path: "/status_line", value: "Reading" }],
    });
    expect(ok.body).toEqual({ version: base + 1 });
    const stale = await api.patch(`/runs/${runId}`, {
      base_version: base,
      ops: [{ op: "replace", path: "/model", value: "m" }],
    });
    expect([stale.status, stale.body.code]).toEqual([
      409,
      "base_version_conflict",
    ]);
    const current = stale.body.current as RunDoc;
    expect([current.version, current.status_line]).toEqual([
      base + 1,
      "Reading",
    ]);
    const step = "01900000-0000-7000-8000-00000000abcd";
    const steps = await api.patch(`/runs/${runId}`, {
      base_version: base + 1,
      ops: [
        { op: "replace", path: "/model", value: "example-model" },
        {
          op: "add",
          path: `/steps/${step}`,
          value: { kind: "tool_call", name: "read" },
        },
        { op: "replace", path: `/steps/${step}/status`, value: "completed" },
      ],
    });
    expect(steps.status).toBe(200);
    const run = (await api.get(`/runs/${runId}`)).body as RunDoc;
    expect(run.model).toBe("example-model");
    expect([run.steps[step]?.ordinal, run.steps[step]?.status]).toEqual([
      1,
      "completed",
    ]);
    const ordinal = await api.patch(`/runs/${runId}`, {
      base_version: run.version,
      ops: [
        {
          op: "add",
          path: "/steps/01900000-0000-7000-8000-00000000abce",
          value: { kind: "status", name: "n", ordinal: 3 },
        },
      ],
    });
    const e1 = (ordinal.body.errors as Json[])[0];
    expect([e1?.pointer, e1?.code]).toEqual([
      "/ops/0/value/ordinal",
      "patch_path_forbidden",
    ]);
    const badId = await api.patch(`/runs/${runId}`, {
      base_version: run.version,
      ops: [{ op: "add", path: `/steps/${SEVEN_UUID}`, value: {} }],
    });
    const e2 = (badId.body.errors as Json[])[0];
    expect([e2?.pointer, e2?.code]).toEqual(["/ops/0/path", "invalid_format"]);
    const many = await api.patch(`/runs/${runId}`, {
      base_version: run.version,
      ops: [
        { op: "replace", path: "/metrics/tokens_in", value: -1 },
        {
          op: "replace",
          path: "/cost",
          value: { amount_micros: 1, currency: "usd", is_estimate: true },
        },
        {
          op: "add",
          path: "/plan",
          value: { items: [{ id: "a", text: "One", status: "later" }] },
        },
      ],
    });
    expect(
      (many.body.errors as Json[]).map((e): unknown => [e.pointer, e.code]),
    ).toEqual([
      ["/ops/0/value", "too_small"],
      ["/ops/1/value/currency", "pattern_mismatch"],
      ["/ops/2/value/items/0/status", "invalid_enum"],
    ]);
    const missing = await api.patch(`/runs/${runId}`, {
      base_version: run.version,
      ops: [
        {
          op: "replace",
          path: "/steps/01900000-0000-7000-8000-00000000abcf/title",
          value: "t",
        },
      ],
    });
    expect((missing.body.errors as Json[])[0]?.code).toBe(
      "patch_target_missing",
    );
    await api.post(`/runs/${runId}/finish`, { state: "completed" });
    const ended = await api.patch(`/runs/${runId}`, {
      base_version: run.version + 1,
      ops: [{ op: "remove", path: "/status_line" }],
    });
    expect([ended.status, ended.body.code, ended.body.reason]).toEqual([
      409,
      "conflict",
      "run_not_active",
    ]);
  });

  it("needs an integer base_version and a non-empty ops list", async () => {
    const [runId, token] = await queuedWithToken(fake, api);
    await api.post(`/runs/${runId}/start`, startBody(token));
    const ops = [{ op: "replace", path: "/status_line", value: "x" }];
    for (const body of [{ ops }, { base_version: true, ops }]) {
      const r = await api.patch(`/runs/${runId}`, body);
      expect([r.status, (r.body.errors as Json[])[0]?.pointer]).toEqual([
        400,
        "/base_version",
      ]);
    }
    const empty = await api.patch(`/runs/${runId}`, {
      base_version: 1,
      ops: [],
    });
    expect((empty.body.errors as Json[])[0]).toEqual({
      pointer: "/ops",
      code: "too_short",
    });
  });

  it("applies run.update over the gateway and answers error frames", async () => {
    const gw = await connect(fake);
    const [runId, token] = await queuedWithToken(fake, api);
    const base = Number(
      (await api.post(`/runs/${runId}/start`, startBody(token))).body.version,
    );
    const update = (
      baseVersion: number,
      ref: string,
      path: string,
      value: string,
    ): void =>
      gw.send("run.update", {
        run_id: runId,
        base_version: baseVersion,
        ref,
        ops: [{ op: "replace", path, value }],
      });
    update(base, "r1", "/status_line", "Thinking");
    update(base + 1, "r2", "/state", "done");
    expect((await gw.until("error")).d).toEqual({
      op: "run.update",
      code: "validation_failed",
      detail: "/ops/0: patch_path_forbidden",
      ref: "r2",
    });
    update(base, "r3", "/status_line", "Late");
    expect((await gw.until("error")).d).toEqual({
      op: "run.update",
      code: "base_version_conflict",
      ref: "r3",
    });
    const run = (await api.get(`/runs/${runId}`)).body as RunDoc;
    expect([run.status_line, run.version]).toEqual(["Thinking", base + 1]);
    gw.close();
  });
});

describe("Invocation-Token header, messages, commands, interactions, autocomplete", () => {
  it("checks the Invocation-Token header of a request", async () => {
    const [runId, token] = await queuedWithToken(fake, api);
    const bound = await api.get(`/runs/${runId}/context`, {
      "invocation-token": token,
    });
    expect(bound.status).toBe(200);
    expect((bound.body.trigger_message as Json).content_md).toBe(
      "Hello @Lotse",
    );
    expect((bound.body.trigger_message as Json).trust).toBe("trigger");
    const bad = await api.get(`/runs/${runId}`, {
      "invocation-token": "a.b.c",
    });
    expect([bad.status, bad.body.reason]).toEqual([403, "invalid"]);
    await api.post(`/runs/${runId}/finish`, {
      state: "failed",
      reason: "adapter_error",
    });
    const inactive = await api.get(`/runs/${runId}`, {
      "invocation-token": token,
    });
    expect([inactive.status, inactive.body.reason]).toEqual([
      403,
      "run_inactive",
    ]);
    const unauthenticated = await fetch(`${fake.baseUrl}/runs/${runId}`, {
      headers: { "invocation-token": token },
    });
    expect(unauthenticated.status).toBe(401);
  });

  it("creates messages idempotently by client_msg_id and serves components and commands", async () => {
    const [runId, token] = await queuedWithToken(fake, api);
    await api.post(`/runs/${runId}/start`, startBody(token));
    const doc = fake.run(runId);
    const body = {
      client_msg_id: "01900000-0000-7000-8000-00000000c001",
      content_md: "Done",
      run_id: runId,
      components: [
        {
          custom_id: "approval.once",
          type: "button",
          definition: { label: "Once", style: "primary" },
        },
      ],
    };
    const created = await api.post(`/threads/${doc.thread_id}/messages`, body);
    expect(created.status).toBe(201);
    const replay = await api.post(`/threads/${doc.thread_id}/messages`, body);
    expect([replay.status, replay.headers.get("idempotent-replay")]).toEqual([
      200,
      "true",
    ]);
    expect(replay.body.id).toBe(created.body.id);
    expect(fake.messages).toHaveLength(1);
    const component = (created.body.components as Json[])[0] ?? {};
    const upd = await api.patch(
      `/components/${component.id}`,
      { state: "disabled" },
      { "if-match": `"${component.version}"` },
    );
    expect([upd.status, upd.body.state, upd.body.version]).toEqual([
      200,
      "disabled",
      2,
    ]);
    const reg = await api.post("/commands", {
      name: "status",
      description: "Show the status",
      scope_kind: "channel",
      scope_id: doc.channel_id,
      options: [],
    });
    expect(reg.status).toBe(201);
    const listed = await api.get(`/channels/${doc.channel_id}/commands`);
    expect((listed.body.items as Json[]).map((c): unknown => c.id)).toEqual([
      reg.body.id,
    ]);
    expect(fake.commands.size).toBe(1);
    const noMatch = await api.put(`/commands/${reg.body.id}`, {
      name: "status",
      description: "x",
    });
    expect(noMatch.status).toBe(428);
    const put = await api.put(
      `/commands/${reg.body.id}`,
      { name: "status", description: "New" },
      { "if-match": `"${reg.body.version}"` },
    );
    expect(put.status).toBe(200);
    expect((await api.delete(`/commands/${reg.body.id}`)).status).toBe(204);
    const interactionId = await fake.deliverInteraction(runId);
    const events = ((await poll(api)).events as Json[]).filter(
      (e) => e.type === "interaction.created",
    );
    expect(events).toHaveLength(1);
    expect(((events[0]?.payload as Json).interaction as Json).id).toBe(
      interactionId,
    );
    expect(claims(String(events[0]?.invocation_token)).interaction_id).toBe(
      interactionId,
    );
    const responded = await api.post(`/interactions/${interactionId}/respond`, {
      state: "acknowledged",
    });
    expect([responded.status, responded.body.state]).toEqual([
      200,
      "acknowledged",
    ]);
  });

  it("replays an Idempotency-Key and refuses its reuse with another body", async () => {
    const body = { name: "deploy", description: "Deploy" };
    const key = { "idempotency-key": "01900000-0000-7000-8000-00000000e001" };
    const first = await api.post("/commands", body, key);
    const again = await api.post("/commands", body, key);
    expect([
      first.status,
      again.status,
      again.headers.get("idempotent-replay"),
    ]).toEqual([201, 201, "true"]);
    expect(again.body.id).toBe(first.body.id);
    const reuse = await api.post(
      "/commands",
      { ...body, description: "Other" },
      key,
    );
    expect([reuse.status, reuse.body.code]).toEqual([
      422,
      "idempotency_key_reuse",
    ]);
  });

  it("invokes commands and records heartbeats", async () => {
    await poll(api);
    const invocationId = await fake.invokeCommand("status");
    const events = ((await poll(api)).events as Json[]).filter(
      (e) => e.type === "command.invoked",
    );
    expect((events[0]?.payload as Json).invocation_id).toBe(invocationId);
    expect(claims(String(events[0]?.invocation_token)).trigger_kind).toBe(
      "command",
    );
    const beat = await api.post("/agents/me/heartbeat", {
      state: "online",
      reported_billing_mode: "personal",
    });
    expect([
      beat.status,
      (beat.body.presence as Json).billing_mode_matches,
    ]).toEqual([200, false]);
    expect(beat.body.interval_ms).toBe(fake.timings["sync.heartbeat_interval"]);
    expect(fake.heartbeats).toEqual([
      { state: "online", reported_billing_mode: "personal" },
    ]);
  });

  it("asks for autocompletion over the gateway and records the answer", async () => {
    const gw = await connect(fake);
    const commandId = await fake.registerCommand("deploy");
    const requestId = await fake.requestAutocomplete(commandId, "pr");
    const frame = (await gw.until("dispatch")).d;
    expect(frame.type).toBe("command.autocomplete_requested");
    expect("seq" in frame).toBe(false);
    const c = claims(String(frame.invocation_token));
    expect([c.trigger_kind, "run_id" in c]).toEqual(["autocomplete", false]);
    gw.send("command.autocomplete_result", {
      request_id: requestId,
      choices: [{ name: "Production", value: "prod" }],
    });
    await eventually(() => fake.autocompleteAnswer(requestId) !== undefined);
    expect(
      fake.autocompleteAnswer(requestId)?.map((choice) => choice.value),
    ).toEqual(["prod"]);
    gw.close();
    await eventually(() => gw.ws.readyState === WebSocket.CLOSED);
    await expect(fake.requestAutocomplete(commandId)).rejects.toThrow(
      "no live gateway connection",
    );
  });
});

describe("faults", () => {
  function message(threadId: string, n: number): [string, Json] {
    return [
      `/threads/${threadId}/messages`,
      {
        client_msg_id: `01900000-0000-7000-8000-00000000d${String(n).padStart(3, "0")}`,
        content_md: `m${n}`,
      },
    ];
  }

  it("uses the Python names as the fault kinds", () => {
    expect(Object.values(FaultKind).sort()).toEqual([
      "disconnect",
      "rate_limited",
      "stored_late",
      "stored_then_502",
    ]);
    expect([
      Fault.storedThen502().kind,
      Fault.storedLate(5).kind,
      Fault.rateLimited(2).kind,
      Fault.disconnect().kind,
    ]).toEqual([
      "stored_then_502",
      "stored_late",
      "rate_limited",
      "disconnect",
    ]);
    expect(() => Fault.storedLate(-1)).toThrow(RangeError);
    expect(() => Fault.rateLimited(-1)).toThrow(RangeError);
    expect(() => Fault.rateLimited(1.5)).toThrow(RangeError);
    expect(() =>
      fake.inject("createMessage", {
        kind: "store_then_502",
      } as unknown as Fault),
    ).toThrow(TypeError);
    expect(() => fake.inject("createMessage", Fault.disconnect(), 0)).toThrow(
      RangeError,
    );
  });

  it("stored_then_502 stores and answers 502 (store_then_502)", async () => {
    const triggered = await fake.trigger();
    fake.inject("createMessage", Fault.storedThen502());
    const [path, body] = message(triggered.threadId, 1);
    expect((await api.post(path, body)).status).toBe(502);
    expect(fake.messages).toHaveLength(1);
    expect((await api.post(path, body)).status).toBe(200); // the stored original
  });

  it("rate_limited answers 429 with Retry-After and stores nothing (rate_limit)", async () => {
    const triggered = await fake.trigger();
    fake.inject("createMessage", Fault.rateLimited(7), 2);
    const [path, body] = message(triggered.threadId, 2);
    for (let i = 0; i < 2; i++) {
      const limited = await api.post(path, body);
      expect([
        limited.status,
        limited.headers.get("retry-after"),
        limited.body.code,
      ]).toEqual([429, "7", "rate_limited"]);
    }
    expect(fake.messages).toHaveLength(0);
    expect((await api.post(path, body)).status).toBe(201);
  });

  it("disconnect drops the connection and stores nothing (drop_connection)", async () => {
    const triggered = await fake.trigger();
    fake.inject("createMessage", Fault.disconnect());
    const [path, body] = message(triggered.threadId, 3);
    await expect(api.post(path, body)).rejects.toThrow();
    expect(fake.messages).toHaveLength(0);
    expect(fake.requests().at(-1)).toEqual({
      method: "POST",
      path: `/api/v1${path}`,
      status: 0,
    });
  });

  it("stored_late answers after the delay of the clock (store_late)", async () => {
    const triggered = await fake.trigger();
    fake.inject("createMessage", Fault.storedLate(5_000));
    const [path, body] = message(triggered.threadId, 4);
    let done = false;
    const late = api.post(path, body).then((r) => {
      done = true;
      return r;
    });
    await eventually(() => fake.clock.sleepers > 0);
    expect(done).toBe(false);
    expect(fake.messages).toHaveLength(0);
    await fake.passTime(5_000);
    expect((await late).status).toBe(201);
    expect(fake.messages).toHaveLength(1);
  });

  it("drops the gateway connection on a faulted op", async () => {
    const gw = await connect(fake);
    await fake.trigger();
    const frame = (await gw.until("dispatch")).d;
    fake.inject("ack", Fault.disconnect());
    gw.send("ack", { seq: frame.seq as number });
    expect(await gw.closeCode()).toBe(1006);
    expect(fake.consumerState("gw").ackedSeq).toBeLessThan(Number(frame.seq));
  });
});

describe("gateway sessions", () => {
  it.each([4015, 4012])(
    "refuses to resume a session after the final close %i",
    async (code) => {
      const first = await connect(fake, "c7");
      let second: Gw | undefined;
      if (code === 4015) second = await connect(fake, "c7");
      else await fake.closeConnections(code);
      expect(await first.closeCode()).toBe(code);
      const resumed = await resume(fake, first.sessionId);
      expect(await resumed.closeCode()).toBe(4006);
      if (second !== undefined) {
        const run = await fake.trigger();
        const frame = (await second.until("dispatch")).d;
        expect((frame.payload as Json).run_id).toBe(run.runId);
        second.close();
      }
    },
  );

  it("lets a resume lose against a newer holder", async () => {
    const first = await connect(fake, "c2");
    fake.inject("heartbeat", Fault.disconnect());
    first.send("heartbeat", { last_seq: 0 });
    await first.closeCode();
    const second = await connect(fake, "c2");
    const late = await resume(fake, first.sessionId);
    expect((await late.recv()).op).toBe("resumed");
    expect(await late.closeCode()).toBe(4015);
    const run = await fake.trigger();
    expect(((await second.until("dispatch")).d.payload as Json).run_id).toBe(
      run.runId,
    );
    second.close();
  });

  it("keeps a session resumable after a client close without a status code", async () => {
    const first = await connect(fake, "c8");
    first.ws.close();
    expect(await first.closeCode()).toBe(1005);
    const again = await resume(fake, first.sessionId);
    expect((await again.recv()).op).toBe("resumed");
    again.close();
  });

  it("continues a dropped session with resume", async () => {
    const first = await connect(fake, "c3");
    fake.inject("heartbeat", Fault.disconnect());
    first.send("heartbeat", { last_seq: 0 });
    await first.closeCode();
    const again = await resume(fake, first.sessionId);
    const resumed = await again.recv();
    expect(resumed.op).toBe("resumed");
    expect(Object.keys(resumed.d).sort()).toEqual([
      "gateway_session_id",
      "head_seq",
      "sync_epoch",
    ]);
    const run = await fake.trigger();
    expect(((await again.until("dispatch")).d.payload as Json).run_id).toBe(
      run.runId,
    );
    again.close();
  });

  it("answers heartbeats and closes on protocol errors", async () => {
    const gw = await connect(fake);
    gw.send("heartbeat", { last_seq: 0 });
    expect(Object.keys((await gw.until("heartbeat_ack")).d)).toEqual([
      "server_time",
    ]);
    gw.send("no.such.op", {});
    expect(await gw.closeCode()).toBe(4001);
    const [unidentified] = await open(fake);
    unidentified.send("ack", { seq: 0 });
    expect(await unidentified.closeCode()).toBe(4003);
    const [bad, nonce] = await open(fake);
    bad.send("identify", {
      protocol_version: 1,
      api_token: "wrong",
      dpop: await proof(fake, "GET", fake.gatewayHtu, nonce),
    });
    expect(await bad.closeCode()).toBe(4004);
  });
});

describe("records, deprecation, settings, webhooks", () => {
  it("marks deprecated operations", async () => {
    fake.deprecate("listServedChannels");
    const r = await api.get("/agents/me/channels");
    const other = await api.get("/agents/me/triggerers");
    expect(r.headers.get("deprecation")).toBe("true");
    expect(r.headers.get("sunset")).toBeTruthy();
    expect(other.headers.get("deprecation")).toBeNull();
    expect((other.body.principal_ids as string[]).length).toBe(2);
  });

  it("records method, path and status only, never a header value or a token", async () => {
    const [runId, token] = await queuedWithToken(fake, api);
    await api.post(`/runs/${runId}/start`, startBody(token), {
      "invocation-token": token,
    });
    const records = fake.requests();
    expect(records.length).toBeGreaterThan(3);
    for (const record of records)
      expect(Object.keys(record).sort()).toEqual(["method", "path", "status"]);
    expect(
      records.find((r) => r.path === `/api/v1/runs/${runId}/start`),
    ).toEqual({
      method: "POST",
      path: `/api/v1/runs/${runId}/start`,
      status: 200,
    });
    const text = JSON.stringify(records) + JSON.stringify(fake.frames());
    expect(text.includes(token)).toBe(false);
    expect(text.includes(fake.credentials.apiToken.reveal())).toBe(false);
    expect(text.toLowerCase().includes("dpop ")).toBe(false);
  });

  it("validates its timings and implements ScenarioServer", () => {
    expect(
      () => new FakeServer({ timings: { "agent.no_such_key": 1 } }),
    ).toThrow(RangeError);
    const shortcut = new FakeServer({ ackTimeoutMs: 1_000, tokenTtlMs: 2_000 });
    expect([
      shortcut.timings["agent.ack_timeout"],
      shortcut.timings["agent.invocation_token_ttl"],
    ]).toEqual([1_000, 2_000]);
    const server: ScenarioServer = fake;
    expect(server.baseUrl).toBe(`${server.origin}/api/v1`);
    expect(fake.gatewayUrl).toBe(`ws${server.origin.slice(4)}/api/v1/gateway`);
    expect(server.sdkSettings.invocationTokenLeewayMs).toBeLessThanOrEqual(
      fake.timings["auth.clock_leeway"],
    );
    expect(server.otherAgentId).not.toBe(server.agentId);
  });

  it("forges tokens for a run that was never delivered", async () => {
    const triggered = await fake.trigger(); // no consumer: the run stays queued without a delivery
    for (const variant of [
      "bad_signature",
      "alg_hs256",
      "typ_access",
      "aud_other_agent",
    ] as const) {
      const forged = (await fake.forgeToken(triggered.runId, variant)).reveal();
      const start = await api.post(
        `/runs/${triggered.runId}/start`,
        startBody(forged),
      );
      const renew = await api.post(
        `/runs/${triggered.runId}/invocation-token`,
        { invocation_token: forged },
      );
      for (const answer of [start, renew])
        expect([answer.status, answer.body.reason]).toEqual([403, "invalid"]);
    }
    expect(await fake.runState(triggered.runId)).toEqual({
      state: "queued",
      stateReason: undefined,
      version: 1,
      starts: 0,
    });
    await expect(
      fake.forgeToken(triggered.runId, "none" as never),
    ).rejects.toThrow(RangeError);
  });

  it("signs webhooks with its own key and the given HMAC secrets", async () => {
    const received: { headers: IncomingMessage["headers"]; body: string }[] =
      [];
    const receiver = createServer((req, res) => {
      const chunks: Buffer[] = [];
      req.on("data", (c: Buffer) => chunks.push(c));
      req.on("end", () => {
        received.push({
          headers: req.headers,
          body: Buffer.concat(chunks).toString("utf8"),
        });
        res.writeHead(204).end();
      });
    });
    await new Promise<void>((resolve) =>
      receiver.listen(0, "127.0.0.1", resolve),
    );
    try {
      const port = (receiver.address() as AddressInfo).port;
      const secret = `${WHSEC}EXAMPLEsecretForTestVectorsOnly1`;
      const envelope = {
        id: uuidv7(),
        type: "message.created",
        payload: { text: "Hi" },
      };
      const answer = await fake.sendWebhook(
        `http://127.0.0.1:${port}/hook`,
        envelope,
        { hmacSecrets: [secret] },
      );
      expect(answer.status).toBe(204);
      const [hit] = received;
      expect(hit?.headers["webhook-id"]).toBe(envelope.id);
      expect(hit?.headers["webhook-timestamp"]).toBe(
        String(Math.floor(fake.clock.now() / 1000)),
      );
      expect(JSON.parse(hit?.body ?? "")).toEqual(envelope);
      const signed = `${envelope.id}.${String(hit?.headers["webhook-timestamp"])}.${hit?.body}`;
      const [v1a, v1] = String(hit?.headers["webhook-signature"]).split(" ");
      const keys = (await (
        await fetch(`${fake.baseUrl}/instance/webhook-keys`)
      ).json()) as Json;
      const der = Buffer.from(
        String((keys.keys as Json[])[0]?.public_key).slice(5),
        "base64",
      );
      const publicKey = createPublicKey({
        key: der,
        format: "der",
        type: "spki",
      });
      expect(v1a?.startsWith("v1a,")).toBe(true);
      expect(
        verifySignature(
          null,
          Buffer.from(signed),
          publicKey,
          Buffer.from(v1a?.slice(4) ?? "", "base64"),
        ),
      ).toBe(true);
      const mac = createHmac("sha256", Buffer.from(secret.slice(6), "base64"))
        .update(signed)
        .digest("base64");
      expect(v1).toBe(`v1,${mac}`);
    } finally {
      await new Promise((resolve) => receiver.close(resolve));
    }
  });

  it("builds its bodies from the fixtures recorded for the Python FakeServer", () => {
    const dir = new URL(
      "../../python/src/bothy_sdk/testing/fixtures/",
      import.meta.url,
    );
    for (const [name, value] of Object.entries(FIXTURES)) {
      const recorded: unknown = JSON.parse(
        readFileSync(new URL(`${name}.json`, dir), "utf8"),
      );
      expect(value, name).toEqual(recorded);
    }
    expect(Object.keys(FIXTURES).sort()).toEqual([
      "agent.followup",
      "agent.invocation",
      "command.autocomplete_requested",
      "command.invoked",
      "discovery",
      "interaction.created",
      "run",
      "run.cancel_requested",
      "run.state_changed",
      "run_context",
    ]);
  });
});

// Every case with the answer the Python FakeServer's `parse_ops` and `apply_ops` gave for it
// (`excerpt_max` 5, a fixed time): the path table of 09 5.3 answers alike in both SDKs.
const PATCH_CASES: readonly {
  ops: unknown;
  errors?: unknown;
  box?: unknown;
}[] = [
  {
    ops: [{ op: "add", path: "/status_line", value: "a\u0007b" }],
    errors: [{ pointer: "/ops/0/value", code: "control_character" }],
  },
  {
    ops: [{ op: "add", path: "/status_line", value: "a\u202eb" }],
    errors: [{ pointer: "/ops/0/value", code: "bidi_control" }],
  },
  {
    ops: [{ op: "remove", path: "/status_line" }],
    box: { metrics: {}, steps: {} },
  },
  {
    ops: [{ op: "add", path: "/last_text_excerpt", value: "abcdefgh" }],
    box: { metrics: {}, steps: {}, last_text_excerpt: "defgh" },
  },
  {
    ops: [{ op: "remove", path: "/last_text_excerpt" }],
    errors: [
      {
        pointer: "/ops/0",
        code: "patch_path_forbidden",
        detail: "this path is not patchable with this operation",
      },
    ],
  },
  {
    ops: [{ op: "replace", path: "/model", value: 5 }],
    errors: [{ pointer: "/ops/0/value", code: "invalid_type" }],
  },
  {
    ops: [
      {
        op: "replace",
        path: "/cost",
        value: {
          amount_micros: 1.5,
          currency: "EUR",
          is_estimate: false,
          extra: 1,
        },
      },
    ],
    errors: [
      { pointer: "/ops/0/value/extra", code: "unknown_member" },
      { pointer: "/ops/0/value/amount_micros", code: "invalid_type" },
      { pointer: "/ops/0/value/is_estimate", code: "invalid" },
    ],
  },
  {
    ops: [{ op: "replace", path: "/cost", value: {} }],
    errors: [
      { pointer: "/ops/0/value/amount_micros", code: "required" },
      { pointer: "/ops/0/value/currency", code: "required" },
      { pointer: "/ops/0/value/is_estimate", code: "required" },
    ],
  },
  {
    ops: [
      {
        op: "replace",
        path: "/cost",
        value: { amount_micros: 12, currency: "EUR", is_estimate: true },
      },
    ],
    box: {
      metrics: {},
      steps: {},
      cost: { amount_micros: 12, currency: "EUR", is_estimate: true },
    },
  },
  {
    ops: [
      {
        op: "add",
        path: "/plan",
        value: {
          items: [
            { id: "a", text: "One", status: "pending" },
            { id: "a", text: "x\u0001", status: "done" },
            5,
            {},
          ],
        },
      },
    ],
    errors: [
      { pointer: "/ops/0/value/items/1/id", code: "invalid" },
      { pointer: "/ops/0/value/items/1/text", code: "control_character" },
      { pointer: "/ops/0/value/items/1/status", code: "invalid_enum" },
      { pointer: "/ops/0/value/items/2", code: "invalid_type" },
      { pointer: "/ops/0/value/items/3/id", code: "required" },
      { pointer: "/ops/0/value/items/3/text", code: "required" },
      { pointer: "/ops/0/value/items/3/status", code: "required" },
    ],
  },
  {
    ops: [{ op: "add", path: "/plan", value: { items: [] } }],
    box: { metrics: {}, steps: {} },
  },
  {
    ops: [{ op: "add", path: "/plan", value: [] }],
    errors: [{ pointer: "/ops/0/value/items", code: "invalid_type" }],
  },
  {
    ops: [
      {
        op: "add",
        path: "/plan",
        value: { items: [{ id: "a", text: "One", status: "pending" }] },
      },
    ],
    box: {
      metrics: {},
      steps: {},
      plan: { items: [{ id: "a", text: "One", status: "pending" }] },
    },
  },
  { ops: [{ op: "remove", path: "/plan" }], box: { metrics: {}, steps: {} } },
  {
    ops: [{ op: "add", path: "/metrics/context_pct", value: 101 }],
    errors: [{ pointer: "/ops/0/value", code: "too_large" }],
  },
  {
    ops: [{ op: "add", path: "/metrics/context_pct", value: -1 }],
    errors: [{ pointer: "/ops/0/value", code: "too_small" }],
  },
  {
    ops: [{ op: "add", path: "/metrics/context_pct", value: "x" }],
    errors: [{ pointer: "/ops/0/value", code: "invalid_type" }],
  },
  {
    ops: [{ op: "add", path: "/metrics/tokens_in", value: 1.5 }],
    errors: [{ pointer: "/ops/0/value", code: "invalid_type" }],
  },
  {
    ops: [{ op: "add", path: "/metrics/tokens_in", value: 9007199254740992 }],
    errors: [{ pointer: "/ops/0/value", code: "too_large" }],
  },
  {
    ops: [{ op: "add", path: "/metrics/tokens_in", value: 7 }],
    box: { metrics: { tokens_in: 7 }, steps: {} },
  },
  {
    ops: [{ op: "remove", path: "/metrics/tokens_in" }],
    errors: [
      {
        pointer: "/ops/0",
        code: "patch_path_forbidden",
        detail: "this path is not patchable with this operation",
      },
    ],
  },
  {
    ops: [{ op: "add", path: "/metrics/other", value: 7 }],
    errors: [
      {
        pointer: "/ops/0",
        code: "patch_path_forbidden",
        detail: "this path is not patchable with this operation",
      },
    ],
  },
  {
    ops: [
      {
        op: "add",
        path: "/metrics/last_api_retry",
        value: { max_retries: 1.5, error: "e\u0002", x: 1 },
      },
    ],
    errors: [
      { pointer: "/ops/0/value/x", code: "unknown_member" },
      { pointer: "/ops/0/value/attempt", code: "required" },
      { pointer: "/ops/0/value/max_retries", code: "invalid_type" },
      { pointer: "/ops/0/value/error", code: "control_character" },
    ],
  },
  {
    ops: [{ op: "add", path: "/metrics/last_api_retry", value: 5 }],
    errors: [{ pointer: "/ops/0/value", code: "invalid_type" }],
  },
  {
    ops: [
      {
        op: "add",
        path: "/metrics/last_api_retry",
        value: { attempt: 2, error: "timeout" },
      },
    ],
    box: {
      metrics: { last_api_retry: { attempt: 2, error: "timeout" } },
      steps: {},
    },
  },
  {
    ops: [{ op: "remove", path: "/metrics/last_api_retry" }],
    box: { metrics: {}, steps: {} },
  },
  {
    ops: [
      {
        op: "add",
        path: "/steps/01900000-0000-7000-8000-00000000abcd",
        value: {
          id: "01900000-0000-7000-8000-00000000abce",
          kind: "nope",
          title: "t\u0003",
          status: "x",
          detail: [],
          parent_step_id: "zz",
          started_at: "yesterday",
          extra: 1,
        },
      },
    ],
    errors: [
      { pointer: "/ops/0/value/extra", code: "unknown_member" },
      { pointer: "/ops/0/value/id", code: "invalid" },
      { pointer: "/ops/0/value/kind", code: "invalid_enum" },
      { pointer: "/ops/0/value/name", code: "required" },
      { pointer: "/ops/0/value/title", code: "control_character" },
      { pointer: "/ops/0/value/status", code: "invalid_enum" },
      { pointer: "/ops/0/value/detail", code: "invalid_type" },
      { pointer: "/ops/0/value/parent_step_id", code: "invalid_format" },
      { pointer: "/ops/0/value/started_at", code: "invalid_format" },
    ],
  },
  {
    ops: [
      {
        op: "add",
        path: "/steps/01900000-0000-7000-8000-00000000abcd",
        value: 5,
      },
    ],
    errors: [{ pointer: "/ops/0/value", code: "invalid_type" }],
  },
  {
    ops: [
      {
        op: "add",
        path: "/steps/01900000-0000-7000-8000-00000000abcd",
        value: { name: "n" },
      },
    ],
    errors: [{ pointer: "/ops/0/value/kind", code: "required" }],
  },
  {
    ops: [
      {
        op: "add",
        path: "/steps/01900000-0000-7000-8000-00000000abcd",
        value: {
          kind: "status",
          name: "n",
          parent_step_id: "01900000-0000-7000-8000-00000000abce",
        },
      },
    ],
    errors: [
      { pointer: "/ops/0/value/parent_step_id", code: "patch_target_missing" },
    ],
  },
  {
    ops: [
      {
        op: "add",
        path: "/steps/01900000-0000-7000-8000-00000000abcd",
        value: {
          kind: "status",
          name: "n",
          started_at: "2026-01-01T00:00:00Z",
          finished_at: "2026-01-01T00:00:01.5+00:00",
        },
      },
      {
        op: "add",
        path: "/steps/01900000-0000-7000-8000-00000000abce",
        value: {
          kind: "tool_call",
          name: "m",
          parent_step_id: "01900000-0000-7000-8000-00000000abcd",
          title: "T",
          status: "pending",
          detail: { a: 1 },
        },
      },
    ],
    box: {
      metrics: {},
      steps: {
        "01900000-0000-7000-8000-00000000abcd": {
          id: "01900000-0000-7000-8000-00000000abcd",
          ordinal: 1,
          kind: "status",
          name: "n",
          title: "",
          status: "running",
          detail: {},
          started_at: "2026-01-01T00:00:00Z",
          finished_at: "2026-01-01T00:00:01.5+00:00",
        },
        "01900000-0000-7000-8000-00000000abce": {
          id: "01900000-0000-7000-8000-00000000abce",
          ordinal: 2,
          kind: "tool_call",
          name: "m",
          title: "T",
          status: "pending",
          detail: { a: 1 },
          started_at: "2026-01-01T00:00:00.000Z",
          parent_step_id: "01900000-0000-7000-8000-00000000abcd",
        },
      },
    },
  },
  {
    ops: [
      {
        op: "add",
        path: "/steps/01900000-0000-7000-8000-00000000abcd",
        value: { kind: "status", name: "n" },
      },
      {
        op: "add",
        path: "/steps/01900000-0000-7000-8000-00000000abcd",
        value: { kind: "status", name: "n" },
      },
    ],
    errors: [{ pointer: "/ops/1", code: "patch_target_exists" }],
  },
  {
    ops: [
      {
        op: "replace",
        path: "/steps/01900000-0000-7000-8000-00000000abcd/status",
        value: "bogus",
      },
    ],
    errors: [{ pointer: "/ops/0/value", code: "invalid_enum" }],
  },
  {
    ops: [
      {
        op: "replace",
        path: "/steps/01900000-0000-7000-8000-00000000abcd/title",
        value: "a\u0085",
      },
    ],
    errors: [{ pointer: "/ops/0/value", code: "control_character" }],
  },
  {
    ops: [
      {
        op: "replace",
        path: "/steps/01900000-0000-7000-8000-00000000abcd/detail",
        value: "x",
      },
    ],
    errors: [{ pointer: "/ops/0/value", code: "invalid_type" }],
  },
  {
    ops: [
      {
        op: "replace",
        path: "/steps/01900000-0000-7000-8000-00000000abcd/finished_at",
        value: "2026-01-01",
      },
    ],
    errors: [{ pointer: "/ops/0/value", code: "invalid_format" }],
  },
  {
    ops: [
      {
        op: "replace",
        path: "/steps/01900000-0000-7000-8000-00000000abcd/name",
        value: "x",
      },
    ],
    errors: [
      {
        pointer: "/ops/0",
        code: "patch_path_forbidden",
        detail: "this path is not patchable with this operation",
      },
    ],
  },
  {
    ops: [
      { op: "remove", path: "/steps/01900000-0000-7000-8000-00000000abcd" },
    ],
    errors: [
      {
        pointer: "/ops/0",
        code: "patch_path_forbidden",
        detail: "this path is not patchable with this operation",
      },
    ],
  },
  {
    ops: [
      {
        op: "replace",
        path: "/followups/01900000-0000-7000-8000-00000000abcd/state",
        value: "delivered",
      },
    ],
    errors: [{ pointer: "/ops/0", code: "patch_target_missing" }],
  },
  {
    ops: [
      {
        op: "replace",
        path: "/followups/01900000-0000-7000-8000-00000000abcd/state",
        value: "other",
      },
    ],
    errors: [{ pointer: "/ops/0/value", code: "invalid_enum" }],
  },
  {
    ops: [
      {
        op: "add",
        path: "/followups/01900000-0000-7000-8000-00000000abcd/state",
        value: "delivered",
      },
    ],
    errors: [
      {
        pointer: "/ops/0",
        code: "patch_path_forbidden",
        detail: "this path is not patchable with this operation",
      },
    ],
  },
  {
    ops: [{ op: "add", path: "/steps/not-a-uuid", value: {} }],
    errors: [{ pointer: "/ops/0/path", code: "invalid_format" }],
  },
  {
    ops: [{ op: "add", path: "status_line", value: "x" }],
    errors: [{ pointer: "/ops/0/path", code: "invalid_format" }],
  },
  {
    ops: [{ op: "add", path: "/a~2b", value: "x" }],
    errors: [{ pointer: "/ops/0/path", code: "invalid_format" }],
  },
  {
    ops: [{ op: "move", path: "/status_line" }],
    errors: [{ pointer: "/ops/0/op", code: "invalid_enum" }],
  },
  {
    ops: [5, { op: "add" }],
    errors: [
      { pointer: "/ops/0", code: "invalid_type" },
      { pointer: "/ops/1", code: "invalid_type" },
    ],
  },
  {
    ops: [{ op: "add", path: "/status_line" }],
    errors: [{ pointer: "/ops/0/value", code: "required" }],
  },
  {
    ops: [{ op: "add", path: "/status_line", value: null }],
    errors: [{ pointer: "/ops/0/value", code: "invalid_type" }],
  },
  {
    ops: [
      {
        op: "add",
        path: "/plan",
        value: { items: [{ id: "a", text: null, status: "pending" }] },
      },
    ],
    errors: [{ pointer: "/ops/0/value/items/0/text", code: "invalid_type" }],
  },
  {
    ops: [{ op: "add", path: "", value: {} }],
    errors: [
      {
        pointer: "/ops/0",
        code: "patch_path_forbidden",
        detail: "this path is not patchable with this operation",
      },
    ],
  },
  {
    ops: [
      { op: "add", path: "/metrics~1last_api_retry", value: { attempt: 1 } },
    ],
    errors: [
      {
        pointer: "/ops/0",
        code: "patch_path_forbidden",
        detail: "this path is not patchable with this operation",
      },
    ],
  },
  { ops: "not-a-list", errors: [{ pointer: "/ops", code: "invalid_type" }] },
];

describe("patch table parity with the Python FakeServer", () => {
  it.each(PATCH_CASES.map((c, i) => [i, c] as const))(
    "case %i",
    (_index, c) => {
      let got: unknown;
      try {
        const parsed = parseOps(c.ops);
        got = {
          box: applyOps({ metrics: {}, steps: {} }, parsed, {
            nowIso: () => "2026-01-01T00:00:00.000Z",
            excerptMax: 5,
          }),
        };
      } catch (error) {
        if (!(error instanceof FakeProblem)) throw error;
        got = { errors: error.errors };
      }
      expect(got).toEqual(
        c.errors !== undefined ? { errors: c.errors } : { box: c.box },
      );
    },
  );
});
