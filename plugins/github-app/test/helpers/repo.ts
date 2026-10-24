// SPDX-License-Identifier: Apache-2.0
/**
 * Paths of the repository the tests read (specification, API contract, configuration
 * reference) and of the app folder.
 *
 * @module
 */

import { readFileSync } from "node:fs";
import { dirname, join } from "node:path";

/** The app folder `plugins/github-app`. */
export const APP_DIR = dirname(dirname(import.meta.dirname));

/** The repository root. */
export const REPO_ROOT = dirname(dirname(APP_DIR));

/**
 * Reads a file of the repository.
 *
 * @param path - The path below the repository root.
 * @returns The UTF-8 text.
 */
export function repoFile(path: string): string {
  return readFileSync(join(REPO_ROOT, path), "utf8");
}

/**
 * Reads a file of the app folder.
 *
 * @param path - The path below `plugins/github-app`.
 * @returns The UTF-8 text.
 */
export function appFile(path: string): string {
  return readFileSync(join(APP_DIR, path), "utf8");
}

/**
 * The scopes of the row "Scopes" of docs/spec/11-extensions.md 10.1: the backticked scopes
 * before "optionally" are required, those after it optional.
 *
 * @returns Required and optional scopes.
 */
export function specScopes(): { required: string[]; optional: string[] } {
  const chapter = repoFile("docs/spec/11-extensions.md");
  const section = chapter.slice(
    chapter.indexOf("### 10.1 Code hosting (GitHub)"),
    chapter.indexOf("### 10.2 "),
  );
  const row =
    section.split("\n").find((line) => line.startsWith("| Scopes |")) ?? "";
  const [required = "", optional = ""] = row.split("optionally");
  const scopes = (text: string): string[] => [
    ...new Set(
      [...text.matchAll(/`([a-z_]+:[a-z_]+)`/g)].map((m) => m[1] ?? ""),
    ),
  ];
  return { required: scopes(required), optional: scopes(optional) };
}
