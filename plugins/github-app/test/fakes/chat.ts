// SPDX-License-Identifier: Apache-2.0
/**
 * The in-memory chat port of the unit tests (brief M2-49 "Scope": `test/fakes/chat.ts`).
 *
 * It behaves like the server where the app depends on it: `createThread` is idempotent by
 * `first_message.client_msg_id`, `createMessage` by `client_msg_id` per thread, a second
 * `respondToInteraction` of an interaction answers `409`, a dropped thread answers `404`. It
 * signs requests of the server with an Ed25519 key whose public key `webhookKeys` lists, as
 * `getWebhookKeys` would. Faults are queued per operation: a problem answer before the effect,
 * or a lost answer after it (`TransportError`). Every call is recorded with its arguments.
 *
 * @module
 */

import { randomBytes, randomUUID } from "node:crypto";

import {
  TransportError,
  problemError,
  uuidv7,
  type components,
} from "bothy-sdk";
import { signWebhook, webhookPublicKey } from "bothy-sdk/testing";

import type {
  ChatPort,
  ForumThreadCreated,
  ForumThreadInput,
  InstanceLimits,
  InteractionRespond,
  Message,
  MessageCreate,
  PrincipalRef,
} from "../../src/chat/port.js";

/** A recorded call. */
export interface ChatCall {
  /** The port method. */
  readonly op: keyof ChatPort;
  /** Its arguments. */
  readonly args: readonly unknown[];
}

/** A thread of the fake. */
export interface FakeThread {
  /** The thread. */
  readonly id: string;
  /** Its forum. */
  readonly channelId: string;
  /** Its title. */
  readonly title: string;
  /** Its messages in order. */
  readonly messages: Message[];
}

/** A queued fault. */
export type ChatFault =
  | {
      readonly kind: "problem";
      readonly status: number;
      readonly body: Record<string, unknown>;
    }
  | { readonly kind: "lose_answer" }
  | {
      readonly kind: "hold";
      readonly until: Promise<void>;
      readonly entered?: () => void;
    };

/** The default limits (the defaults of `message.max_content_bytes` and `thread.title_max_chars`). */
export const DEFAULT_LIMITS: InstanceLimits = {
  message_max_content_bytes: 16_384,
  message_max_attachments: 10,
  file_max_upload_bytes: 104_857_600,
  thread_title_max_chars: 200,
  voice_max_duration_ms: 300_000,
};

/** The fake. */
export class FakeChat implements ChatPort {
  /** The app's own principal. */
  readonly principal: PrincipalRef = {
    id: randomUUID(),
    kind: "app",
    handle: "github",
    display_name: "GitHub",
  };
  /** The threads by ID. */
  readonly threads = new Map<string, FakeThread>();
  /** Every call in order. */
  readonly calls: ChatCall[] = [];
  /** The answers of `respondToInteraction` by interaction. */
  readonly responses = new Map<string, InteractionRespond>();
  /** The current limits. */
  limits_: InstanceLimits = { ...DEFAULT_LIMITS };
  readonly #seed = new Uint8Array(randomBytes(32));
  readonly #byFirstMessage = new Map<string, string>();
  readonly #faults = new Map<keyof ChatPort, ChatFault[]>();
  readonly #dropped = new Set<string>();
  readonly #messages = new Map<string, Message>();

  /**
   * Queues faults for an operation; each call takes the first.
   *
   * @param op - The port method.
   * @param faults - The faults.
   */
  fail(op: keyof ChatPort, ...faults: ChatFault[]): void {
    this.#faults.set(op, [...(this.#faults.get(op) ?? []), ...faults]);
  }

  /**
   * A fault that holds the call until the returned `release` runs.
   *
   * @returns The fault, a promise of the moment the call waits, and `release`.
   */
  static hold(): {
    fault: ChatFault;
    entered: Promise<void>;
    release: () => void;
  } {
    let release = (): void => undefined;
    let enter = (): void => undefined;
    const until = new Promise<void>((resolve) => {
      release = resolve;
    });
    const entered = new Promise<void>((resolve) => {
      enter = resolve;
    });
    return { fault: { kind: "hold", until, entered: enter }, entered, release };
  }

  /**
   * A problem answer of the server.
   *
   * @param status - The status.
   * @param code - The problem code.
   * @param errors - Field errors of `400 validation_failed`.
   * @returns The fault.
   */
  static problem(
    status: number,
    code: string,
    errors: { pointer: string; code: string }[] = [],
  ): ChatFault {
    return {
      kind: "problem",
      status,
      body: {
        type: "about:blank",
        title: code,
        status,
        code,
        ...(errors.length ? { errors } : {}),
      },
    };
  }

  /**
   * Makes a thread unreachable: posting into it answers `404 not_found`.
   *
   * @param threadId - The thread.
   */
  drop(threadId: string): void {
    this.#dropped.add(threadId);
  }

  /**
   * Signs a request of the server as the job `runs.app_dispatch` or a webhook delivery does.
   *
   * @param body - The JSON body.
   * @param id - The `webhook-id` (the event ID); default a new UUID.
   * @param timestamp - Unix seconds; default now.
   * @returns The headers.
   */
  sign(
    body: string,
    id: string = uuidv7(),
    timestamp = Math.floor(Date.now() / 1000),
  ): Record<string, string> {
    return {
      "content-type": "application/json",
      "webhook-id": id,
      "webhook-timestamp": String(timestamp),
      "webhook-signature": signWebhook({
        id,
        timestamp,
        body,
        seeds: [this.#seed],
      }),
    };
  }

  /** The calls of one operation. */
  callsOf(op: keyof ChatPort): ChatCall[] {
    return this.calls.filter((c) => c.op === op);
  }

  /** The threads of a forum in creation order. */
  threadsOf(channelId: string): FakeThread[] {
    return [...this.threads.values()].filter((t) => t.channelId === channelId);
  }

  #fault(op: keyof ChatPort): ChatFault | undefined {
    const list = this.#faults.get(op);
    return list?.shift();
  }

