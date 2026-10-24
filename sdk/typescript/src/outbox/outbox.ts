// SPDX-License-Identifier: Apache-2.0
/**
 * Outbox for replies and other writes that must never go out twice or get lost
 * (docs/spec/04-sync-protocol.md 13.2, docs/spec/06-api.md 7; port of `bothy_sdk/outbox.py`).
 *
 * A reply gets its `client_msg_id` (UUIDv7) stored before the first send, and every retry sends
 * the same ID, so the server answers a repeat with the original (`Idempotent-Replay: true`).
 * Other writes ({@link Outbox.call}) get an `Idempotency-Key` stored the same way. Entries leave
 * in order per thread (FIFO), threads in parallel. Transient failures use the retry policy of
 * the client; after the last attempt the entry stays pending and {@link Outbox.flush} (the
 * adapter calls it at its start and after every `ready`) sends it again with the same ID. `400`,
 * `404`, `409` (except `request_in_progress`), `410`, `413` and `422` (also
 * `idempotency_key_reuse`) mark it failed with its
 * problem code: it leaves the pending entries, stays visible in {@link Outbox.failed} and
 * `onFailed` is called; nothing is dropped silently (04 13.2 point 5). A `2xx` deletes it.
 *
 * Log lines carry entry numbers and problem codes, never content, a token or a key.
 *
 * @module
 */

import type { ClientPathsWithMethod, MaybeOptionalInit } from "openapi-fetch";

import { type ApiClient, IDEMPOTENCY_KEY_HEADER, unwrap } from "../client.js";
import { checkComponents } from "../components/index.js";
import {
  InvocationTokenInvalid,
  ProblemError,
  RequestInProgress,
  TransportError,
} from "../errors.js";
import type { components, paths } from "../generated/openapi.js";
import { uuidv7 } from "../ids.js";
import type { Logger } from "../logger.js";
import {
  MemoryOutboxStore,
  type OutboxEntry,
  OutboxState,
  type OutboxStore,
} from "./store.js";

type MessageCreate = components["schemas"]["MessageCreate"];
type ComponentCreate = components["schemas"]["ComponentCreate"];

/** Statuses that mark an entry failed instead of retrying it later. */
export const PERMANENT_STATUSES: ReadonlySet<number> = new Set([
  400, 404, 409, 410, 413, 422,
]);

/** The methods of a write {@link Outbox.call} stores. */
export type OutboxMethod = "post" | "put" | "patch" | "delete";

/** A path of docs/api/openapi.yaml that has `method`. */
export type OutboxPath<M extends OutboxMethod> = ClientPathsWithMethod<
  ApiClient,
  M
>;

/**
 * The parameters and body of a write as openapi-fetch takes them. Every value must be plain JSON
 * (it is stored); `params.header` must not carry `Idempotency-Key` (the outbox sets it) or any
 * credential.
 */
export type OutboxInit<
  M extends OutboxMethod,
  P extends OutboxPath<M>,
> = MaybeOptionalInit<paths[P], M>;

/** Options of {@link Outbox}. */
export interface OutboxOptions {
  /** The storage (default {@link MemoryOutboxStore}). */
  readonly store?: OutboxStore | undefined;
  /** Called with every entry that failed permanently. */
  readonly onFailed?:
    ((entry: OutboxEntry) => Promise<void> | void) | undefined;
  /**
   * Returns the newest invocation token of a run. With it, an entry of a run goes out bound to
   * that run (07 SR9) whenever it is sent, also from a later `flush`, and an entry of a run
   * without a known token fails instead of going out unbound.
   */
  readonly tokenFor?:
    ((runId: string) => Promise<string | undefined>) | undefined;
  /** The logger (default the client's logger). */
  readonly logger?: Logger | undefined;
}

/** Options of {@link Outbox.sendMessage}; each maps to a member of `MessageCreate`. */
export interface SendMessageOptions {
  /** The run the reply belongs to (inside a run the adapter passes it). */
  readonly runId?: string | undefined;
  /** The message it answers, if any. */
  readonly replyToMessageId?: string | undefined;
  /** `text` (default) or `card` (a message with components). */
  readonly kind?: "text" | "card" | undefined;
  /** The components of a card, held to the limits of `checkComponents` before storing. */
  readonly components?: readonly ComponentCreate[] | undefined;
}

/** Options of {@link Outbox.call}. */
export interface CallOptions {
  /** The thread whose order the write keeps. */
  readonly threadId: string;
  /** The run it belongs to, if any. */
  readonly runId?: string | undefined;
}

const FORBIDDEN_HEADERS: ReadonlySet<string> = new Set([
  IDEMPOTENCY_KEY_HEADER.toLowerCase(),
  "authorization",
  "dpop",
  "invocation-token",
]);
const METHODS: ReadonlySet<string> = new Set([
  "post",
  "put",
  "patch",
  "delete",
]);

/** A loose view of `ApiClient.request` for a stored call. */
type StoredRequest = (
  method: string,
  path: string,
  init: Record<string, unknown>,
) => Promise<{ data?: unknown; error?: unknown; response: Response }>;

