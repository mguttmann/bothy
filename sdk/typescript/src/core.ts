// SPDX-License-Identifier: Apache-2.0
/**
 * Browser-safe entry of the SDK (`bothy-sdk/core`, docs/spec/11-erweiterungen.md 11.3): only
 * the parts without secrets: generated types, the tolerance layer, errors, settings, the
 * `components` builders, `renderContext`, `uuidv7` and `pages`. It imports no `node:` module and none of `auth`, `invocation`,
 * `gateway`, `webhooks` or `outbox` (`test/packaging.test.ts`).
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
export { uuidv7 } from "./ids.js";
export * from "./errors.js";
export * from "./config.js";
export * from "./pages.js";
export * from "./components/index.js";
export {
  CONTEXT_NOTICE,
  DATA_NOTICE,
  RenderedContext,
  renderContext,
  type RenderContextOptions,
} from "./runs/context.js";
