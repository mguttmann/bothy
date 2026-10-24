// SPDX-License-Identifier: Apache-2.0
/**
 * Mirroring of replies and the loop protection (docs/spec/11-extensions.md 10.1 rows "Scopes"
 * and "Loops"; brief M2-49 Design note 12).
 *
 * {@link mirrorDecision} decides at receipt of `message.created`, in this order: the app's own
 * message (`own_message`), a message whose `origin` is not `human` (`not_human`), a message of
 * another kind than `text` (`not_text`), a deleted message (`deleted`), a thread without an
 * issue or pull request (`thread_not_mapped`); otherwise the message is mirrored. Echo protection
 * in the other direction is {@link createdKey}: the store records each issue and comment the app
 * created on GitHub, and their `opened` or `created` events are not posted.
 *
 * @example
 * ```ts
 * const decision = mirrorDecision(message, ownId, (thread) => store.keyOfThread(thread));
 * if (decision.mirror) queue(decision.key);
 * ```
 *
 * @module
 */

import type { components } from "bothy-sdk";

import type { GitHubEvent } from "../github/events.js";

/** A message as the server delivers it. */
export type ChatMessage = components["schemas"]["Message"];

/** Why a message is not mirrored. */
export type MirrorSkip =
  | "own_message"
  | "not_human"
  | "not_text"
  | "deleted"
  | "thread_not_mapped";

/** The outcome of {@link mirrorDecision}. */
export type MirrorDecision =
  | {
      /** The message goes to GitHub. */
      readonly mirror: true;
      /** The key of the issue or pull request of its thread. */
      readonly key: string;
    }
  | {
      /** The message stays in the chat. */
      readonly mirror: false;
      /** Why. */
      readonly reason: MirrorSkip;
    };

/**
 * Decides whether a new message goes to GitHub.
 *
 * @param message - The message of `message.created`.
 * @param ownPrincipalId - The app's principal ID from `getMe`.
 * @param keyOfThread - The key of the issue or pull request a thread belongs to.
 * @returns Mirror with the key, or skip with the reason.
 */
export function mirrorDecision(
  message: ChatMessage,
  ownPrincipalId: string,
  keyOfThread: (threadId: string) => string | undefined,
): MirrorDecision {
  if (message.author.id === ownPrincipalId)
    return { mirror: false, reason: "own_message" };
  if (message.origin !== "human") return { mirror: false, reason: "not_human" };
  if (message.kind !== "text") return { mirror: false, reason: "not_text" };
  if (message.deleted_at !== undefined)
    return { mirror: false, reason: "deleted" };
  const key = keyOfThread(message.thread_id);
  if (key === undefined) return { mirror: false, reason: "thread_not_mapped" };
  return { mirror: true, key };
}

/**
 * Returns the store key of a GitHub object the app created.
 *
 * @param kind - `issue` for an issue, `comment` for an issue comment.
 * @param repository - The configured repository name.
 * @param id - The issue number or the comment ID.
 * @returns The key.
 */
export function createdKey(
  kind: "issue" | "comment",
  repository: string,
  id: number,
): string {
  return `${kind}:${repository.toLowerCase()}:${String(id)}`;
}

/**
 * Returns the store key of the object an event announces as new, if it is one the app may have
 * created itself: an `issues` `opened` event names the issue, an `issue_comment` `created`
 * event the comment.
 *
 * @param event - The parsed event.
 * @returns The key to look up, or `undefined` for every other event.
 */
export function echoKey(event: GitHubEvent): string | undefined {
  if (event.event === "issues" && event.action === "opened")
    return createdKey("issue", event.repository, event.number);
  if (
    event.event === "issue_comment" &&
    event.action === "created" &&
    event.commentId !== undefined
  )
    return createdKey("comment", event.repository, event.commentId);
  return undefined;
}