/**
 * Sends replies and writes in order per thread and never twice.
 *
 * @example
 * ```ts
 * const outbox = new Outbox(api, { store: new FileOutboxStore("/var/lib/lotse/outbox.json") });
 * await outbox.flush();
 * const entry = await outbox.sendMessage(threadId, "Done", { runId });
 * entry.state; // "sent", "pending" (resent by the next flush) or "failed"
 * ```
 */
export class Outbox {
  /** The REST client. */
  readonly api: ApiClient;
  /** The storage. */
  readonly store: OutboxStore;
  private readonly onFailed: OutboxOptions["onFailed"];
  private readonly tokenFor: OutboxOptions["tokenFor"];
  private readonly logger: Logger;
  private readonly locks = new Map<string, Promise<unknown>>();

  /**
   * Creates the outbox.
   *
   * @param api - The REST client of `createApiClient`.
   * @param options - Store, `onFailed`, `tokenFor` and logger.
   */
  constructor(api: ApiClient, options: OutboxOptions = {}) {
    this.api = api;
    this.store = options.store ?? new MemoryOutboxStore();
    this.onFailed = options.onFailed;
    this.tokenFor = options.tokenFor;
    this.logger = options.logger ?? api.logger;
  }

  /**
   * Stores a reply with a new `client_msg_id` and sends it (`createMessage`).
   *
   * @param threadId - The thread.
   * @param contentMd - The Markdown text.
   * @param options - Run, the message it answers, kind and components.
   * @returns The entry after the send attempt (sent, failed or still pending).
   * @throws `RangeError` before storing when the components break the limits of
   *   `component.max_per_message` or `component.custom_id_max_chars`; an error of the store.
   */
  async sendMessage(
    threadId: string,
    contentMd: string,
    options: SendMessageOptions = {},
  ): Promise<OutboxEntry> {
    const body: MessageCreate = {
      client_msg_id: uuidv7(),
      kind: options.kind ?? "text",
      content_md: contentMd,
    };
    if (options.runId !== undefined) body.run_id = options.runId;
    if (options.replyToMessageId !== undefined)
      body.reply_to_message_id = options.replyToMessageId;
    if (options.components !== undefined && options.components.length > 0) {
      checkComponents(options.components, this.api.settings);
      body.components = [...options.components];
    }
    return this.store_(threadId, "message", body.client_msg_id, body, options);
  }

  /**
   * Stores a write with a new `Idempotency-Key` and sends it.
   *
   * The write is stored as method, path, parameters and body, so a later process sends the same
   * request with the same key from {@link Outbox.flush}.
   *
   * @param method - `post`, `put`, `patch` or `delete`.
   * @param path - The path template of the operation, for example `/commands`.
   * @param init - Its parameters and body as openapi-fetch takes them (plain JSON).
   * @param options - The thread whose order it keeps and the run.
   * @returns The entry after the send attempt.
   * @throws `TypeError` before storing for a value that is not plain JSON or a header the outbox
   *   owns (`Idempotency-Key`) or that carries a credential; an error of the store.
   *
   * @example
   * ```ts
   * await outbox.call("post", "/commands", { body: { name: "status", description: "Status" } }, {
   *   threadId,
   * });
   * ```
   */
  async call<M extends OutboxMethod, P extends OutboxPath<M>>(
    method: M,
    path: P,
    init: OutboxInit<M, P>,
    options: CallOptions,
  ): Promise<OutboxEntry> {
    if (
      !METHODS.has(method) ||
      typeof path !== "string" ||
      !path.startsWith("/")
    )
      throw new TypeError("outbox call needs a write method and a path");
    const raw = (init ?? {}) as { params?: unknown; body?: unknown };
    const params = raw.params ?? {};
    storable(params, "params");
    const header = (params as { header?: unknown }).header;
    if (header !== undefined && header !== null)
      for (const name of Object.keys(header))
        if (FORBIDDEN_HEADERS.has(name.toLowerCase()))
          throw new TypeError(
            "outbox call: params.header must not carry Idempotency-Key or a credential",
          );
    const payload: Record<string, unknown> = { method, path, params };
    if ("body" in raw) {
      storable(raw.body, "body");
      payload.body = raw.body;
    }
    return this.store_(options.threadId, "call", uuidv7(), payload, options);
  }

  /**
   * Sends every pending entry again, in order per thread, threads in parallel.
   *
   * @returns When every thread was drained up to its first transient failure.
   */
  async flush(): Promise<void> {
    const threads = new Set(
      (await this.store.pending()).map((e) => e.threadId),
    );
    await Promise.all([...threads].map((t) => this.drain(t)));
  }

  /**
   * Returns every entry that failed permanently.
   *
   * @returns The failed entries in order.
   */
  failed(): Promise<OutboxEntry[]> {
    return this.store.failed();
  }

  private async store_(
    threadId: string,
    kind: OutboxEntry["kind"],
    key: string,
    payload: object,
    options: { readonly runId?: string | undefined },
  ): Promise<OutboxEntry> {
    // Every write made inside a run goes out under that run's binding (09 4.7, 07 SR9), also one
    // into another thread that is not the run's reply.
    const runId = options.runId ?? this.api.boundRunId();
    const entry: OutboxEntry = {
      entryId: 0,
      threadId,
      kind,
      key,
      payload: payload as Record<string, unknown>,
      state: OutboxState.PENDING,
      ...(runId === undefined ? {} : { runId }),
    };
    const stored = await this.store.add(entry);
    return (await this.drain(threadId, stored)) ?? stored;
  }

