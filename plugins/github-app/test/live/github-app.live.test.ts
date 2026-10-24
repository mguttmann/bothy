// SPDX-License-Identifier: Apache-2.0
// The app against the real server (brief M2-49 Design note 19; PLAN 8 Exit criterion 9): the
// server of this tree through sdkserver with a TLS front for the app's addresses, the app and the
// Fake-GitHub API in this process, every operator step through the public API. Waits poll for a
// state with the patience as the only bound.

import { randomUUID } from "node:crypto";
import { mkdtemp, readFile, rm, writeFile } from "node:fs/promises";
import { tmpdir } from "node:os";
import { join } from "node:path";

import { uuidv7 } from "bothy-sdk";
import { afterAll, beforeAll, describe, expect, inject, it } from "vitest";

import { startApp, type RunningApp } from "../../src/app.js";
import { createLogger } from "../../src/log.js";
import { FakeGitHub } from "../fakes/github-api.js";
import { fixture, REPOSITORY } from "../helpers/fixtures.js";
import { eventually } from "../helpers/patience.js";
import { appFile } from "../helpers/repo.js";
import { testConfig } from "../helpers/world.js";
import { guardedFetch } from "../setup/no-network.js";
import { startLiveServer, type LiveServer } from "./sdkserver.js";

const HOST = "github-app.example.com";
const REQUIRED = ["messages:write", "commands:write", "components:write"];
const GITHUB_TOKEN = `ghp-live-${randomUUID()}`;
const WEBHOOK_SECRET = `github-secret-${randomUUID()}`;
// Generous bounds: an answer of the app is never late on a loaded machine.
const SERVER_CONFIG = {
  "app.interaction_timeout": "20s",
  "command.autocomplete_timeout": "20s",
  "http.request_timeout": "60s",
};

type Json = Record<string, unknown>;

let server: LiveServer | undefined;
let github: FakeGitHub | undefined;
let app: RunningApp | undefined;
let dir = "";
let forum = "";
let appId = "";
let manifest: Json = {};
const logs: Json[] = [];

function items(body: Json): Json[] {
  return Array.isArray(body.items) ? (body.items as Json[]) : [];
}

async function newToken(scopes?: string[]): Promise<void> {
  const created = await live().call("owner", "createToken", {
    path: { principal_id: appId },
    body: {
      name: `github-app-${uuidv7()}`,
      ...(scopes === undefined ? {} : { scopes }),
      expires_at: new Date(Date.now() + 86_400_000).toISOString(),
    },
  });
  expect(created.status).toBe(201);
  await writeFile(join(dir, "app-token"), String(created.body.secret), {
    mode: 0o600,
  });
}

function live(): LiveServer {
  if (server === undefined) throw new Error("the server is not running");
  return server;
}

async function runApp(): Promise<void> {
  await app?.stop();
  const config = testConfig({
    listen: "127.0.0.1:0",
    server_url: live().origin,
    token_file: join(dir, "app-token"),
    webhook_secret_file: join(dir, "webhook-secret"),
    github_token_file: join(dir, "github-token"),
    github_api_url: github?.url ?? "",
    repositories: [{ repository: REPOSITORY, channel_id: forum }],
    state_dir: join(dir, "state"),
  });
  app = await startApp(config, {
    fetch: guardedFetch,
    logger: createLogger({
      write: (line) => {
        logs.push(JSON.parse(line) as Json);
      },
    }),
  });
  await live().appBackend(app.port);
  if (github !== undefined)
    github.autoEcho = { url: `${app.url}/github/webhook` };
}

function deliver(event: string, payload: unknown): Promise<number> {
  if (github === undefined || app === undefined) throw new Error("not started");
  return github.deliver(`${app.url}/github/webhook`, event, payload);
}

async function threads(): Promise<Json[]> {
  const page = await live().call("alex", "listChannelThreads", {
    path: { channel_id: forum },
  });
  expect(page.status).toBe(200);
  return items(page.body);
}

async function messages(threadId: string): Promise<Json[]> {
  const page = await live().call("alex", "listThreadMessages", {
    path: { thread_id: threadId },
  });
  expect(page.status).toBe(200);
  return items(page.body);
}

async function threadTitled(prefix: string): Promise<Json | undefined> {
  return (await threads()).find((t) => String(t.title).startsWith(prefix));
}

