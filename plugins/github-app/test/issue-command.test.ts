// SPDX-License-Identifier: Apache-2.0
// `/issue` with a form (brief M2-49 Design note 11; PLAN row "/issue with a form creates an issue
// through the fake API and replies in the thread").

import { randomUUID } from "node:crypto";

import { afterEach, describe, expect, it } from "vitest";

import { TEXTS } from "../src/worker/jobs.js";
import { fixture } from "./helpers/fixtures.js";
import { envelope, FORUM, startWorld, type World } from "./helpers/world.js";

let world: World | undefined;

afterEach(async () => {
  await world?.close();
  world = undefined;
});

const ALEX = {
  id: randomUUID(),
  kind: "user",
  handle: "alex",
  display_name: "Alex",
};

function invoked(title: string, channel = FORUM): Record<string, unknown> {
  return envelope("command.invoked", {
    invocation_id: randomUUID(),
    command_id: randomUUID(),
    command_name: "issue",
    control: false,
    channel_id: channel,
    invoked_by: ALEX,
    options: [{ name: "title", value: title }],
  });
}

function submitted(
  card: { id: string; components: { custom_id?: string }[] },
  values: Record<string, unknown>,
  customId = card.components[0]?.custom_id ?? "",
): Record<string, unknown> {
  return envelope("interaction.created", {
    interaction: {
      id: randomUUID(),
      component_id: randomUUID(),
      custom_id: customId,
      message_id: card.id,
      principal_id: ALEX.id,
      client_interaction_id: randomUUID(),
      values,
      state: "dispatched",
      created_at: new Date().toISOString(),
    },
  });
}

async function openForm(w: World, title = "Crash on start") {
  const response = await w.serverRequest("/chat/interactions", invoked(title));
  expect(response.status).toBe(202);
  await w.app.idle();
  const post = w.chat.threadsOf(FORUM).at(-1);
  const card = post?.messages.find((m) => m.kind === "card");
  if (post === undefined || card === undefined) throw new Error("no form card");
  return { post, card };
}

function formFields(card: {
  components: unknown[];
}): { name: string; type: string }[] {
  const definition = (
    card.components[0] as {
      definition: { fields: { name: string; type: string }[] };
    }
  ).definition;
  return definition.fields;
}

