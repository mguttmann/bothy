// SPDX-License-Identifier: Apache-2.0
/**
 * TypeScript SDK of the public API (docs/spec/11-erweiterungen.md section 11).
 *
 * The types in `./generated/openapi.ts` come from docs/api/openapi.yaml through the pinned
 * openapi-typescript (`make -C sdk/typescript generate`, never edited by hand). The runtime
 * client is openapi-fetch; {@link createApiClient} always adds the tolerance layer of
 * `./tolerance.ts` (gate G5). Authentication, DPoP, timeouts, retries, the gateway and the
 * rest of the hand layer come with M2-30.
 *
 * @example
 * ```ts
 * const api = createApiClient({ baseUrl: "https://example.com/api/v1" });
 * const { data } = await api.GET("/events", { params: { query: { after: 0 } } });
 * for (const event of data ? knownEvents(data) : []) console.log(event.type);
 * ```
 *
 * @packageDocumentation
 */

import createClient, { type Client, type ClientOptions } from "openapi-fetch";

import type { paths } from "./generated/openapi.js";
import { toleranceMiddleware } from "./tolerance.js";

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

/** An openapi-fetch client for every operation of docs/api/openapi.yaml. */
export type ApiClient = Client<paths>;

/**
 * Creates the openapi-fetch client for the public API with the tolerance middleware
 * registered.
 *
 * @param options - Options of openapi-fetch, such as `baseUrl` (ending in `/api/v1`), `fetch`
 *   and `headers`.
 * @returns A client whose JSON responses never contain a `null` member.
 */
export function createApiClient(options: ClientOptions = {}): ApiClient {
  const client = createClient<paths>(options);
  client.use(toleranceMiddleware);
  return client;
}
