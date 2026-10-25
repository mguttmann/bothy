// SPDX-License-Identifier: Apache-2.0
//
// The heartbeat building block for `agent_heartbeat.v1` (docs/spec/09-agents.md 4.8); port of
// `sdk/python/tests/test_heartbeat.py`. The server's decoder (M2-20) refuses an unknown key and
// a value outside its form; the SDK refuses the same before anything is sent, and its error
// names the field, never the value. `HeartbeatSender` keeps the `interval_ms` of the answer.

import { readFileSync } from "node:fs";

import { afterEach, beforeEach, describe, expect, it } from "vitest";

import { createApiClient } from "../src/client.js";
import { DEFAULTS } from "../src/config.js";
import type { components } from "../src/generated/openapi.js";
import {
  CAPABILITY_KEYS,
  CODE_FORM,
  DETAILS_KEYS,
  HEARTBEAT_MEMBERS,
  HeartbeatSender,
  NAME_FORM,
  VERSION_FORM,
  buildHeartbeat,
  restHeartbeatSend,
  type BuildHeartbeatOptions,
} from "../src/heartbeat/index.js";
import { uuidv7 } from "../src/ids.js";
import { silentLogger } from "../src/logger.js";
import { FakeClock, settle } from "../src/testing/clock.js";
import { FakeServer } from "../src/testing/fakeserver.js";
import { eventually as patiently } from "../src/testing/patience.js";

type Heartbeat = components["schemas"]["Heartbeat"];
type Json = Record<string, unknown>;

const RUN_A = uuidv7();
const RUN_B = uuidv7();
const CHANNEL = uuidv7();

const FULL_DETAILS: Json = {
  capabilities: {
    followups: "queue",
    components: true,
    voice_input: false,
    title: true,
    files_out: false,
  },
  active_runs: [RUN_A, RUN_B.toUpperCase()],
  degraded_reasons: ["runtime_unreachable", "billing_evidence_unknown"],
  adapter: { name: "bothy-sdk-typescript", version: "0.1.0+local" },
  runtime: { name: "example-runtime" },
  protocol_version: 1,
};

/** FULL_DETAILS with `value` at the dotted `path` below details. */
function detailsWith(path: string, value: unknown): Json {
  const details: Json = {};
  for (const [k, v] of Object.entries(FULL_DETAILS))
    details[k] =
      typeof v === "object" && v !== null && !Array.isArray(v) ? { ...v } : v;
  const keys = path.split(".");
  const last = keys.pop() ?? "";
  let target = details;
  for (const key of keys) target = target[key] as Json;
  target[last] = value;
  return details;
}

/** `call` throws a RangeError whose message names `field` and never shows `value`. */
function raisesNaming(
  field: string,
  call: () => unknown,
  value?: unknown,
): void {
  let caught: unknown;
  try {
    call();
  } catch (error) {
    caught = error;
  }
  expect(caught).toBeInstanceOf(RangeError);
  const message = (caught as Error).message;
  expect(message).toContain(field);
  if (typeof value === "string" && value) expect(message).not.toContain(value);
}