describe("/issue", () => {
  it("opens a forum post with a form and creates exactly one issue from it", async () => {
    world = await startWorld();
    const { post, card } = await openForm(world);
    expect(post.title).toBe("Crash on start");
    expect(formFields(card).map((f) => f.name)).toEqual(["body"]);
    const component = card.components[0] as unknown as {
      single_use: boolean;
      restricted_to: string;
    };
    expect(component.single_use).toBe(true);
    expect(component.restricted_to).toBe("any");

    const form = submitted(card, { body: "It fails at boot." });
    const answer = await world.serverRequest("/chat/interactions", form);
    expect(answer.status).toBe(202);
    await world.app.idle();
    await world.github.echoesDone();
    await world.app.idle();

    const issues = world.github.issues();
    expect(issues).toHaveLength(1);
    expect(issues[0]?.path).toBe("/repos/example-org/widgets/issues");
    expect(issues[0]?.body).toEqual({
      title: "Crash on start",
      body: "It fails at boot.\n\nCreated from chat.",
    });
    const interactionId = (form.payload as { interaction: { id: string } })
      .interaction.id;
    expect(world.chat.responses.get(interactionId)).toEqual({
      state: "acknowledged",
      response: { text: "Created issue #100" },
    });
    const texts = post.messages.map((m) => m.content_md);
    expect(texts.filter((t) => t.startsWith("Created GitHub issue"))).toEqual([
      "Created GitHub issue [#100](https://github.com/example-org/widgets/issues/100).",
    ]);
    // The echo `issues` `opened` of the app's own issue adds no message (answered 204 at
    // receipt, or skipped by its job when it arrived while the form job ran).
    expect(world.github.deliveries).toHaveLength(1);
    expect(
      world.logs.filter((l) => l.reason === "own_github_object"),
    ).toHaveLength(1);
    expect(post.messages).toHaveLength(3);

    // A later comment of another person lands in the same thread.
    await world.deliver(
      "issue_comment",
      fixture("issue_comment", "created", { number: 100, commentId: 5 }),
    );
    await world.app.idle();
    expect(post.messages).toHaveLength(4);
    expect(world.chat.threadsOf(FORUM)).toHaveLength(1);
  });

  it("offers a repository choice when two repositories share the forum", async () => {
    world = await startWorld({
      repositories: [["example-org/gadgets", FORUM]],
    });
    const { card } = await openForm(world);
    expect(formFields(card)).toEqual([
      expect.objectContaining({ name: "body", type: "textarea" }),
      expect.objectContaining({ name: "repository", type: "choice" }),
    ]);
    await world.serverRequest(
      "/chat/interactions",
      submitted(card, { body: "x", repository: "example-org/gadgets" }),
    );
    await world.app.idle();
    expect(world.github.issues().map((r) => r.path)).toEqual([
      "/repos/example-org/gadgets/issues",
    ]);
  });

  it.each([
    ["422", { status: 422 }],
    ["302", { status: 302, location: "https://example.com/elsewhere" }],
  ])(
    "answers failed and posts a new form card on a GitHub %s",
    async (_name, fault) => {
      world = await startWorld();
      const { post, card } = await openForm(world);
      world.github.fail(fault);
      const form = submitted(card, { body: "x" });
      await world.serverRequest("/chat/interactions", form);
      await world.app.idle();
      const id = (form.payload as { interaction: { id: string } }).interaction
        .id;
      expect(world.chat.responses.get(id)).toEqual({
        state: "failed",
        response: { text: TEXTS.refused(fault.status) },
      });
      const cards = post.messages.filter((m) => m.kind === "card");
      expect(cards).toHaveLength(2);
      // The new card works: its submission creates the issue.
      await world.serverRequest(
        "/chat/interactions",
        submitted(cards[1] as never, { body: "x" }),
      );
      await world.app.idle();
      expect(
        world.github.issues().filter((r) => r.status === 201),
      ).toHaveLength(1);
    },
  );

  it.each([
    ["500", { status: 500 }],
    ["time-out", { hang: true } as const],
  ])(
    "answers failed without a new card and without a second POST on %s",
    async (_name, fault) => {
      // A short sdk.http_timeout only for the answer that never comes: only the bound ends it,
      // however late it fires. The 500 keeps the default bound, so it is always answered.
      const hang = "hang" in fault;
      const short = { settings: { httpTimeoutMs: 100 } }; // never-answers: hang only
      world = await startWorld(hang ? short : {});
      const { post, card } = await openForm(world);
      world.github.fail(fault);
      const form = submitted(card, { body: "x" });
      await world.serverRequest("/chat/interactions", form);
      await world.app.idle();
      const id = (form.payload as { interaction: { id: string } }).interaction
        .id;
      expect(world.chat.responses.get(id)).toEqual({
        state: "failed",
        response: { text: TEXTS.unknown },
      });
      expect(post.messages.filter((m) => m.kind === "card")).toHaveLength(1);
      // The 500 was answered, so its POST arrived; a short bound may end the hanging call before
      // its POST arrives, so there it is at most one.
      const sent = world.github.issues().length;
      if (hang) expect(sent).toBeLessThanOrEqual(1);
      else expect(sent).toBe(1);
      // The draft is gone: the same card again is expired.
      const again = submitted(card, { body: "x" });
      await world.serverRequest("/chat/interactions", again);
      await world.app.idle();
      const againId = (again.payload as { interaction: { id: string } })
        .interaction.id;
      expect(world.chat.responses.get(againId)?.state).toBe("rejected");
      expect(world.github.issues()).toHaveLength(sent);
    },
  );

  it("treats a dropped connection as an unknown outcome without a second POST", async () => {
    world = await startWorld();
    const { card } = await openForm(world);
    world.github.fail({ drop: true });
    const form = submitted(card, { body: "x" });
    await world.serverRequest("/chat/interactions", form);
    await world.app.idle();
    expect(world.github.issues()).toHaveLength(1);
    const id = (form.payload as { interaction: { id: string } }).interaction.id;
    expect(world.chat.responses.get(id)).toEqual({
      state: "failed",
      response: { text: TEXTS.unknown },
    });
  });

  it("treats a job found attempted without result as unknown after a restart", async () => {
    world = await startWorld();
    const { card } = await openForm(world);
    await world.stopApp();
    // A job as a crash between the mark and the answer leaves it.
    const { FileStore } = await import("../src/store/file-store.js");
    const { createLogger } = await import("../src/log.js");
    const store = new FileStore({
      dir: world.config.stateDir,
      clock: { now: () => Date.now() },
      windowMs: world.config.dedupeWindowMs,
      logger: createLogger({ write: () => undefined }),
    });
    await store.open();
    const interactionId = randomUUID();
    await store.admit({
      seenKey: "wh:crashed",
      status: 202,
      maxJobs: 10,
      job: {
        id: randomUUID(),
        kind: "issue_form",
        created_at: Date.now(),
        data: {
          interaction_id: interactionId,
          custom_id: card.components[0]?.custom_id ?? "",
          message_id: card.id,
          values: { body: "x" },
        },
        progress: { github: "attempted" },
      },
    });
    await store.close();
    const again = await startWorld({
      dir: world.dir,
      chat: world.chat,
      github: world.github,
    });
    try {
      await again.app.idle();
      expect(world.github.issues()).toHaveLength(0);
      expect(world.chat.responses.get(interactionId)).toEqual({
        state: "failed",
        response: { text: TEXTS.unknown },
      });
    } finally {
      await again.stopApp();
    }
  });

  it("queues one job for the same webhook-id twice", async () => {
    world = await startWorld();
    const event = invoked("Twice");
    const id = randomUUID();
    expect(
      (await world.serverRequest("/chat/interactions", event, id)).status,
    ).toBe(202);
    expect(
      (await world.serverRequest("/chat/interactions", event, id)).status,
    ).toBe(202);
    await world.app.idle();
    expect(world.chat.threadsOf(FORUM)).toHaveLength(1);
  });

  it("rejects an unknown custom_id", async () => {
    world = await startWorld();
    const { card } = await openForm(world);
    const form = submitted(card, { body: "x" }, `issue:${randomUUID()}`);
    // The card is found by its message when the custom_id is unknown; an unknown message too:
    (
      form.payload as { interaction: { message_id: string } }
    ).interaction.message_id = randomUUID();
    await world.serverRequest("/chat/interactions", form);
    await world.app.idle();
    const id = (form.payload as { interaction: { id: string } }).interaction.id;
    expect(world.chat.responses.get(id)).toEqual({
      state: "rejected",
      response: { text: TEXTS.expired },
    });
    expect(world.github.issues()).toHaveLength(0);
  });

  it("answers autocomplete with an empty list", async () => {
    world = await startWorld();
    const response = await world.serverRequest(
      "/chat/interactions",
      envelope("command.autocomplete_requested", {
        request_id: randomUUID(),
        command_name: "issue",
      }),
    );
    expect(response.status).toBe(200);
    expect(await response.json()).toEqual({ choices: [] });
  });

  it("ignores /issue in a forum that is not configured and unknown requests", async () => {
    world = await startWorld();
    const other = randomUUID();
    expect(
      (await world.serverRequest("/chat/interactions", invoked("x", other)))
        .status,
    ).toBe(202);
    const unknown = envelope("command.invoked", {
      command_name: "other",
      channel_id: FORUM,
      options: [],
    });
    expect(
      (await world.serverRequest("/chat/interactions", unknown)).status,
    ).toBe(202);
    await world.app.idle();
    expect(world.chat.threads.size).toBe(0);
    expect(world.logs.some((l) => l.reason === "channel_not_mapped")).toBe(
      true,
    );
    expect(world.logs.some((l) => l.reason === "unknown_request")).toBe(true);
  });

  it("refuses a request that is not signed by the server", async () => {
    world = await startWorld();
    const body = JSON.stringify({ ...invoked("x"), id: randomUUID() });
    const response = await fetch(`${world.app.url}/chat/interactions`, {
      method: "POST",
      headers: { ...world.chat.sign(body), "webhook-signature": "v1a,AAAA" },
      body,
    });
    expect(response.status).toBe(401);
    await world.app.idle();
    expect(world.chat.threads.size).toBe(0);
  });
});
