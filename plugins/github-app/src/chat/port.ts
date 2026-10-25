// SPDX-License-Identifier: Apache-2.0
/**
 * The app's port to the server (brief M2-49 Design note 9): the few operations of the public
 * API the app uses, typed with the schema types the SDK exports from the generated core (no own
 * type repeats a schema, ADR 0026). `chat/sdk-adapter.ts` implements it on `createApiClient`;
 * unit tests use an in-memory implementation. Failures are the SDK's errors: `ProblemError` with
 * status and code for an answer, `TransportError` or `TimeoutError` for none.
 *
 * @example
 * ```ts
 * const { threadId } = await chat.createForumThread({ channelId, title, firstMessage, idempotencyKey });
 * await chat.postMessage(threadId, { client_msg_id: uuidv7(), content_md: "Hello" });
 * ```
 *
 * @module
 */

import type { KeyObject } from "node:crypto";

import type { components } from "bothy-sdk";

/** A message (`Message`). */
export type Message = components["schemas"]["Message"];
/** The body of `createMessage` (`MessageCreate`). */
export type MessageCreate = components["schemas"]["MessageCreate"];
/** The limits of `getInstance` (`InstanceLimits`). */
export type InstanceLimits = components["schemas"]["InstanceLimits"];
/** A principal (`PrincipalRef`). */
export type PrincipalRef = components["schemas"]["PrincipalRef"];
/** The body of `respondToInteraction` (`InteractionRespond`). */
export type InteractionRespond = components["schemas"]["InteractionRespond"];

/** The input of {@link ChatPort.createForumThread}. */
export interface ForumThreadInput {
  /** The forum. */
  readonly channelId: string;
  /** The title, already cleaned and cut. */
  readonly title: string;
  /** The first message; its `client_msg_id` makes the call idempotent. */
  readonly firstMessage: MessageCreate;
  /** `Idempotency-Key` of the call, stored in the job before the first attempt. */
  readonly idempotencyKey: string;
}

/** The answer of {@link ChatPort.createForumThread}. */
export interface ForumThreadCreated {
  /** The new thread. */
  readonly threadId: string;
}

/** The operations of the public API the app uses. */
export interface ChatPort {
  /**
   * `getMe`: the app's own principal.
   *
   * @returns The principal.
   */
  me(): Promise<PrincipalRef>;
  /**
   * `getInstance`: the limits of the instance.
   *
   * @returns The limits.
   */
  limits(): Promise<InstanceLimits>;
  /**
   * The trusted `v1a` keys of `getWebhookKeys`, cached by the SDK's `WebhookKeyCache`.
   *
   * @returns The keys.
   */
  webhookKeys(): Promise<readonly (KeyObject | string)[]>;
  /**
   * `createThread` in a forum with a first message, in one call.
   *
   * @param input - Forum, title, first message and idempotency key.
   * @returns The new thread.
   */
  createForumThread(input: ForumThreadInput): Promise<ForumThreadCreated>;
  /**
   * `createMessage` into a thread.
   *
   * @param threadId - The thread.
   * @param message - The message with its stored `client_msg_id`.
   * @returns The message (the original on a replay).
   */
  postMessage(threadId: string, message: MessageCreate): Promise<Message>;
  /**
   * `getMessage`: the full message.
   *
   * @param messageId - The message.
   * @returns The message.
   */
  getMessage(messageId: string): Promise<Message>;
  /**
   * `respondToInteraction`.
   *
   * @param interactionId - The interaction.
   * @param body - State and response text.
   * @param idempotencyKey - `Idempotency-Key`, stored in the job before the first attempt.
   */
  respondInteraction(
    interactionId: string,
    body: InteractionRespond,
    idempotencyKey: string,
  ): Promise<void>;
}
