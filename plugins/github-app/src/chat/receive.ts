// SPDX-License-Identifier: Apache-2.0
/**
 * The three endpoints of the app after the checks of `http/server.ts` (brief M2-49 Design
 * notes 4 to 7 and 12).
 *
 * - `POST /github/webhook`: `X-Hub-Signature-256` first (`401`), then `X-GitHub-Event` and
 *   `X-GitHub-Delivery` (`400`), the JSON (`400`), the duplicate check by delivery identifier,
 *   the event list, and in one store write the seen identifier and the job (`202`, or `204` when
 *   there is nothing to do).
 * - `POST /chat/interactions`: the SDK's `verifyWebhook` with the keys of `getWebhookKeys`
 *   (`401`), the duplicate check by `webhook-id`; `command.invoked` of `issue` and
 *   `interaction.created` queue a job (`202`), `command.autocomplete_requested` answers
 *   `{"choices": []}`; anything else `202` with Warn reason `unknown_request`.
 * - `POST /chat/events`: the same verification; `message.created` passes the mirror decision
 *   and queues `mirror` (`202`) or answers `204`; any other type `204`.
 *
 * A full queue, a failing store or a secret file that fails its check answers `503`.
 *
 * @example
 * ```ts
 * const routes = createRoutes(context);
 * await startHttpServer({ ...address, routes, ready, logger, maxBodyBytes });
 * ```
 *
 * @module
 */

import {
  WebhookVerificationError,
  uuidv7,
  verifyWebhook,
  type Clock,
  type Logger,
  type VerifiedWebhook,
} from "bothy-sdk";

import type { AppConfig } from "../config.js";
import { echoKey, mirrorDecision, type ChatMessage } from "../domain/mirror.js";
import { parseGitHubEvent } from "../github/events.js";
import { verifyGitHubSignature } from "../github/signature.js";
import type { RawRequest, Reply, RouteHandler } from "../http/server.js";
import type { JsonValue, NewJob, Store } from "../store/store.js";
import type { ChatPort } from "./port.js";

/** What the endpoints need. */
export interface ReceiveContext {
  /** The configuration. */
  readonly config: AppConfig;
  /** State and queue. */
  readonly store: Store;
  /** The keys of `getWebhookKeys`. */
  readonly chat: Pick<ChatPort, "webhookKeys">;
  /** Reads the GitHub webhook secret at each use (with the file check). */
  readonly webhookSecret: () => Promise<string>;
  /** The app's principal ID from `getMe`. */
  readonly ownPrincipalId: () => string;
  /** Called after a job was queued. */
  readonly wake: () => void;
  /** Time source of `webhook-timestamp` and receipt times. */
  readonly clock: Pick<Clock, "now">;
  /** The app's logger. */
  readonly logger: Logger;
}

const HEADER_SIGNATURE = "x-hub-signature-256";
const HEADER_EVENT = "x-github-event";
const HEADER_DELIVERY = "x-github-delivery";

function header(request: RawRequest, name: string): string | undefined {
  const value = request.headers[name];
  return typeof value === "string" && value !== "" ? value : undefined;
}

function decodeJson(body: Uint8Array): unknown {
  // Invalid UTF-8 is refused as JSON is (Design note 4).
  const text = new TextDecoder("utf-8", { fatal: true }).decode(body);
  return JSON.parse(text) as unknown;
}

function newJob(
  context: ReceiveContext,
  kind: string,
  data: JsonValue,
): NewJob {
  return {
    id: uuidv7(),
    kind,
    created_at: context.clock.now(),
    data,
    progress: {},
  };
}

/**
 * Remembers `seenKey` and queues `job` in one write and answers accordingly.
 *
 * @returns The reply: `status` when accepted, the first status of a duplicate (or `204` for
 *   GitHub), `503` when the queue is full or the store fails.
 */
async function admit(
  context: ReceiveContext,
  input: {
    seenKey: string;
    status: number;
    job?: NewJob;
    duplicateStatus?: number;
  },
  fields: Record<string, string | number>,
): Promise<Reply> {
  let result;
  try {
    result = await context.store.admit({
      seenKey: input.seenKey,
      status: input.status,
      job: input.job,
      maxJobs: context.config.maxQueuedJobs,
    });
  } catch {
    context.logger.error("store.failed", {
      ...fields,
      status: 503,
      reason: "store",
    });
    return { status: 503 };
  }
  if (result.kind === "duplicate") {
    const status = input.duplicateStatus ?? result.status;
    context.logger.info("delivery.duplicate", {
      ...fields,
      status,
      reason: "duplicate",
    });
    return { status };
  }
  if (result.kind === "full") {
    context.logger.warn("delivery.queue_full", {
      ...fields,
      status: 503,
      reason: "queue_full",
    });
    return { status: 503 };
  }
  if (input.job !== undefined) context.wake();
  context.logger.info("delivery.accepted", {
    ...fields,
    status: input.status,
    ...(input.job === undefined
      ? {}
      : { job_id: input.job.id, job_kind: input.job.kind }),
  });
  return { status: input.status };
}

