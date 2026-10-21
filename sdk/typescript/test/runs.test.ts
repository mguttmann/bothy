// SPDX-License-Identifier: Apache-2.0
//
// Run patcher and run handle: paths, throttle, coalescing, rebase, steps, follow-ups, finish
// (docs/spec/09-agenten.md 5.3, docs/DECISIONS.md D-M2-29-9; port of
// sdk/python/tests/test_runs.py). The patches go to the FakeServer, which applies the path
// table of 09 5.3 and `base_version`; a wrapped `fetch` records the request bodies so a test
// can see what one patch carried. The few answers the FakeServer has no rule for (an agent
// session, a conflict that never ends) come from a scripted `fetch`.

import { readFileSync } from "node:fs";
import { fileURLToPath } from "node:url";

import { afterEach, beforeEach, describe, expect, it } from "vitest";

import { createApiClient, unwrap, type ApiClient } from "../src/client.js";
import {
  BaseVersionConflict,
  ConflictError,
  GatewayOpError,
  InvocationTokenInvalid,
  ProblemError,
  RateLimitedError,
  ValidationFailed,
} from "../src/errors.js";
import type { components } from "../src/generated/openapi.js";
import { uuidv7 } from "../src/ids.js";
import type { Logger } from "../src/logger.js";
import { silentLogger } from "../src/logger.js";
import {
  isRunEnded,
  METRIC_FIELDS,
  PATCH_PATHS,
  PatchPathError,
  RunHandle,
  RunPatcher,
} from "../src/runs/index.js";
import { FakeClock } from "../src/testing/clock.js";
import { FakeServer } from "../src/testing/fakeserver.js";
import { Fault } from "../src/testing/faults.js";

type S = components["schemas"];
type Json = Record<string, unknown>;

