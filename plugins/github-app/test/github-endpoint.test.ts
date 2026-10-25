// SPDX-License-Identifier: Apache-2.0
// The endpoint POST /github/webhook (brief M2-49 Design notes 4 to 6).

import { request } from "node:http";

import { afterEach, describe, expect, it } from "vitest";

import { FakeChat } from "./fakes/chat.js";
import { fixture } from "./helpers/fixtures.js";
import { startWorld, type World } from "./helpers/world.js";

let world: World | undefined;

afterEach(async () => {
  await world?.close();
  world = undefined;
});

function jobs(w: World): number {
  return w.logs.filter(
    (l) => l.msg === "delivery.accepted" && l.job_id !== undefined,
  ).length;
}

describe("POST /github/webhook", () => {
  it("accepts a valid issues opened with 202 and one job", async () => {
    world = await startWorld();
    expect(await world.deliver("issues", fixture("issues", "opened"))).toBe(
      202,
    );
    expect(jobs(world)).toBe(1);
  });

  it.each(["alter", "omit", "truncate"] as const)(
    "answers 401 to a %s signature without a job or a call to the chat",
    async (signature) => {
      world = await startWorld();
      expect(
        await world.deliver("issues", fixture("issues", "opened"), {
          signature,
        }),
      ).toBe(401);
      await world.app.idle();
      expect(jobs(world)).toBe(0);
      expect(
        world.chat.calls.filter((c) => c.op !== "me" && c.op !== "limits"),
      ).toHaveLength(0);
    },
  );

  it("answers 401 to another secret", async () => {
    world = await startWorld();
    expect(
      await world.deliver("issues", fixture("issues", "opened"), {
        secret: "another",
      }),
    ).toBe(401);
  });

  it("answers 204 to ping and to an unsupported action", async () => {
    world = await startWorld();
    expect(await world.deliver("ping", fixture("ping", ""))).toBe(204);
    expect(
      await world.deliver("issues", {
        ...fixture("issues", "opened"),
        action: "labeled",
      }),
    ).toBe(204);
    expect(await world.deliver("star", fixture("issues", "opened"))).toBe(204);
    expect(jobs(world)).toBe(0);
  });

  it("answers 204 to a reused delivery identifier without a second job", async () => {
    world = await startWorld();
    const payload = fixture("issues", "opened");
    expect(await world.deliver("issues", payload, { deliveryId: "d-1" })).toBe(
      202,
    );
    expect(await world.deliver("issues", payload, { deliveryId: "d-1" })).toBe(
      204,
    );
    expect(jobs(world)).toBe(1);
  });

  it("answers 204 to an unknown repository", async () => {
    world = await startWorld();
    const payload = fixture("issues", "opened", {
      repository: "example-org/other",
    });
    expect(await world.deliver("issues", payload)).toBe(204);
    expect(
      world.logs.some((l) => l.reason === "repository_not_configured"),
    ).toBe(true);
  });

  it("answers 400 without X-GitHub-Event, to invalid JSON and to a payload without number", async () => {
    world = await startWorld();
    expect(
      await world.deliver("issues", fixture("issues", "opened"), {
        omitEvent: true,
      }),
    ).toBe(400);
    expect(
      await world.deliver(
        "issues",
        {},
        { rawBody: new TextEncoder().encode("{not json") },
      ),
    ).toBe(400);
    expect(
      await world.deliver(
        "issues",
        {},
        { rawBody: new Uint8Array([0x7b, 0xff, 0x7d]) },
      ),
    ).toBe(400);
    const payload = fixture("issues", "opened");
    delete (payload.issue as Record<string, unknown>).number;
    expect(await world.deliver("issues", payload)).toBe(400);
  });

  it("answers 415 to text/plain", async () => {
    world = await startWorld();
    expect(
      await world.deliver("issues", fixture("issues", "opened"), {
        contentType: "text/plain",
      }),
    ).toBe(415);
    expect(
      await world.deliver("issues", fixture("issues", "opened"), {
        contentType: "application/json; charset=utf-8",
      }),
    ).toBe(202);
  });

  it("answers 413 to a body above github_app.max_body_bytes and stops reading", async () => {
    world = await startWorld({ members: { max_body_bytes: 64 } });
    expect(await world.deliver("issues", fixture("issues", "opened"))).toBe(
      413,
    );
    // A chunked stream that grows beyond the bound is cut too.
    const url = new URL(`${world.app.url}/github/webhook`);
    const status = await new Promise<number>((resolve, reject) => {
      const req = request(
        {
          host: url.hostname,
          port: url.port,
          path: url.pathname,
          method: "POST",
          headers: { "content-type": "application/json" },
        },
        (res) => {
          res.resume();
          resolve(res.statusCode ?? 0);
        },
      );
      req.on("error", reject);
      req.write("x".repeat(100));
      // The rest is never sent: the app answers without waiting for the end.
    });
    expect(status).toBe(413);
  });

  it("answers 405 to another method and 404 to an unknown path", async () => {
    world = await startWorld();
    expect((await fetch(`${world.app.url}/github/webhook`)).status).toBe(405);
    expect(
      (await fetch(`${world.app.url}/other`, { method: "POST" })).status,
    ).toBe(404);
  });

  it("answers 503 when the queue is full", async () => {
    world = await startWorld({ members: { max_queued_jobs: 1 } });
    // The first job is held inside its step, so it stays queued.
    const held = FakeChat.hold();
    world.chat.fail("createForumThread", held.fault);
    expect(
      await world.deliver("issues", fixture("issues", "opened", { number: 1 })),
    ).toBe(202);
    await held.entered;
    expect(
      await world.deliver("issues", fixture("issues", "opened", { number: 2 })),
    ).toBe(503);
    held.release();
    await world.app.idle();
    expect(
      await world.deliver("issues", fixture("issues", "opened", { number: 3 })),
    ).toBe(202);
  });

  it("answers 503 when the webhook secret file fails its check", async () => {
    world = await startWorld();
    const { chmod } = await import("node:fs/promises");
    await chmod(world.config.webhookSecretFile, 0o644);
    expect(await world.deliver("issues", fixture("issues", "opened"))).toBe(
      503,
    );
  });

  it("answers 503 while stopping and lets the running step finish", async () => {
    world = await startWorld();
    const held = FakeChat.hold();
    world.chat.fail("createForumThread", held.fault);
    expect(
      await world.deliver("issues", fixture("issues", "opened", { number: 1 })),
    ).toBe(202);
    await held.entered;
    const stopped = world.stopApp();
    expect(
      await world.deliver("issues", fixture("issues", "opened", { number: 2 })),
    ).toBe(503);
    held.release();
    await stopped;
    expect(world.chat.threads.size).toBe(1);
  });
});
