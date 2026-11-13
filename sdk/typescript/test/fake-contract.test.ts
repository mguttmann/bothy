// SPDX-License-Identifier: Apache-2.0
//
// The FakeServer against the contract (docs/api/openapi.yaml): every route it serves is an
// operation of the contract with that method, path and `operationId`, and every answer it gives
// has the form the contract describes for that operation and status. A new route fails the gate
// until a request of this file exercises it. The check reads the schemas of the contract:
// types, required and unknown members, enums, patterns, bounds and the formats `uuid` and
// `date-time`; other formats are not checked.

import { createHash } from "node:crypto";
import { readFileSync } from "node:fs";

import { parse } from "yaml";
import { afterAll, beforeAll, describe, expect, it } from "vitest";

import { createApiClient, unwrap, type ApiClient } from "../src/client.js";
import type { components } from "../src/generated/openapi.js";
import { uuidv7 } from "../src/ids.js";
import { silentLogger } from "../src/logger.js";
import { FakeServer, ROUTES } from "../src/testing/fakeserver.js";

type S = components["schemas"];
type Schema = Record<string, unknown>;

interface Spec {
  paths: Record<string, Record<string, Schema>>;
  components: {
    schemas: Record<string, Schema>;
    responses: Record<string, Schema>;
  };
}

// Parsed once at import: the specification is large.
const SPEC = parse(
  readFileSync(
    new URL("../../../docs/api/openapi.yaml", import.meta.url),
    "utf8",
  ),
) as Spec;

const API = "/api/v1";
// The formats the check reads: a lowercase canonical UUID and an RFC 3339 timestamp.
const FORMATS: Readonly<Record<string, RegExp>> = {
  uuid: /^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$/,
  "date-time":
    /^\d{4}-\d{2}-\d{2}T\d{2}:\d{2}:\d{2}(\.\d+)?(Z|[+-]\d{2}:\d{2})$/,
};

function isObject(value: unknown): value is Record<string, unknown> {
  return typeof value === "object" && value !== null && !Array.isArray(value);
}

/** Follows `$ref` into `components`. */
function resolve(schema: Schema): Schema {
  let current = schema;
  while (typeof current.$ref === "string") {
    const [, , kind, name] = current.$ref.split("/");
    const found = (
      SPEC.components as unknown as Record<string, Record<string, Schema>>
    )[kind ?? ""]?.[name ?? ""];
    if (found === undefined) throw new Error(`unresolved ${current.$ref}`);
    current = found;
  }
  return current;
}

function hasType(value: unknown, type: string): boolean {
  switch (type) {
    case "object":
      return isObject(value);
    case "array":
      return Array.isArray(value);
    case "string":
      return typeof value === "string";
    case "boolean":
      return typeof value === "boolean";
    case "integer":
      return typeof value === "number" && Number.isInteger(value);
    case "number":
      return typeof value === "number";
    case "null":
      return value === null;
    default:
      return false;
  }
}

