// SPDX-License-Identifier: Apache-2.0
//
// Commands: reconciliation of the declared list, the limits and autocomplete; port of
// `sdk/python/tests/test_commands.py`. docs/DECISIONS.md D-M2-29-12: own commands are those of
// `listChannelCommands` over `listServedChannels` owned by the agent; missing ones are
// registered, changed ones updated with `If-Match` of their version, undeclared ones removed; a
// `409 conflict` on register is skipped; `undefined` makes no request. Autocomplete runs under a
// guard that ends at `expires_at`; too many choices throw and nothing is sent.

import { afterEach, beforeEach, describe, expect, it } from "vitest";

import { createApiClient, type ApiClient } from "../src/client.js";
import {
  AutocompleteTooManyChoicesError,
  MemoryCommandStore,
  autocompleteRequest,
  guardedAutocomplete,
  reconcileCommands,
  restAutocompleteSender,
  CommandSpec,
  type AutocompleteRequest,
  type CommandSpecOptions,
} from "../src/commands/index.js";
import { DEFAULTS, resolveSettings } from "../src/config.js";
import { ForbiddenError } from "../src/errors.js";
import type { components } from "../src/generated/openapi.js";
import { silentLogger } from "../src/logger.js";
import { FakeClock } from "../src/testing/clock.js";
import { FakeServer } from "../src/testing/fakeserver.js";

type S = components["schemas"];
type Json = Record<string, unknown>;

const BASE = "https://example.com/api/v1";
const AGENT = "01900000-0000-7000-8000-000000000301";
const OTHER = "01900000-0000-7000-8000-000000000302";
const CHANNEL_1 = "01900000-0000-7000-8000-000000000103";
const CHANNEL_2 = "01900000-0000-7000-8000-000000000113";
const SPACE = "01900000-0000-7000-8000-000000000104";

function id(n: number): string {
  return `01900000-0000-7000-8000-${n.toString().padStart(12, "0")}`;
}

function command(
  n: number,
  name: string,
  description: string,
  owner: string,
  version = 1,
): S["Command"] {
  return {
    id: id(n),
    owner_principal_id: owner,
    name,
    description,
    scope_kind: "instance",
    options: [],
    required_level: "trigger",
    control: false,
    version,
  };
}

function channel(channelId: string): S["AgentChannelConfig"] {
  return {
    channel_id: channelId,
    space_id: SPACE,
    behavior: {
      require_mention: true,
      thread_require_mention: false,
      free_response: false,
      auto_thread: true,
      allow_mentioned_agents: true,
      history_backfill_limit: 50,
      prompt: "",
      skills: [],
      settings_version: 1,
    },
  };
}

function problem(status: number, code: string): Response {
  return Response.json(
    { type: "about:blank", title: code, status, code },
    { status, headers: { "Content-Type": "application/problem+json" } },
  );
}

interface Seen {
  readonly method: string;
  readonly path: string;
  readonly headers: Headers;
  readonly body: Json | undefined;
}

/** A fetch playing the five command operations an adapter uses. */
class CommandServer {
  readonly requests: Seen[] = [];
  readonly conflictOn = new Set<string>();
  readonly forbiddenOn = new Set<string>();
  readonly goneOnDelete = new Set<string>();
  private next = 0x900;

  constructor(readonly listed: Record<string, S["Command"][]>) {}

  readonly fetch = async (request: Request): Promise<Response> => {
    const text = await request.text();
    const body = text ? (JSON.parse(text) as Json) : undefined;
    const path = new URL(request.url).pathname.replace(/^\/api\/v1/, "");
    const { method } = request;
    this.requests.push({ method, path, headers: request.headers, body });
    if (method === "GET" && path === "/agents/me/channels")
      return Response.json({ items: Object.keys(this.listed).map(channel) });
    if (method === "GET" && /^\/channels\/[^/]+\/commands$/.test(path))
      return Response.json({
        items: this.listed[path.split("/")[2] ?? ""] ?? [],
      });
    if (method === "POST" && path === "/commands") {
      const name = String(body?.name);
      if (this.conflictOn.has(name)) return problem(409, "conflict");
      if (this.forbiddenOn.has(name)) return problem(403, "forbidden");
      this.next += 1;
      return Response.json(
        command(this.next, name, String(body?.description), AGENT),
        {
          status: 201,
        },
      );
    }
    if (method === "PUT" && path.startsWith("/commands/"))
      return Response.json(
        command(
          Number(path.slice(-12)),
          String(body?.name),
          String(body?.description),
          AGENT,
          9,
        ),
      );
    if (method === "DELETE" && path.startsWith("/commands/")) {
      if (this.goneOnDelete.has(path.slice("/commands/".length)))
        return problem(404, "not_found");
      return new Response(null, { status: 204 });
    }
    if (method === "POST" && path.startsWith("/command-autocompletions/"))
      return new Response(null, { status: 204 });
    return problem(404, "not_found");
  };

