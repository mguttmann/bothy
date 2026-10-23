// SPDX-License-Identifier: Apache-2.0
//
// AZ-P6 for the SDK (docs/spec/07-permissions.md 11.2; 09 4.5; T-TK-05): an adapter built on the SDK
// never starts the runtime with a missing token, a wrong signature, a foreign `aud`, an expired
// token, a repeated `jti`, a different `billing_mode` or a higher tool class. The events come
// from the FakeServer; a tampering source in front of the adapter removes or replaces the
// delivered token the way an attacker on the path would (port of `azp6_adapter_never_starts`
// and the AG-08 adapter side of bothy_sdk/testing/scenarios.py). Tokens are compared by
// fingerprint inside the test only and never printed (SEC-LOG-2).

import { createHash } from "node:crypto";

import { afterEach, beforeEach, describe, expect, it } from "vitest";

import type { Secret } from "../src/auth/secret.js";
import {
  Envelope,
  SourceReady,
  type Delivery,
  type EventSource,
} from "../src/gateway/index.js";
import type { components } from "../src/generated/openapi.js";
import { storedClaims } from "../src/invocation/index.js";
import { silentLogger } from "../src/logger.js";
import {
  Adapter,
  AgentHandler,
  type ActualBilling,
  type Invocation,
  type RunHandle,
  type StartOptions,
} from "../src/runs/index.js";
import { FakeServer } from "../src/testing/fakeserver.js";
import {
  FORGED_VARIANTS,
  type ForgedVariant,
} from "../src/testing/scenario-server.js";
import {
  eventually as patiently,
  PATIENCE_MS,
  withinPatience,
} from "../src/testing/patience.js";

type S = components["schemas"];

const STEP_MS = 50;
// The runner limit of the tests that start an adapter: the patience, never a few seconds.
const TEST_TIMEOUT = PATIENCE_MS;
const KEEP = Symbol("keep");
const REMOVE = Symbol("remove");

type Rewrite = (env: Envelope) => Promise<Secret | typeof KEEP | typeof REMOVE>;

function fingerprint(token: Secret | string): string {
  const text = typeof token === "string" ? token : token.reveal();
  return createHash("sha256").update(text).digest("hex").slice(0, 16);
}

async function until(
  fake: FakeServer,
  predicate: () => unknown,
): Promise<void> {
  await patiently(predicate, "the condition of the test", {
    step: () => fake.passTime(STEP_MS),
  });
}

/** The runtime spy: `execute` is the runtime; every call is logged in order. */
class SpyHandler extends AgentHandler {
  readonly log: string[];
  readonly invocations: Invocation[] = [];
  readonly jtis: string[] = [];
  billing: ((inv: Invocation) => string) | undefined;
  toolClass: S["ToolClass"] = "chat";
  release: Promise<void> | undefined;

  constructor(log: string[]) {
    super();
    this.log = log;
  }

  count(name: string): number {
    return this.log.filter((e) => e === name).length;
  }

  override actualBilling(inv: Invocation): ActualBilling {
    return { mode: this.billing?.(inv) ?? inv.claims.billingMode };
  }

  override actualToolClass(): S["ToolClass"] {
    return this.toolClass;
  }

  override async prepare(inv: Invocation): Promise<StartOptions> {
    this.log.push("prepare");
    this.invocations.push(inv);
    if (this.release !== undefined) await this.release;
    return { model: "model-test" };
  }

  override execute(_inv: Invocation, run: RunHandle): Promise<void> {
    this.log.push("execute");
    if (run.tokenJti !== undefined) this.jtis.push(run.tokenJti);
    return Promise.resolve();
  }
}

/** Records every delivery and lets `rewrite` replace or remove a delivered token. */
class TamperingSource implements EventSource {
  readonly deliveries: Delivery[] = [];

  constructor(
    readonly inner: EventSource,
    private readonly rewrite?: Rewrite,
  ) {}

  get closeReason(): EventSource["closeReason"] {
    return this.inner.closeReason;
  }

  async *[Symbol.asyncIterator](): AsyncGenerator<Delivery, void, undefined> {
    for await (const delivery of this.inner) {
      let out = delivery;
      if (
        this.rewrite !== undefined &&
        delivery instanceof Envelope &&
        delivery.token !== undefined
      ) {
        const result = await this.rewrite(delivery);
        if (result === REMOVE)
          out = new Envelope({ ...delivery, token: undefined });
        else if (result !== KEEP)
          out = new Envelope({ ...delivery, token: result });
      }
      this.deliveries.push(out);
      yield out;
    }
  }

  envelopes(type: string): Envelope[] {
    return this.deliveries.filter(
      (d): d is Envelope => d instanceof Envelope && d.type === type,
    );
  }

  settle(seq: number): Promise<void> {
    return this.inner.settle(seq);
  }

  advance(seq: number): Promise<void> {
    return this.inner.advance(seq);
  }

  ack(seq: number): Promise<void> {
    return this.inner.ack(seq);
  }