  /**
   * Sends the pending entries of one thread in order and stops at the first transient error;
   * returns the state of `until` after the attempt (it stays pending when an earlier entry of
   * the thread is still pending).
   */
  private drain(
    threadId: string,
    until?: OutboxEntry,
  ): Promise<OutboxEntry | undefined> {
    const previous = this.locks.get(threadId) ?? Promise.resolve();
    const run = previous.then(async () => {
      let result = until;
      const pending = (await this.store.pending()).filter(
        (e) => e.threadId === threadId,
      );
      for (const entry of pending) {
        if (until !== undefined && entry.entryId > until.entryId) break;
        const sent = await this.send(entry);
        if (until !== undefined && entry.entryId === until.entryId)
          result = sent;
        if (sent.state === OutboxState.PENDING) break;
      }
      return result;
    });
    const settled = run.catch(() => undefined);
    this.locks.set(threadId, settled);
    void settled.then(() => {
      if (this.locks.get(threadId) === settled) this.locks.delete(threadId);
    });
    return run;
  }

  private async send(entry: OutboxEntry): Promise<OutboxEntry> {
    let bound: (<T>(fn: () => Promise<T>) => Promise<T>) | undefined;
    if (entry.runId === undefined) {
      // Stored outside any run: never under the binding of whatever run is current now.
      bound = (fn) => this.api.unbound(fn);
    } else if (this.tokenFor === undefined) {
      bound = (fn) => fn();
    } else {
      const { runId } = entry;
      const token = await this.tokenFor(runId);
      if (token !== undefined)
        bound = (fn) => this.api.bindRun(runId, token, fn);
    }
    if (bound === undefined) return this.fail(entry, "run_token_unavailable");
    try {
      await bound(() => this.request(entry));
    } catch (error) {
      if (error instanceof InvocationTokenInvalid)
        // invalid or run_inactive: the token of the run is no longer usable (07 SR9).
        return this.fail(entry, error.code);
      if (error instanceof ProblemError) {
        // `409 request_in_progress`: the first request with this key still runs and the answer
        // is not stored, so the entry stays pending for the next flush (06 7 point 2).
        if (
          PERMANENT_STATUSES.has(error.status) &&
          !(error instanceof RequestInProgress)
        )
          return this.fail(entry, error.code);
        this.logger.info(`outbox entry ${entry.entryId} stays pending`, {
          code: error.code,
        });
        return entry;
      }
      if (error instanceof TransportError) {
        this.logger.info(`outbox entry ${entry.entryId} stays pending`, {
          error: error.name,
        });
        return entry;
      }
      throw error;
    }
    await this.store.mark(entry.entryId, OutboxState.SENT);
    return { ...entry, state: OutboxState.SENT };
  }

  private async fail(entry: OutboxEntry, code: string): Promise<OutboxEntry> {
    await this.store.mark(entry.entryId, OutboxState.FAILED, code);
    const failed: OutboxEntry = {
      ...entry,
      state: OutboxState.FAILED,
      problemCode: code,
    };
    this.logger.warn(`outbox entry ${entry.entryId} failed`, { code });
    await this.onFailed?.(failed);
    return failed;
  }

  private async request(entry: OutboxEntry): Promise<void> {
    if (entry.kind === "message") {
      await unwrap(
        this.api.POST("/threads/{thread_id}/messages", {
          params: { path: { thread_id: entry.threadId } },
          body: entry.payload as MessageCreate,
        }),
        "createMessage",
      );
      return;
    }
    const { method, path, params, body } = entry.payload as {
      method: string;
      path: string;
      params?: { header?: Record<string, unknown> };
      body?: unknown;
    };
    const init: Record<string, unknown> = {
      params: {
        ...params,
        header: { ...params?.header, [IDEMPOTENCY_KEY_HEADER]: entry.key },
      },
    };
    if ("body" in entry.payload) init.body = body;
    const request = this.api.request.bind(this.api) as unknown as StoredRequest;
    await unwrap(
      request(method, path, init),
      `${method.toUpperCase()} ${path}`,
    );
  }
}

/** Throws unless `value` is plain JSON: null, booleans, strings, finite numbers, arrays, objects. */
function storable(value: unknown, where: string): void {
  if (value === null || typeof value === "string" || typeof value === "boolean")
    return;
  if (typeof value === "number") {
    if (Number.isFinite(value)) return;
  } else if (Array.isArray(value)) {
    value.forEach((v: unknown, i) => storable(v, `${where}[${i}]`));
    return;
  } else if (
    typeof value === "object" &&
    Object.getPrototypeOf(value) === Object.prototype
  ) {
    for (const [k, v] of Object.entries(value)) storable(v, `${where}.${k}`);
    return;
  }
  throw new TypeError(`outbox cannot store the value at ${where}`);
}