  calls(method: string): Seen[] {
    return this.requests.filter((r) => r.method === method);
  }

  writes(): Seen[] {
    return this.requests.filter((r) => r.method !== "GET");
  }
}

function client(server: CommandServer): ApiClient {
  return createApiClient({
    baseUrl: BASE,
    token: "example-api-token",
    fetch: server.fetch,
    clock: new FakeClock(),
    settings: { retryMaxAttempts: 1 },
    logger: silentLogger,
  });
}

function world(): CommandServer {
  const same = command(1, "same", "Unchanged", AGENT);
  return new CommandServer({
    [CHANNEL_1]: [
      same,
      command(2, "changed", "Old text", AGENT, 4),
      command(3, "stale", "No longer declared", AGENT),
      command(4, "foreign", "Owned by Texter", OTHER),
      command(5, "taken", "Texter got the name first", OTHER),
    ],
    // The same instance-wide command shows up in every channel; it counts once.
    [CHANNEL_2]: [
      same,
      command(6, "elsewhere", "Texter, second channel", OTHER),
    ],
  });
}

const spec = (
  name: string,
  description: string,
  extra: CommandSpecOptions = {},
): CommandSpec => new CommandSpec(name, description, extra);

describe("reconcileCommands", () => {
  it("creates, updates and removes", async () => {
    const server = world();
    server.conflictOn.add("taken");
    const declared = [
      spec("same", "Unchanged"),
      spec("changed", "New text"),
      spec("new", "Brand new"),
      spec("taken", "Wanted, but the server refuses with conflict"),
    ];
    const result = await reconcileCommands(client(server), AGENT, declared);
    expect(result).toEqual({
      created: ["new"],
      updated: ["changed"],
      removed: ["stale"],
    });

    expect(
      server
        .calls("POST")
        .map((r) => r.body?.name)
        .sort(),
    ).toEqual(["new", "taken"]);
    const [put, ...morePuts] = server.calls("PUT");
    expect(morePuts).toEqual([]);
    expect(put?.path).toBe(`/commands/${id(2)}`);
    expect(put?.headers.get("If-Match")).toBe('"4"');
    expect(put?.body?.description).toBe("New text");
    const [del, ...moreDeletes] = server.calls("DELETE");
    expect(moreDeletes).toEqual([]);
    expect(del?.path).toBe(`/commands/${id(3)}`);
    // Commands of other owners are never touched, and the unchanged one gets no write.
    const untouched = new Set([id(1), id(4), id(5), id(6)]);
    expect(
      server
        .writes()
        .filter((r) => untouched.has(r.path.split("/").pop() ?? "")),
    ).toEqual([]);
    const listed = server.calls("GET").map((r) => r.path);
    expect(
      listed.filter((p) => p === `/channels/${CHANNEL_1}/commands`),
    ).toHaveLength(1);
    expect(
      listed.filter((p) => p === `/channels/${CHANNEL_2}/commands`),
    ).toHaveLength(1);
  });

  it("updates a command whose options or control flag were dropped", async () => {
    const stop: S["Command"] = {
      ...command(7, "stop", "Stop", AGENT, 3),
      control: true,
      options: [
        { name: "reason", description: "Why", type: "string", required: false },
      ],
    };
    const server = new CommandServer({ [CHANNEL_1]: [stop] });
    const result = await reconcileCommands(client(server), AGENT, [
      spec("stop", "Stop"),
    ]);
    expect(result.updated).toEqual(["stop"]);
    const [put] = server.calls("PUT");
    expect(put?.headers.get("If-Match")).toBe('"3"');
    expect(put?.body).toEqual({
      name: "stop",
      description: "Stop",
      options: [],
      control: false,
    });
  });

  it("raises another refusal", async () => {
    const server = world();
    server.forbiddenOn.add("new");
    await expect(
      reconcileCommands(client(server), AGENT, [spec("new", "Brand new")]),
    ).rejects.toBeInstanceOf(ForbiddenError);
  });

  it("makes no request without declared commands and removes all for an empty list", async () => {
    let server = world();
    expect(await reconcileCommands(client(server), AGENT, undefined)).toEqual({
      created: [],
      updated: [],
      removed: [],
    });
    expect(server.requests).toEqual([]);

    server = world();
    const result = await reconcileCommands(client(server), AGENT, []);
    expect(result).toEqual({
      created: [],
      updated: [],
      removed: ["same", "changed", "stale"],
    });
    expect(server.calls("POST")).toEqual([]);
    expect(server.calls("PUT")).toEqual([]);
    expect(
      server
        .calls("DELETE")
        .map((r) => r.path.split("/").pop())
        .sort(),
    ).toEqual([id(1), id(2), id(3)].sort());
  });

  it("removes an undeclared command the store recorded from an earlier reconcile", async () => {
    const store = new MemoryCommandStore();
    const first = world();
    await reconcileCommands(
      client(first),
      AGENT,
      [spec("same", "Unchanged"), spec("hidden", "Soon out of sight")],
      { store },
    );
    const recorded = await store.load();
    expect(Object.keys(recorded).sort()).toEqual(["hidden", "same"]);
    expect(recorded.same).toBe(id(1));
    const hiddenId = recorded.hidden ?? "";

    // The command is no longer listed in a served channel (its channel was unbound), but the
    // store still knows it: it is removed by its ID.
    const second = world();
    const result = await reconcileCommands(
      client(second),
      AGENT,
      [spec("same", "Unchanged")],
      {
        store,
      },
    );
    expect([...result.removed].sort()).toEqual(["changed", "hidden", "stale"]);
    expect(
      second
        .calls("DELETE")
        .map((r) => r.path.split("/").pop())
        .sort(),
    ).toEqual([id(2), id(3), hiddenId].sort());
    expect(await store.load()).toEqual({ same: id(1) });

    // A recorded command the server no longer shows (404) is forgotten, not reported.
    await store.save({ same: id(1), ghost: id(0x777) });
    const third = world();
    third.goneOnDelete.add(id(0x777));
    const quiet = await reconcileCommands(
      client(third),
      AGENT,
      [
        spec("same", "Unchanged"),
        spec("changed", "Old text"),
        spec("stale", "No longer declared"),
      ],
      { store },
    );
    expect(quiet).toEqual({ created: [], updated: [], removed: [] });
    expect(third.calls("DELETE").map((r) => r.path.split("/").pop())).toEqual([
      id(0x777),
    ]);
    expect(await store.load()).toEqual({
      same: id(1),
      changed: id(2),
      stale: id(3),
    });
  });

  it("checks the limits before any request", async () => {
    const server = world();
    const tight = resolveSettings({ commandDescriptionMaxChars: 5 });
    await expect(
      reconcileCommands(client(server), AGENT, [spec("long", "123456")], {
        settings: tight,
      }),
    ).rejects.toThrow(/longer than the limit/);
    const few = resolveSettings({ commandMaxPerPrincipal: 2 });
    await expect(
      reconcileCommands(
        client(server),
        AGENT,
        [spec("a", "A"), spec("b", "B"), spec("c", "C")],
        { settings: few },
      ),
    ).rejects.toThrow(/command\.max_per_principal/);
    expect(server.requests).toEqual([]);
  });
});