describe("buildHeartbeat", () => {
  it("builds the closed object and refuses every other key", () => {
    const beat: Heartbeat = buildHeartbeat("degraded", {
      runtimeVersion: "2.3.1",
      reportedBillingMode: "organization",
      billingModesSupported: ["organization", "personal"],
      billingEvidence: {
        provider: "anthropic",
        key_type: "api_key",
        method: "env",
      },
      reportedToolClass: "scoped",
      details: FULL_DETAILS,
      projectReadiness: [{ channel_id: CHANNEL, ready: true }],
    });
    const details = beat.details as Json;
    expect(details.active_runs).toEqual([RUN_A, RUN_B]);
    expect(details.capabilities).toEqual(FULL_DETAILS.capabilities);
    expect(beat.billing_evidence).toEqual({
      provider: "anthropic",
      key_type: "api_key",
      method: "env",
    });
    expect(beat.state).toBe("degraded");
    expect(beat.project_readiness).toEqual([
      { channel_id: CHANNEL, ready: true },
    ]);
    expect(JSON.parse(JSON.stringify(beat))).toEqual(beat);
    expect(buildHeartbeat()).toEqual({ state: "online" });
    expect(
      buildHeartbeat("online", { billingEvidence: { key_type: "unknown" } }),
    ).toEqual({ state: "online", billing_evidence: { key_type: "unknown" } });

    for (const path of [
      "rate_limit",
      "billing_mode",
      "capabilities.streaming",
      "adapter.path",
      "runtime.version",
    ])
      raisesNaming(`details.${path}`, () =>
        buildHeartbeat("online", { details: detailsWith(path, "x") }),
      );
    raisesNaming("billing_evidence.key_value", () =>
      buildHeartbeat("online", {
        billingEvidence: {
          key_type: "api_key",
          key_value: "secret",
        } as BuildHeartbeatOptions["billingEvidence"],
      }),
    );
    // An unknown key that is not even a name is not echoed either.
    raisesNaming(
      "details",
      () => buildHeartbeat("online", { details: { "/home/alex/.token": "x" } }),
      "/home/alex/.token",
    );
    raisesNaming("options.quota", () =>
      buildHeartbeat("online", {
        quota: {},
      } as unknown as BuildHeartbeatOptions),
    );
    expect(new Set(DETAILS_KEYS)).toEqual(
      new Set([
        "capabilities",
        "active_runs",
        "degraded_reasons",
        "adapter",
        "runtime",
        "protocol_version",
      ]),
    );
    expect(new Set(CAPABILITY_KEYS)).toEqual(
      new Set(["followups", "components", "voice_input", "title", "files_out"]),
    );

    raisesNaming("details.adapter.name", () =>
      buildHeartbeat("online", { details: { adapter: { version: "1.0" } } }),
    );
    raisesNaming("details.runtime.name", () =>
      buildHeartbeat("online", { details: { runtime: {} } }),
    );
    raisesNaming("details.capabilities.followups", () =>
      buildHeartbeat("online", {
        details: { capabilities: { followups: "interrupt" } },
      }),
    );
    raisesNaming("details.capabilities.components", () =>
      buildHeartbeat("online", {
        details: { capabilities: { components: 1 } },
      }),
    );
    for (const bad of [0, -1, true, "1", 1.5, 2 ** 31, Number.NaN])
      raisesNaming("details.protocol_version", () =>
        buildHeartbeat("online", { details: { protocol_version: bad } }),
      );
    raisesNaming("details.active_runs", () =>
      buildHeartbeat("online", {
        details: { active_runs: [RUN_A, RUN_A.toUpperCase()] },
      }),
    );
    raisesNaming(
      "details.active_runs",
      () =>
        buildHeartbeat("online", { details: { active_runs: ["not-a-uuid"] } }),
      "not-a-uuid",
    );
    raisesNaming("details.active_runs", () =>
      buildHeartbeat("online", { details: { active_runs: "abc" } }),
    );
    raisesNaming("details.degraded_reasons", () =>
      buildHeartbeat("online", {
        details: {
          degraded_reasons: ["busy_mode_invalid", "busy_mode_invalid"],
        },
      }),
    );
    raisesNaming("billing_modes_supported", () =>
      buildHeartbeat("online", {
        billingModesSupported: ["personal", "personal"],
      }),
    );
    raisesNaming("project_readiness", () =>
      buildHeartbeat("online", {
        projectReadiness: [
          { channel_id: CHANNEL, ready: true },
          { channel_id: CHANNEL, ready: false },
        ],
      }),
    );
    raisesNaming("project_readiness", () =>
      buildHeartbeat("online", {
        projectReadiness: [
          { channel_id: CHANNEL, ready: "yes" as unknown as boolean },
        ],
      }),
    );
    raisesNaming("state", () => buildHeartbeat("offline" as "online"));
    raisesNaming("reported_billing_mode", () =>
      buildHeartbeat("online", {
        reportedBillingMode: "corporate_example" as "personal",
      }),
    );
    raisesNaming("reported_tool_class", () =>
      buildHeartbeat("online", { reportedToolClass: "root" as "chat" }),
    );
    raisesNaming("billing_evidence.key_type", () =>
      buildHeartbeat("online", {
        billingEvidence: {
          provider: "anthropic",
        } as BuildHeartbeatOptions["billingEvidence"],
      }),
    );
    raisesNaming("billing_evidence.key_type", () =>
      buildHeartbeat("online", {
        billingEvidence: { key_type: "prefix_example" as "none" },
      }),
    );
  });

  // Values a heartbeat must never carry: paths, URLs, secrets, content, whitespace, control and
  // bidi characters, and a trailing newline (the `$` pitfall of some regex engines).
  const BAD_VALUES = [
    "/opt/agents/bin/runtime",
    "..\\runtime",
    "C:\\agents\\runtime",
    "https://example.com/runtime",
    "sk-ant-api03-AbCd/Ef+Gh==",
    "Bearer abc.def",
    "token=abcdef",
    "two words",
    "runtime\n",
    "run\ttime",
    "r\u00e9ntime",
    "runtime\u202e",
    "",
    "-runtime",
  ];

  const FIELDS: Record<string, (v: unknown) => unknown> = {
    runtime_version: (v) =>
      buildHeartbeat("online", { runtimeVersion: v as string }),
    "billing_evidence.provider": (v) =>
      buildHeartbeat("online", {
        billingEvidence: { key_type: "api_key", provider: v as string },
      }),
    "billing_evidence.method": (v) =>
      buildHeartbeat("online", {
        billingEvidence: { key_type: "api_key", method: v as string },
      }),
    "details.degraded_reasons": (v) =>
      buildHeartbeat("online", { details: { degraded_reasons: [v] } }),
    "details.adapter.name": (v) =>
      buildHeartbeat("online", { details: { adapter: { name: v } } }),
    "details.adapter.version": (v) =>
      buildHeartbeat("online", {
        details: { adapter: { name: "a", version: v } },
      }),
    "details.runtime.name": (v) =>
      buildHeartbeat("online", { details: { runtime: { name: v } } }),
  };

  it.each(Object.keys(FIELDS).flatMap((f) => BAD_VALUES.map((v) => [f, v])))(
    "%s refuses %j",
    (field, value) => {
      raisesNaming(field ?? "", () => FIELDS[field ?? ""]?.(value), value);
    },
  );

  it.each(Object.keys(FIELDS))("%s refuses a non-string", (field) => {
    raisesNaming(field, () => FIELDS[field]?.(42));
  });

  it("holds the forms and members of docs/api/openapi.yaml", () => {
    const spec = readFileSync(
      new URL("../../../docs/api/openapi.yaml", import.meta.url),
      "utf8",
    );
    const schema = (name: string): string => {
      const found = new RegExp(
        `^    ${name}:\\n([\\s\\S]*?)(?=^    \\S)`,
        "m",
      ).exec(spec);
      if (found === null) throw new Error(`schema ${name} not found`);
      return found[1] ?? "";
    };
    const beat = schema("Heartbeat");
    const evidence = schema("BillingEvidence");
    const pattern = (text: string, member: string): string | undefined =>
      new RegExp(
        `^        ${member}:\\n(?:          .*\\n)*?          pattern: '([^']+)'`,
        "m",
      ).exec(text)?.[1];
    expect(pattern(beat, "runtime_version")).toBe(VERSION_FORM);
    expect(pattern(evidence, "provider")).toBe(CODE_FORM);
    expect(pattern(evidence, "method")).toBe(CODE_FORM);
    const described =
      /^ {8}details:\n[\s\S]*?\n {12}(Further fields[^\n]*)/m.exec(beat)?.[1] ??
      "";
    const forms: Record<string, string | undefined> = Object.fromEntries(
      [
        ...described.matchAll(/(Names|versions|codes)(?: match)? `([^`]+)`/g),
      ].map((m): [string, string] => [m[1] ?? "", m[2] ?? ""]),
    );
    expect(forms).toEqual({
      Names: NAME_FORM,
      versions: VERSION_FORM,
      codes: CODE_FORM,
    });
    const listed = /Keys: (.*?)\. Names/.exec(described)?.[1] ?? "";
    const top = [...listed.matchAll(/(?:^|\), )`([a-z_]+)` \(/g)].map(
      (m) => m[1],
    );
    expect(new Set(top)).toEqual(new Set(DETAILS_KEYS));
    const members = [...beat.matchAll(/^ {8}([a-z_]+):$/gm)].map((m) => m[1]);
    expect(new Set(members)).toEqual(new Set(HEARTBEAT_MEMBERS));
  });
});

describe("buildHeartbeat rateLimit", () => {
  it("keeps state and resets_at and refuses everything else", () => {
    expect(buildHeartbeat("online", { rateLimit: { state: "ok" } })).toEqual({
      state: "online",
      rate_limit: { state: "ok" },
    });
    expect(
      buildHeartbeat("online", {
        rateLimit: {
          state: "near_limit",
          resets_at: "2026-10-10T11:30:00+02:00",
        },
      }).rate_limit,
    ).toEqual({ state: "near_limit", resets_at: "2026-10-10T11:30:00+02:00" });
    const bad: [unknown, string][] = [
      [{ state: "exhausted" }, "rate_limit.state"],
      [{}, "rate_limit.state"],
      [{ state: "ok", quota: 5 }, "rate_limit.quota"],
      [{ state: "ok", resets_at: "soon" }, "rate_limit.resets_at"],
      [
        { state: "ok", resets_at: "2026-10-10T09:30:00" },
        "rate_limit.resets_at",
      ],
      ["ok", "rate_limit"],
    ];
    for (const [value, field] of bad)
      expect(() =>
        buildHeartbeat("online", {
          rateLimit: value as Heartbeat["rate_limit"],
        }),
      ).toThrow(new RegExp(`^${field.replace(".", "\\.")}:`));
  });
});

describe("HeartbeatSender", () => {
  let fake: FakeServer;

  beforeEach(async () => {
    fake = new FakeServer({ timings: { "sync.heartbeat_interval": 7_000 } });
    await fake.start();
  });

  afterEach(async () => {
    await fake.stop();
  });

  it("sends rate_limit and nothing else new (M6-14)", async () => {
    const api = createApiClient({
      baseUrl: fake.baseUrl,
      token: fake.credentials.apiToken,
      dpopKey: fake.credentials.dpop,
      clock: fake.clock,
      settings: { ...fake.sdkSettings, retryMaxAttempts: 1 },
    });
    const details = { protocol_version: 1 };
    const send = restHeartbeatSend(api);
    await send(buildHeartbeat("online", { details }));
    await send(
      buildHeartbeat("online", {
        details,
        rateLimit: { state: "limited", resets_at: "2026-10-10T09:30:00.000Z" },
      }),
    );
    const [plain, limited] = fake.heartbeats;
    expect(limited?.rate_limit).toEqual({
      state: "limited",
      resets_at: "2026-10-10T09:30:00.000Z",
    });
    expect(
      Object.keys(limited ?? {}).filter((k) => !(k in (plain ?? {}))),
    ).toEqual(["rate_limit"]);
  });

  it("sends through sendHeartbeat and keeps the interval of the answer", async () => {
    const api = createApiClient({
      baseUrl: fake.baseUrl,
      token: fake.credentials.apiToken,
      dpopKey: fake.credentials.dpop,
      clock: fake.clock,
      settings: { ...fake.sdkSettings, retryMaxAttempts: 1 },
    });
    let n = 0;
    const sender = new HeartbeatSender({
      build: () =>
        buildHeartbeat("online", { details: { protocol_version: ++n } }),
      send: restHeartbeatSend(api),
      clock: fake.clock,
      logger: silentLogger,
    });
    expect(sender.intervalMs).toBe(DEFAULTS.pollIntervalMs);
    sender.start();
    // The answer sets the interval before the loop sleeps on it.
    await waitFor(
      () => sender.intervalMs === 7_000 && fake.clock.sleepers === 1,
    );
    expect(fake.heartbeats).toHaveLength(1);
    await fake.passTime(6_999);
    expect(fake.heartbeats).toHaveLength(1);
    await fake.passTime(1);
    await waitFor(() => fake.heartbeats.length === 2);
    expect(
      fake.heartbeats.map((b) => (b.details as Json).protocol_version),
    ).toEqual([1, 2]);
    await sender.stop();
    await fake.passTime(20_000);
    expect(fake.heartbeats).toHaveLength(2);
  });
});

describe("HeartbeatSender over a gateway callback", () => {
  it("keeps the interval, retries a failure in the next interval and skips an empty build", async () => {
    const clock = new FakeClock();
    const sent: Heartbeat[] = [];
    const answers: (number | undefined | Error)[] = [
      3_000,
      new Error("closed"),
      undefined,
    ];
    let skip = false;
    const sender = new HeartbeatSender({
      build: () => (skip ? undefined : buildHeartbeat()),
      send: (beat) => {
        sent.push(beat);
        const answer = answers.shift();
        return answer instanceof Error
          ? Promise.reject(answer)
          : Promise.resolve(answer);
      },
      clock,
      settings: { pollIntervalMs: 1_000 },
      logger: silentLogger,
    });
    sender.start();
    sender.start(); // a second start is a no-op
    await settle();
    expect(sent).toHaveLength(1);
    expect(sender.intervalMs).toBe(3_000);
    await clock.advance(3_000);
    expect(sent).toHaveLength(2); // failed: the interval stays
    expect(sender.intervalMs).toBe(3_000);
    await clock.advance(3_000);
    expect(sent).toHaveLength(3); // no interval in the answer: it stays
    expect(sender.intervalMs).toBe(3_000);
    skip = true;
    await clock.advance(3_000);
    expect(sent).toHaveLength(3);
    await sender.stop();
    expect(clock.sleepers).toBe(0);
  });

  it("logs a build that throws and keeps the loop running", async () => {
    const clock = new FakeClock();
    const sent: Heartbeat[] = [];
    const logged: string[] = [];
    let fail = true;
    const sender = new HeartbeatSender({
      build: () =>
        fail
          ? buildHeartbeat("degraded", {
              details: { degraded_reasons: ["Rate Limited"] },
            })
          : buildHeartbeat(),
      send: (beat) => {
        sent.push(beat);
        return Promise.resolve(undefined);
      },
      clock,
      settings: { pollIntervalMs: 1_000 },
      logger: {
        ...silentLogger,
        info: (msg, fields) => logged.push(`${msg} ${JSON.stringify(fields)}`),
      },
    });
    sender.start();
    await settle();
    expect(sent).toEqual([]);
    expect(logged).toEqual([
      'heartbeat failed; retrying in the next interval {"error":"RangeError"}',
    ]);
    fail = false;
    await clock.advance(1_000);
    expect(sent).toEqual([{ state: "online" }]);
    await sender.stop();
  });

  it("sends one heartbeat on demand", async () => {
    const sent: Heartbeat[] = [];
    const sender = new HeartbeatSender({
      build: () => buildHeartbeat("degraded"),
      send: (beat) => {
        sent.push(beat);
        return Promise.resolve(2_500);
      },
      clock: new FakeClock(),
    });
    expect(await sender.beatOnce()).toBe(true);
    expect(sent).toEqual([{ state: "degraded" }]);
    expect(sender.intervalMs).toBe(2_500);
  });
});

async function waitFor(predicate: () => boolean): Promise<void> {
  await patiently(predicate, "the condition of the test");
}
