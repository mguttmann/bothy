// SPDX-License-Identifier: Apache-2.0
/**
 * The closed list of GitHub events the app handles (brief M2-49 Design note 6).
 *
 * Supported: `ping`; `issues` with `opened`, `closed`, `reopened`; `issue_comment` with
 * `created` (on issues and pull requests, `issue.pull_request` marks a pull request);
 * `pull_request` with `opened`, `closed` (merged when `pull_request.merged` is true),
 * `reopened`; `pull_request_review` with `submitted`. Every other event or action is ignored
 * with reason `unsupported`; a repository outside `github_app.repositories` with reason
 * `repository_not_configured`. The parser keeps only the fields the app uses; a missing required
 * field makes the payload invalid. The raw payload is never stored.
 *
 * @example
 * ```ts
 * const parsed = parseGitHubEvent("issues", payload, config.repositories);
 * if (parsed.kind === "event") queue(parsed.event);
 * ```
 *
 * @module
 */

import type { RepositoryMapping } from "../config.js";
import { findRepository, itemKey } from "../domain/keys.js";

/** Issue or pull request. */
export type ItemKind = "issue" | "pull_request";

/** The supported event names other than `ping`. */
export type GitHubEventName =
  "issues" | "issue_comment" | "pull_request" | "pull_request_review";

/** A parsed delivery: only the fields the app uses. */
export interface GitHubEvent {
  /** The `X-GitHub-Event` value. */
  readonly event: GitHubEventName;
  /** The `action` of the payload. */
  readonly action: string;
  /** The configured repository name (`<owner>/<name>`). */
  readonly repository: string;
  /** The forum of the repository. */
  readonly channelId: string;
  /** The key of the issue or pull request (`domain/keys.ts`). */
  readonly key: string;
  /** Issue or pull request. */
  readonly item: ItemKind;
  /** The number of the issue or pull request. */
  readonly number: number;
  /** The title of the issue or pull request. */
  readonly title: string;
  /** The `html_url` the message links to: the comment's or review's, else the item's. */
  readonly htmlUrl: string;
  /** The login of the person who caused the event (`sender.login`). */
  readonly actor: string;
  /** The body of the issue, pull request, comment or review; empty when there is none. */
  readonly text: string;
  /** The comment ID of `issue_comment`. */
  readonly commentId?: number | undefined;
  /** The review state of `pull_request_review` (`approved`, `commented`, ...). */
  readonly reviewState?: string | undefined;
  /** Whether a closed pull request was merged. */
  readonly merged?: boolean | undefined;
}

/** What {@link parseGitHubEvent} found. */
export type ParsedDelivery =
  | {
      /** A supported event. */
      readonly kind: "event";
      /** Its fields. */
      readonly event: GitHubEvent;
    }
  | {
      /** GitHub's `ping`. */
      readonly kind: "ping";
    }
  | {
      /** Nothing to do. */
      readonly kind: "ignored";
      /** Why. */
      readonly reason: "unsupported" | "repository_not_configured";
    }
  | {
      /** A required field is missing. */
      readonly kind: "invalid";
    };

const SUPPORTED: Readonly<Record<GitHubEventName, readonly string[]>> = {
  issues: ["opened", "closed", "reopened"],
  issue_comment: ["created"],
  pull_request: ["opened", "closed", "reopened"],
  pull_request_review: ["submitted"],
};

type Json = Record<string, unknown>;

/** A required member is missing or has the wrong type. */
class InvalidPayload extends Error {}

function object(value: unknown): Json {
  if (typeof value !== "object" || value === null || Array.isArray(value))
    throw new InvalidPayload();
  return value as Json;
}

function string(value: unknown): string {
  if (typeof value !== "string") throw new InvalidPayload();
  return value;
}

function optionalText(value: unknown): string {
  if (value === undefined || value === null) return "";
  return string(value);
}

function positive(value: unknown): number {
  if (typeof value !== "number" || !Number.isSafeInteger(value) || value <= 0)
    throw new InvalidPayload();
  return value;
}

function isSupported(event: string): event is GitHubEventName {
  return Object.hasOwn(SUPPORTED, event);
}

/**
 * Parses a verified delivery.
 *
 * @param eventName - The `X-GitHub-Event` header.
 * @param payload - The parsed JSON body.
 * @param repositories - `github_app.repositories`.
 * @returns The parsed event, `ping`, an ignored delivery with its reason, or `invalid`.
 */
export function parseGitHubEvent(
  eventName: string,
  payload: unknown,
  repositories: readonly RepositoryMapping[],
): ParsedDelivery {
  if (eventName === "ping") return { kind: "ping" };
  try {
    const body = object(payload);
    const action = body.action;
    if (
      !isSupported(eventName) ||
      typeof action !== "string" ||
      !SUPPORTED[eventName].includes(action)
    )
      return { kind: "ignored", reason: "unsupported" };
    const repository = findRepository(
      repositories,
      string(object(body.repository).full_name),
    );
    if (repository === undefined)
      return { kind: "ignored", reason: "repository_not_configured" };
    const actor = string(object(body.sender).login);
    const common = {
      event: eventName,
      action,
      repository: repository.repository,
      channelId: repository.channelId,
      actor,
    };
    switch (eventName) {
      case "issues": {
        const issue = object(body.issue);
        const number = positive(issue.number);
        return {
          kind: "event",
          event: {
            ...common,
            key: itemKey(repository.repository, number),
            item: issue.pull_request === undefined ? "issue" : "pull_request",
            number,
            title: string(issue.title),
            htmlUrl: string(issue.html_url),
            text: optionalText(issue.body),
          },
        };
      }
      case "issue_comment": {
        const issue = object(body.issue);
        const comment = object(body.comment);
        const number = positive(issue.number);
        return {
          kind: "event",
          event: {
            ...common,
            key: itemKey(repository.repository, number),
            item: issue.pull_request === undefined ? "issue" : "pull_request",
            number,
            title: string(issue.title),
            htmlUrl: string(comment.html_url),
            text: optionalText(comment.body),
            commentId: positive(comment.id),
          },
        };
      }
      case "pull_request": {
        const pr = object(body.pull_request);
        const number = positive(pr.number);
        return {
          kind: "event",
          event: {
            ...common,
            key: itemKey(repository.repository, number),
            item: "pull_request",
            number,
            title: string(pr.title),
            htmlUrl: string(pr.html_url),
            text: optionalText(pr.body),
            merged: pr.merged === true,
          },
        };
      }
      case "pull_request_review": {
        const pr = object(body.pull_request);
        const review = object(body.review);
        const number = positive(pr.number);
        return {
          kind: "event",
          event: {
            ...common,
            key: itemKey(repository.repository, number),
            item: "pull_request",
            number,
            title: string(pr.title),
            htmlUrl: string(review.html_url),
            text: optionalText(review.body),
            reviewState: string(review.state),
          },
        };
      }
    }
  } catch (err) {
    if (err instanceof InvalidPayload) return { kind: "invalid" };
    throw err;
  }
}