  async #before(
    op: keyof ChatPort,
    args: unknown[],
  ): Promise<ChatFault | undefined> {
    this.calls.push({ op, args: structuredClone(args) });
    const fault = this.#fault(op);
    if (fault?.kind === "hold") {
      fault.entered?.();
      await fault.until;
      return undefined;
    }
    if (fault?.kind === "problem")
      throw problemError(fault.status, fault.body, { operation: op });
    return fault;
  }

  #after(op: keyof ChatPort, fault: ChatFault | undefined): void {
    if (fault?.kind === "lose_answer") throw new TransportError(op);
  }

  #message(
    thread: FakeThread,
    input: MessageCreate,
    author: PrincipalRef,
  ): Message {
    const existing = thread.messages.find(
      (m) => m.client_msg_id === input.client_msg_id,
    );
    if (existing !== undefined) return existing;
    const message: Message = {
      id: randomUUID(),
      thread_id: thread.id,
      channel_id: thread.channelId,
      thread_seq: thread.messages.length + 1,
      author,
      client_msg_id: input.client_msg_id,
      origin: author.kind === "app" ? "app" : "human",
      kind: input.kind ?? "text",
      content_md: input.content_md ?? "",
      content_format: 1,
      silent: false,
      attachments: [],
      mention_ids: [],
      reactions: [],
      pinned: false,
      components: (input.components ?? []).map((c) => ({
        ...c,
        id: randomUUID(),
        state: "active",
      })) as unknown as components["schemas"]["Component"][],
      version: 1,
      created_at: new Date().toISOString(),
    };
    thread.messages.push(message);
    this.#messages.set(message.id, message);
    return message;
  }

  /**
   * Adds a message of a person to a thread (a reply in the chat).
   *
   * @param threadId - The thread.
   * @param content - The text.
   * @param author - The person.
   * @returns The message.
   */
  personWrites(
    threadId: string,
    content: string,
    author: PrincipalRef,
  ): Message {
    const thread = this.threads.get(threadId);
    if (thread === undefined) throw new Error("no such thread");
    return this.#message(
      thread,
      { client_msg_id: uuidv7(), content_md: content },
      author,
    );
  }

  /** {@inheritDoc ChatPort.me} */
  async me(): Promise<PrincipalRef> {
    const fault = await this.#before("me", []);
    this.#after("me", fault);
    return this.principal;
  }

  /** {@inheritDoc ChatPort.limits} */
  limits(): Promise<InstanceLimits> {
    this.calls.push({ op: "limits", args: [] });
    return Promise.resolve({ ...this.limits_ });
  }

  /** {@inheritDoc ChatPort.webhookKeys} */
  webhookKeys(): Promise<string[]> {
    return Promise.resolve([webhookPublicKey(this.#seed)]);
  }

  /** {@inheritDoc ChatPort.createForumThread} */
  async createForumThread(
    input: ForumThreadInput,
  ): Promise<ForumThreadCreated> {
    const fault = await this.#before("createForumThread", [input]);
    const known = this.#byFirstMessage.get(input.firstMessage.client_msg_id);
    let threadId = known;
    if (threadId === undefined) {
      const thread: FakeThread = {
        id: randomUUID(),
        channelId: input.channelId,
        title: input.title,
        messages: [],
      };
      this.threads.set(thread.id, thread);
      this.#message(thread, input.firstMessage, this.principal);
      this.#byFirstMessage.set(input.firstMessage.client_msg_id, thread.id);
      threadId = thread.id;
    }
    this.#after("createForumThread", fault);
    return { threadId };
  }

  /** {@inheritDoc ChatPort.postMessage} */
  async postMessage(
    threadId: string,
    message: MessageCreate,
  ): Promise<Message> {
    const fault = await this.#before("postMessage", [threadId, message]);
    const thread = this.threads.get(threadId);
    if (thread === undefined || this.#dropped.has(threadId))
      throw problemError(
        404,
        { code: "not_found", status: 404, title: "Not found" },
        { operation: "postMessage" },
      );
    const created = this.#message(thread, message, this.principal);
    this.#after("postMessage", fault);
    return created;
  }

  /** {@inheritDoc ChatPort.getMessage} */
  async getMessage(messageId: string): Promise<Message> {
    const fault = await this.#before("getMessage", [messageId]);
    const message = this.#messages.get(messageId);
    if (message === undefined)
      throw problemError(
        404,
        { code: "not_found", status: 404, title: "Not found" },
        { operation: "getMessage" },
      );
    this.#after("getMessage", fault);
    return message;
  }

  /** {@inheritDoc ChatPort.respondInteraction} */
  async respondInteraction(
    interactionId: string,
    body: InteractionRespond,
    idempotencyKey: string,
  ): Promise<void> {
    const fault = await this.#before("respondInteraction", [
      interactionId,
      body,
      idempotencyKey,
    ]);
    if (this.responses.has(interactionId))
      throw problemError(
        409,
        { code: "invalid_state_transition", status: 409, title: "Conflict" },
        { operation: "respondToInteraction" },
      );
    this.responses.set(interactionId, body);
    this.#after("respondInteraction", fault);
  }
}