/** The places where `value` leaves `schema`, each as `<pointer>: <rule>`. */
function violations(value: unknown, schema: Schema, at = ""): string[] {
  const s = resolve(schema);
  const out: string[] = [];
  for (const part of (s.allOf ?? []) as Schema[])
    out.push(...violations(value, part, at));
  for (const key of ["oneOf", "anyOf"] as const) {
    const alternatives = s[key] as Schema[] | undefined;
    if (alternatives === undefined) continue;
    const found = alternatives.map((a) => violations(value, a, at));
    if (!found.some((v) => v.length === 0)) {
      const nearest = [...found].sort((a, b) => a.length - b.length)[0] ?? [];
      out.push(
        `${at}: matches no alternative of ${key} (${nearest.join("; ")})`,
      );
    }
  }
  if ("const" in s && value !== s.const) out.push(`${at}: not the constant`);
  if (Array.isArray(s.enum) && !s.enum.includes(value))
    out.push(`${at}: not in the enum`);
  const types =
    typeof s.type === "string" ? [s.type] : ((s.type as string[]) ?? []);
  if (types.length > 0 && !types.some((t) => hasType(value, t))) {
    out.push(`${at}: not of type ${types.join("|")}`);
    return out;
  }
  if (typeof value === "string") {
    if (
      typeof s.pattern === "string" &&
      !new RegExp(s.pattern, "u").test(value)
    )
      out.push(`${at}: pattern`);
    const format = FORMATS[typeof s.format === "string" ? s.format : ""];
    if (format !== undefined && !format.test(value))
      out.push(`${at}: not of format ${String(s.format)}`);
  }
  if (typeof value === "number") {
    if (typeof s.minimum === "number" && value < s.minimum)
      out.push(`${at}: below the minimum`);
    if (typeof s.maximum === "number" && value > s.maximum)
      out.push(`${at}: above the maximum`);
  }
  if (Array.isArray(value) && isObject(s.items))
    for (const [i, item] of value.entries())
      out.push(...violations(item, s.items, `${at}/${i}`));
  if (isObject(value)) {
    const properties = (s.properties ?? {}) as Record<string, Schema>;
    for (const name of (s.required ?? []) as string[])
      if (!(name in value)) out.push(`${at}/${name}: required`);
    for (const [name, member] of Object.entries(value)) {
      const known = properties[name];
      if (known !== undefined)
        out.push(...violations(member, known, `${at}/${name}`));
      else if (s.additionalProperties === false)
        out.push(`${at}/${name}: unknown member`);
      else if (isObject(s.additionalProperties))
        out.push(
          ...violations(member, s.additionalProperties, `${at}/${name}`),
        );
    }
  }
  return out;
}

/** One answer of the FakeServer as the wire carried it. */
interface Exchange {
  readonly method: string;
  readonly path: string;
  readonly status: number;
  readonly contentType: string;
  readonly text: string;
}

const COMPILED = ROUTES.map(
  ([method, path, operationId]) =>
    [
      method,
      new RegExp(`^${path.replace(/\{(\w+)\}/g, "[^/]+")}$`),
      path,
      operationId,
    ] as const,
);

function routeOf(exchange: Exchange): readonly [string, string] | undefined {
  const found = COMPILED.find(
    ([method, pattern]) =>
      method === exchange.method &&
      pattern.test(exchange.path.slice(API.length)),
  );
  return found === undefined ? undefined : [found[2], found[3]];
}

/** The schema the contract gives for the answer, or `undefined` when it has no body. */
function answerSchema(
  path: string,
  exchange: Exchange,
): Schema | undefined | "undocumented" {
  const operation = SPEC.paths[path]?.[exchange.method.toLowerCase()];
  const responses = (operation?.responses ?? {}) as Record<string, Schema>;
  const documented = responses[String(exchange.status)] ?? responses.default;
  if (documented === undefined) return "undocumented";
  const content = resolve(documented).content as
    Record<string, { schema?: Schema }> | undefined;
  if (content === undefined) return undefined;
  const media = exchange.contentType.split(";")[0]?.trim() ?? "";
  return content[media]?.schema ?? "undocumented";
}

describe("the routes of the FakeServer", () => {
  it("are operations of openapi.yaml with the same method, path and operationId", () => {
    expect(ROUTES.length).toBeGreaterThan(0);
    for (const [method, path, operationId] of ROUTES)
      expect(
        [method, path, SPEC.paths[path]?.[method.toLowerCase()]?.operationId],
        `${method} ${path}`,
      ).toEqual([method, path, operationId]);
    const ids = ROUTES.map(([, , operationId]) => operationId);
    expect(new Set(ids).size).toBe(ids.length);
    const routes = ROUTES.map(([method, path]) => `${method} ${path}`);
    expect(new Set(routes).size).toBe(routes.length);
  });
});

