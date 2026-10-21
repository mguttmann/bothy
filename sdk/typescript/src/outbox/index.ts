// SPDX-License-Identifier: Apache-2.0
/**
 * The outbox (docs/spec/11-erweiterungen.md 11.2 row `outbox`; 04 13.2; 06 7): replies get
 * their `client_msg_id` and other writes their `Idempotency-Key` stored before the first send in
 * a replaceable {@link OutboxStore} ({@link MemoryOutboxStore}, {@link FileOutboxStore}), and
 * every repeat sends the same one. Node.js only (the file store and the run binding of the
 * client).
 *
 * @example
 * ```ts
 * const outbox = new Outbox(api, { store: new FileOutboxStore("/var/lib/lotse/outbox.json") });
 * await outbox.flush();
 * await outbox.sendMessage(threadId, "Done", { runId });
 * ```
 *
 * @module
 */

export {
  Outbox,
  PERMANENT_STATUSES,
  type CallOptions,
  type OutboxInit,
  type OutboxMethod,
  type OutboxOptions,
  type OutboxPath,
  type SendMessageOptions,
} from "./outbox.js";
export {
  FileOutboxStore,
  MemoryOutboxStore,
  OutboxState,
  type OutboxEntry,
  type OutboxEntryKind,
  type OutboxStore,
} from "./store.js";
