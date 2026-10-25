// SPDX-License-Identifier: Apache-2.0
/**
 * Webhook payloads for the tests (brief M2-49 Design note 17): the examples of
 * `@octokit/webhooks-examples`, which collects the example payloads of the GitHub webhook
 * documentation (see `test/fixtures/README.md`), chosen by event and action and changed only in
 * repository name, numbers and IDs; three variants the package does not hold are minimal files
 * under `test/fixtures/github/`. Nothing of the package is copied into the repository.
 *
 * @module
 */

import { readFileSync } from "node:fs";
import { createRequire } from "node:module";
import { dirname, join } from "node:path";

/** The repository every fixture is moved to. */
export const REPOSITORY = "example-org/widgets";

type Json = Record<string, unknown>;

interface WebhookDefinition {
  readonly name: string;
  readonly examples: readonly Json[];
}

const require = createRequire(import.meta.url);
const definitions = JSON.parse(
  readFileSync(require.resolve("@octokit/webhooks-examples"), "utf8"),
) as WebhookDefinition[];

/**
 * The first example of `event` with `action` from the package (any action for `ping`).
 *
 * @param event - The `X-GitHub-Event`.
 * @param action - The `action`; omitted for `ping`.
 * @returns A deep copy.
 * @throws `Error` when the package holds no such example.
 */
export function packageExample(event: string, action?: string): Json {
  const definition = definitions.find((d) => d.name === event);
  const example = definition?.examples.find(
    (e) => action === undefined || e.action === action,
  );
  if (example === undefined)
    throw new Error(`no example for ${event} ${action ?? ""}`);
  return structuredClone(example);
}

/**
 * One of the minimal files of `test/fixtures/github/`.
 *
 * @param name - The file name without `.json`.
 * @returns The payload.
 */
export function localExample(name: string): Json {
  const dir = join(dirname(import.meta.dirname), "fixtures", "github");
  return JSON.parse(readFileSync(join(dir, `${name}.json`), "utf8")) as Json;
}

function member(value: Json, key: string): Json {
  const inner = value[key];
  if (typeof inner !== "object" || inner === null)
    throw new Error(`fixture without ${key}`);
  return inner as Json;
}

/** The changes a test may make to an example: repository, numbers and IDs. */
export interface Retarget {
  /** `<owner>/<name>`; default {@link REPOSITORY}. */
  readonly repository?: string | undefined;
  /** The issue or pull request number. */
  readonly number?: number | undefined;
  /** The comment ID of `issue_comment`. */
  readonly commentId?: number | undefined;
}

/**
 * Moves an example to another repository, number and comment ID; `html_url` values follow.
 *
 * @param payload - A copy of an example.
 * @param target - The new repository, number and comment ID.
 * @returns The same object, changed.
 */
export function retarget(payload: Json, target: Retarget = {}): Json {
  const repository = target.repository ?? REPOSITORY;
  const [owner = "", name = ""] = repository.split("/");
  const repo = member(payload, "repository");
  const oldName = String(repo.full_name);
  repo.full_name = repository;
  repo.name = name;
  if (typeof repo.owner === "object" && repo.owner !== null)
    (repo.owner as Json).login = owner;
  const item =
    (payload.issue as Json | undefined) ??
    (payload.pull_request as Json | undefined);
  if (item !== undefined) {
    const oldNumber = Number(item.number);
    const number = target.number ?? oldNumber;
    item.number = number;
    if (typeof payload.number === "number") payload.number = number;
    if (payload.pull_request !== undefined && payload.issue === undefined)
      (payload.pull_request as Json).number = number;
    const move = (url: unknown): unknown =>
      typeof url === "string"
        ? url
            .replace(oldName, repository)
            .replace(
              new RegExp(`/(issues|pull)/${String(oldNumber)}(?=$|#|/)`),
              `/$1/${String(number)}`,
            )
        : url;
    item.html_url = move(item.html_url);
    for (const key of ["comment", "review"]) {
      const inner = payload[key] as Json | undefined;
      if (inner !== undefined) inner.html_url = move(inner.html_url);
    }
  }
  const comment = payload.comment as Json | undefined;
  if (comment !== undefined && target.commentId !== undefined) {
    comment.id = target.commentId;
    comment.html_url = String(comment.html_url).replace(
      /#issuecomment-\d+$/,
      `#issuecomment-${String(target.commentId)}`,
    );
  }
  return payload;
}

/**
 * A payload of `event` and `action` moved to the target: the package's example, or the minimal
 * file for `issues` `closed`, a merged `pull_request` and an `issue_comment` on a pull request.
 *
 * @param event - The `X-GitHub-Event`.
 * @param action - The `action` (`closed.merged` and `created.pull_request` name the variants).
 * @param target - Repository, number and comment ID.
 * @returns The payload.
 */
export function fixture(
  event: string,
  action: string,
  target: Retarget = {},
): Json {
  const local: Record<string, string> = {
    "issues closed": "issues.closed",
    "pull_request closed.merged": "pull_request.closed.merged",
    "issue_comment created.pull_request": "issue_comment.created.pull_request",
  };
  const file = local[`${event} ${action}`];
  const payload =
    file !== undefined
      ? localExample(file)
      : packageExample(event, event === "ping" ? undefined : action);
  return retarget(payload, target);
}