const SPEC = fileURLToPath(
  new URL("../../../docs/spec/09-agenten.md", import.meta.url),
);
const CORR = "0192f4a0-7b1c-7000-8a00-000000000009";
const UUID7 =
  /^[0-9a-f]{8}-[0-9a-f]{4}-7[0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$/;

interface Sent {
  method: string;
  path: string;
  body: Json | undefined;
  headers: Headers;
}

/** Waits in real time until `check` holds (HTTP to the FakeServer takes real turns). */
async function eventually(check: () => boolean, ms = 3_000): Promise<void> {
  const until = Date.now() + ms;
  while (!check()) {
    if (Date.now() > until) throw new Error("condition not reached in time");
    await new Promise((r) => setTimeout(r, 5));
  }
}

/** Lets pending I/O run for a while without moving the fake clock. */
async function quiet(ms = 60): Promise<void> {
  await new Promise((r) => setTimeout(r, ms));
}

async function bodyOf(request: Request): Promise<Json | undefined> {
  const text = await request.clone().text();
  return text ? (JSON.parse(text) as Json) : undefined;
}

/** A client of Lotse against the FakeServer whose `fetch` records every request. */
function fakeClient(
  fake: FakeServer,
  sent: Sent[],
  settings: Json = {},
  logger: Logger = silentLogger,
): ApiClient {
  return createApiClient({
    baseUrl: fake.baseUrl,
    token: fake.credentials.apiToken,
    dpopKey: fake.credentials.dpop,
    clock: fake.clock,
    logger,
    settings: { ...fake.sdkSettings, ...settings },
    fetch: async (request: Request) => {
      sent.push({
        method: request.method,
        path: new URL(request.url).pathname,
        body: await bodyOf(request),
        headers: request.headers,
      });
      return globalThis.fetch(request);
    },
  });
}

/** A client whose answers come from `answer` (the shape of Python's `httpx.MockTransport`). */
function scriptedClient(
  answer: (request: Request, body: Json | undefined, n: number) => Response,
  sent: Sent[],
  settings: Json = {},
): ApiClient {
  return createApiClient({
    baseUrl: "https://example.com/api/v1",
    token: "runs-test-token",
    clock: new FakeClock(),
    logger: silentLogger,
    settings: { retryMaxAttempts: 1, ...settings },
    fetch: async (request: Request) => {
      const body = await bodyOf(request);
      sent.push({
        method: request.method,
        path: new URL(request.url).pathname,
        body,
        headers: request.headers,
      });
      return answer(request, body, sent.length);
    },
  });
}

function json(status: number, body: unknown, headers: Json = {}): Response {
  return new Response(JSON.stringify(body), {
    status,
    headers: {
      "Content-Type":
        status >= 400 ? "application/problem+json" : "application/json",
      ...(headers as Record<string, string>),
    },
  });
}

function problem(status: number, code: string, extra: Json = {}): Json {
  return {
    type: "about:blank",
    title: code,
    status,
    code,
    correlation_id: CORR,
    ...extra,
  };
}

interface Started {
  readonly runId: string;
  readonly threadId: string;
  readonly token: string;
  readonly run: S["Run"];
}

/** Triggers a run, fetches its invocation over REST and starts it (`startRun` answered 200). */
async function startRun(api: ApiClient, fake: FakeServer): Promise<Started> {
  const poll = (): Promise<S["EventPage"]> =>
    unwrap(
      api.GET("/agents/me/consumers/{consumer}/events", {
        params: { path: { consumer: "rest" } },
      }),
      "pollAgentEvents",
    );
  await poll(); // the first poll creates the cursor at the head
  const triggered = await fake.trigger();
  const page = await poll();
  const event = page.events.find((e) => e.type === "agent.invocation") as
    (S["Event"] & { invocation_token?: string }) | undefined;
  const token = String(event?.invocation_token);
  const run = await unwrap(
    api.POST("/runs/{run_id}/start", {
      params: { path: { run_id: triggered.runId } },
      body: {
        invocation_token: token,
        billing_mode_used: "organization",
        tool_class_used: "scoped",
      },
    }),
    "startRun",
  );
  return { runId: triggered.runId, threadId: triggered.threadId, token, run };
}

function patches(sent: readonly Sent[]): Sent[] {
  return sent.filter((s) => s.method === "PATCH");
}

function opsOf(item: Sent | undefined): Json[] {
  return (item?.body?.ops ?? []) as Json[];
}

/** One concrete path of a table row (a metric name and UUIDv7s for the placeholders). */
function concrete(pattern: string): string {
  return pattern
    .replace("<field>", "tokens_in")
    .replace("<id>", uuidv7())
    .replace("<message_id>", uuidv7());
}

describe("path table of 09 5.3", () => {
  it("equals the table of the specification", () => {
    const text = readFileSync(SPEC, "utf8");
    const section = text.slice(
      text.indexOf("### 5.3 "),
      text.indexOf("### 5.4 "),
    );
    const expected = new Set<string>();
    const key = (path: string, ops: Iterable<string>): string =>
      `${path} ${[...ops].sort().join(",")}`;
    for (const line of section.split("\n")) {
      if (!line.startsWith("| `/")) continue;
      const cells = line
        .replace(/^\||\|$/g, "")
        .split("|")
        .map((c) => c.trim());
      const ops = [...(cells[2] ?? "").matchAll(/`(add|replace|remove)`/g)].map(
        (m) => m[1] ?? "",
      );
      if (!ops.length) continue; // "nie": the server owns these paths
      const path = (cells[0] ?? "")
        .replace(/`/g, "")
        .replace("<feld>", "<field>");
      if (path === "/steps/<id>") {
        expected.add(key(path, ["add"]));
        for (const m of (cells[2] ?? "").matchAll(
          /`(status|title|detail|finished_at)`/g,
        ))
          expected.add(key(`/steps/<id>/${m[1] ?? ""}`, ["replace"]));
        continue;
      }
      expected.add(key(path, ops));
    }
    expect(new Set(PATCH_PATHS.map((p) => key(p.pattern, p.ops)))).toEqual(
      expected,
    );
  });

  it("allows exactly the ops of each row and nothing on other paths", () => {
    for (const row of PATCH_PATHS) {
      const path = concrete(row.pattern);
      expect(row.matches(path)).toBe(true);
      for (const op of ["add", "replace", "remove"] as const)
        expect(
          PATCH_PATHS.some((p) => p.matches(path) && p.ops.has(op)),
          `${op} ${path}`,
        ).toBe(row.ops.has(op));
    }
    for (const path of [
      "/state",
      "/version",
      "/billing_mode_expected",
      "/started_at",
      "/metrics/unknown",
      "/steps/x",
      "/steps/x/status",
      `/steps/${uuidv7()}/ordinal`,
      `/followups/${uuidv7()}`,
      "/status_line/x",
    ])
      expect(
        PATCH_PATHS.some((p) => p.matches(path)),
        path,
      ).toBe(false);
    expect(METRIC_FIELDS).toContain("context_pct");
  });
});

describe("RunPatcher against the FakeServer", () => {
  let fake: FakeServer;
  const sent: Sent[] = [];

  beforeEach(async () => {
    fake = new FakeServer();
    await fake.start();
    sent.length = 0;
  });

  afterEach(async () => {
    await fake.stop();
  });

  it("refuses a path or op outside 09 5.3 before any request, /state included", async () => {
    const api = fakeClient(fake, sent);
    const { runId, run } = await startRun(api, fake);
    const patcher = new RunPatcher(api, runId, run.version);
    sent.length = 0;
    const refused: [S["JsonPatchOperation"]["op"], string][] = [
      ["replace", "/state"],
      ["add", "/model"],
      ["replace", "/steps/x"],
      ["remove", "/cost"],
      ["replace", "/version"],
    ];
    for (const [op, path] of refused) {
      const error = await patcher.sendOps([{ op, path, value: "x" }]).then(
        () => undefined,
        (e: unknown) => e,
      );
      expect(error).toBeInstanceOf(PatchPathError);
      expect(String(error)).toMatch(/not allowed/);
    }
    expect(sent).toEqual([]);
    expect(fake.requests().filter((r) => r.method === "PATCH")).toEqual([]);
  });

  it("sends raw ops that pass the table and returns the new version", async () => {
    const api = fakeClient(fake, sent);
    const { runId, run } = await startRun(api, fake);
    const patcher = new RunPatcher(api, runId, run.version);
    const version = await patcher.sendOps([
      { op: "replace", path: "/status_line", value: "Reading" },
    ]);
    expect(version).toBe(run.version + 1);
    expect(fake.run(runId).status_line).toBe("Reading");
  });

  it("coalesces and sends at most once per agent.run_update_min_interval", async () => {
    const api = fakeClient(fake, sent, { runUpdateMinIntervalMs: 500 });
    const { runId, run } = await startRun(api, fake);
    sent.length = 0;
    const patcher = new RunPatcher(api, runId, run.version);
    patcher.setStatusLine("one");
    patcher.setStatusLine("two");
    const step = patcher.addStep("tool_call", "search");
    patcher.updateStep(step, { status: "completed" });
    await fake.passTime(0);
    await eventually(() => patcher.patchesSent === 1);
    expect(patches(sent)).toHaveLength(1);
    const ops = opsOf(patches(sent)[0]);
    expect(ops).toContainEqual({
      op: "replace",
      path: "/status_line",
      value: "two",
    });
    const adds = ops.filter((o) => o.path === `/steps/${step}`);
    expect(adds).toHaveLength(1);
    expect(adds[0]?.op).toBe("add");
    expect((adds[0]?.value as Json).status).toBe("completed");
    expect(fake.run(runId).steps[step]?.status).toBe("completed");

    patcher.setModel("m1");
    await fake.passTime(200);
    await quiet();
    expect(patches(sent)).toHaveLength(1); // still throttled
    await fake.passTime(310);
    await eventually(() => patches(sent).length === 2);
    expect(patches(sent)[1]?.body?.base_version).toBe(run.version + 1);
    expect(fake.run(runId).model).toBe("m1");
    await patcher.aclose();
  });

  it("keeps a later replace of a path in the place of the first, other ops in order", async () => {
    const api = fakeClient(fake, sent);
    const { runId, run } = await startRun(api, fake);
    sent.length = 0;
    const patcher = new RunPatcher(api, runId, run.version);
    patcher.setStatusLine("one");
    patcher.setModel("m1");
    patcher.clearStatusLine();
    patcher.setMetric("tokens_in", 7);
    await patcher.flush();
    expect(opsOf(patches(sent)[0])).toEqual([
      { op: "remove", path: "/status_line" },
      { op: "replace", path: "/model", value: "m1" },
      { op: "replace", path: "/metrics/tokens_in", value: 7 },
    ]);
    await patcher.aclose();
  });

  it("sends the members the setters build", async () => {
    const api = fakeClient(fake, sent);
    const { runId, run } = await startRun(api, fake);
    const patcher = new RunPatcher(api, runId, run.version);
    patcher.setLastTextExcerpt("Draft so far");
    patcher.setLastApiRetry(2, {
      maxRetries: 5,
      retryDelayMs: 1000,
      error: "overloaded",
    });
    patcher.setCost(1500, "USD");
    patcher.setPlan([{ id: "p1", text: "Read", status: "in_progress" }]);
    const parent = patcher.addStep("workflow", "plan", { title: "Plan" });
    const child = patcher.addStep("tool_call", "read", {
      parentStepId: parent,
      detail: { file: "notes.txt" },
      status: "pending",
    });
    await patcher.flush();
    patcher.updateStep(child, {
      status: "completed",
      title: "Read notes",
      detail: { lines: 3 },
      finishedAt: new Date(Date.UTC(2026, 0, 1, 0, 0, 1)),
    });
    await patcher.flush();
    const doc = fake.run(runId);
    expect(doc.last_text_excerpt).toBe("Draft so far");
    expect(doc.metrics.last_api_retry).toEqual({
      attempt: 2,
      max_retries: 5,
      retry_delay_ms: 1000,
      error: "overloaded",
    });
    expect(doc.cost).toEqual({
      amount_micros: 1500,
      currency: "USD",
      is_estimate: true,
    });
    expect(doc.plan?.items).toEqual([
      { id: "p1", text: "Read", status: "in_progress" },
    ]);
    expect(doc.steps[child]).toMatchObject({
      parent_step_id: parent,
      status: "completed",
      title: "Read notes",
      detail: { lines: 3 },
      finished_at: "2026-01-01T00:00:01.000Z",
    });
    expect(opsOf(patches(sent).at(-1)).map((o) => o.path)).toEqual([
      `/steps/${child}/status`,
      `/steps/${child}/title`,
      `/steps/${child}/detail`,
      `/steps/${child}/finished_at`,
    ]);
    patcher.clearLastApiRetry();
    patcher.clearPlan();
    await patcher.flush();
    expect(fake.run(runId).metrics.last_api_retry).toBeUndefined();
    expect(fake.run(runId).plan).toBeUndefined();
    expect(() => patcher.setMetric("tokens_total", 1)).toThrow(
      /unknown metric/,
    );
    await patcher.aclose();
  });

  it("rebases once on 409 base_version_conflict with current.version", async () => {
    const api = fakeClient(fake, sent);
    const { runId, run } = await startRun(api, fake);
    sent.length = 0;
    const patcher = new RunPatcher(api, runId, run.version - 1);
    patcher.setModel("model-x");
    await patcher.flush();
    const sentPatches = patches(sent);
    expect(sentPatches.map((p) => p.body?.base_version)).toEqual([
      run.version - 1,
      run.version,
    ]);
    expect(opsOf(sentPatches[0])).toEqual(opsOf(sentPatches[1]));
    expect(
      fake
        .requests()
        .filter((r) => r.method === "PATCH")
        .map((r) => r.status),
    ).toEqual([409, 200]);
    expect(patcher.baseVersion).toBe(run.version + 1);
    expect(fake.run(runId).model).toBe("model-x");
  });

  it("gives UUIDv7 step IDs without ordinal", async () => {
    const api = fakeClient(fake, sent);
    const { runId, run } = await startRun(api, fake);
    sent.length = 0;
    const patcher = new RunPatcher(api, runId, run.version);
    const ids = [0, 1, 2].map((i) => patcher.addStep("tool_call", `step-${i}`));
    expect(ids.every((i) => UUID7.test(i))).toBe(true);
    expect(ids).toEqual([...ids].sort());
    await patcher.flush();
    for (const op of opsOf(patches(sent)[0])) {
      const value = op.value as Json;
      expect("ordinal" in value).toBe(false);
      expect(value.id).toBe(String(op.path).split("/").at(-1));
    }
    expect(Object.keys(fake.run(runId).steps).sort()).toEqual(ids);
  });

  it("drops a refused patch, so it never blocks later updates", async () => {
    const api = fakeClient(fake, sent);
    const { runId, run } = await startRun(api, fake);
    sent.length = 0;
    const patcher = new RunPatcher(api, runId, run.version);
    patcher.setCost(5, "usd");
    await expect(patcher.flush()).rejects.toBeInstanceOf(ValidationFailed);
    patcher.setStatusLine("still moving");
    await patcher.flush();
    expect(patches(sent).map((p) => opsOf(p).map((o) => o.path))).toEqual([
      ["/cost"],
      ["/status_line"],
    ]);
    expect(patcher.ended).toBe(false);
  });

  it("keeps the changes after a transient refusal and sends them again", async () => {
    const api = fakeClient(fake, sent, { retryMaxAttempts: 1 });
    const { runId, run } = await startRun(api, fake);
    sent.length = 0;
    fake.inject("patchRun", Fault.rateLimited(0));
    const patcher = new RunPatcher(api, runId, run.version);
    patcher.setModel("m1");
    await expect(patcher.flush()).rejects.toBeInstanceOf(RateLimitedError);
    await patcher.flush();
    await eventually(() => patches(sent).length === 2);
    expect(opsOf(patches(sent)[1]).map((o) => o.path)).toEqual(["/model"]);
    expect(fake.run(runId).model).toBe("m1");
  });

  it("stops on 409 conflict run_not_active and calls onEnded once", async () => {
    const api = fakeClient(fake, sent);
    const { runId, run } = await startRun(api, fake);
    let ended = 0;
    const patcher = new RunPatcher(api, runId, run.version, {
      onEnded: () => ended++,
    });
    await unwrap(
      api.POST("/runs/{run_id}/finish", {
        params: { path: { run_id: runId } },
        body: { state: "completed" },
      }),
      "finishRun",
    );
    sent.length = 0;
    patcher.setModel("m");
    await patcher.flush();
    expect(patcher.ended).toBe(true);
    expect(ended).toBe(1);
    patcher.setModel("again");
    await patcher.flush();
    expect(patches(sent)).toHaveLength(1);
    expect(fake.requests().at(-1)?.status).toBe(409);
  });

  it("logs a failed background patch by code only", async () => {
    const lines: string[] = [];
    const record = (msg: string, fields?: unknown): void => {
      lines.push(`${msg} ${JSON.stringify(fields)}`);
    };
    const logger: Logger = {
      debug: record,
      info: record,
      warn: record,
      error: record,
    };
    const api = fakeClient(fake, sent, {}, logger);
    const { runId, run, token } = await startRun(api, fake);
    const patcher = new RunPatcher(api, runId, run.version);
    patcher.setCost(5, "secret-canary");
    await fake.passTime(0);
    await eventually(() => lines.some((l) => l.includes("patch failed")));
    const line = lines.find((l) => l.includes("patch failed")) ?? "";
    expect(line).toContain("validation_failed");
    expect(line).toContain(runId);
    const all = lines.join("\n");
    expect(all).not.toContain("secret-canary");
    expect(all).not.toContain(token);
    expect(all).not.toContain(fake.credentials.apiToken.reveal());
  });

  it("stops the throttle timer without sending on aclose", async () => {
    const api = fakeClient(fake, sent);
    const { runId, run } = await startRun(api, fake);
    sent.length = 0;
    const patcher = new RunPatcher(api, runId, run.version);
    patcher.setModel("m");
    await patcher.aclose();
    await fake.passTime(1_000);
    await quiet();
    expect(patches(sent)).toEqual([]);
    // A later change schedules again.
    patcher.setModel("m2");
    await fake.passTime(0);
    await eventually(() => patches(sent).length === 1);
    await patcher.aclose();
  });

  it("sends changes made while a patch was on its way in the next round", async () => {
    const api = fakeClient(fake, sent, { runUpdateMinIntervalMs: 500 });
    const { runId, run } = await startRun(api, fake);
    sent.length = 0;
    const patcher = new RunPatcher(api, runId, run.version);
    patcher.setModel("m1");
    await fake.passTime(0);
    await eventually(() => patches(sent).length === 1);
    patcher.setStatusLine("during"); // the first patch may still be on its way
    await eventually(() => patcher.patchesSent === 1);
    await fake.passTime(500);
    await eventually(() => patches(sent).length === 2);
    await eventually(() => patcher.patchesSent === 2);
    expect(fake.run(runId).status_line).toBe("during");
    await patcher.aclose();
  });

  it("rebases after a gateway base_version_conflict by loading getRun", async () => {
    const api = fakeClient(fake, sent);
    const { runId, run } = await startRun(api, fake);
    sent.length = 0;
    const gatewaySent: [number, readonly S["JsonPatchOperation"][], string][] =
      [];
    const patcher = new RunPatcher(api, runId, run.version - 1);
    patcher.useGateway({
      sendRunUpdate: (id, base, ops, ref) => {
        expect(id).toBe(runId);
        gatewaySent.push([base, ops, ref ?? ""]);
        return Promise.resolve();
      },
    });
    patcher.setModel("model-gw");
    await patcher.flush();
    expect(patcher.baseVersion).toBe(run.version); // assumed: no success reply
    await patcher.onGatewayError(
      new GatewayOpError("run.update", "base_version_conflict"),
    );
    expect(sent.map((s) => s.method)).toEqual(["GET"]);
    expect(gatewaySent.map((g) => g[0])).toEqual([
      run.version - 1,
      run.version,
    ]);
    expect(gatewaySent[1]?.[1]).toEqual(gatewaySent[0]?.[1]);
    expect(gatewaySent.map((g) => g[2])).toEqual([`${runId}:1`, `${runId}:2`]);
    expect(patcher.baseVersion).toBe(run.version + 1);
    await patcher.onGatewayError(
      new GatewayOpError("run.update", "conflict", {
        detail: "run_not_active",
      }),
    );
    expect(patcher.ended).toBe(true);
  });
});

describe("RunPatcher with scripted answers", () => {
  it("restores a failed add before a step update made while it was on its way", async () => {
    const sent: Sent[] = [];
    let release!: () => void;
    const gate = new Promise<void>((r) => {
      release = r;
    });
    const api = createApiClient({
      baseUrl: "https://example.com/api/v1",
      token: "runs-test-token",
      logger: silentLogger,
      settings: { retryMaxAttempts: 1 },
      fetch: async (request: Request) => {
        sent.push({
          method: request.method,
          path: new URL(request.url).pathname,
          body: await bodyOf(request),
          headers: request.headers,
        });
        if (sent.length === 1) {
          await gate;
          return json(503, problem(503, "unavailable"));
        }
        return json(200, { version: 2 });
      },
    });
    const patcher = new RunPatcher(api, uuidv7(), 1);
    const step = patcher.addStep("tool_call", "search");
    const first = patcher.flush();
    await eventually(() => sent.length === 1);
    patcher.updateStep(step, { status: "completed" });
    release();
    await expect(first).rejects.toBeInstanceOf(ProblemError);
    await patcher.flush();
    expect(
      opsOf(sent[1]).map((o) => `${String(o.op)} ${String(o.path)}`),
    ).toEqual([`add /steps/${step}`, `replace /steps/${step}/status`]);
    await patcher.aclose();
  });

  it("surfaces base_version_conflict after sdk.retry_max_attempts rounds and keeps the changes", async () => {
    const sent: Sent[] = [];
    const api = scriptedClient(
      () =>
        json(
          409,
          problem(409, "base_version_conflict", {
            current: { version: 7 },
          }),
        ),
      sent,
      { retryMaxAttempts: 2 },
    );
    const patcher = new RunPatcher(api, uuidv7(), 3);
    patcher.setModel("m");
    await expect(patcher.flush()).rejects.toBeInstanceOf(BaseVersionConflict);
    expect(sent.map((s) => s.body?.base_version)).toEqual([3, 7]);
    // The conflict is transient: the next flush sends the same change again.
    await expect(patcher.flush()).rejects.toBeInstanceOf(BaseVersionConflict);
    expect(opsOf(sent.at(-1)).map((o) => o.path)).toEqual(["/model"]);
  });

  it("restores the changes after a transport error", async () => {
    const sent: Sent[] = [];
    let fail = true;
    const api = createApiClient({
      baseUrl: "https://example.com/api/v1",
      token: "runs-test-token",
      logger: silentLogger,
      settings: { retryMaxAttempts: 1 },
      fetch: async (request: Request) => {
        sent.push({
          method: request.method,
          path: new URL(request.url).pathname,
          body: await bodyOf(request),
          headers: request.headers,
        });
        if (fail) throw new TypeError("connection reset");
        return json(200, { version: 2 });
      },
    });
    const patcher = new RunPatcher(api, uuidv7(), 1);
    patcher.setModel("m1");
    await expect(patcher.flush()).rejects.toThrow(/transport error/);
    fail = false;
    await patcher.flush();
    expect(opsOf(sent[1]).map((o) => o.path)).toEqual(["/model"]);
    expect(patcher.baseVersion).toBe(2);
  });
});

describe("follow-up states", () => {
  it("only move forward", () => {
    const api = scriptedClient(() => json(200, { version: 2 }), []);
    const patcher = new RunPatcher(api, uuidv7(), 1);
    const [m1, m2] = [uuidv7(), uuidv7()];
    patcher.followupDelivered(m1);
    patcher.followupAbsorbed(m1);
    expect(() => patcher.followupDelivered(m1)).toThrow(/forward/);
    patcher.followupAbsorbed(m2);
    expect(() => patcher.followupAbsorbed(m2)).toThrow(/forward/);
    expect(() => patcher.followupDelivered(m2)).toThrow(/forward/);
  });

  it("are sent as replace of /followups/<message_id>/state", async () => {
    const sent: Sent[] = [];
    const api = scriptedClient(() => json(200, { version: 2 }), sent);
    const patcher = new RunPatcher(api, uuidv7(), 1);
    const m = uuidv7();
    patcher.followupDelivered(m);
    patcher.followupAbsorbed(m);
    await patcher.flush();
    expect(opsOf(sent[0])).toEqual([
      { op: "replace", path: `/followups/${m}/state`, value: "absorbed" },
    ]);
    await patcher.aclose();
  });
});

describe("isRunEnded", () => {
  it("is true for run_not_active and run_inactive only", () => {
    expect(
      isRunEnded(
        new ConflictError(409, "conflict", { reason: "run_not_active" }),
      ),
    ).toBe(true);
    expect(
      isRunEnded(
        new InvocationTokenInvalid(403, "invocation_token_invalid", {
          reason: "run_inactive",
        }),
      ),
    ).toBe(true);
    expect(
      isRunEnded(new ConflictError(409, "conflict", { reason: "other" })),
    ).toBe(false);
    expect(
      isRunEnded(
        new InvocationTokenInvalid(403, "invocation_token_invalid", {
          reason: "expired",
        }),
      ),
    ).toBe(false);
    expect(isRunEnded(new ProblemError(500, "internal_error"))).toBe(false);
  });
});

describe("RunHandle", () => {
  let fake: FakeServer;
  const sent: Sent[] = [];

  beforeEach(async () => {
    fake = new FakeServer();
    await fake.start();
    sent.length = 0;
  });

  afterEach(async () => {
    await fake.stop();
  });

  it("refuses patches before the start", () => {
    const api = fakeClient(fake, sent);
    const handle = new RunHandle(api, uuidv7(), uuidv7());
    expect(() => handle.patch).toThrow(/not started/);
    expect(handle.startedPatcher).toBeUndefined();
    expect(handle.version).toBe(0);
    expect(handle.tokenJti).toBeUndefined();
  });

  it("finish flushes first, sends RunFinish and runs once", async () => {
    const api = fakeClient(fake, sent);
    const { runId, threadId, run } = await startRun(api, fake);
    const handle = new RunHandle(api, runId, threadId);
    handle.started(run, "jti-1");
    expect(handle.tokenJti).toBe("jti-1");
    expect(handle.version).toBe(run.version);
    sent.length = 0;
    handle.patch.setStatusLine("almost done");
    await handle.finish("completed", { summary: "done" });
    expect(sent.map((s) => [s.method, s.path.split("/").at(-1)])).toEqual([
      ["PATCH", runId],
      ["POST", "finish"],
    ]);
    expect(sent[1]?.body).toEqual({ state: "completed", summary: "done" });
    expect(handle.finished).toBe(true);
    const view = await fake.runState(runId);
    expect(view.state).toBe("completed");
    expect(fake.run(runId).status_line).toBe("almost done");
    await handle.finish("completed"); // once only
    expect(sent).toHaveLength(2);
  });

  it("treats a repeated equal end state as success", async () => {
    const api = fakeClient(fake, sent);
    const { runId, threadId, run, token } = await startRun(api, fake);
    const first = new RunHandle(api, runId, threadId);
    first.started(run, "jti-1");
    await first.finish("failed", { reason: "runtime_error" });
    // Another handle of the same run (an adapter after a restart) ends it again: without a
    // run binding the server answers 200 for the same state, inside it 403 run_inactive.
    const again = new RunHandle(api, runId, threadId);
    again.started(run, "jti-1");
    await again.finish("failed", { reason: "runtime_error" });
    const bound = new RunHandle(api, runId, threadId);
    bound.started(run, "jti-1");
    await api.bindRun(runId, token, () =>
      bound.finish("failed", { reason: "runtime_error" }),
    );
    expect(
      fake
        .requests()
        .filter((r) => r.path.endsWith("/finish"))
        .map((r) => r.status),
    ).toEqual([200, 200, 403]);
    expect(bound.finished).toBe(true);
  });

  it("finish still sends finishRun when the last patch hits a transport error", async () => {
    const sent: Sent[] = [];
    const api = createApiClient({
      baseUrl: "https://example.com/api/v1",
      token: "runs-test-token",
      logger: silentLogger,
      settings: { retryMaxAttempts: 1 },
      fetch: async (request: Request) => {
        sent.push({
          method: request.method,
          path: new URL(request.url).pathname,
          body: await bodyOf(request),
          headers: request.headers,
        });
        if (request.method === "PATCH") throw new TypeError("connection reset");
        return json(200, { state: "completed" });
      },
    });
    const handle = new RunHandle(api, uuidv7(), uuidv7());
    handle.started(
      { ...(await startRun(fakeClient(fake, []), fake)).run },
      "jti-1",
    );
    handle.patch.setModel("m");
    await handle.finish("completed");
    expect(sent.map((s) => s.method)).toEqual(["PATCH", "POST"]);
    expect(sent[1]?.path.endsWith("/finish")).toBe(true);
  });

  it("surfaces another refusal of finishRun", async () => {
    const api = fakeClient(fake, sent);
    const { runId, threadId, run } = await startRun(api, fake);
    const handle = new RunHandle(api, runId, threadId);
    handle.started(run, "jti-1");
    await expect(handle.finish("cancelled")).rejects.toBeInstanceOf(
      ValidationFailed,
    );
  });

  it("finish drops a flush that finds the run ended", async () => {
    const api = fakeClient(fake, sent);
    const { runId, threadId, run, token } = await startRun(api, fake);
    await unwrap(
      api.POST("/runs/{run_id}/finish", {
        params: { path: { run_id: runId } },
        body: { state: "completed" },
      }),
      "finishRun",
    );
    const handle = new RunHandle(api, runId, threadId);
    handle.started(run, "jti-1");
    handle.patch.setModel("late");
    await api.bindRun(runId, token, () => handle.finish("completed"));
    expect(handle.finished).toBe(true);
    expect(handle.cancelled.aborted).toBe(true);
  });

  it("stops the patcher and marks the handle cancelled on run_inactive", async () => {
    const api = fakeClient(fake, sent);
    const { runId, threadId, run, token } = await startRun(api, fake);
    const handle = new RunHandle(api, runId, threadId);
    const calls: string[] = [];
    handle.onCancel(() => calls.push("cancel"));
    handle.started(run, "jti-1");
    await unwrap(
      api.POST("/runs/{run_id}/finish", {
        params: { path: { run_id: runId } },
        body: { state: "completed" },
      }),
      "finishRun",
    );
    sent.length = 0;
    await api.bindRun(runId, token, async () => {
      handle.patch.setModel("m");
      await handle.patch.flush();
    });
    expect(handle.patch.ended).toBe(true);
    expect(handle.cancelled.aborted).toBe(true);
    expect(calls).toEqual(["cancel"]);
    expect(fake.requests().at(-1)?.status).toBe(403);
    handle.patch.setModel("again");
    await handle.patch.flush();
    expect(patches(sent)).toHaveLength(1);
  });

  it("calls the cancel callbacks once and at once after the fact", () => {
    const api = fakeClient(fake, sent);
    const handle = new RunHandle(api, uuidv7(), uuidv7());
    const calls: string[] = [];
    const off = handle.onCancel(() => calls.push("a"));
    handle.onCancel(() => calls.push("b"));
    off();
    handle.setCancelled();
    handle.setCancelled();
    handle.onCancel(() => calls.push("late"));
    expect(calls).toEqual(["b", "late"]);
    expect(handle.cancelled.aborted).toBe(true);
  });

  it("awaitInput flushes first, waits and calls onWaiting", async () => {
    const api = fakeClient(fake, sent);
    const { runId, threadId, run } = await startRun(api, fake);
    let waited = 0;
    const handle = new RunHandle(api, runId, threadId, {
      onWaiting: () => {
        waited++;
        return Promise.resolve();
      },
    });
    handle.started(run, "jti-1");
    sent.length = 0;
    handle.patch.setModel("m-wait");
    await handle.awaitInput("Waiting for Sam");
    expect(sent.map((s) => [s.method, s.path.split("/").at(-1)])).toEqual([
      ["PATCH", runId],
      ["POST", "await-input"],
    ]);
    expect(sent[1]?.body).toEqual({ status_line: "Waiting for Sam" });
    expect(handle.waiting).toBe(true);
    expect(waited).toBe(1);
    expect((await fake.runState(runId)).state).toBe("waiting_input");
    // A resume answer gives the new base and clears the waiting mark.
    handle.started({ ...run, version: 42 }, "jti-2");
    expect(handle.waiting).toBe(false);
    expect(handle.version).toBe(42);
    expect(handle.tokenJti).toBe("jti-2");
    await handle.patch.aclose();
  });

  it("loadContext calls getRunContext", async () => {
    const api = fakeClient(fake, sent);
    const { runId, threadId } = await startRun(api, fake);
    const handle = new RunHandle(api, runId, threadId);
    const ctx = await handle.loadContext();
    expect(ctx.run_id).toBe(runId);
    expect(ctx.trigger_message.trust).toBe("trigger");
    expect(sent.at(-1)?.path).toBe(`/api/v1/runs/${runId}/context`);
  });

  it("send replies through the outbox with run_id set", async () => {
    const api = fakeClient(fake, sent);
    const calls: unknown[] = [];
    const runId = uuidv7();
    const threadId = uuidv7();
    const handle = new RunHandle(api, runId, threadId, {
      outbox: {
        sendMessage: (thread, content, options) => {
          calls.push([thread, content, options]);
          return Promise.resolve("entry");
        },
      },
    });
    expect(await handle.send("Done", { clientMsgId: "x" })).toBe("entry");
    expect(calls).toEqual([[threadId, "Done", { clientMsgId: "x", runId }]]);
    await expect(new RunHandle(api, runId, threadId).send("x")).rejects.toThrow(
      /outbox/,
    );
  });
});

describe("RunHandle.reportSession", () => {
  it("posts reportAgentSession with If-Match and an Idempotency-Key", async () => {
    const sent: Sent[] = [];
    const threadId = uuidv7();
    const session = {
      id: uuidv7(),
      thread_id: threadId,
      agent_id: uuidv7(),
      external_session_ref: "sess-7f3a",
      is_current: true,
      started: true,
      cost: { amount_micros: 0, currency: "USD", is_estimate: true },
      created_at: "2026-10-21T09:00:00.000Z",
      version: 4,
    };
    const api = scriptedClient(() => json(200, session, { ETag: '"4"' }), sent);
    const handle = new RunHandle(api, uuidv7(), threadId);
    const answer = await handle.reportSession(
      { external_session_ref: "sess-7f3a", started: true },
      { ifMatch: 3 },
    );
    expect(answer).toEqual(session);
    expect(sent[0]?.method).toBe("POST");
    expect(sent[0]?.path).toBe(
      `/api/v1/threads/${threadId}/agent-sessions/report`,
    );
    expect(sent[0]?.headers.get("If-Match")).toBe('"3"');
    expect(sent[0]?.headers.get("Idempotency-Key")).toMatch(UUID7);
    expect(sent[0]?.body).toEqual({
      external_session_ref: "sess-7f3a",
      started: true,
    });

    await handle.reportSession({ external_session_ref: "sess-7f3a" });
    expect(sent[1]?.headers.get("If-Match")).toBeNull();
  });

  it("surfaces 412 version_conflict", async () => {
    const api = scriptedClient(
      () => json(412, problem(412, "version_conflict")),
      [],
    );
    const handle = new RunHandle(api, uuidv7(), uuidv7());
    await expect(
      handle.reportSession({ external_session_ref: "s" }, { ifMatch: 1 }),
    ).rejects.toMatchObject({ status: 412, code: "version_conflict" });
  });
});
