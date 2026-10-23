// SPDX-License-Identifier: Apache-2.0
/**
 * The gateway client of an agent and its event sources (docs/spec/11-extensions.md 11.2 row
 * `gateway`; 04 6, 09 4.2). {@link Gateway} speaks the WebSocket protocol with reconnects;
 * {@link GatewaySource} and {@link PollingSource} offer the same {@link EventSource} interface
 * over the gateway and over REST polling. Node.js only.
 *
 * @example
 * ```ts
 * const gw = new Gateway(gatewayUrl(baseUrl, "/api/v1/gateway"), credentials, {
 *   consumer: "default",
 *   agentId,
 * });
 * const source: EventSource = new GatewaySource(gw);
 * for await (const delivery of source) {
 *   if (delivery instanceof Envelope && delivery.seq !== undefined) {
 *     await handle(delivery);
 *     await source.settle(delivery.seq);
 *   } else if (delivery instanceof CursorAdvance) await source.advance(delivery.seq);
 * }
 * ```
 *
 * @module
 */

export {
  CloseReason,
  Gateway,
  GatewayEvent,
  IDENTIFY_ON,
  PROTOCOL_VERSIONS,
  RESUME_ON,
  STOP_ON,
  gatewayUrl,
  type GatewayOptions,
  type GatewaySocket,
  type GatewaySocketConstructor,
} from "./gateway.js";
export {
  AckWindow,
  CursorAdvance,
  Envelope,
  GatewaySource,
  PollingSource,
  SourceReady,
  envelopeFrom,
  type Delivery,
  type EnvelopeFields,
  type EphemeralEvent,
  type EventSource,
  type GatewayLike,
  type PollingSourceOptions,
} from "./sources.js";