describe("CommandSpec.toRegister", () => {
  it("holds command.description_max_chars", () => {
    const limit = DEFAULTS.commandDescriptionMaxChars;
    expect(spec("ok", "x".repeat(limit)).toRegister().description).toBe(
      "x".repeat(limit),
    );
    expect(() => spec("long", "x".repeat(limit + 1)).toRegister()).toThrow(
      /longer than the limit/,
    );
    const tight = resolveSettings({ commandDescriptionMaxChars: 5 });
    expect(spec("ok", "12345").toRegister(tight).name).toBe("ok");
    expect(() => spec("long", "123456").toRegister(tight)).toThrow(RangeError);
  });

  it("maps a spec to CommandRegister", () => {
    expect(
      spec("deploy", "Deploy the service", {
        scopeKind: "channel",
        scopeId: CHANNEL_1,
        requiredLevel: "trigger",
        control: true,
      }).toRegister(),
    ).toEqual({
      name: "deploy",
      description: "Deploy the service",
      scope_kind: "channel",
      scope_id: CHANNEL_1,
      required_level: "trigger",
      control: true,
    });
    expect(spec("plain", "Plain").toRegister()).toEqual({
      name: "plain",
      description: "Plain",
    });
  });
});

describe("reconcileCommands against the FakeServer", () => {
  let fake: FakeServer;

  beforeEach(async () => {
    fake = new FakeServer();
    await fake.start();
  });

  afterEach(async () => {
    await fake.stop();
  });

  it("creates, updates and removes the agent's own commands", async () => {
    const api = createApiClient({
      baseUrl: fake.baseUrl,
      token: fake.credentials.apiToken,
      dpopKey: fake.credentials.dpop,
      clock: fake.clock,
      settings: fake.sdkSettings,
    });
    await fake.registerCommand("stale");
    await fake.registerCommand("changed");
    const first = await reconcileCommands(api, fake.agentId, [
      spec("changed", "The new text"),
      spec("status", "Show the run status"),
    ]);
    expect(first).toEqual({
      created: ["status"],
      updated: ["changed"],
      removed: ["stale"],
    });
    const names = [...fake.commands.values()].map((c) => [
      c.name,
      c.description,
      c.version,
    ]);
    expect(names.sort()).toEqual([
      ["changed", "The new text", 2],
      ["status", "Show the run status", 1],
    ]);
    // A second reconcile with the same list changes nothing.
    const again = await reconcileCommands(api, fake.agentId, [
      spec("changed", "The new text"),
      spec("status", "Show the run status"),
    ]);
    expect(again).toEqual({ created: [], updated: [], removed: [] });
  });
});

