// SPDX-License-Identifier: Apache-2.0
/**
 * Keys of issues and pull requests and the lookup of configured repositories (brief M2-49
 * Design note 9).
 *
 * Issues and pull requests share the numbers of a repository, so one key `<owner>/<name>#<n>`
 * in lower case, built from the configured repository name, names either. The thread map of the
 * store uses it.
 *
 * @example
 * ```ts
 * itemKey("Example-Org/Widgets", 12); // "example-org/widgets#12"
 * ```
 *
 * @module
 */

import type { RepositoryMapping } from "../config.js";

/**
 * Returns the key of an issue or pull request.
 *
 * @param repository - The configured `<owner>/<name>`.
 * @param number - The issue or pull request number.
 * @returns `<owner>/<name>#<number>` in lower case.
 */
export function itemKey(repository: string, number: number): string {
  return `${repository.toLowerCase()}#${String(number)}`;
}

/**
 * Finds the configured mapping of a repository, compared without case.
 *
 * @param repositories - `github_app.repositories`.
 * @param fullName - `repository.full_name` of a delivery.
 * @returns The mapping, or `undefined` when the repository is not configured.
 */
export function findRepository(
  repositories: readonly RepositoryMapping[],
  fullName: string,
): RepositoryMapping | undefined {
  const folded = fullName.toLowerCase();
  return repositories.find((r) => r.repository.toLowerCase() === folded);
}

/**
 * Returns the configured repositories of a forum, in configuration order.
 *
 * @param repositories - `github_app.repositories`.
 * @param channelId - The forum.
 * @returns Their names; empty when the forum is not configured.
 */
export function repositoriesOfForum(
  repositories: readonly RepositoryMapping[],
  channelId: string,
): string[] {
  const folded = channelId.toLowerCase();
  return repositories
    .filter((r) => r.channelId === folded)
    .map((r) => r.repository);
}