/**
 * The handler of `POST /github/webhook`.
 *
 * @param context - Configuration, store, secret source, clock and logger.
 * @returns The route handler.
 */
export function githubWebhookRoute(context: ReceiveContext): RouteHandler {
  return async (request) => {
    const { logger } = context;
    let secret: string;
    try {
      secret = await context.webhookSecret();
    } catch {
      logger.error("github.delivery.refused", {
        status: 503,
        reason: "secret_file",
      });
      return { status: 503 };
    }
    const signature = verifyGitHubSignature(
      secret,
      request.body,
      request.headers[HEADER_SIGNATURE],
    );
    if (!signature.ok) {
      logger.warn("github.delivery.refused", {
        status: 401,
        reason: signature.reason,
      });
      return { status: 401 };
    }
    const event = header(request, HEADER_EVENT);
    const delivery = header(request, HEADER_DELIVERY);
    if (event === undefined || delivery === undefined) {
      logger.warn("github.delivery.refused", {
        status: 400,
        reason: "missing_header",
      });
      return { status: 400 };
    }
    const fields = { event, delivery_id: delivery };
    let payload: unknown;
    try {
      payload = decodeJson(request.body);
    } catch {
      logger.warn("github.delivery.refused", {
        ...fields,
        status: 400,
        reason: "invalid_json",
      });
      return { status: 400 };
    }
    const parsed = parseGitHubEvent(
      event,
      payload,
      context.config.repositories,
    );
    const seenKey = `gh:${delivery}`;
    if (context.store.state().seen[seenKey] !== undefined) {
      logger.info("github.delivery.duplicate", {
        ...fields,
        status: 204,
        reason: "duplicate",
      });
      return { status: 204 };
    }
    switch (parsed.kind) {
      case "invalid":
        logger.warn("github.delivery.refused", {
          ...fields,
          status: 400,
          reason: "invalid_payload",
        });
        return { status: 400 };
      case "ping":
        logger.info("github.delivery.ignored", {
          ...fields,
          status: 204,
          reason: "ping",
        });
        return admit(
          context,
          { seenKey, status: 204, duplicateStatus: 204 },
          fields,
        );
      case "ignored":
        logger.info("github.delivery.ignored", {
          ...fields,
          status: 204,
          reason: parsed.reason,
        });
        return admit(
          context,
          { seenKey, status: 204, duplicateStatus: 204 },
          fields,
        );
      case "event": {
        const withAction = { ...fields, action: parsed.event.action };
        const echo = echoKey(parsed.event);
        if (echo !== undefined && echo in context.store.state().created) {
          logger.info("github.delivery.ignored", {
            ...withAction,
            status: 204,
            reason: "own_github_object",
          });
          return admit(
            context,
            { seenKey, status: 204, duplicateStatus: 204 },
            withAction,
          );
        }
        const job = newJob(context, "github_event", {
          event: parsed.event as unknown as JsonValue,
        });
        return admit(
          context,
          { seenKey, status: 202, job, duplicateStatus: 204 },
          withAction,
        );
      }
    }
  };
}

/**
 * Verifies a request of the server with the SDK and the keys of `getWebhookKeys`.
 *
 * @returns The verified delivery, or the reply that refuses it.
 */
async function verifyServerRequest(
  context: ReceiveContext,
  request: RawRequest,
): Promise<VerifiedWebhook | Reply> {
  let keys;
  try {
    keys = await context.chat.webhookKeys();
  } catch (err) {
    context.logger.error("chat.request.refused", {
      status: 503,
      reason: "webhook_keys",
      code: err instanceof Error ? err.name : "unknown",
    });
    return { status: 503 };
  }
  try {
    return verifyWebhook({
      body: request.body,
      headers: request.headers,
      keys,
      now: () => context.clock.now(),
    });
  } catch (err) {
    if (err instanceof WebhookVerificationError) {
      context.logger.warn("chat.request.refused", {
        status: 401,
        reason: "verification",
        code: err.code,
      });
      return { status: 401 };
    }
    context.logger.error("chat.request.refused", {
      status: 503,
      reason: "webhook_config",
      code: err instanceof Error ? err.name : "unknown",
    });
    return { status: 503 };
  }
}

function isReply(value: VerifiedWebhook | Reply): value is Reply {
  return "status" in value;
}

type EventBody = { type?: unknown; payload?: unknown };

