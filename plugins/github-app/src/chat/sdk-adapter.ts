// SPDX-License-Identifier: Apache-2.0
/**
 * The {@link ChatPort} on the SDK's `createApiClient` (brief M2-49 Design note 9): `getMe`,
 * `getInstance`, `createThread`, `createMessage`, `getMessage`, `respondToInteraction` and the
 * keys of `getWebhookKeys` through one `WebhookKeyCache`. The client retries transient failures
 * within `sdk.retry_max_attempts`; writes carry their stored `client_msg_id` or
 * `Idempotency-Key`, so a retry or a rerun after a crash repeats the same request.
 *
 * @example
 * ```ts
 * const chat = new SdkChatPort(createApiClient({ baseUrl: `${serverUrl}/api/v1`, token }));
 * ```
 *
 * @module
 */

import type { KeyObject } from "node:crypto";

import { WebhookKeyCache, unwrap, type ApiClient } from "bothy-sdk";

import type {
  ChatPort,
  ForumThreadCreated,
  ForumThreadInput,
  InstanceLimits,
  InteractionRespond,
  Message,
  MessageCreate,
  PrincipalRef,
} from "./port.js";

/** {@link ChatPort} over a client of `createApiClient`. */
export class SdkChatPort implements ChatPort {
  readonly #api: ApiClient;
  readonly #keys: WebhookKeyCache;

  /**
   * Creates the adapter.
   *
   * @param api - The SDK client with the app's token.
   */
  constructor(api: ApiClient) {
    this.#api = api;
    this.#keys = new WebhookKeyCache({
      client: api,
      clock: api.clock,
      settings: api.settings,
    });
  }

  /** `getMe` (see `ChatPort.me`). */
  async me(): Promise<PrincipalRef> {
    return (await unwrap(this.#api.GET("/me"), "getMe")).principal;
  }

  /** `getInstance` (see `ChatPort.limits`). */
  async limits(): Promise<InstanceLimits> {
    return (await unwrap(this.#api.GET("/instance"), "getInstance")).limits;
  }

  /** The keys of `getWebhookKeys` (see `ChatPort.webhookKeys`). */
  webhookKeys(): Promise<KeyObject[]> {
    return this.#keys.keys();
  }

  /** `createThread` in a forum (see `ChatPort.createForumThread`). */
  async createForumThread(
    input: ForumThreadInput,
  ): Promise<ForumThreadCreated> {
    const created = await unwrap(
      this.#api.POST("/channels/{channel_id}/threads", {
        params: { path: { channel_id: input.channelId } },
        headers: { "Idempotency-Key": input.idempotencyKey },
        body: { title: input.title, first_message: input.firstMessage },
      }),
      "createThread",
    );
    return { threadId: created.thread.id };
  }

  /** `createMessage` (see `ChatPort.postMessage`). */
  postMessage(threadId: string, message: MessageCreate): Promise<Message> {
    return unwrap(
      this.#api.POST("/threads/{thread_id}/messages", {
        params: { path: { thread_id: threadId } },
        body: message,
      }),
      "createMessage",
    );
  }

  /** `getMessage` (see `ChatPort.getMessage`). */
  getMessage(messageId: string): Promise<Message> {
    return unwrap(
      this.#api.GET("/messages/{message_id}", {
        params: { path: { message_id: messageId } },
      }),
      "getMessage",
    );
  }

  /** `respondToInteraction` (see `ChatPort.respondInteraction`). */
  async respondInteraction(
    interactionId: string,
    body: InteractionRespond,
    idempotencyKey: string,
  ): Promise<void> {
    await unwrap(
      this.#api.POST("/interactions/{interaction_id}/respond", {
        params: { path: { interaction_id: interactionId } },
        headers: { "Idempotency-Key": idempotencyKey },
        body,
      }),
      "respondToInteraction",
    );
  }
}
