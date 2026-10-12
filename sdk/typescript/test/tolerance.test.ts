// SPDX-License-Identifier: Apache-2.0
//
// Gate G5 for TypeScript (docs/spec/06-api.md 15, ADR 0053): the shared fixtures of
// sdk/testdata/tolerance/ go through createApiClient with a fake fetch on
// https://example.com/api/v1; no request leaves the process.

import { readFileSync } from "node:fs";
import { describe, expect, it } from "vitest";

import {
  KNOWN_EVENT_TYPES,
  createApiClient,
  decodeEvent,
  knownEvents,
  toleranceMiddleware,
  type components,
  type Event,
  type EventPage,
} from "../src/index.js";

type Schemas = components["schemas"];
type Thread = Schemas["Thread"];

const BASE_URL = "https://example.com/api/v1";
const UNKNOWN_EVENT_TYPE = "future.example";

function fixture(name: string): Record<string, unknown> {
  const url = new URL(`../../testdata/tolerance/${name}.json`, import.meta.url);
  return JSON.parse(readFileSync(url, "utf8")) as Record<string, unknown>;
}

interface Answered {
  page: EventPage;
  urls: string[];
}

async function listEvents(name: string, path = "/events"): Promise<Answered> {
  const body = readFileSync(
    new URL(`../../testdata/tolerance/${name}.json`, import.meta.url),
    "utf8",
  );
  const urls: string[] = [];
  const client = createApiClient({
    baseUrl: BASE_URL,
    fetch: (request: Request) => {
      urls.push(request.url);
      return Promise.resolve(
        new Response(body, {
          status: 200,
          headers: { "content-type": "application/json; charset=utf-8" },
        }),
      );
    },
  });
  if (path === "/events") {
    const { data, error } = await client.GET("/events", {
      params: { query: { after: 0 } },
    });
    expect(error).toBeUndefined();
    if (data === undefined) throw new Error("no page");
    return { page: data, urls };
  }
  const { data, error } = await client.GET(
    "/agents/me/consumers/{consumer}/events",
    { params: { path: { consumer: "primary" } } },
  );
  expect(error).toBeUndefined();
  if (data === undefined) throw new Error("no page");
  return { page: data, urls };
}

function threadUpdated(event: Event): Schemas["ThreadUpdatedEvent"] {
  const decoded = decodeEvent(event);
  if (decoded.kind !== "known" || decoded.event.type !== "thread.updated") {
    throw new Error(`expected thread.updated, got ${JSON.stringify(decoded)}`);
  }
  return decoded.event;
}

function firstEvent(page: EventPage): Event {
  const [event] = page.events;
  if (event === undefined) throw new Error("empty page");
  return event;
}

describe("tolerance", () => {
  it("G5: unknown field is ignored", async () => {
    const raw = fixture("unknown_field");
    const { page, urls } = await listEvents("unknown_field");
    expect(urls).toEqual([`${BASE_URL}/events?after=0`]);
    expect(page.events).toHaveLength(1);
    const event = threadUpdated(firstEvent(page));
    const rawEvents = raw.events as Record<string, unknown>[];
    const rawEvent = rawEvents[0] as Record<string, unknown>;
    expect(event.type).toBe("thread.updated");
    expect(event.seq).toBe(rawEvent.seq);
    expect(event.id).toBe(rawEvent.id);
    expect(event.payload.thread.title).toBe("Invoice export fails");
    expect(page.next_cursor).toBe(raw.next_cursor);
    expect(page.sync_epoch).toBe(raw.sync_epoch);
    // Unknown members pass through untouched; every known member equals the fixture.
    expect(page).toEqual(raw);
    expect(knownEvents(page)).toEqual([event]);
  });

  it("G5: unknown enum value keeps its raw value", async () => {
    const { page } = await listEvents("unknown_enum_value");
    const event = threadUpdated(firstEvent(page));
    const state: string = event.payload.thread.state;
    const kind: string = event.payload.thread.kind;
    const actor: string | undefined = event.actor?.kind;
    expect([state, kind, actor]).toEqual([
      "frozen_example",
      "sidebar_example",
      "robot_example",
    ]);
    expect(knownEvents(page)).toHaveLength(1);
  });

  it("G5: unknown event type is skipped", async () => {
    const { page } = await listEvents("unknown_event_type");
    const known = knownEvents(page);
    expect(known.map((e) => [e.type, e.seq])).toEqual([
      ["thread.updated", 201],
      ["thread.updated", 203],
    ]);
    const middle = page.events[1];
    const decoded = decodeEvent(middle);
    expect(decoded).toEqual({
      kind: "unknown",
      type: UNKNOWN_EVENT_TYPE,
      seq: 202,
      raw: middle,
    });
    if (decoded.kind === "known") {
      expect.unreachable(`classified as ${decoded.event.type}`);
    }
    expect(page.next_cursor).toBeGreaterThanOrEqual(202);
    // The payload of the unknown event fits thread.moved_out; only `type` decides.
    const movedOut = decodeEvent({
      ...(middle as object),
      type: "thread.moved_out",
    });
    expect(movedOut.kind).toBe("known");
  });

  it("G5: null member is treated as absent", async () => {
    const raw = fixture("null_member");
    const rawEvent = (raw.events as Record<string, unknown>[])[0];
    expect(rawEvent?.correlation_id).toBeNull();
    const { page } = await listEvents("null_member");
    const event = threadUpdated(firstEvent(page));
    expect("correlation_id" in event).toBe(false);
    expect("closed_at" in event.payload.thread).toBe(false);
    expect(event.payload.thread.tag_ids).toEqual([]);
  });

  it("G5: pollAgentEvents is tolerant like listEvents", async () => {
    for (const name of [
      "unknown_field",
      "unknown_enum_value",
      "unknown_event_type",
      "null_member",
    ]) {
      const polled = await listEvents(name, "poll");
      const listed = await listEvents(name);
      expect(polled.page).toEqual(listed.page);
      expect(polled.urls).toEqual([
        `${BASE_URL}/agents/me/consumers/primary/events`,
      ]);
    }
  });

  it("known event types are unique", () => {
    expect(new Set(KNOWN_EVENT_TYPES).size).toBe(KNOWN_EVENT_TYPES.length);
    expect(KNOWN_EVENT_TYPES.length).toBeGreaterThan(0);
  });

  it("fixture unknowns are unknown to the spec", () => {
    // Each assignment must fail to compile (tsc in `make typecheck` and `generate-check`); if
    // the spec ever gains one of these values, the unused directive breaks the build.
    // @ts-expect-error the fixture's event type is not an Event type
    const eventType: Event["type"] = UNKNOWN_EVENT_TYPE;
    // @ts-expect-error not a ThreadState
    const state: Schemas["ThreadState"] = "frozen_example";
    // @ts-expect-error not a Thread kind
    const kind: Thread["kind"] = "sidebar_example";
    // @ts-expect-error not an ActorKind
    const actor: Schemas["ActorKind"] = "robot_example";
    // @ts-expect-error not a member of EventPage
    const pageMember: keyof EventPage = "future_page_member";
    // @ts-expect-error not a member of the event envelope
    const envelopeMember: keyof Schemas["ThreadUpdatedEvent"] =
      "future_envelope_member";
    // @ts-expect-error not a member of ThreadPayload
    const payloadMember: keyof Schemas["ThreadPayload"] =
      "future_payload_member";
    // @ts-expect-error not a member of Thread
    const threadMember: keyof Thread = "future_thread_member";

    const unknownField = fixture("unknown_field");
    const event = (unknownField.events as Record<string, unknown>[])[0] ?? {};
    const payload = event.payload as Record<string, unknown>;
    expect(pageMember in unknownField).toBe(true);
    expect(envelopeMember in event).toBe(true);
    expect(payloadMember in payload).toBe(true);
    expect(threadMember in (payload.thread as object)).toBe(true);

    const enumEvent =
      (fixture("unknown_enum_value").events as Record<string, unknown>[])[0] ??
      {};
    const thread = (enumEvent.payload as { thread: Record<string, unknown> })
      .thread;
    expect([
      thread.state,
      thread.kind,
      (enumEvent.actor as { kind: unknown }).kind,
    ]).toEqual([state, kind, actor]);
    const typed = (
      fixture("unknown_event_type").events as Record<string, unknown>[]
    )[1];
    expect(typed?.type).toBe(eventType);
  });
});