  answerAutocomplete(
    requestId: string,
    choices: readonly S["CommandChoice"][],
  ): Promise<void> {
    return this.inner.answerAutocomplete(requestId, choices);
  }

  close(): Promise<void> {
    return this.inner.close();
  }
}

interface Seen {
  readonly method: string;
  readonly path: string;
  readonly status: number;
}

let fake: FakeServer;

beforeEach(async () => {
  fake = new FakeServer();
  await fake.start();
});

afterEach(async () => {
  await fake.stop();
});

/** Runs an adapter for `handler` with a tampering source until `body` returns. */
async function running(
  handler: SpyHandler,
  consumer: string,
  body: (source: TamperingSource, seen: Seen[]) => Promise<void>,
  rewrite?: Rewrite,
): Promise<Seen[]> {
  const seen: Seen[] = [];
  const adapter = new Adapter(
    {
      baseUrl: fake.baseUrl,
      instanceId: fake.instanceId,
      agentId: fake.agentId,
      credentials: fake.credentials,
      consumer,
      settings: fake.sdkSettings,
    },
    handler,
    {
      clock: fake.clock,
      logger: silentLogger,
      fetch: async (request) => {
        const response = await globalThis.fetch(request);
        const path = new URL(request.url).pathname;
        seen.push({ method: request.method, path, status: response.status });
        if (request.method === "POST")
          handler.log.push(
            `${path.split("/").at(-1) ?? ""}:${response.status}`,
          );
        return response;
      },
    },
  );
  const source = new TamperingSource(await adapter.makeSource(), rewrite);
  adapter.source = source;
  const done = adapter.run();
  done.catch(() => undefined);
  try {
    await until(fake, () =>
      source.deliveries.some((d) => d instanceof SourceReady),
    );
    await body(source, seen);
  } finally {
    await adapter.stop();
    await withinPatience(
      done.catch(() => undefined),
      "the end of the adapter loop",
    );
  }
  return seen;
}

function posts(seen: Seen[], suffix: string): Seen[] {
  return seen.filter((s) => s.method === "POST" && s.path.endsWith(suffix));
}

/** Waits until the run failed and checks that nothing started; returns the reason. */
async function refusedReason(runId: string): Promise<string | undefined> {
  await until(
    fake,
    async () => (await fake.runState(runId)).state === "failed",
  );
  const view = await fake.runState(runId);
  expect(view.starts).toBe(0);
  return view.stateReason;
}

/** A rewrite that waits per run for the token the test decides on. */
function slots(): {
  rewrite: Rewrite;
  set(runId: string, value: Secret | typeof REMOVE): void;
} {
  const waiting = new Map<string, (value: Secret | typeof REMOVE) => void>();
  const ready = new Map<string, Promise<Secret | typeof REMOVE>>();
  const slot = (runId: string): Promise<Secret | typeof REMOVE> => {
    let found = ready.get(runId);
    if (found === undefined) {
      found = new Promise((resolve) => waiting.set(runId, resolve));
      ready.set(runId, found);
    }
    return found;
  };
  return {
    rewrite: async (env) => {
      const runId = (env.raw.payload as { run_id?: unknown } | undefined)
        ?.run_id;
      if (env.type !== "agent.invocation" || typeof runId !== "string")
        return KEEP;
      return slot(runId);
    },
    set(runId, value) {
      void slot(runId);
      waiting.get(runId)?.(value);
    },
  };
}

