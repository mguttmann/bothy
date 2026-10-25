// SPDX-License-Identifier: Apache-2.0
/**
 * The real server of the live test: `sdkserver` of `services/server/test/sdkserver`, driven
 * over its JSON-line protocol on stdin (answers `SDKSERVER <json>` on stdout) as the SDK's
 * `test/live/sdkserver.ts` drives it, with the app ops of brief M2-49 Design note 19: `start`
 * with `app_front`, `app_backend`, `create_forum` and `call`. It runs inside the test worker,
 * because only the process that owns the binary's stdin can drive it. No token is printed: the
 * program writes answers of `call` into files of mode `0600`, which this driver reads and
 * removes.
 *
 * @module
 */

import { spawn, type ChildProcess } from "node:child_process";
import { chmod, mkdtemp, readFile, rm, unlink } from "node:fs/promises";
import { tmpdir } from "node:os";
import { join } from "node:path";
import { createInterface } from "node:readline";

import { DpopKey } from "bothy-sdk";

import { repositoryRoot } from "./global-setup.js";

const ANSWER = "SDKSERVER ";
// Waits for one answer (`start` builds a PostgreSQL container and the world) and for the exit
// after `stop`, the values of the SDK's driver.
const CALL_WAIT_MS = 300_000;
const STOP_WAIT_MS = 60_000;

/** One answer line, decoded. */
type Answer = Record<string, unknown>;

/** The result of {@link LiveServer.call}. */
export interface CallResult {
  /** The HTTP status. */
  readonly status: number;
  /** The JSON body, if any. */
  readonly body: Record<string, unknown>;
}

/** The lines of a stream in order. */
class Lines {
  readonly #queue: string[] = [];
  readonly #waiters: ((line: string | undefined) => void)[] = [];
  #ended = false;

  constructor(stream: NodeJS.ReadableStream) {
    const rl = createInterface({ input: stream, crlfDelay: Infinity });
    rl.on("line", (line) => {
      const waiter = this.#waiters.shift();
      if (waiter) waiter(line);
      else this.#queue.push(line);
    });
    rl.on("close", () => {
      this.#ended = true;
      for (const waiter of this.#waiters.splice(0)) waiter(undefined);
    });
  }

  next(timeoutMs: number): Promise<string | undefined> {
    const queued = this.#queue.shift();
    if (queued !== undefined) return Promise.resolve(queued);
    if (this.#ended) return Promise.resolve(undefined);
    return new Promise((resolve, reject) => {
      const waiter = (line: string | undefined): void => {
        clearTimeout(timer);
        resolve(line);
      };
      const timer = setTimeout(() => {
        const i = this.#waiters.indexOf(waiter);
        if (i >= 0) this.#waiters.splice(i, 1);
        reject(new Error("sdkserver did not answer in time"));
      }, timeoutMs);
      this.#waiters.push(waiter);
    });
  }
}

function text(answer: Answer, key: string): string {
  const value = answer[key];
  if (typeof value !== "string" || value === "")
    throw new Error(`sdkserver answer without ${key}`);
  return value;
}

/** A running `sdkserver` with the app ops. */
export class LiveServer {
  readonly #proc: ChildProcess;
  readonly #lines: Lines;
  readonly #exited: Promise<void>;
  #lock: Promise<unknown> = Promise.resolve();
  /** The work directory of mode `0700` (token and answer files). */
  readonly dir: string;
  /** The forwarder origin, `http://127.0.0.1:<port>`. */
  origin = "";
  /** The port of the TLS front. */
  frontPort = 0;

  /**
   * Wraps a spawned process; call {@link LiveServer.start} next.
   *
   * @param proc - The `sdkserver` process with piped stdin and stdout.
   * @param dir - The work directory.
   */
  constructor(proc: ChildProcess, dir: string) {
    if (!proc.stdin || !proc.stdout)
      throw new TypeError("sdkserver needs piped stdin and stdout");
    this.#proc = proc;
    this.dir = dir;
    this.#lines = new Lines(proc.stdout);
    proc.stdin.on("error", () => undefined);
    this.#exited = new Promise((resolve) => {
      if (proc.exitCode !== null || proc.signalCode !== null) resolve();
      else {
        proc.once("exit", () => resolve());
        proc.once("error", () => resolve());
      }
    });
  }