// Autocomplete ----------------------------------------------------------------------------------

function request(clock: FakeClock, expiresInMs: number): AutocompleteRequest {
  return {
    requestId: id(0x602),
    commandId: id(0x501),
    option: "target",
    value: "st",
    options: [],
    channelId: CHANNEL_1,
    threadId: undefined,
    userId: id(0x201),
    expiresAt: clock.now() + expiresInMs,
  };
}

class Sender {
  readonly sent: [string, S["CommandChoice"][]][] = [];

  readonly send = (
    requestId: string,
    choices: readonly S["CommandChoice"][],
  ): Promise<void> => {
    this.sent.push([requestId, [...choices]]);
    return Promise.resolve();
  };
}

async function sleeping(clock: FakeClock, n: number): Promise<void> {
  for (let i = 0; i < 1000; i++) {
    if (clock.sleepers === n) return;
    await new Promise((r) => setImmediate(r));
  }
  throw new Error("sleepers not reached");
}

function choices(n: number): S["CommandChoice"][] {
  return Array.from({ length: n }, (_, i) => ({
    name: `choice ${i}`,
    value: `v${i}`,
  }));
}

describe("guardedAutocomplete", () => {
  it("sends the choices of a handler that answers before expires_at", async () => {
    const clock = new FakeClock();
    const sender = new Sender();
    const req = request(clock, 3_000);
    const task = guardedAutocomplete(
      async () => {
        await clock.sleep(2_000);
        return choices(2);
      },
      req,
      sender.send,
      { clock },
    );
    await sleeping(clock, 2);
    await clock.advance(2_000);
    expect(await task).toBe(true);
    expect(sender.sent).toEqual([[req.requestId, choices(2)]]);
    // The guard is gone: moving past the deadline sends nothing more.
    await clock.advance(5_000);
    expect(sender.sent).toHaveLength(1);
    expect(clock.sleepers).toBe(0);
  });

  it("aborts the handler at expires_at and then sends nothing", async () => {
    const clock = new FakeClock();
    const sender = new Sender();
    let aborted = false;
    const task = guardedAutocomplete(
      async (_req, signal) => {
        signal.addEventListener("abort", () => {
          aborted = true;
        });
        await clock.sleep(10_000);
        return choices(1);
      },
      request(clock, 3_000),
      sender.send,
      { clock, logger: silentLogger },
    );
    await sleeping(clock, 2);
    await clock.advance(2_750);
    let done = false;
    void task.then(() => (done = true));
    await new Promise((r) => setImmediate(r));
    expect(done).toBe(false);
    await clock.advance(250);
    expect(await task).toBe(false);
    expect(aborted).toBe(true);
    // The handler finishes later: its choices are never sent.
    await clock.advance(10_000);
    expect(sender.sent).toEqual([]);
  });

  it("sends nothing for an expired request and never calls the handler", async () => {
    const clock = new FakeClock();
    const sender = new Sender();
    const calls: AutocompleteRequest[] = [];
    const handler = (
      req: AutocompleteRequest,
    ): Promise<S["CommandChoice"][]> => {
      calls.push(req);
      return Promise.resolve(choices(1));
    };
    expect(
      await guardedAutocomplete(handler, request(clock, 0), sender.send, {
        clock,
      }),
    ).toBe(false);
    expect(calls).toEqual([]);
    expect(sender.sent).toEqual([]);
  });

  it("refuses more than command.autocomplete_max_choices choices", async () => {
    const clock = new FakeClock();
    const sender = new Sender();
    const limit = DEFAULTS.autocompleteMaxChoices;
    await expect(
      guardedAutocomplete(
        () => Promise.resolve(choices(limit + 1)),
        request(clock, 3_000),
        sender.send,
        {
          clock,
        },
      ),
    ).rejects.toBeInstanceOf(AutocompleteTooManyChoicesError);
    expect(sender.sent).toEqual([]);

    const exactly = (): Promise<S["CommandChoice"][]> =>
      Promise.resolve(choices(limit));
    expect(
      await guardedAutocomplete(exactly, request(clock, 3_000), sender.send, {
        clock,
      }),
    ).toBe(true);
    expect(sender.sent[0]?.[1]).toHaveLength(limit);

    await expect(
      guardedAutocomplete(exactly, request(clock, 3_000), sender.send, {
        clock,
        settings: resolveSettings({ autocompleteMaxChoices: 2 }),
      }),
    ).rejects.toBeInstanceOf(RangeError);
    expect(sender.sent).toHaveLength(1);
  });

  it("propagates a handler error and sends nothing", async () => {
    const clock = new FakeClock();
    const sender = new Sender();
    await expect(
      guardedAutocomplete(
        () => Promise.reject(new Error("broken")),
        request(clock, 3_000),
        sender.send,
        {
          clock,
        },
      ),
    ).rejects.toThrow("broken");
    expect(sender.sent).toEqual([]);
    expect(clock.sleepers).toBe(0);
  });
});

