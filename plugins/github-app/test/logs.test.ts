// SPDX-License-Identifier: Apache-2.0
// Logs carry codes only (brief M2-49 Design note 15; SEC-LOG-2, SEC-LOG-4): canary values run
// through every flow and no log line contains one.

import { randomUUID } from "node:crypto";

import { afterEach, describe, expect, it } from "vitest";

import type { PrincipalRef } from "../src/chat/port.js";
import {
  LOG_ATTRIBUTES,
  allowedFields,
  createLogger,
  sdkLogger,
} from "../src/log.js";
import { FakeChat } from "./fakes/chat.js";
import { fixture } from "./helpers/fixtures.js";
import {
  envelope,
  FORUM,
  SECRETS,
  startWorld,
  type World,
} from "./helpers/world.js";

let world: World | undefined;

afterEach(async () => {
  await world?.close();
  world = undefined;
});

const c = (name: string): string => `canary${name}${randomUUID().slice(0, 8)}`;

describe("logs", () => {
  it("never contain titles, bodies, comments, form values, names, logins, repositories, tokens or secrets", async () => {
    const repo = `${c("org")}/${c("repo")}`.toLowerCase();
    const canaries = {
      title: c("title"),
      body: c("body"),
      comment: c("comment"),
      login: c("login"),
      formTitle: c("formtitle"),
      formBody: c("formbody"),
      displayName: c("name"),
      reply: c("reply"),
      repo,
      ...SECRETS,
    };
    world = await startWorld({ repositories: [[repo, FORUM]] });
    const opened = fixture("issues", "opened", { repository: repo, number: 3 });
    (opened.issue as Record<string, unknown>).title = canaries.title;
    (opened.issue as Record<string, unknown>).body = canaries.body;
    (opened.sender as Record<string, unknown>).login = canaries.login;
    await world.deliver("issues", opened);
    const comment = fixture("issue_comment", "created", {
      repository: repo,
      number: 3,
      commentId: 9,
    });
    (comment.comment as Record<string, unknown>).body = canaries.comment;
    await world.deliver("issue_comment", comment);
    await world.deliver("issues", opened, { signature: "alter" });
    await world.deliver("issues", { ...opened, action: "labeled" });
    await world.app.idle();

    const invoked = envelope("command.invoked", {
      command_name: "issue",
      channel_id: FORUM,
      options: [{ name: "title", value: canaries.formTitle }],
      invoked_by: {
        id: randomUUID(),
        kind: "user",
        handle: canaries.login,
        display_name: canaries.displayName,
      },
    });
    await world.serverRequest("/chat/interactions", invoked);
    await world.app.idle();
    const post = world.chat.threadsOf(FORUM).at(-1);
    const card = post?.messages.find((m) => m.kind === "card");
    await world.serverRequest(
      "/chat/interactions",
      envelope("interaction.created", {
        interaction: {
          id: randomUUID(),
          custom_id: (card?.components[0] as { custom_id?: string } | undefined)
            ?.custom_id,
          message_id: card?.id,
          values: { body: canaries.formBody, repository: repo },
        },
      }),
    );
    await world.app.idle();
    await world.github.echoesDone();
    await world.app.idle();

    const person: PrincipalRef = {
      id: randomUUID(),
      kind: "user",
      handle: canaries.login,
      display_name: canaries.displayName,
    };
    const thread = world.chat.threadsOf(FORUM)[0];
    const reply = world.chat.personWrites(
      thread?.id ?? "",
      canaries.reply,
      person,
    );
    await world.serverRequest(
      "/chat/events",
      envelope("message.created", { message: reply }),
    );
    world.github.fail({ status: 500 });
    await world.serverRequest(
      "/chat/events",
      envelope("message.created", {
        message: world.chat.personWrites(
          thread?.id ?? "",
          canaries.reply,
          person,
        ),
      }),
    );
    world.chat.fail("postMessage", FakeChat.problem(403, "scope_missing"));
    await world.deliver("issues", { ...opened, action: "closed" });
    await world.app.idle();
    await world.github.echoesDone();
    await world.app.idle();

    // The flows ran: an issue and comments reached GitHub, messages reached the chat.
    expect(world.github.issues().length).toBeGreaterThan(0);
    expect(world.github.comments().length).toBeGreaterThan(0);
    expect(world.rawLogs.length).toBeGreaterThan(10);
    for (const line of world.rawLogs)
      for (const [name, value] of Object.entries(canaries))
        expect(line.toLowerCase(), `${name} in a log line`).not.toContain(
          value.toLowerCase(),
        );
  });

  it("drops every attribute outside the allow list", () => {
    const lines: string[] = [];
    const logger = createLogger({ write: (l) => lines.push(l), now: () => 0 });
    logger.info("x.code", {
      status: 1,
      title: "secret",
      body: "secret",
      url: "secret",
      nested: { a: 1 },
    });
    logger.debug("hidden");
    sdkLogger(logger).warn(
      "the server marked X as deprecated (Deprecation: secret)",
      { operation: "GET /me", deprecation: "secret" },
    );
    sdkLogger(logger).error("x", { status: 500 });
    sdkLogger(logger).info("x", {});
    sdkLogger(logger).debug("x", {});
    expect(lines.map((l) => JSON.parse(l) as unknown)).toEqual([
      {
        time: "1970-01-01T00:00:00.000Z",
        level: "info",
        msg: "x.code",
        status: 1,
      },
      {
        time: "1970-01-01T00:00:00.000Z",
        level: "warn",
        msg: "sdk.client",
        operation: "GET /me",
      },
      {
        time: "1970-01-01T00:00:00.000Z",
        level: "error",
        msg: "sdk.client",
        status: 500,
      },
      { time: "1970-01-01T00:00:00.000Z", level: "info", msg: "sdk.client" },
    ]);
    expect(allowedFields(undefined)).toEqual({});
    expect(LOG_ATTRIBUTES).toContain("reason");
  });
});
