// SPDX-License-Identifier: Apache-2.0
// One thread per issue or pull request (brief M2-49 Design note 9; PLAN row "an event for a
// known issue lands in the same thread, a new one creates a thread").

import { afterEach, describe, expect, it } from "vitest";

import { FakeChat } from "./fakes/chat.js";
import { fixture } from "./helpers/fixtures.js";
import { FORUM, logsWith, startWorld, type World } from "./helpers/world.js";

let world: World | undefined;

afterEach(async () => {
  await world?.close();
  world = undefined;
});

describe("threads", () => {
  it("opens one forum post per issue and posts later events into it", async () => {
    world = await startWorld();
    expect(await world.deliver("issues", fixture("issues", "opened", { number: 12 }))).toBe(202);
    await world.app.idle();
    const posts = world.chat.threadsOf(FORUM);
    expect(posts).toHaveLength(1);
    const [first] = posts;
    expect(first?.title).toBe("#12 Spelling error in the README file");
    expect(first?.messages[0]?.content_md).toContain("**Issue [#12](https://github.com/example-org/widgets/issues/12) opened**");

    expect(
      await world.deliver("issue_comment", fixture("issue_comment", "created", { number: 12, commentId: 777 })),
    ).toBe(202);
    await world.app.idle();
    expect(world.chat.threadsOf(FORUM)).toHaveLength(1);
    expect(first?.messages).toHaveLength(2);
    expect(first?.messages[1]?.content_md).toContain("commented on [#12]");

    expect(await world.deliver("issues", fixture("issues", "opened", { number: 13 }))).toBe(202);
    await world.app.idle();
    expect(world.chat.threadsOf(FORUM).map((t) => t.title)).toEqual([
      "#12 Spelling error in the README file",
      "#13 Spelling error in the README file",
    ]);
  });

  it("keeps the mapping across a restart on the same state directory", async () => {
    world = await startWorld();
    await world.deliver("issues", fixture("issues", "opened", { number: 12 }));
    await world.app.idle();
    await world.stopApp();
    const again = await startWorld({ dir: world.dir, chat: world.chat, github: world.github });
    try {
      expect(await again.deliver("issues", fixture("issues", "reopened", { number: 12 }))).toBe(202);
      await again.app.idle();
      const posts = again.chat.threadsOf(FORUM);
      expect(posts).toHaveLength(1);
      expect(posts[0]?.messages.at(-1)?.content_md).toContain("**Issue [#12]");
      expect(posts[0]?.messages.at(-1)?.content_md).toContain("reopened**");
    } finally {
      await again.stopApp();
    }
  });

  it("titles a pull request PR #<n>, also when its first event is a comment", async () => {
    world = await startWorld();
    await world.deliver("issue_comment", fixture("issue_comment", "created.pull_request", { number: 7 }));
    await world.app.idle();
    expect(world.chat.threadsOf(FORUM).map((t) => t.title)).toEqual([
      "PR #7 Update the README with new information.",
    ]);
  });

  it("replaces an unreachable thread once and logs thread_unreachable", async () => {
    world = await startWorld();
    await world.deliver("issues", fixture("issues", "opened", { number: 12 }));
    await world.app.idle();
    const [old] = world.chat.threadsOf(FORUM);
    world.chat.drop(old?.id ?? "");
    await world.deliver("issues", fixture("issues", "reopened", { number: 12 }));
    await world.app.idle();
    const posts = world.chat.threadsOf(FORUM);
    expect(posts).toHaveLength(2);
    expect(posts[1]?.messages[0]?.content_md).toContain("reopened**");
    expect(logsWith(world, "thread_unreachable")).toHaveLength(1);
    // A second 404 (the new post is out of reach too) ends the job after one more post.
    world.chat.drop(posts[1]?.id ?? "");
    world.chat.fail("createForumThread", FakeChat.problem(404, "not_found"));
    await world.deliver("issues", fixture("issues", "closed", { number: 12 }));
    await world.app.idle();
    expect(world.chat.threadsOf(FORUM)).toHaveLength(2);
    expect(logsWith(world, "thread_unreachable")).toHaveLength(2);
  });

  it("sends the same client_msg_id again after a crash between createThread and the map", async () => {
    world = await startWorld();
    world.chat.fail("createForumThread", { kind: "lose_answer" });
    await world.deliver("issues", fixture("issues", "opened", { number: 12 }));
    await world.app.idle();
    const calls = world.chat.callsOf("createForumThread");
    expect(calls).toHaveLength(2);
    const ids = calls.map(
      (c) => (c.args[0] as { firstMessage: { client_msg_id: string } }).firstMessage.client_msg_id,
    );
    expect(ids[0]).toBe(ids[1]);
    expect(world.chat.threadsOf(FORUM)).toHaveLength(1);
    await world.deliver("issues", fixture("issues", "reopened", { number: 12 }));
    await world.app.idle();
    expect(world.chat.threadsOf(FORUM)).toHaveLength(1);
  });

  it("retries a too-long answer once with new limits and a refused title with #<n>", async () => {
    world = await startWorld();
    world.chat.fail(
      "createForumThread",
      FakeChat.problem(400, "validation_failed", [{ pointer: "/title", code: "invalid" }]),
    );
    await world.deliver("issues", fixture("issues", "opened", { number: 21 }));
    await world.app.idle();
    expect(world.chat.threadsOf(FORUM).map((t) => t.title)).toEqual(["#21"]);
    world.chat.fail(
      "createForumThread",
      FakeChat.problem(400, "validation_failed", [{ pointer: "/first_message/content_md", code: "too_long" }]),
    );
    const before = world.chat.callsOf("limits").length;
    await world.deliver("issues", fixture("issues", "opened", { number: 22 }));
    await world.app.idle();
    expect(world.chat.callsOf("limits").length).toBe(before + 1);
    expect(world.chat.threadsOf(FORUM)).toHaveLength(2);
  });
});
