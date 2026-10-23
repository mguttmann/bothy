// SPDX-License-Identifier: Apache-2.0
/**
 * TypeScript SDK of the public API (docs/spec/11-extensions.md section 11), Node.js entry.
 *
 * The types in `./generated/openapi.ts` come from docs/api/openapi.yaml through the pinned
 * openapi-typescript (`make -C sdk/typescript generate`, never edited by hand). On that core the
 * hand layer of 11 11.2 adds the modules `auth`, `retry`, `gateway`, `invocation`, `runs`,
 * `components`, `commands`, `outbox`, `heartbeat` and `webhooks`; {@link createApiClient} always
 * registers the tolerance layer of `./tolerance.ts` (gate G5). This entry handles tokens and keys
 * and is for Node.js only; `bothy-sdk/core` is the browser-safe subset and `bothy-sdk/testing`
 * holds the FakeServer and the shared scenarios (docs/decisions.d/M2-30.md D-M2-30-2).
 *
 * @example
 * ```ts
 * const api = createApiClient({ baseUrl: "https://example.com/api/v1", token: new Secret(token) });
 * const { data } = await api.GET("/events", { params: { query: { after: 0 } } });
 * for (const event of data ? knownEvents(data) : []) console.log(event.type);
 * ```
 *
 * @packageDocumentation
 */

export type {
  components,
  operations,
  paths,
  webhooks,
} from "./generated/openapi.js";
export {
  KNOWN_EVENT_TYPES,
  decodeEvent,
  isKnownEvent,
  knownEvents,
  toleranceMiddleware,
  type Event,
  type EventPage,
  type EventType,
  type KnownEvent,
  type UnknownEvent,
} from "./tolerance.js";
export type { ClientOptions } from "openapi-fetch";

export * from "./client.js";
export * from "./errors.js";
export * from "./config.js";
export * from "./clock.js";
export * from "./logger.js";
export * from "./ids.js";
export * from "./pages.js";
export * from "./auth/index.js";
export * from "./retry/index.js";
export * from "./webhooks/index.js";
export * from "./invocation/index.js";
export * from "./gateway/index.js";
export * from "./runs/index.js";
export * from "./components/index.js";
export * from "./commands/index.js";
export * from "./outbox/index.js";
export * from "./heartbeat/index.js";