describe("the answers of the FakeServer", () => {
  let fake: FakeServer;
  let api: ApiClient;
  const exchanges: Exchange[] = [];

  beforeAll(async () => {
    fake = new FakeServer({ costs: true });
    await fake.start();
    api = createApiClient({
      baseUrl: fake.baseUrl,
      token: fake.credentials.apiToken,
      dpopKey: fake.credentials.dpop,
      clock: fake.clock,
      logger: silentLogger,
      settings: { ...fake.sdkSettings, retryMaxAttempts: 1 },
      fetch: async (request: Request) => {
        const response = await globalThis.fetch(request);
        exchanges.push({
          method: request.method,
          path: new URL(request.url).pathname,
          status: response.status,
          contentType: response.headers.get("content-type") ?? "",
          text: await response.clone().text(),
        });
        return response;
      },
    });
    await everyOperation();
  });

  afterAll(async () => {
    await fake.stop();
  });

  const run = (run_id: string) => ({ params: { path: { run_id } } });
  const key = () => ({ "Idempotency-Key": uuidv7() });

  async function poll(): Promise<S["EventPage"]> {
    return unwrap(
      api.GET("/agents/me/consumers/{consumer}/events", {
        params: { path: { consumer: "contract" } },
      }),
      "pollAgentEvents",
    );
  }

  /** The token of the newest delivered event of `type`. */
  async function tokenOf(type: string): Promise<string> {
    const event = (await poll()).events.findLast((e) => e.type === type) as
      { invocation_token?: string } | undefined;
    if (event?.invocation_token === undefined)
      throw new Error(`no ${type} with a token`);
    return event.invocation_token;
  }

  function startBody(invocation_token: string): S["RunStart"] {
    return {
      invocation_token,
      billing_mode_used: "organization",
      tool_class_used: "scoped",
    };
  }

  /** Calls every operation of {@link ROUTES} once with a request the FakeServer accepts. */
  async function everyOperation(): Promise<void> {
    await api.GET("/instance/discovery");
    await api.GET("/instance/jwks");
    await api.GET("/instance/webhook-keys");
    await api.GET("/instance");
    await poll(); // the first poll creates the cursor at the head
    await api.POST("/agents/me/heartbeat", { body: { state: "online" } });
    await api.GET("/agents/me/triggerers");
    const channels = await unwrap(
      api.GET("/agents/me/channels"),
      "listServedChannels",
    );
    const channel_id = channels.items[0]?.channel_id ?? "";
    await api.GET("/agents/me/channels/{channel_id}/config", {
      params: { path: { channel_id } },
    });

    // A run from its trigger to its end, with a follow-up, a session and costs.
    const triggered = await fake.trigger();
    const first = await tokenOf("agent.invocation");
    await api.POST("/agents/me/consumers/{consumer}/ack", {
      params: { path: { consumer: "contract" } },
      body: { seq: (await poll()).next_cursor },
    });
    await api.GET("/agents/me/consumers/{consumer}", {
      params: { path: { consumer: "contract" } },
    });
    await api.GET("/runs/{run_id}/context", run(triggered.runId));
    const started = await unwrap(
      api.POST("/runs/{run_id}/start", {
        ...run(triggered.runId),
        body: startBody(first),
      }),
      "startRun",
    );
    await api.PATCH("/runs/{run_id}", {
      ...run(triggered.runId),
      body: {
        base_version: started.version,
        ops: [{ op: "add", path: "/status_line", value: "Thinking" }],
      },
    });
    const thread_id = triggered.threadId;
    await api.POST("/threads/{thread_id}/agent-sessions/report", {
      params: { path: { thread_id }, header: key() },
      body: { external_session_ref: "sess-7f3a", model: "model-large" },
    });
    await api.POST("/runs/{run_id}/await-input", {
      ...run(triggered.runId),
      body: { status_line: "Which file?" },
    });
    await fake.deliverFollowup(triggered.runId);
    await api.POST("/runs/{run_id}/resume", {
      ...run(triggered.runId),
      body: startBody(await tokenOf("agent.followup")),
    });
    await api.GET("/runs/{run_id}", run(triggered.runId));

    // A card with a component, an interaction with it, and an upload attached to a message.
    const card = await unwrap(
      api.POST("/threads/{thread_id}/messages", {
        params: { path: { thread_id } },
        body: {
          client_msg_id: uuidv7(),
          kind: "card",
          content_md: "Approve?",
          run_id: triggered.runId,
          components: [
            { custom_id: "approval.once", type: "button", definition: {} },
          ],
        },
      }),
      "createMessage",
    );
    const component = card.components[0];
    await api.PATCH("/components/{component_id}", {
      params: {
        path: { component_id: component?.id ?? "" },
        header: { "If-Match": `"${String(component?.version)}"`, ...key() },
      },
      body: { state: "disabled" },
    });
    const interaction_id = await fake.deliverInteraction(triggered.runId);
    await poll(); // the interaction.created event
    await api.POST("/interactions/{interaction_id}/respond", {
      params: { path: { interaction_id }, header: key() },
      body: { state: "acknowledged" },
    });
    const data = new TextEncoder().encode("the rest of the answer");
    const upload = async (): Promise<S["UploadSession"]> =>
      unwrap(
        api.POST("/channels/{channel_id}/uploads", {
          params: { path: { channel_id }, header: key() },
          body: {
            name: "answer.txt",
            size_bytes: data.length,
            sha256: createHash("sha256").update(data).digest("hex"),
          },
        }),
        "createUpload",
      );
    const session = await upload();
    const part = await unwrap(
      api.PUT("/uploads/{file_id}/parts/{part_number}", {
        params: {
          path: { file_id: session.file_id, part_number: 1 },
          header: {
            "Content-Digest": `sha-256=:${createHash("sha256").update(data).digest("base64")}:`,
          },
        },
        headers: { "Content-Type": "application/octet-stream" },
        body: data as unknown as string,
        bodySerializer: (body: string) => body,
      }),
      "uploadPart",
    );
    await api.POST("/uploads/{file_id}/complete", {
      params: { path: { file_id: session.file_id }, header: key() },
      body: { parts: [part] },
    });
    await api.POST("/threads/{thread_id}/messages", {
      params: { path: { thread_id } },
      body: {
        client_msg_id: uuidv7(),
        content_md: "The head of the answer",
        attachment_file_ids: [session.file_id],
      },
    });
    await api.DELETE("/uploads/{file_id}", {
      params: { path: { file_id: (await upload()).file_id } },
    });
    await api.POST("/runs/{run_id}/finish", {
      ...run(triggered.runId),
      body: {
        state: "completed",
        summary: "Answered",
        costs: [{ provider: "example", model: "model-large", currency: "USD" }],
      },
    });

    // A queued run whose token is renewed, and the run of a model command.
    const queued = await fake.trigger();
    await api.POST("/runs/{run_id}/invocation-token", {
      ...run(queued.runId),
      body: { invocation_token: await tokenOf("agent.invocation") },
    });
    const command = await unwrap(
      api.POST("/commands", {
        params: { header: key() },
        body: { name: "recap", description: "Recap the thread" },
      }),
      "registerCommand",
    );
    await api.PUT("/commands/{command_id}", {
      params: {
        path: { command_id: command.id },
        header: { "If-Match": `"${command.version}"`, ...key() },
      },
      body: { name: "recap", description: "Recap the whole thread" },
    });
    await api.GET("/channels/{channel_id}/commands", {
      params: { path: { channel_id } },
    });
    await fake.trigger("Recap please", { command: "recap" });
    await fake.invokeCommand("recap");
    await poll(); // the agent.invocation of the command run and command.invoked
    const world = (
      fake as unknown as {
        world: {
          autocompleteEvent(
            command: S["Command"],
            value: string,
            expiresInMs: number,
          ): { payload: { request_id: string } };
        };
      }
    ).world;
    const request_id = world.autocompleteEvent(command, "re", 60_000).payload
      .request_id;
    await api.POST("/command-autocompletions/{request_id}/answer", {
      params: { path: { request_id } },
      body: { choices: [{ name: "Recap", value: "recap" }] },
    });
    await api.DELETE("/commands/{command_id}", {
      params: { path: { command_id: command.id } },
    });

    // Refusals in the problem format.
    await api.GET("/runs/{run_id}", run(uuidv7()));
    await api.POST("/runs/{run_id}/start", {
      ...run(queued.runId),
      body: startBody("not-a-token"),
    });
    await api.POST("/runs/{run_id}/finish", {
      ...run(queued.runId),
      body: { state: "failed" },
    });
  }

  it("cover every route", () => {
    const answered = new Set(
      exchanges
        .filter((e) => e.status >= 200 && e.status < 300)
        .map((e) => routeOf(e)?.[1]),
    );
    expect([...answered].sort()).toEqual(
      ROUTES.map(([, , operationId]) => operationId).sort(),
    );
  });

  it("have the form openapi.yaml gives for the operation and status", () => {
    const found: string[] = [];
    for (const exchange of exchanges) {
      const route = routeOf(exchange);
      const where = `${exchange.method} ${exchange.path} ${exchange.status}`;
      if (route === undefined) {
        found.push(`${where}: no route`);
        continue;
      }
      const schema = answerSchema(route[0], exchange);
      if (schema === "undocumented") {
        found.push(`${where}: ${route[1]} documents no such answer`);
      } else if (schema === undefined) {
        if (exchange.text !== "")
          found.push(`${where}: a body, none documented`);
      } else {
        const body: unknown = JSON.parse(exchange.text);
        found.push(...violations(body, schema).map((v) => `${where} ${v}`));
      }
    }
    expect(found).toEqual([]);
    // Refusals were part of the run, so the problem format is checked too.
    expect(exchanges.some((e) => e.status >= 400)).toBe(true);
  });

  it("are checked by a check that finds what leaves the contract", () => {
    const schema = SPEC.components.schemas.Run ?? {};
    const exchange = exchanges.find(
      (e) => routeOf(e)?.[1] === "getRun" && e.status === 200,
    );
    const good = JSON.parse(exchange?.text ?? "{}") as Record<string, unknown>;
    expect(violations(good, schema)).toEqual([]);
    const { state, ...withoutState } = good;
    expect(state).toBe("running");
    expect(violations(withoutState, schema)).toEqual(["/state: required"]);
    expect(violations({ ...good, state: "paused" }, schema)).toEqual([
      "/state: not in the enum",
    ]);
    expect(violations({ ...good, version: "7" }, schema)).toEqual([
      "/version: not of type integer",
    ]);
    expect(violations({ ...good, id: "not-a-uuid" }, schema)).toEqual([
      "/id: not of format uuid",
    ]);
    expect(violations({ ...good, queued_at: "yesterday" }, schema)).toEqual([
      "/queued_at: not of format date-time",
    ]);
    const part = SPEC.components.schemas.UploadPartResult ?? {};
    expect(violations({ part_number: 1, etag: '"a"', more: 1 }, part)).toEqual([
      "/more: unknown member",
    ]);
    const page = SPEC.components.schemas.EventPage ?? {};
    const events = { events: [{ type: "agent.invocation" }] };
    expect(
      violations(events, page).some((v) =>
        v.startsWith("/events/0: matches no"),
      ),
    ).toBe(true);
  });

  it("deliver events of the form of their type", () => {
    const pages = exchanges.filter(
      (e) => routeOf(e)?.[1] === "pollAgentEvents" && e.status === 200,
    );
    const types = new Set(
      pages.flatMap((e) =>
        (JSON.parse(e.text) as S["EventPage"]).events.map((x) => x.type),
      ),
    );
    for (const type of [
      "agent.invocation",
      "agent.followup",
      "interaction.created",
      "command.invoked",
      "run.state_changed",
    ])
      expect(types.has(type as S["Event"]["type"]), type).toBe(true);
  });
});
