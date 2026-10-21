// SPDX-License-Identifier: Apache-2.0
/**
 * Vitest globalSetup of the live files: builds the test binary of `sdkserver`
 * (`services/server/test/sdkserver`) once per run, the way the session fixture `sdkserver_binary`
 * of `sdk/python/tests/realserver/conftest.py` does, and hands its path to the test files under
 * the key `sdkserverBinary` (`inject("sdkserverBinary")`). Without the Go tool chain the run
 * fails; it never skips (brief M2-30 Boundaries).
 *
 * @module
 */

import { spawn } from "node:child_process";
import { existsSync } from "node:fs";
import { mkdtemp, rm } from "node:fs/promises";
import { tmpdir } from "node:os";
import { delimiter, dirname, join } from "node:path";

import type { TestProject } from "vitest/node";

declare module "vitest" {
  /** The values the live globalSetup provides to the test files. */
  export interface ProvidedContext {
    /** Absolute path of the built `sdkserver.test` binary. */
    sdkserverBinary: string;
  }
}

/**
 * Returns the directory above this file that holds `docs/api`, the repository root.
 *
 * @returns The absolute path of the repository root.
 * @throws `Error` when no such directory exists.
 */
export function repositoryRoot(): string {
  let dir = import.meta.dirname;
  for (;;) {
    if (existsSync(join(dir, "docs", "api"))) return dir;
    const parent = dirname(dir);
    if (parent === dir) throw new Error("repository root not found");
    dir = parent;
  }
}

/**
 * Finds `go` on `PATH` like Python's `shutil.which`.
 *
 * @returns The absolute path of `go`.
 * @throws `Error` when `go` is not on `PATH`.
 */
function findGo(): string {
  for (const dir of (process.env.PATH ?? "").split(delimiter)) {
    if (!dir) continue;
    const candidate = join(dir, "go");
    if (existsSync(candidate)) return candidate;
  }
  throw new Error("the live tests need the Go tool chain (go not on PATH)");
}

/**
 * Runs `command` with `args` in `cwd`, its output on the terminal of the run.
 *
 * @param command - The program.
 * @param args - Its arguments, no shell.
 * @param cwd - The working directory.
 * @throws `Error` when the program cannot start or exits with another code than 0.
 */
function run(
  command: string,
  args: readonly string[],
  cwd: string,
): Promise<void> {
  return new Promise((resolve, reject) => {
    const child = spawn(command, args, {
      cwd,
      stdio: ["ignore", "inherit", "inherit"],
    });
    child.once("error", reject);
    child.once("exit", (code, signal) => {
      if (code === 0) resolve();
      else
        reject(
          new Error(
            `${command} ${args.join(" ")} ended with ${signal ?? `code ${String(code)}`}`,
          ),
        );
    });
  });
}

/**
 * Builds `sdkserver.test` into a new temporary directory and provides its path.
 *
 * @param project - The live project of the run; receives `sdkserverBinary`.
 * @returns The teardown, which removes the temporary directory.
 * @throws `Error` when `go` is missing or the build fails; the run then fails.
 */
export default async function setup(
  project: TestProject,
): Promise<() => Promise<void>> {
  const go = findGo();
  const work = await mkdtemp(join(tmpdir(), "bothy-sdkserver-"));
  const teardown = (): Promise<void> =>
    rm(work, { recursive: true, force: true });
  try {
    const binary = join(work, "sdkserver.test");
    await run(
      go,
      ["test", "-c", "-tags", "sdkserver", "-o", binary, "./test/sdkserver"],
      join(repositoryRoot(), "services", "server"),
    );
    project.provide("sdkserverBinary", binary);
  } catch (err) {
    await teardown();
    throw err;
  }
  return teardown;
}