function envelope(verified: VerifiedWebhook): {
  type: string;
  payload: Record<string, unknown>;
} {
  const raw: EventBody =
    verified.event.kind === "known"
      ? (verified.event.event as EventBody)
      : ((verified.event.raw ?? {}) as EventBody);
  const type =
    verified.event.kind === "known" ? String(raw.type) : verified.event.type;
  const payload =
    typeof raw.payload === "object" && raw.payload !== null
      ? (raw.payload as Record<string, unknown>)
      : {};
  return { type, payload };
}

function optionValue(payload: Record<string, unknown>, name: string): string {
  const options = Array.isArray(payload.options)
    ? (payload.options as unknown[])
    : [];
  for (const option of options) {
    if (typeof option !== "object" || option === null) continue;
    const entry = option as { name?: unknown; value?: unknown };
    if (entry.name === name && typeof entry.value === "string")
      return entry.value;
  }
  return "";
}

/**
 * The handler of `POST /chat/interactions`.
 *
 * @param context - Configuration, store, keys, clock and logger.
 * @returns The route handler.
 */
export function interactionsRoute(context: ReceiveContext): RouteHandler {
  return async (request) => {
    const verified = await verifyServerRequest(context, request);
    if (isReply(verified)) return verified;
    const { type, payload } = envelope(verified);
    const fields = { event: type, webhook_id: verified.id };
    if (type === "command.autocomplete_requested") {
      context.logger.info("chat.autocomplete.answered", {
        ...fields,
        status: 200,
      });
      return { status: 200, json: { choices: [] } };
    }
    const seenKey = `wh:${verified.id}`;
    if (type === "command.invoked" && payload.command_name === "issue") {
      const channel = payload.channel_id;
      if (typeof channel !== "string") {
        context.logger.warn("chat.request.refused", {
          ...fields,
          status: 400,
          reason: "invalid_payload",
        });
        return { status: 400 };
      }
      const job = newJob(context, "issue_command", {
        channel_id: channel.toLowerCase(),
        title: optionValue(payload, "title"),
      });
      return admit(context, { seenKey, status: 202, job }, fields);
    }
    if (type === "interaction.created") {
      const interaction = payload.interaction as
        Record<string, unknown> | undefined;
      const id = interaction?.id;
      const messageId = interaction?.message_id;
      if (typeof id !== "string" || typeof messageId !== "string") {
        context.logger.warn("chat.request.refused", {
          ...fields,
          status: 400,
          reason: "invalid_payload",
        });
        return { status: 400 };
      }
      const values =
        typeof interaction?.values === "object" && interaction.values !== null
          ? (interaction.values as Record<string, JsonValue>)
          : {};
      const job = newJob(context, "issue_form", {
        interaction_id: id,
        custom_id:
          typeof interaction?.custom_id === "string"
            ? interaction.custom_id
            : "",
        message_id: messageId,
        values,
      });
      return admit(context, { seenKey, status: 202, job }, fields);
    }
    context.logger.warn("chat.request.ignored", {
      ...fields,
      status: 202,
      reason: "unknown_request",
    });
    return admit(context, { seenKey, status: 202 }, fields);
  };
}

/**
 * The handler of `POST /chat/events`.
 *
 * @param context - Configuration, store, keys, own principal, clock and logger.
 * @returns The route handler.
 */
export function eventsRoute(context: ReceiveContext): RouteHandler {
  return async (request) => {
    const verified = await verifyServerRequest(context, request);
    if (isReply(verified)) return verified;
    const { type, payload } = envelope(verified);
    const fields = { event: type, webhook_id: verified.id };
    const seenKey = `wh:${verified.id}`;
    if (type !== "message.created") {
      context.logger.info("chat.event.ignored", {
        ...fields,
        status: 204,
        reason: "unsupported",
      });
      return admit(context, { seenKey, status: 204 }, fields);
    }
    const message = payload.message as ChatMessage | undefined;
    if (
      typeof message !== "object" ||
      typeof message.thread_id !== "string" ||
      typeof message.author !== "object"
    ) {
      context.logger.warn("chat.request.refused", {
        ...fields,
        status: 400,
        reason: "invalid_payload",
      });
      return { status: 400 };
    }
    const decision = mirrorDecision(
      message,
      context.ownPrincipalId(),
      (thread) => context.store.state().thread_keys[thread],
    );
    if (!decision.mirror) {
      context.logger.info("chat.event.ignored", {
        ...fields,
        status: 204,
        reason: decision.reason,
        thread_id: message.thread_id,
      });
      return admit(context, { seenKey, status: 204 }, fields);
    }
    const job = newJob(context, "mirror", {
      message_id: message.id,
      key: decision.key,
      display_name: message.author.display_name,
      content: message.content_md,
      truncated: message.content_truncated === true,
    });
    return admit(
      context,
      { seenKey, status: 202, job },
      { ...fields, thread_id: message.thread_id },
    );
  };
}
