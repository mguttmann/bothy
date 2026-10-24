// SPDX-License-Identifier: Apache-2.0
/**
 * The four jobs of the app and the handling of the server's answers (brief M2-49 Design notes
 * 9, 11, 12 and 13).
 *
 * - `github_event`: one message per GitHub event into the thread of its issue or pull request,
 *   or a new forum post for an unknown key; a thread out of reach (`404`) is replaced once.
 * - `issue_command`: `/issue` opens its own forum post with a form card and stores a draft.
 * - `issue_form`: the submitted form creates the issue on GitHub and answers in the thread.
 * - `mirror`: a person's reply in a linked thread becomes a GitHub comment.
 *
 * Every write to the server gets its `client_msg_id` or `Idempotency-Key` stored in the job's
 * progress before the first attempt, so a rerun repeats the same request. A write to GitHub is
 * marked `attempted` before it is sent and never repeated. {@link JobRunner.run} answers `done`
 * (the job ends) or `retry` (transient: the job stays queued and the worker pauses).
 *
 * @example
 * ```ts
 * const runner = new JobRunner(deps);
 * const outcome = await runner.run(job); // "done" | "retry"
 * ```
 *
 * @module
 */

import {
  ProblemError,
  TransportError,
  form,
  uuidv7,
  type Logger,
  type components,
} from "bothy-sdk";

import type { AppConfig } from "../config.js";
import { ConfigError } from "../config.js";
import type {
  ChatPort,
  InstanceLimits,
  MessageCreate,
} from "../chat/port.js";
import { findRepository, repositoriesOfForum } from "../domain/keys.js";
import { createdKey, echoKey } from "../domain/mirror.js";
import {
  bareTitle,
  cleanTitle,
  codeSpan,
  link,
  renderEvent,
  renderMirror,
  threadTitle,
} from "../domain/render.js";
import type { GitHubClient, GitHubResult } from "../github/client.js";
import type { GitHubEvent } from "../github/events.js";
import type { Draft, JobRecord, JsonValue, Store } from "../store/store.js";

/** The kinds of jobs. */
export type JobKind = "github_event" | "issue_command" | "issue_form" | "mirror";

/** Input of `github_event`. */
export interface GitHubEventJob {
  /** The parsed event (never the raw payload). */
  readonly event: GitHubEvent;
}

/** Input of `issue_command`. */
export interface IssueCommandJob {
  /** The forum the command ran in. */
  readonly channel_id: string;
  /** The option `title` as given. */
  readonly title: string;
}

/** Input of `issue_form`. */
export interface IssueFormJob {
  /** The interaction. */
  readonly interaction_id: string;
  /** The `custom_id` of the form, if the event carried one. */
  readonly custom_id: string;
  /** The message of the card. */
  readonly message_id: string;
  /** The submitted values. */
  readonly values: Readonly<Record<string, JsonValue>>;
}

/** Input of `mirror`. */
export interface MirrorJob {
  /** The message. */
  readonly message_id: string;
  /** The key of the issue or pull request of its thread. */
  readonly key: string;
  /** The author's display name. */
  readonly display_name: string;
  /** The content as delivered. */
  readonly content: string;
  /** Whether the delivered content was cut (`content_truncated`). */
  readonly truncated: boolean;
}

/** What {@link JobRunner.run} decided. */
export type JobOutcome = "done" | "retry";

/** The texts the person sees (Design note 11; English, without mentions). */
export const TEXTS = Object.freeze({
  /** The draft of the form is unknown or pruned. */
  expired: "This form expired. Run /issue again.",
  /** GitHub created the issue. */
  created: (n: number) => `Created issue #${String(n)}`,
  /** GitHub answered `3xx` or `4xx`. */
  refused: (status: number) =>
    `GitHub refused the issue (status ${String(status)}).`,
  /** GitHub's outcome is unknown. */
  unknown:
    "GitHub did not confirm the issue. Check the repository before you try again.",
  /** Title of a forum post whose option `title` is empty after cleaning. */
  newIssueTitle: "New issue",
  /** Title of the form. */
  cardTitle: "New GitHub issue",
  /** Text of the form card. */
  cardText: "Describe the issue and submit the form.",
  /** The submitted repository is not one the form offered. */
  chooseRepository: "Choose one of the listed repositories.",
  /** The GitHub token could not be read: nothing was sent. */
  notSent: "The app could not send the issue to GitHub. Try again later.",
});

/** The prefix of the `custom_id` of the form; the draft ID follows. */
export const FORM_PREFIX = "issue:";

