// SPDX-License-Identifier: Apache-2.0
/**
 * Tolerance of the TypeScript SDK for what a newer server may send (gate G5,
 * docs/spec/06-api.md 15).
 *
 * TypeScript has no runtime validation, so an unknown field or enum value passes the generated
 * types by construction and arrives unchanged. This layer adds the two steps that need code:
 * events of an unknown type are told apart by `type` alone ({@link decodeEvent},
 * {@link knownEvents}), and a received JSON `null` member is removed before the caller sees it
 * ({@link toleranceMiddleware}, docs/spec/06-api.md 2 "Kein null", ADR 0053).
 *
 * Unlike the Python SDK, a page of events is passed through as received: the caller skips
 * unknown events with {@link knownEvents} or {@link decodeEvent} and still advances with
 * `next_cursor` (docs/spec/04-sync-protocol.md 7.4).
 *
 * @packageDocumentation
 */

import type { Middleware } from "openapi-fetch";

import type { components } from "./generated/openapi.js";

/** Any durable event of this SDK version (the `Event` union of docs/api/openapi.yaml). */
export type Event = components["schemas"]["Event"];

/** A page of events as `listEvents` and `pollAgentEvents` answer it. */
export type EventPage = components["schemas"]["EventPage"];

/** The discriminator values of {@link Event}. */
export type EventType = Event["type"];

/**
 * The discriminator values of the `Event` union, in the order of the spec's
 * `discriminator.mapping`.
 *
 * `tsc` holds the list equal to the generated types in both directions: `satisfies` rejects a
 * value the spec does not have, and the check below rejects a spec value missing here, so
 * `make -C sdk/typescript generate-check` fails until the list follows a spec change.
 */
export const KNOWN_EVENT_TYPES = [
  "message.created",
  "message.updated",
  "message.deleted",
  "reaction.added",
  "reaction.removed",
  "pin.added",
  "pin.removed",
  "thread.created",
  "thread.updated",
  "thread.closed",
  "thread.reopened",
  "thread.moved",
  "thread.moved_out",
  "agent_session.changed",
  "run.state_changed",
  "space.created",
  "space.updated",
  "space.archived",
  "channel.created",
  "channel.updated",
  "channel.settings_changed",
  "channel.archived",
  "command.changed",
  "file.updated",
  "user.updated",
  "agent.updated",
  "instance.updated",
  "status.updated",
  "grant.changed",
  "group.changed",
  "access.changed",
  "access.expiring",
  "device.pending",
  "session.revoked",
  "read_state.updated",
  "draft.updated",
  "draft.deleted",
  "settings.updated",
  "sidebar.updated",
  "sidebar_channel.updated",
  "notification_pref.updated",
  "quiet_hours.updated",
  "gdpr.request_updated",
  "gdpr.request_due",
  "import.progress",
  "agent.invocation",
  "agent.followup",
  "run.cancel_requested",
  "interaction.created",
  "command.invoked",
  "agent.config_changed",
  "host.updated",
  "terminal_session.changed",
] as const satisfies readonly EventType[];

// Compile-time check: every event type of the spec is listed above. If one is missing, the
// type below is that literal and the assignment of `true` fails to compile.
type UnlistedEventType = Exclude<EventType, (typeof KNOWN_EVENT_TYPES)[number]>;
const everyEventTypeIsListed: [UnlistedEventType] extends [never]
  ? true
  : UnlistedEventType = true;
void everyEventTypeIsListed;

const KNOWN: ReadonlySet<string> = new Set<string>(KNOWN_EVENT_TYPES);

/** An event whose type this SDK version knows, with its generated type. */
export interface KnownEvent {
  /** Discriminator: the type is known. */
  readonly kind: "known";
  /** The event, narrowed further by a check of `event.type`. */
  readonly event: Event;
}

/**
 * An event whose `type` this SDK version does not know, or a value that is no event object.
 * The caller skips it and keeps advancing its cursor (docs/spec/04-sync-protocol.md 7.1).
 */
