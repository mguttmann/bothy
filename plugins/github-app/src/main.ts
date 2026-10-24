// SPDX-License-Identifier: Apache-2.0
/**
 * The command line entry: `node dist/main.js --config <file>` (brief M2-49 Design note 2).
 *
 * Loads the configuration, starts the app and stops it on `SIGTERM` or `SIGINT`: the exit code
 * is 0 after a clean stop and 1 on any start failure. A start failure prints one line that names
 * the failed step; configuration errors name key and file, never a value.
 *
 * @example
 * ```sh
 * node dist/main.js --config /etc/github-app/config.json
 * ```
 *
 * @module
 */

import { realpathSync } from "node:fs";
import { fileURLToPath } from "node:url";

import { startApp, type AppDeps, type RunningApp } from "./app.js";
import { ConfigError, loadConfig } from "./config.js";
import { StoreLockedError } from "./store/store.js";

/** What {@link main} needs from its process. */
export interface MainIo {
  /** Writes one line of the start failure to stderr. */
  readonly error: (line: string) => void;
  /**
   * Resolves on the first stop signal (`SIGTERM` or `SIGINT`).
   *
   * @returns A promise of the signal.
   */
  readonly signal: () => Promise<void>;
  /** Called once the app runs (tests). */
  readonly started?: ((app: RunningApp) => void) | undefined;
}

/** The usage line. */
export const USAGE = "usage: github-app --config <file>";

/**
 * Returns the value of `--config`.
 *
 * @param argv - The arguments after the program.
 * @returns The file, or `undefined` when the arguments are not exactly `--config <file>`.
 */
export function configArgument(argv: readonly string[]): string | undefined {
  if (argv.length === 2 && argv[0] === "--config" && argv[1] !== "")
    return argv[1];
  if (argv.length === 1 && argv[0]?.startsWith("--config=") === true) {
    const value = argv[0].slice("--config=".length);
    return value === "" ? undefined : value;
  }
  return undefined;
}

function describe(err: unknown): string {
  if (err instanceof ConfigError || err instanceof StoreLockedError)
    return err.message;
  // Other errors (the server, the listener) are named by their class only: their messages
  // could carry an address or an answer of the server.
  return `start failed (${err instanceof Error ? err.name : "unknown error"})`;
}

/**
 * Runs the app until a stop signal.
 *
 * @param argv - The arguments after the program.
 * @param io - stderr and the stop signal.
 * @param deps - Injectable dependencies of `startApp`.
 * @returns The exit code: 0 after a clean stop, 1 on a start failure, 2 on wrong arguments.
 */
export async function main(
  argv: readonly string[],
  io: MainIo,
  deps: AppDeps = {},
): Promise<number> {
  const file = configArgument(argv);
  if (file === undefined) {
    io.error(USAGE);
    return 2;
  }
  let app: RunningApp;
  try {
    app = await startApp(await loadConfig(file), deps);
  } catch (err) {
    io.error(describe(err));
    return 1;
  }
  io.started?.(app);
  await io.signal();
  await app.stop();
  return 0;
}

/* v8 ignore start: the process wiring runs only as a program. */
function isProgram(): boolean {
  const entry = process.argv[1];
  if (entry === undefined) return false;
  try {
    return realpathSync(entry) === realpathSync(fileURLToPath(import.meta.url));
  } catch {
    return false;
  }
}

if (isProgram()) {
  const io: MainIo = {
    error: (line) => {
      process.stderr.write(`${line}\n`);
    },
    signal: () =>
      new Promise((resolve) => {
        process.once("SIGTERM", () => {
          resolve();
        });
        process.once("SIGINT", () => {
          resolve();
        });
      }),
  };
  void main(process.argv.slice(2), io).then((code) => {
    process.exitCode = code;
  });
}
/* v8 ignore stop */