beforeAll(async () => {
  dir = await mkdtemp(join(tmpdir(), "github-app-live-app-"));
  await writeFile(join(dir, "webhook-secret"), WEBHOOK_SECRET, { mode: 0o600 });
  await writeFile(join(dir, "github-token"), GITHUB_TOKEN, { mode: 0o600 });
  github = await FakeGitHub.start({
    token: GITHUB_TOKEN,
    secret: WEBHOOK_SECRET,
    repositories: [REPOSITORY],
  });
  server = await startLiveServer(
    inject("sdkserverBinary"),
    SERVER_CONFIG,
    HOST,
  );
  forum = await server.createForum("tickets");
  const base = `https://${HOST}:${String(server.frontPort)}`;
  manifest = JSON.parse(appFile("manifest.json")) as Json;
  manifest.delivery = {
    mode: "webhook",
    events_url: `${base}/chat/events`,
    interactions_url: `${base}/chat/interactions`,
  };
});

afterAll(async () => {
  await app?.stop();
  await github?.stop();
  await server?.stop();
  if (dir !== "") await rm(dir, { recursive: true, force: true });
});

describe("the GitHub app against the real server", () => {
  it("installs with exactly the required scopes as the documented operator steps", async () => {
    const valid = await live().call("owner", "validateManifest", {
      body: manifest,
    });
    expect(valid.status).toBe(200);
    expect(valid.body.valid).toBe(true);
    const short = await live().call("owner", "installApp", {
      body: {
        manifest,
        granted_scopes: ["messages:write", "commands:write"],
        reason: "test",
      },
    });
    expect(short.status).toBe(400);
    expect(short.body.code).toBe("validation_failed");
    expect(JSON.stringify(short.body.errors)).toContain("/granted_scopes");
    const installed = await live().call("owner", "installApp", {
      body: { manifest, granted_scopes: REQUIRED, reason: "test" },
    });
    expect(installed.status).toBe(201);
    appId = String(installed.body.id);
    const grant = await live().call("owner", "createGrant", {
      body: {
        subject_kind: "app",
        subject_id: appId,
        target_kind: "channel",
        target_id: forum,
        level: "write",
        reason: "test",
      },
    });
    expect(grant.status).toBe(201);
    await newToken();
    await runApp();
  });

  it("posts one thread per issue with messages of origin app", async () => {
    expect(
      await deliver("issues", fixture("issues", "opened", { number: 12 })),
    ).toBe(202);
    await eventually(
      async () => (await threadTitled("#12 ")) !== undefined,
      "the post of #12",
    );
    expect(
      await deliver(
        "issue_comment",
        fixture("issue_comment", "created", { number: 12, commentId: 41 }),
      ),
    ).toBe(202);
    expect(
      await deliver("issues", fixture("issues", "opened", { number: 13 })),
    ).toBe(202);
    await eventually(
      async () => (await threadTitled("#13 ")) !== undefined,
      "the post of #13",
    );
    const thread12 = await threadTitled("#12 ");
    await eventually(
      async () => (await messages(String(thread12?.id))).length === 2,
      "two messages in #12",
    );
    const posts = await threads();
    expect(posts.map((t) => String(t.title).split(" ")[0]).sort()).toEqual([
      "#12",
      "#13",
    ]);
    const inThread = await messages(String(thread12?.id));
    expect(inThread.map((m) => m.origin)).toEqual(["app", "app"]);
  });

  it("creates an issue from /issue with the form and answers in the post", async () => {
    const commands = await live().call("alex", "listChannelCommands", {
      path: { channel_id: forum },
    });
    expect(commands.status).toBe(200);
    const command = items(commands.body).find((c) => c.name === "issue");
    expect(command).toBeDefined();
    const before = (await threads()).length;
    const invoked = await live().call("alex", "invokeCommand", {
      path: { channel_id: forum, command_id: String(command?.id) },
      body: { options: [{ name: "title", value: "Crash on start" }] },
    });
    expect(invoked.status).toBe(202);
    await eventually(
      async () => (await threadTitled("Crash on start")) !== undefined,
      "the post of /issue",
    );
    const post = await threadTitled("Crash on start");
    let card: Json | undefined;
    await eventually(async () => {
      card = (await messages(String(post?.id))).find((m) => m.kind === "card");
      return card !== undefined;
    }, "the form card");
    const component = (card?.components as Json[])[0];
    const submitted = await live().call("alex", "createInteraction", {
      path: { component_id: String(component?.id) },
      body: {
        client_interaction_id: uuidv7(),
        values: { body: "It fails at boot." },
      },
    });
    expect(submitted.status).toBe(201);
    await eventually(
      () => (github?.issues().length ?? 0) === 1,
      "the issue at the Fake-GitHub",
    );
    expect(github?.issues()[0]?.body).toEqual({
      title: "Crash on start",
      body: "It fails at boot.\n\nCreated from chat.",
    });
    await eventually(
      async () =>
        (await messages(String(post?.id))).some((m) =>
          String(m.content_md).startsWith("Created GitHub issue"),
        ),
      "the confirmation in the post",
    );
    await github?.echoesDone();
    await app?.idle();
    expect(logs.some((l) => l.reason === "own_github_object")).toBe(true);
    expect((await messages(String(post?.id))).length).toBe(3);
    expect((await threads()).length).toBe(before + 1);
  });

  it("mirrors a person's reply once and never the app's own message", async () => {
    const scopes = await live().call("owner", "setAppScopes", {
      path: { app_id: appId },
      body: {
        granted_scopes: [...REQUIRED, "messages:read", "events:subscribe"],
        event_subscriptions: ["message.created"],
        reason: "test",
      },
    });
    expect(scopes.status).toBe(200);
    const delivery = manifest.delivery as Json;
    const subscription = await live().call(
      "owner",
      "createWebhookSubscription",
      {
        path: { app_id: appId },
        body: {
          url: String(delivery.events_url),
          event_types: ["message.created"],
        },
      },
    );
    expect(subscription.status).toBe(201);
    const subscriptionId = String((subscription.body.subscription as Json).id);
    await newToken();
    await runApp();

    const thread12 = await threadTitled("#12 ");
    const before = (await messages(String(thread12?.id))).length;
    expect(
      await deliver(
        "issue_comment",
        fixture("issue_comment", "created", { number: 12, commentId: 42 }),
      ),
    ).toBe(202);
    await eventually(
      async () => (await messages(String(thread12?.id))).length === before + 1,
      "the app's comment message",
    );
    await eventually(async () => {
      const page = await live().call("owner", "listWebhookDeliveries", {
        path: { subscription_id: subscriptionId },
      });
      return items(page.body).some((d) => d.status === "succeeded");
    }, "the delivery of the app's own message.created");
    await app?.idle();
    expect(github?.comments()).toHaveLength(0);
    expect(logs.some((l) => l.reason === "own_message")).toBe(true);

    const reply = await live().call("alex", "createMessage", {
      path: { thread_id: String(thread12?.id) },
      body: { client_msg_id: uuidv7(), content_md: "Thanks, looking into it." },
    });
    expect(reply.status).toBe(201);
    await eventually(
      () => (github?.comments().length ?? 0) === 1,
      "the mirrored comment",
    );
    expect(github?.comments()[0]?.path).toBe(
      "/repos/example-org/widgets/issues/12/comments",
    );
    expect(String(github?.comments()[0]?.body.body)).toContain(
      "Thanks, looking into it.",
    );
    await github?.echoesDone();
    await app?.idle();
    // The world's agent operates the space and may answer the reply with a run message; the app
    // adds only its one comment message, and the echo of the mirrored comment adds none.
    const after = (await messages(String(thread12?.id))).slice(before);
    expect(
      after
        .filter((m) => m.origin !== "agent")
        .map((m) => `${String(m.origin)}/${String(m.kind)}`),
    ).toEqual(["app/text", "human/text"]);
  });

  it("logs 403 scope_missing once when the token lacks messages:write", async () => {
    await newToken(["commands:write", "components:write"]);
    await runApp();
    const at = logs.length;
    expect(
      await deliver("issues", fixture("issues", "opened", { number: 14 })),
    ).toBe(202);
    await eventually(
      () => logs.slice(at).some((l) => l.reason === "scope_missing"),
      "the scope_missing line",
    );
    await app?.idle();
    expect(
      logs.slice(at).filter((l) => l.reason === "scope_missing"),
    ).toHaveLength(1);
    expect(await threadTitled("#14 ")).toBeUndefined();
  });

  it("left no request off the loopback host and wrote no secret into the logs", async () => {
    const all = JSON.stringify(logs);
    expect(all).not.toContain(GITHUB_TOKEN);
    expect(all).not.toContain(WEBHOOK_SECRET);
    expect(all).not.toContain(await readFile(join(dir, "app-token"), "utf8"));
    expect(globalThis.fetch).toBe(guardedFetch);
    for (const request of github?.requests ?? [])
      expect(request.path.startsWith("/repos/")).toBe(true);
  });
});