/** The dependencies of {@link JobRunner}. */
export interface JobDeps {
  /** The configuration. */
  readonly config: AppConfig;
  /** State and queue. */
  readonly store: Store;
  /** The port to the server. */
  readonly chat: ChatPort;
  /** The GitHub client. */
  readonly github: GitHubClient;
  /** The app's logger. */
  readonly logger: Logger;
  /** Wall time in Unix milliseconds. */
  readonly now: () => number;
  /** The current limits; {@link JobDeps.refreshLimits} reloads them. */
  readonly limits: () => InstanceLimits;
  /** Reloads the limits from `getInstance`. */
  readonly refreshLimits: () => Promise<void>;
  /** Called on `401 unauthenticated`: the endpoints answer `503` until `getMe` succeeds. */
  readonly onUnauthenticated: () => void;
}

type Progress = Record<string, JsonValue>;

/** Ends a job early with its log line already written. */
class JobEnded extends Error {}

function text(value: JsonValue | undefined): string | undefined {
  return typeof value === "string" ? value : undefined;
}

function fieldCodes(err: ProblemError): string {
  return err.errors.map((e) => e.code).join(",");
}

function isTooLong(err: ProblemError): boolean {
  return (
    err.status === 400 &&
    err.code === "validation_failed" &&
    err.errors.some((e) => e.code === "too_long")
  );
}

function onTitle(err: ProblemError): boolean {
  return (
    err.status === 400 &&
    err.code === "validation_failed" &&
    err.errors.some((e) => e.pointer === "/title")
  );
}

/** Runs one job; the worker calls it in creation order, one at a time. */
export class JobRunner {
  readonly #deps: JobDeps;

  /**
   * Creates the runner.
   *
   * @param deps - Configuration, store, ports, logger and clock.
   */
  constructor(deps: JobDeps) {
    this.#deps = deps;
  }

  /**
   * Runs `job` until it ends or meets a transient failure.
   *
   * @param job - The queued job; its progress is saved after each step.
   * @returns `done` when the job ends (also after a permanent failure, logged), `retry` when
   *   it stays queued.
   */
  async run(job: JobRecord): Promise<JobOutcome> {
    const base = { job_id: job.id, job_kind: job.kind };
    try {
      switch (job.kind as JobKind) {
        case "github_event":
          await this.#githubEvent(job, job.data as unknown as GitHubEventJob);
          break;
        case "issue_command":
          await this.#issueCommand(job, job.data as unknown as IssueCommandJob);
          break;
        case "issue_form":
          await this.#issueForm(job, job.data as unknown as IssueFormJob);
          break;
        case "mirror":
          await this.#mirror(job, job.data as unknown as MirrorJob);
          break;
        default:
          this.#deps.logger.error("job.unknown_kind", base);
      }
      return "done";
    } catch (err) {
      return this.#failed(err, base);
    }
  }

