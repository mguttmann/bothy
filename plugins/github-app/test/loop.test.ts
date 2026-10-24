// SPDX-License-Identifier: Apache-2.0
// Mirroring and loop protection (brief M2-49 Design note 12; PLAN row "the app's own messages
// are never mirrored back (loop protection)").

import { randomUUID } from "node:crypto";

import { afterEach, describe, expect, it } from "vitest";

import type { Message, PrincipalRef } from "../src/chat/port.js";
import { fixture } from "./helpers/fixtures.js";
import {
  envelope,
  FORUM,
  logsWith,
  startWorld,
  type World,
} from "./helpers/world.js";

let world: World | undefined;

afterEach(async () => {
  await world?.close();
  world = undefined;
});

const ALEX: PrincipalRef = {
  id: randomUUID(),
  kind: "user",
  handle: "alex",
  display_name: "Alex",
};

async function linkedThread(w: World): Promise<string> {
  await w.deliver("issues", fixture("issues", "opened", { number: 12 }));
  await w.app.idle();
  const thread = w.chat.threadsOf(FORUM)[0];
  if (thread === undefined) throw new Error("no thread");
  return thread.id;
}

function created(message: Message): Record<string, unknown> {
  return envelope("message.created", { message });
}

describe("mirroring and loops", () => {
  it("never mirrors the app's own message", async () => {
    world = await startWorld();
    const thread = await linkedThread(world);
    const own = world.chat.threads.get(thread)?.messages[0];
    if (own === undefined) throw new Error("no message");
    const response = await world.serverRequest("/chat/events", created(own));
    expect(response.status).toBe(204);
    await world.app.idle();
    expect(world.github.comments()).toHaveLength(0);
    expect(logsWith(world, "own_message")).toHaveLength(1);
  });

  it.each([
    ["agent", { origin: "agent" }, "not_human"],
    [
      "app",
      { origin: "app", author: { ...ALEX, id: randomUUID(), kind: "app" } },
      "not_human",
    ],
    ["import", { origin: "import" }, "not_human"],
    ["system", { origin: "system", kind: "system" }, "not_human"],
    ["card", { kind: "card" }, "not_text"],
    ["deleted", { deleted_at: new Date().toISOString() }, "deleted"],
  ])("does not mirror a message of kind %s", async (_name, change, reason) => {
    world = await startWorld();
    const thread = await linkedThread(world);
    const message = {
      ...world.chat.personWrites(thread, "Hello", ALEX),
      ...change,
    } as Message;
    expect(
      (await world.serverRequest("/chat/events", created(message))).status,
    ).toBe(204);
    await world.app.idle();
    expect(world.github.comments()).toHaveLength(0);
    expect(logsWith(world, reason)).toHaveLength(1);
  });

  it("does not mirror a message in an unmapped thread", async () => {
    world = await startWorld();
    await linkedThread(world);
    const message = {
      ...world.chat.personWrites(
        world.chat.threadsOf(FORUM)[0]?.id ?? "",
        "Hello",
        ALEX,
      ),
      thread_id: randomUUID(),
    } as Message;
    expect(
      (await world.serverRequest("/chat/events", created(message))).status,
    ).toBe(204);
    await world.app.idle();
    expect(world.github.comments()).toHaveLength(0);
    expect(logsWith(world, "thread_not_mapped")).toHaveLength(1);
  });

  it("mirrors a person's text once, without mentions or links, and its echo adds nothing", async () => {
    world = await startWorld();
    const thread = await linkedThread(world);
    const before = world.chat.threads.get(thread)?.messages.length ?? 0;
    const message = world.chat.personWrites(
      thread,
      "Thanks @sam, see [x](https://example.com)",
      ALEX,
    );
    expect(
      (await world.serverRequest("/chat/events", created(message))).status,
    ).toBe(202);
    await world.app.idle();
    await world.github.echoesDone();
    await world.app.idle();
    const comments = world.github.comments();
    expect(comments).toHaveLength(1);
    expect(comments[0]?.path).toBe(
      "/repos/example-org/widgets/issues/12/comments",
    );
    expect(comments[0]?.body).toEqual({
      body: "`Alex` wrote in chat:\n\n```\nThanks @sam, see [x](https://example.com)\n```",
    });
    // The person's message plus nothing from the echo.
    expect(world.chat.threads.get(thread)?.messages).toHaveLength(before + 1);
    expect(logsWith(world, "own_github_object")).toHaveLength(1);
  });

  it("loads a truncated message with getMessage", async () => {
    world = await startWorld();
    const thread = await linkedThread(world);
    const message = world.chat.personWrites(thread, "The full text", ALEX);
    const truncated = {
      ...message,
      content_md: "The full",
      content_truncated: true,
    };
    await world.serverRequest("/chat/events", created(truncated));
    await world.app.idle();
    expect(world.chat.callsOf("getMessage")).toHaveLength(1);
    expect(world.github.comments()[0]?.body.body).toContain("The full text");
  });

  it("logs a GitHub 500 on the comment and does not repeat it", async () => {
    world = await startWorld();
    const thread = await linkedThread(world);
    world.github.fail({ status: 500 });
    const message = world.chat.personWrites(thread, "Hello", ALEX);
    await world.serverRequest("/chat/events", created(message));
    await world.app.idle();
    expect(world.github.comments()).toHaveLength(1);
    expect(
      world.logs.filter((l) => l.msg === "mirror.failed" && l.status === 500),
    ).toHaveLength(1);
  });

  it("ignores other event types", async () => {
    world = await startWorld();
    const response = await world.serverRequest(
      "/chat/events",
      envelope("thread.created", {}),
    );
    expect(response.status).toBe(204);
  });
});
