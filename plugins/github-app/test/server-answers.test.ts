// SPDX-License-Identifier: Apache-2.0
// Answers of the server (brief M2-49 Design note 13) and the worker's transient retry (Design
// note 8).

import { chmod } from "node:fs/promises";
import { randomUUID } from "node:crypto";

import { afterEach, describe, expect, it } from "vitest";

import { TEXTS } from "../src/worker/jobs.js";
import { FakeChat } from "./fakes/chat.js";
import { fixture } from "./helpers/fixtures.js";
import { eventually } from "./helpers/patience.js";
import { envelope, FORUM, startWorld, type World } from "./helpers/world.js";

let world: World | undefined;

afterEach(async () => {
  await world?.close();
  world = undefined;
});

describe("answers of the server", () => {
  it("answers 503 after 401 unauthenticated until getMe succeeds again", async () => {
    world = await startWorld();
    const held = FakeChat.hold();
    world.chat.fail("me", held.fault);
    world.chat.fail(
      "createForumThread",
      FakeChat.problem(401, "unauthenticated"),
    );
    expect(
      await world.deliver("issues", fixture("issues", "opened", { number: 1 })),
    ).toBe(202);
    await held.entered;
    expect(
      world.logs.filter((l) => l.reason === "unauthenticated"),
    ).toHaveLength(1);
    expect(
      await world.deliver("issues", fixture("issues", "opened", { number: 2 })),
    ).toBe(503);
    held.release();
    const w = world;
    await eventually(
      () => w.logs.some((l) => l.msg === "app.authenticated"),
      "getMe succeeds again",
    );
    expect(
      await world.deliver("issues", fixture("issues", "opened", { number: 3 })),
    ).toBe(202);
  });

  it("keeps a job queued after a transient failure and retries it with the same identifiers", async () => {
    world = await startWorld();
    world.chat.fail(
      "createForumThread",
      FakeChat.problem(503, "unavailable"),
      FakeChat.problem(429, "rate_limited"),
    );
    await world.deliver("issues", fixture("issues", "opened", { number: 1 }));
    await world.app.idle();
    const calls = world.chat.callsOf("createForumThread");
    expect(calls).toHaveLength(3);
    const ids = new Set(
      calls.map(
        (c) => (c.args[0] as { idempotencyKey: string }).idempotencyKey,
      ),
    );
    expect(ids.size).toBe(1);
    expect(world.logs.filter((l) => l.msg === "worker.paused")).toHaveLength(2);
    expect(world.chat.threadsOf(FORUM)).toHaveLength(1);
  });

  it.each([
    [403, "forbidden", "forbidden"],
    [403, "channel_read_only", "channel_read_only"],
    [422, "idempotency_key_reuse", "idempotency_key_reuse"],
    [400, "validation_failed", "validation_failed"],
    [409, "conflict", "conflict"],
  ])("ends the job on %d %s", async (status, code, reason) => {
    world = await startWorld();
    world.chat.fail("createForumThread", FakeChat.problem(status, code));
    await world.deliver("issues", fixture("issues", "opened", { number: 1 }));
    await world.app.idle();
    expect(world.chat.callsOf("createForumThread")).toHaveLength(1);
    expect(
      world.logs.some((l) => l.reason === reason && l.status === status),
    ).toBe(true);
  });

  it("ends a mirror job on 404 of getMessage", async () => {
    world = await startWorld();
    await world.deliver("issues", fixture("issues", "opened", { number: 1 }));
    await world.app.idle();
    const thread = world.chat.threadsOf(FORUM)[0];
    const message = world.chat.personWrites(thread?.id ?? "", "x", {
      id: randomUUID(),
      kind: "user",
      handle: "sam",
      display_name: "Sam",
    });
    await world.serverRequest(
      "/chat/events",
      envelope("message.created", {
        message: { ...message, id: randomUUID(), content_truncated: true },
      }),
    );
    await world.app.idle();
    expect(world.github.comments()).toHaveLength(0);
  });

  it("answers failed and keeps the draft when the GitHub token file fails its check", async () => {
    world = await startWorld();
    await world.serverRequest(
      "/chat/interactions",
      envelope("command.invoked", {
        command_name: "issue",
        channel_id: FORUM,
        options: [{ name: "title", value: "T" }],
      }),
    );
    await world.app.idle();
    const post = world.chat.threadsOf(FORUM)[0];
    const card = post?.messages.find((m) => m.kind === "card");
    await chmod(world.config.githubTokenFile, 0o644);
    const interaction = {
      id: randomUUID(),
      custom_id: (card?.components[0] as { custom_id: string }).custom_id,
      message_id: card?.id,
      values: {},
    };
    await world.serverRequest(
      "/chat/interactions",
      envelope("interaction.created", { interaction }),
    );
    await world.app.idle();
    expect(world.chat.responses.get(interaction.id)).toEqual({
      state: "failed",
      response: { text: TEXTS.notSent },
    });
    expect(world.github.requests).toHaveLength(0);
    expect(post?.messages.filter((m) => m.kind === "card")).toHaveLength(2);
  });

  it("rejects a repository the form did not offer", async () => {
    world = await startWorld({
      repositories: [["example-org/gadgets", FORUM]],
    });
    await world.serverRequest(
      "/chat/interactions",
      envelope("command.invoked", {
        command_name: "issue",
        channel_id: FORUM,
        options: [{ name: "title", value: "" }],
      }),
    );
    await world.app.idle();
    const post = world.chat.threadsOf(FORUM)[0];
    expect(post?.title).toBe(TEXTS.newIssueTitle);
    const card = post?.messages.find((m) => m.kind === "card");
    const interaction = {
      id: randomUUID(),
      custom_id: "other",
      message_id: card?.id,
      values: { repository: "example-org/elsewhere" },
    };
    await world.serverRequest(
      "/chat/interactions",
      envelope("interaction.created", { interaction }),
    );
    await world.app.idle();
    expect(world.chat.responses.get(interaction.id)).toEqual({
      state: "rejected",
      response: { text: TEXTS.chooseRepository },
    });
  });

  it("refuses requests of the server with an invalid payload", async () => {
    world = await startWorld();
    expect(
      (
        await world.serverRequest(
          "/chat/interactions",
          envelope("command.invoked", { command_name: "issue" }),
        )
      ).status,
    ).toBe(400);
    expect(
      (
        await world.serverRequest(
          "/chat/interactions",
          envelope("interaction.created", {}),
        )
      ).status,
    ).toBe(400);
    expect(
      (
        await world.serverRequest(
          "/chat/events",
          envelope("message.created", {}),
        )
      ).status,
    ).toBe(400);
  });
});