  /** Classifies a failure (Design note 13); every permanent one is logged once. */
  #failed(err: unknown, base: Record<string, string>): JobOutcome {
    const { logger } = this.#deps;
    if (err instanceof JobEnded) return "done";
    if (err instanceof TransportError) {
      logger.warn("job.transient", { ...base, operation: err.operation, reason: "transport" });
      return "retry";
    }
    if (err instanceof ConfigError) {
      logger.error("job.secret_file", { ...base, reason: "secret_file" });
      return "done";
    }
    if (!(err instanceof ProblemError)) {
      logger.error("job.failed", { ...base, reason: "internal" });
      return "done";
    }
    const fields = {
      ...base,
      operation: err.operation ?? "",
      status: err.status,
      code: err.code,
    };
    if (err.status === 429 || err.status >= 500) {
      logger.warn("job.transient", { ...fields, reason: "server_unavailable" });
      return "retry";
    }
    if (err.status === 401) {
      logger.error("job.unauthenticated", { ...fields, reason: "unauthenticated" });
      this.#deps.onUnauthenticated();
      return "done";
    }
    if (err.status === 403 && err.code === "scope_missing") {
      logger.warn("job.scope_missing", {
        ...base,
        operation: err.operation ?? "",
        status: err.status,
        reason: "scope_missing",
      });
      return "done";
    }
    if (err.status === 422 && err.code === "idempotency_key_reuse") {
      logger.error("job.idempotency_key_reuse", { ...fields, reason: "idempotency_key_reuse" });
      return "done";
    }
    if (err.status === 400) {
      logger.warn("job.refused", { ...fields, code: fieldCodes(err) || err.code, reason: "validation_failed" });
      return "done";
    }
    logger.warn("job.refused", { ...fields, reason: err.reason ?? err.code });
    return "done";
  }

  /** Stores a value in the progress before the step that uses it. */
  async #ensure(job: JobRecord, key: string, make: () => string): Promise<string> {
    const existing = text(job.progress[key]);
    if (existing !== undefined) return existing;
    const value = make();
    job.progress[key] = value;
    await this.#deps.store.saveJob(job);
    return value;
  }

  async #set(job: JobRecord, values: Progress): Promise<void> {
    Object.assign(job.progress, values);
    await this.#deps.store.saveJob(job);
  }

  /** Creates a forum post; the `400` retries of Design note 10 included. */
  async #createThread(
    job: JobRecord,
    input: {
      channelId: string;
      title: () => string;
      bare: string;
      content: () => string;
    },
  ): Promise<string> {
    let bare = job.progress.bare_title === true;
    let refreshed = job.progress.limits_refreshed === true;
    for (;;) {
      const msgId = await this.#ensure(job, "thread_msg_id", uuidv7);
      const key = await this.#ensure(job, "thread_idem", uuidv7);
      try {
        const { threadId } = await this.#deps.chat.createForumThread({
          channelId: input.channelId,
          title: bare ? input.bare : input.title(),
          firstMessage: { client_msg_id: msgId, content_md: input.content() },
          idempotencyKey: key,
        });
        return threadId;
      } catch (err) {
        if (!(err instanceof ProblemError)) throw err;
        let again = false;
        if (isTooLong(err) && !refreshed) {
          await this.#deps.refreshLimits();
          refreshed = true;
          again = true;
        } else if (onTitle(err) && !bare) {
          bare = true;
          again = true;
        }
        if (!again) throw err;
        // A refused request created nothing; the next attempt gets new identifiers.
        await this.#set(job, {
          bare_title: bare,
          limits_refreshed: refreshed,
          thread_msg_id: uuidv7(),
          thread_idem: uuidv7(),
        });
      }
    }
  }

  async #githubEvent(job: JobRecord, data: GitHubEventJob): Promise<void> {
    const { event } = data;
    const { store, chat, logger } = this.#deps;
    const fields = {
      job_id: job.id,
      job_kind: job.kind,
      event: event.event,
      action: event.action,
      number: event.number,
    };
    const echo = echoKey(event);
    if (echo !== undefined && echo in store.state().created) {
      logger.info("github.event.skipped", { ...fields, reason: "own_github_object" });
      return;
    }
    const mapped = store.state().threads[event.key];
    if (mapped !== undefined && job.progress.replaced !== true) {
      let refreshed = job.progress.limits_refreshed === true;
      for (;;) {
        const msgId = await this.#ensure(job, "msg_id", uuidv7);
        try {
          await chat.postMessage(mapped.thread_id, {
            client_msg_id: msgId,
            content_md: renderEvent(event, this.#deps.limits()),
          });
          logger.info("github.event.posted", { ...fields, thread_id: mapped.thread_id });
          return;
        } catch (err) {
          if (err instanceof ProblemError && err.status === 404) {
            await store.update((s) => {
              if (s.threads[event.key]?.thread_id === mapped.thread_id) {
                delete s.threads[event.key];
                delete s.thread_keys[mapped.thread_id];
              }
            });
            await this.#set(job, { replaced: true });
            logger.warn("github.thread.replaced", {
              ...fields,
              thread_id: mapped.thread_id,
              reason: "thread_unreachable",
            });
            break;
          }
          if (err instanceof ProblemError && isTooLong(err) && !refreshed) {
            await this.#deps.refreshLimits();
            refreshed = true;
            await this.#set(job, { limits_refreshed: true, msg_id: uuidv7() });
            continue;
          }
          throw err;
        }
      }
    }
    const threadId = await this.#createThread(job, {
      channelId: event.channelId,
      title: () =>
        threadTitle(
          event.item,
          event.number,
          event.title,
          this.#deps.limits().thread_title_max_chars,
        ),
      bare: bareTitle(event.item, event.number),
      content: () => renderEvent(event, this.#deps.limits()),
    });
    await store.update((s) => {
      s.threads[event.key] = { thread_id: threadId, channel_id: event.channelId };
      s.thread_keys[threadId] = event.key;
    });
    logger.info("github.event.thread_created", { ...fields, thread_id: threadId, channel_id: event.channelId });
  }

  async #issueCommand(job: JobRecord, data: IssueCommandJob): Promise<void> {
    const { store, logger, config } = this.#deps;
    const fields = { job_id: job.id, job_kind: job.kind, channel_id: data.channel_id };
    const repositories = repositoriesOfForum(config.repositories, data.channel_id);
    if (repositories.length === 0) {
      logger.warn("issue.command.ignored", { ...fields, reason: "channel_not_mapped" });
      return;
    }
    const draftId = await this.#ensure(job, "draft_id", uuidv7);
    const max = this.#deps.limits().thread_title_max_chars;
    const title = cleanTitle(data.title, max);
    let threadId = text(job.progress.thread_id);
    if (threadId === undefined) {
      const names = repositories.map((r) => codeSpan(r)).join(" or ");
      threadId = await this.#createThread(job, {
        channelId: data.channel_id,
        title: () => cleanTitle(data.title, this.#deps.limits().thread_title_max_chars) || TEXTS.newIssueTitle,
        bare: TEXTS.newIssueTitle,
        content: () => `New GitHub issue for ${names}. Fill in the form below.`,
      });
      await this.#set(job, { thread_id: threadId });
    }
    const cardId = await this.#postCard(job, threadId, draftId, repositories, "card");
    const draft: Draft = {
      id: draftId,
      thread_id: threadId,
      channel_id: data.channel_id,
      card_message_id: cardId,
      title: title || TEXTS.newIssueTitle,
      repositories,
      created_at: this.#deps.now(),
    };
    await store.update((s) => {
      s.drafts[draftId] = draft;
    });
    logger.info("issue.command.form_posted", { ...fields, thread_id: threadId });
  }

  /** Posts the form card of a draft once per progress slot and returns its message ID. */
  async #postCard(
    job: JobRecord,
    threadId: string,
    draftId: string,
    repositories: readonly string[],
    slot: string,
  ): Promise<string> {
    const done = text(job.progress[`${slot}_message_id`]);
    if (done !== undefined) return done;
    const msgId = await this.#ensure(job, `${slot}_msg_id`, uuidv7);
    const fields: Parameters<typeof form>[2][number][] = [
      { name: "body", label: "Description", type: "textarea", required: false },
    ];
    if (repositories.length > 1)
      fields.push({
        name: "repository",
        label: "Repository",
        type: "choice",
        required: true,
        choices: [...repositories],
      });
    const message: MessageCreate = {
      client_msg_id: msgId,
      kind: "card",
      content_md: TEXTS.cardText,
      components: [
        form(`${FORM_PREFIX}${draftId}`, TEXTS.cardTitle, fields, {
          singleUse: true,
          restrictedTo: "any",
        }),
      ],
    };
    const posted = await this.#deps.chat.postMessage(threadId, message);
    await this.#set(job, { [`${slot}_message_id`]: posted.id });
    return posted.id;
  }

  /** `respondToInteraction` once per slot; a `409` (already answered) counts as done. */
  async #respond(
    job: JobRecord,
    interactionId: string,
    slot: string,
    body: components["schemas"]["InteractionRespond"],
  ): Promise<void> {
    if (job.progress[`${slot}_done`] === true) return;
    const key = await this.#ensure(job, `${slot}_idem`, uuidv7);
    try {
      await this.#deps.chat.respondInteraction(interactionId, body, key);
    } catch (err) {
      if (!(err instanceof ProblemError && err.status === 409)) throw err;
    }
    await this.#set(job, { [`${slot}_done`]: true });
  }

  async #issueForm(job: JobRecord, data: IssueFormJob): Promise<void> {
    const { store, logger, chat } = this.#deps;
    const fields = { job_id: job.id, job_kind: job.kind };
    const drafts = store.state().drafts;
    const byId = data.custom_id.startsWith(FORM_PREFIX)
      ? drafts[data.custom_id.slice(FORM_PREFIX.length)]
      : undefined;
    const draft =
      byId ??
      Object.values(drafts).find((d) => d.card_message_id === data.message_id);
    if (draft === undefined) {
      await this.#respond(job, data.interaction_id, "respond", {
        state: "rejected",
        response: { text: TEXTS.expired },
      });
      logger.info("issue.form.expired", { ...fields, reason: "draft_unknown" });
      return;
    }
    const chosen = data.values.repository;
    const repository =
      draft.repositories.length === 1
        ? draft.repositories[0]
        : typeof chosen === "string"
          ? draft.repositories.find((r) => r === chosen)
          : undefined;
    if (repository === undefined) {
      await this.#respond(job, data.interaction_id, "respond", {
        state: "rejected",
        response: { text: TEXTS.chooseRepository },
      });
      logger.info("issue.form.refused", { ...fields, reason: "repository_not_offered" });
      return;
    }
    let result = job.progress.github_result as GitHubResult | undefined;
    if (result === undefined) {
      if (job.progress.github === "attempted") {
        result = { kind: "unknown", reason: "restart" };
      } else {
        const description = typeof data.values.body === "string" ? data.values.body.trim() : "";
        const body = description === "" ? "Created from chat." : `${description}\n\nCreated from chat.`;
        await this.#set(job, { github: "attempted" });
        result = await this.#deps.github.createIssue(repository, draft.title, body);
      }
      await this.#set(job, { github_result: result as unknown as JsonValue });
    }
    switch (result.kind) {
      case "created": {
        const created = result;
        const key = `${repository.toLowerCase()}#${String(created.number)}`;
        await store.update((s) => {
          s.created[createdKey("issue", repository, created.number)] = this.#deps.now();
          s.threads[key] = { thread_id: draft.thread_id, channel_id: draft.channel_id };
          s.thread_keys[draft.thread_id] = key;
        });
        await this.#respond(job, data.interaction_id, "respond", {
          state: "acknowledged",
          response: { text: TEXTS.created(created.number) },
        });
        if (job.progress.confirmed !== true) {
          const msgId = await this.#ensure(job, "confirm_msg_id", uuidv7);
          await chat.postMessage(draft.thread_id, {
            client_msg_id: msgId,
            content_md: `Created GitHub issue ${link(`#${String(created.number)}`, created.htmlUrl)}.`,
          });
          await this.#set(job, { confirmed: true });
        }
        await store.update((s) => {
          delete s.drafts[draft.id];
        });
        logger.info("issue.form.created", { ...fields, number: created.number, thread_id: draft.thread_id, outcome: "created" });
        return;
      }
      case "refused":
      case "not_sent": {
        await this.#respond(job, data.interaction_id, "respond", {
          state: "failed",
          response: {
            text: result.kind === "refused" ? TEXTS.refused(result.status) : TEXTS.notSent,
          },
        });
        const cardId = await this.#postCard(job, draft.thread_id, draft.id, draft.repositories, "retry_card");
        await store.update((s) => {
          const current = s.drafts[draft.id];
          if (current !== undefined) s.drafts[draft.id] = { ...current, card_message_id: cardId };
        });
        logger.warn("issue.form.refused", {
          ...fields,
          ...(result.kind === "refused" ? { status: result.status } : { reason: result.reason }),
          outcome: result.kind,
        });
        return;
      }
      case "unknown": {
        await this.#respond(job, data.interaction_id, "respond", {
          state: "failed",
          response: { text: TEXTS.unknown },
        });
        await store.update((s) => {
          delete s.drafts[draft.id];
        });
        logger.warn("issue.form.unknown", {
          ...fields,
          ...(result.status === undefined ? {} : { status: result.status }),
          outcome: "unknown",
          reason: result.reason ?? "github_outcome_unknown",
        });
        return;
      }
    }
  }

  async #mirror(job: JobRecord, data: MirrorJob): Promise<void> {
    const { store, logger, config } = this.#deps;
    const fields = { job_id: job.id, job_kind: job.kind };
    const hash = data.key.lastIndexOf("#");
    const repository = findRepository(config.repositories, data.key.slice(0, hash));
    const number = Number(data.key.slice(hash + 1));
    if (repository === undefined || !Number.isSafeInteger(number)) {
      logger.warn("mirror.skipped", { ...fields, reason: "repository_not_configured" });
      return;
    }
    if (job.progress.github === "attempted") {
      logger.warn("mirror.unknown", { ...fields, number, outcome: "unknown", reason: "github_outcome_unknown" });
      return;
    }
    let content = text(job.progress.content) ?? data.content;
    if (data.truncated && job.progress.content === undefined) {
      content = (await this.#deps.chat.getMessage(data.message_id)).content_md;
      await this.#set(job, { content });
    }
    await this.#set(job, { github: "attempted" });
    const result = await this.#deps.github.createComment(
      repository.repository,
      number,
      renderMirror(data.display_name, content),
    );
    if (result.kind === "created") {
      await store.update((s) => {
        s.created[createdKey("comment", repository.repository, result.id)] = this.#deps.now();
      });
      logger.info("mirror.created", { ...fields, number, outcome: "created" });
      return;
    }
    logger.warn("mirror.failed", {
      ...fields,
      number,
      outcome: result.kind,
      ...(result.kind === "not_sent"
        ? { reason: result.reason }
        : result.status === undefined
          ? { reason: "github_outcome_unknown" }
          : { status: result.status }),
    });
  }
}
