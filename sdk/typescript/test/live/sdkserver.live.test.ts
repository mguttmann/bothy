// SPDX-License-Identifier: Apache-2.0
//
// Smoke test of the live driver: SdkServer over the real server of services/server/test/sdkserver
// (brief M2-30 Design note 12). It needs the Go tool chain and a Docker-compatible runtime;
// without them it fails, it never skips.

import { afterAll, beforeAll, describe, expect, inject, it } from "vitest";

import { Secret } from "../../src/auth/secret.js";
import { createApiClient, unwrap } from "../../src/client.js";
import { FORGED_VARIANTS } from "../../src/testing/scenario-server.js";
import {
  RealServer,
  SDK_SETTINGS,
  TIMINGS,
  startRealServer,
} from "./sdkserver.js";

const UUID = /^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$/;

describe("RealServer against the real server", () => {
  let server: RealServer;

  beforeAll(async () => {
    server = await startRealServer(inject("sdkserverBinary"));
  });

  afterAll(async () => {
    await server?.stop();
  });

  it("reports origin, IDs and Lotse's DPoP-bound credentials", () => {
    expect(server.origin).toMatch(/^http:\/\/127\.0\.0\.1:\d+$/);
    expect(server.baseUrl).toBe(`${server.origin}/api/v1`);
    for (const id of [server.instanceId, server.agentId, server.otherAgentId])
      expect(id).toMatch(UUID);
    expect(server.agentId).not.toBe(server.otherAgentId);
    expect(server.credentials.apiToken).toBeInstanceOf(Secret);
    expect(server.credentials.dpop?.alg).toBe("ES256");
    expect(server.sdkSettings).toBe(SDK_SETTINGS);
    expect(server.timings["agent.ack_timeout"]).toBe(2_000);
    expect(Object.keys(server.timings).sort()).toEqual(
      Object.keys(TIMINGS).sort(),
    );
  });

  it("triggers a run that is queued and not started", async () => {
    const t = await server.trigger();
    for (const id of [t.runId, t.threadId, t.messageId])
      expect(id).toMatch(UUID);
    const view = await server.runState(t.runId);
    expect(view.state).toBe("queued");
    expect(view.starts).toBe(0);
    expect(view.version).toBeGreaterThanOrEqual(1);
  });

  it("forges a token for every variant", async () => {
    const t = await server.trigger("Hello @Lotse, forged");
    const tokens: Secret[] = [];
    for (const variant of FORGED_VARIANTS) {
      const token = await server.forgeToken(t.runId, variant);
      expect(token).toBeInstanceOf(Secret);
      expect(token.reveal().split(".")).toHaveLength(3);
      tokens.push(token);
    }
    for (let i = 0; i < tokens.length; i++)
      for (let j = i + 1; j < tokens.length; j++)
        expect(tokens[i]?.equals(tokens[j] as Secret)).toBe(false);
  });

  it("answers an unknown run with an error and keeps serving", async () => {
    await expect(
      server.runState("00000000-0000-7000-8000-000000000000"),
    ).rejects.toThrow(/sdkserver run:/);
    const t = await server.trigger();
    expect((await server.runState(t.runId)).state).toBe("queued");
  });

  it("restarts on the same data with a rewound consumer", async () => {
    const t = await server.trigger();
    const api = createApiClient({
      baseUrl: server.baseUrl,
      token: server.credentials.apiToken,
      dpopKey: server.credentials.dpop,
      settings: server.sdkSettings,
    });
    // The first poll creates the consumer at head_seq (04 15.1 point 2); DPoP runs for real.
    const page = await unwrap(
      api.GET("/agents/me/consumers/{consumer}/events", {
        params: { path: { consumer: "smoke" } },
      }),
      "pollAgentEvents",
    );
    await server.restart("smoke", page.next_cursor);
    const view = await server.runState(t.runId);
    expect(view.state).toBe("queued");
    expect(view.starts).toBe(0);
    // An unknown consumer answers an error after the new server runs; the program keeps serving.
    await expect(server.restart("nobody", 0)).rejects.toThrow(
      /sdkserver restart:/,
    );
    expect((await server.runState(t.runId)).state).toBe("queued");
  });

  it("passes real time", async () => {
    const before = Date.now();
    await server.passTime(50);
    expect(Date.now() - before).toBeGreaterThanOrEqual(45);
  });
});
