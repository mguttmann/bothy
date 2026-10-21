// SPDX-License-Identifier: Apache-2.0
/**
 * The heartbeat building block for `agent_heartbeat.v1` (docs/spec/09-agenten.md 4.8;
 * docs/spec/11-erweiterungen.md 11.2 row `heartbeat`): {@link buildHeartbeat} refuses every
 * field that could carry a secret, content or a path, and {@link HeartbeatSender} sends the
 * heartbeat through the gateway or `sendHeartbeat` at the interval the server asks for.
 *
 * @example
 * ```ts
 * const beat = buildHeartbeat("online", {
 *   runtimeVersion: "2.3.1",
 *   details: { adapter: { name: "example-adapter" }, protocol_version: 1 },
 * });
 * beat.details; // { adapter: { name: "example-adapter" }, protocol_version: 1 }
 * ```
 *
 * @module
 */

export {
  CAPABILITY_KEYS,
  CODE_FORM,
  DETAILS_KEYS,
  HEARTBEAT_MEMBERS,
  NAME_FORM,
  VERSION_FORM,
  buildHeartbeat,
  type BuildHeartbeatOptions,
} from "./build.js";
export {
  HeartbeatSender,
  restHeartbeatSend,
  type HeartbeatSend,
  type HeartbeatSenderOptions,
} from "./sender.js";