describe("AZ-P6: the runtime never starts on a refused token", () => {
  const variants: ("missing" | ForgedVariant)[] = [
    "missing",
    ...FORGED_VARIANTS,
  ];
  it.each(variants)(
    "token %s: finishRun failed invalid_invocation_token, acknowledged, no runtime",
    async (variant: "missing" | ForgedVariant) => {
      const log: string[] = [];
      const handler = new SpyHandler(log);
      const tamper = slots();
      const consumer = `azp6-${variant.replaceAll("_", "-")}`;
      const seen = await running(
        handler,
        consumer,
        async (source) => {
          const { runId } = await fake.trigger(`AZ-P6 ${variant} @Lotse`);
          tamper.set(
            runId,
            variant === "missing"
              ? REMOVE
              : await fake.forgeToken(runId, variant),
          );
          expect(await refusedReason(runId)).toBe("invalid_invocation_token");
          const seq = source.envelopes("agent.invocation")[0]?.seq ?? 0;
          await until(fake, () => fake.consumerState(consumer).ackedSeq >= seq);
        },
        tamper.rewrite,
      );
      expect(posts(seen, "/finish").map((s) => s.status)).toEqual([200]);
      expect(posts(seen, "/start")).toEqual([]);
      expect(handler.count("prepare")).toBe(0);
      expect(handler.count("execute")).toBe(0);
    },
    TEST_TIMEOUT,
  );

  it.each([
    ["a different billing_mode", "billing_mode_mismatch"],
    ["billing_mode unknown", "billing_mode_mismatch"],
    ["a higher tool class", "tool_class_mismatch"],
  ])(
    "%s: finishRun failed %s, no runtime",
    async (what, reason) => {
      const log: string[] = [];
      const handler = new SpyHandler(log);
      if (what === "a different billing_mode")
        handler.billing = (inv) =>
          inv.claims.billingMode === "organization"
            ? "personal"
            : "organization";
      else if (what === "billing_mode unknown")
        handler.billing = () => "unknown";
      else handler.toolClass = "host"; // the FakeServer's tokens say scoped
      const seen = await running(
        handler,
        `azp6-${reason.replaceAll("_", "-")}`,
        async () => {
          const { runId } = await fake.trigger(`AZ-P6 ${what} @Lotse`);
          expect(await refusedReason(runId)).toBe(reason);
        },
      );
      expect(posts(seen, "/finish").map((s) => s.status)).toEqual([200]);
      expect(posts(seen, "/start")).toEqual([]);
      expect(handler.count("execute")).toBe(0);
    },
    TEST_TIMEOUT,
  );

  it(
    "an expired token: renewed once, the runtime starts only with the renewed one",
    async () => {
      const log: string[] = [];
      const handler = new SpyHandler(log);
      let open: () => void = () => undefined;
      handler.release = new Promise((resolve) => {
        open = resolve;
      });
      let expiredJti = "";
      let runId = "";
      const seen = await running(handler, "azp6-expired", async () => {
        runId = (await fake.trigger("AZ-P6 expired @Lotse")).runId;
        await until(fake, () => handler.count("prepare") === 1);
        expiredJti = handler.invocations[0]?.claims.jti ?? "";
        await fake.passTime(
          fake.timings["agent.invocation_token_ttl"] +
            fake.sdkSettings.invocationTokenLeewayMs +
            1_000,
        );
        open();
        await until(fake, () => handler.count("execute") === 1);
        await until(
          fake,
          async () => (await fake.runState(runId)).state === "completed",
        );
      });
      expect(posts(seen, "/invocation-token").map((s) => s.status)).toEqual([
        200,
      ]);
      expect(posts(seen, "/start").map((s) => s.status)).toEqual([200]);
      expect(handler.jtis).toHaveLength(1);
      expect(handler.jtis[0]).not.toBe(expiredJti);
      expect(log.indexOf("invocation-token:200")).toBeLessThan(
        log.indexOf("start:200"),
      );
      expect(log.indexOf("start:200")).toBeLessThan(log.indexOf("execute"));
      expect((await fake.runState(runId)).starts).toBe(1);
    },
    TEST_TIMEOUT,
  );

  it(
    "a repeated jti under a new event: ignored, not acknowledged, nothing recorded",
    async () => {
      const log: string[] = [];
      const handler = new SpyHandler(log);
      const seen = await running(handler, "azp6-replay", async (source) => {
        const { runId } = await fake.trigger("AZ-P6 replay @Lotse");
        await until(
          fake,
          async () => (await fake.runState(runId)).state === "completed",
        );
        const first = source.envelopes("agent.invocation")[0];
        expect(first?.token).toBeDefined();
        const replayId = await fake.deliverWithToken(
          first?.id ?? "",
          first?.token as Secret,
        );
        await until(fake, () =>
          source.envelopes("agent.invocation").some((e) => e.id === replayId),
        );
        const replay = source
          .envelopes("agent.invocation")
          .find((e) => e.id === replayId);
        expect(replay?.token && fingerprint(replay.token)).toBe(
          first?.token && fingerprint(first.token),
        );
        await fake.passTime(fake.timings["agent.ack_persist_interval"] * 2);
        expect(fake.consumerState("azp6-replay").ackedSeq).toBeLessThan(
          replay?.seq ?? 0,
        );
      });
      // Only the first run's start and its completion; nothing for the replay.
      expect(posts(seen, "/start")).toHaveLength(1);
      expect(posts(seen, "/finish")).toHaveLength(1);
      expect(handler.count("prepare")).toBe(1);
      expect(handler.count("execute")).toBe(1);
    },
    TEST_TIMEOUT,
  );

  it(
    "a valid token: the runtime is called once, after 200 on startRun",
    async () => {
      const log: string[] = [];
      const handler = new SpyHandler(log);
      let token: Secret | undefined;
      await running(handler, "azp6-valid", async (source) => {
        const { runId } = await fake.trigger("AZ-P6 valid @Lotse");
        await until(
          fake,
          async () => (await fake.runState(runId)).state === "completed",
        );
        token = source.envelopes("agent.invocation")[0]?.token;
      });
      expect(handler.count("execute")).toBe(1);
      expect(log.indexOf("start:200")).toBeGreaterThanOrEqual(0);
      expect(log.indexOf("start:200")).toBeLessThan(log.indexOf("execute"));
      expect(handler.jtis).toEqual([storedClaims(token as Secret).jti]);
    },
    TEST_TIMEOUT,
  );
});