  #send(request: Answer, timeoutMs = CALL_WAIT_MS): Promise<Answer> {
    const op = String(request.op);
    const result = this.#lock.then(async () => {
      this.#proc.stdin?.write(`${JSON.stringify(request)}\n`);
      for (;;) {
        const line = await this.#lines.next(timeoutMs);
        if (line === undefined) throw new Error(`sdkserver ended during ${op}`);
        const trimmed = line.trim();
        if (!trimmed.startsWith(ANSWER)) continue;
        const answer = JSON.parse(trimmed.slice(ANSWER.length)) as Answer;
        if (answer.op === "error")
          throw new Error(`sdkserver ${op}: ${String(answer.detail)}`);
        return answer;
      }
    });
    this.#lock = result.catch(() => undefined);
    return result;
  }

  /**
   * Starts the server with `config` and the TLS front for `host`.
   *
   * @param config - Dotted keys of the configuration reference to their values.
   * @param host - The host of the app's addresses.
   */
  async start(config: Record<string, string>, host: string): Promise<void> {
    const key = await DpopKey.generate({ path: join(this.dir, "dpop.pem") });
    const ready = await this.#send({
      op: "start",
      config,
      agent_jkt: key.jkt,
      dir: this.dir,
      app_front: { host },
    });
    this.origin = text(ready, "origin");
    this.frontPort = Number(ready.front_port);
    // Lotse's token of the SDK scenarios is not needed here.
    await unlink(text(ready, "token_file"));
  }

  /**
   * Registers the app's port behind the front.
   *
   * @param port - The app's listener.
   */
  async appBackend(port: number): Promise<void> {
    await this.#send({ op: "app_backend", port });
  }

  /**
   * Creates a forum in the space Support.
   *
   * @param name - Its name.
   * @returns Its ID.
   */
  async createForum(name: string): Promise<string> {
    return text(await this.#send({ op: "create_forum", name }), "channel_id");
  }

  /**
   * Performs an operation of the public API as the owner or Alex.
   *
   * @param as - `owner` or `alex`.
   * @param operationId - The `operationId`.
   * @param options - Path parameters, query and body.
   * @returns Status and JSON body.
   */
  async call(
    as: "owner" | "alex",
    operationId: string,
    options: {
      path?: Record<string, string>;
      query?: Record<string, string>;
      body?: unknown;
    } = {},
  ): Promise<CallResult> {
    const answer = await this.#send({
      op: "call",
      as,
      operation_id: operationId,
      path_params: options.path ?? {},
      query: options.query ?? {},
      ...(options.body === undefined ? {} : { body: options.body }),
    });
    const file = text(answer, "file");
    const raw = JSON.parse(await readFile(file, "utf8")) as {
      status: number;
      body?: Record<string, unknown>;
    };
    await unlink(file);
    return { status: raw.status, body: raw.body ?? {} };
  }

  /** Stops the program and waits for its exit; kills it by PID only if it does not exit. */
  async stop(): Promise<void> {
    await this.#send({ op: "stop" }, STOP_WAIT_MS).catch(() => undefined);
    this.#proc.stdin?.end();
    let timer: NodeJS.Timeout | undefined;
    const exitedInTime = await Promise.race([
      this.#exited.then(() => true),
      new Promise<boolean>((resolve) => {
        timer = setTimeout(() => resolve(false), STOP_WAIT_MS);
      }),
    ]);
    clearTimeout(timer);
    if (!exitedInTime && this.#proc.pid !== undefined) {
      process.kill(this.#proc.pid, "SIGKILL");
      await this.#exited;
    }
    await rm(this.dir, { recursive: true, force: true });
  }
}

/**
 * Starts `sdkserver` with the TLS front for `host`.
 *
 * @param binary - The path `global-setup.ts` provides as `sdkserverBinary`.
 * @param config - The server configuration of `start`.
 * @param host - The host of the app's addresses.
 * @returns The started server.
 * @throws `Error` when the program cannot start (no Docker-compatible runtime, for example).
 */
export async function startLiveServer(
  binary: string,
  config: Record<string, string>,
  host: string,
): Promise<LiveServer> {
  const dir = await mkdtemp(join(tmpdir(), "github-app-live-"));
  await chmod(dir, 0o700);
  const proc = spawn(
    binary,
    ["-test.run", "^TestSDKServer$", "-test.timeout", "0"],
    {
      cwd: join(repositoryRoot(), "services", "server", "test", "sdkserver"),
      stdio: ["pipe", "pipe", "ignore"],
    },
  );
  const server = new LiveServer(proc, dir);
  try {
    await server.start(config, host);
  } catch (err) {
    await server.stop();
    throw err;
  }
  return server;
}