describe("autocomplete request and REST answer", () => {
  it("reads the payload of command.autocomplete_requested", () => {
    const payload: S["CommandAutocompleteRequestedPayload"] = {
      request_id: id(0x602),
      command_id: id(0x501),
      channel_id: CHANNEL_1,
      user_id: id(0x201),
      option: "target",
      value: "st",
      options: [{ name: "env", value: "prod" }],
      expires_at: "2026-01-01T00:00:03.000Z",
    };
    expect(autocompleteRequest(payload)).toEqual({
      requestId: id(0x602),
      commandId: id(0x501),
      option: "target",
      value: "st",
      options: [{ name: "env", value: "prod" }],
      channelId: CHANNEL_1,
      threadId: undefined,
      userId: id(0x201),
      expiresAt: Date.UTC(2026, 0, 1, 0, 0, 3),
    });
    expect(
      autocompleteRequest({
        ...payload,
        thread_id: id(0x701),
        options: undefined,
      } as never).threadId,
    ).toBe(id(0x701));
    expect(() =>
      autocompleteRequest({ ...payload, expires_at: "soon" }),
    ).toThrow(RangeError);
  });

  it("answers through answerAutocomplete", async () => {
    const server = world();
    await restAutocompleteSender(client(server))(id(0x602), choices(2));
    const [answer] = server.calls("POST");
    expect(answer?.path).toBe(`/command-autocompletions/${id(0x602)}/answer`);
    expect(answer?.body).toEqual({ choices: choices(2) });
  });
});