export interface UnknownEvent {
  /** Discriminator: the type is unknown. */
  readonly kind: "unknown";
  /** The received `type`, or `""` when it is missing or not a string. */
  readonly type: string;
  /** The received `seq` when it is a safe integer; absent otherwise (ephemeral events). */
  readonly seq?: number;
  /** The value as received. */
  readonly raw: unknown;
}

function isObject(value: unknown): value is Record<string, unknown> {
  return typeof value === "object" && value !== null && !Array.isArray(value);
}

/**
 * Tells whether a value is an event object of a type this SDK version knows.
 *
 * @param value - Any decoded JSON value.
 * @returns True exactly when `value` is an object whose `type` is in {@link KNOWN_EVENT_TYPES}.
 */
export function isKnownEvent(value: unknown): value is Event {
  return (
    isObject(value) && typeof value.type === "string" && KNOWN.has(value.type)
  );
}

/**
 * Classifies one received event by its `type` alone, before any payload is looked at.
 *
 * @param raw - One item of `EventPage.events` or a gateway dispatch, as received.
 * @returns `{ kind: "known", event }` for a type in {@link KNOWN_EVENT_TYPES}, otherwise an
 *   {@link UnknownEvent}; never a known type for an event of another type, even when its
 *   payload would fit.
 * @example
 * ```ts
 * const decoded = decodeEvent(item);
 * if (decoded.kind === "known" && decoded.event.type === "thread.updated") {
 *   show(decoded.event.payload.thread);
 * }
 * ```
 */
export function decodeEvent(raw: unknown): KnownEvent | UnknownEvent {
  if (isKnownEvent(raw)) {
    return { kind: "known", event: raw };
  }
  const type = isObject(raw) && typeof raw.type === "string" ? raw.type : "";
  const seq = isObject(raw) ? raw.seq : undefined;
  if (typeof seq === "number" && Number.isSafeInteger(seq)) {
    return { kind: "unknown", type, seq, raw };
  }
  return { kind: "unknown", type, raw };
}

/**
 * The events of a page whose type this SDK version knows, in order; unknown ones are skipped.
 * Advance with `page.next_cursor` regardless of what was skipped.
 *
 * @param page - A page from `listEvents` or `pollAgentEvents`.
 * @returns The known events of the page.
 */
export function knownEvents(page: EventPage): Event[] {
  return page.events.filter(isKnownEvent);
}

function isJson(response: Response): boolean {
  const media = response.headers
    .get("content-type")
    ?.split(";")[0]
    ?.trim()
    .toLowerCase();
  // `application/json` and every structured `+json` type (problem documents, the JWKS).
  return (
    media !== undefined &&
    (media === "application/json" ||
      (media.startsWith("application/") && media.endsWith("+json")))
  );
}

// JSON.parse reviver: a member whose value is null is dropped (returning undefined deletes it);
// null items of an array stay, because only members can be absent.
function withoutNullMembers(
  this: unknown,
  _key: string,
  value: unknown,
): unknown {
  return value === null && !Array.isArray(this) ? undefined : value;
}

/**
 * openapi-fetch middleware that treats a received JSON `null` member as an absent member.
 *
 * For `application/json` and `application/*+json` responses it parses the body, drops
 * every object member whose value is `null` and returns an equivalent `Response` (same status,
 * status text and headers). Other media types, empty bodies and malformed JSON pass through
 * unchanged. {@link createApiClient} always registers it.
 */
export const toleranceMiddleware: Middleware = {
  async onResponse({ response }) {
    if (!isJson(response)) {
      return response;
    }
    const text = await response.clone().text();
    if (text === "") {
      return response;
    }
    let parsed: unknown;
    try {
      parsed = JSON.parse(text, withoutNullMembers);
    } catch {
      return response;
    }
    const headers = new Headers(response.headers);
    headers.delete("content-length");
    return new Response(JSON.stringify(parsed), {
      status: response.status,
      statusText: response.statusText,
      headers,
    });
  },
};