describe("decodeEvent", () => {
  it("classifies by type only", () => {
    expect(decodeEvent({ seq: 4 })).toEqual({
      kind: "unknown",
      type: "",
      seq: 4,
      raw: { seq: 4 },
    });
    expect(decodeEvent({ type: 7, seq: 1.5 })).toEqual({
      kind: "unknown",
      type: "",
      raw: { type: 7, seq: 1.5 },
    });
    expect(decodeEvent("text")).toEqual({
      kind: "unknown",
      type: "",
      raw: "text",
    });
    expect(decodeEvent(null)).toEqual({ kind: "unknown", type: "", raw: null });
    expect(decodeEvent(["thread.updated"])).toEqual({
      kind: "unknown",
      type: "",
      raw: ["thread.updated"],
    });
  });
});

describe("toleranceMiddleware", () => {
  async function through(response: Response): Promise<Response> {
    const onResponse = toleranceMiddleware.onResponse;
    if (onResponse === undefined) throw new Error("no onResponse");
    const request = new Request(`${BASE_URL}/events`);
    const result = await onResponse({
      request,
      response,
      options: {} as never,
      schemaPath: "/events",
      params: {},
      id: "test",
    });
    if (!(result instanceof Response)) throw new Error("no response");
    return result;
  }

  it("removes null members of problem documents and keeps null array items", async () => {
    const response = new Response(
      JSON.stringify({
        code: "rate_limited",
        detail: null,
        list: [null, { a: null, b: 1 }],
      }),
      {
        status: 429,
        statusText: "Too Many Requests",
        headers: {
          "content-type": "application/problem+json",
          "retry-after": "3",
        },
      },
    );
    const cleaned = await through(response);
    expect(cleaned.status).toBe(429);
    expect(cleaned.statusText).toBe("Too Many Requests");
    expect(cleaned.headers.get("retry-after")).toBe("3");
    expect(await cleaned.json()).toEqual({
      code: "rate_limited",
      list: [null, { b: 1 }],
    });
  });

  it("removes null members of structured JSON types such as the JWKS", async () => {
    const response = new Response(
      JSON.stringify({ keys: [{ kid: "k1", alg: null }] }),
      { headers: { "content-type": "application/jwk-set+json" } },
    );
    expect(await (await through(response)).json()).toEqual({
      keys: [{ kid: "k1" }],
    });
  });

  it("passes other media types and malformed JSON through unchanged", async () => {
    const text = new Response("a\nnull", {
      headers: { "content-type": "application/x-ndjson" },
    });
    expect(await through(text)).toBe(text);
    const broken = new Response("{", {
      headers: { "content-type": "application/json" },
    });
    const passed = await through(broken);
    expect(passed).toBe(broken);
    expect(await passed.text()).toBe("{");
    const empty = new Response(null, { status: 204 });
    expect(await through(empty)).toBe(empty);
    const blank = new Response("", {
      headers: { "content-type": "application/json" },
    });
    expect(await through(blank)).toBe(blank);
  });
});
