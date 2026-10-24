// SPDX-License-Identifier: Apache-2.0
/**
 * The real server of the SDK scenarios: `sdkserver` of `services/server/test/sdkserver`, ported
 * from `sdk/python/tests/realserver/conftest.py`.
 *
 * {@link startRealServer} starts the binary that `global-setup.ts` built, with short real-time
 * values of the server keys, and returns a {@link RealServer}: the `ScenarioServer` of
 * `src/testing/scenario-server.ts` over its JSON-line protocol on stdin (answers
 * `SDKSERVER <json>` on stdout). It runs inside the test worker, because only the process that
 * owns the binary's stdin can drive it. It needs the Go tool chain and a Docker-compatible
 * runtime; without them the tests fail, they never skip. {@link RealServer.stop} sends `stop`
 * and waits for the exit; only when the program does not exit is it killed, by PID.
 *
 * @example
 * ```ts
 * let server: RealServer;
 * beforeAll(async () => {
 *   server = await startRealServer(inject("sdkserverBinary"));
 * });
 * afterAll(() => server?.stop());
 * ```
 *
 * @module
 */

import { spawn, type ChildProcess } from "node:child_process";
import { chmod, mkdtemp, readFile, rm, unlink } from "node:fs/promises";
import { tmpdir } from "node:os";
import { join } from "node:path";
import { createInterface } from "node:readline";

import { DpopKey } from "../../src/auth/dpop.js";
import { Secret } from "../../src/auth/secret.js";
import type { Credentials } from "../../src/auth/types.js";
import { systemClock, type Clock } from "../../src/clock.js";
import {
  parseDuration,
  resolveSettings,
  type SdkSettings,
} from "../../src/config.js";
import type {
  ForgedVariant,
  RunView,
  ScenarioServer,
  Triggered,
} from "../../src/testing/scenario-server.js";
import { repositoryRoot } from "./global-setup.js";

const ANSWER = "SDKSERVER ";

/**
 * Short real-time values of the server keys, the `config` of `start`
 * (`services/server/test/sdkserver/README.md` "Timing values", the same set as the Python
 * fixture and `realServerTimings` of `protocol_test.go`). The adapter leeway of
 * {@link SDK_SETTINGS} stays at most `auth.clock_leeway` (09 4.5 "Keys and rotation" point 2).
 */
export const TIMINGS: Readonly<Record<string, string>> = Object.freeze({
  "agent.invocation_token_ttl": "5s",
  "auth.clock_leeway": "1s",
  "agent.ack_timeout": "2s",
  "agent.ack_persist_interval": "100ms",
  "sync.heartbeat_interval": "2s",
});

/** The SDK settings of the adapter under test, the values of the Python fixture. */
export const SDK_SETTINGS: SdkSettings = resolveSettings({
  invocationTokenLeewayMs: 1_000,
  reconnectBackoffBaseMs: 100,
  reconnectBackoffCapMs: 500,
  retryBackoffBaseMs: 100,
  retryBackoffCapMs: 1_000,
  pollIntervalMs: 200,
});

// Wait for one answer (`start` builds a PostgreSQL container and the world) and for the exit
// after `stop`; the values of the Python fixture.
const CALL_WAIT_MS = 300_000;
const STOP_WAIT_MS = 60_000;

const NIL_UUID = "00000000-0000-0000-0000-000000000000";

/** One answer line of `sdkserver`, decoded. */
type Answer = Record<string, unknown>;

/** The lines of a stream in order; `undefined` once the stream ended. */
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

  /** The next line, `undefined` at the end, a rejection after `timeoutMs`. */
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

function integer(answer: Answer, key: string): number {
  const value = Number(answer[key]);
  if (!Number.isInteger(value))
    throw new Error(`sdkserver answer without ${key}`);
  return value;
}

/**
 * `ScenarioServer` over a running `sdkserver` process (Python `RealServer`). Create it with
 * {@link startRealServer}; the constructor and {@link RealServer.start} are its two steps.
 */
export class RealServer implements ScenarioServer {
  readonly #proc: ChildProcess;
  readonly #dir: string;
  readonly #lines: Lines;
  readonly #exited: Promise<void>;
  // Serialises the calls: one request, then its answer (the protocol answers in order).
  #lock: Promise<unknown> = Promise.resolve();

  /** The system clock: the server runs in another process with real time. */
  readonly clock: Clock = systemClock;
  /** {@link SDK_SETTINGS}. */
  readonly sdkSettings: SdkSettings = SDK_SETTINGS;
  /** {@link TIMINGS} in milliseconds. */
  readonly timings: Readonly<Record<string, number>> = Object.freeze(
    Object.fromEntries(
      Object.entries(TIMINGS).map(([k, v]) => [k, parseDuration(v)]),
    ),
  );
  /** The forwarder origin, set by {@link RealServer.start}. */
  origin = "";
  /** `origin + "/api/v1"`, set by {@link RealServer.start}. */
  baseUrl = "";
  /** The instance, set by {@link RealServer.start}. */
  instanceId = NIL_UUID;
  /** Lotse, set by {@link RealServer.start}. */
  agentId = NIL_UUID;
  /** Texter, set by {@link RealServer.start}. */
  otherAgentId = NIL_UUID;
  /** Lotse's API token and DPoP key, set by {@link RealServer.start}. */
  credentials: Credentials = { apiToken: new Secret("unset") };

  /**
   * Wraps a spawned process; call {@link RealServer.start} next.
   *
   * @param proc - The `sdkserver` process with piped stdin and stdout.
   * @param workdir - A directory of mode `0700` for the key and token files; {@link RealServer.stop}
   *   removes it.
   */
  constructor(proc: ChildProcess, workdir: string) {
    if (!proc.stdin || !proc.stdout)
      throw new TypeError("sdkserver needs piped stdin and stdout");
    this.#proc = proc;
    this.#dir = workdir;
    this.#lines = new Lines(proc.stdout);
    // A write after the exit fails with EPIPE; the reader then sees the end of stdout.
    proc.stdin.on("error", () => undefined);
    this.#exited = new Promise((resolve) => {
      if (proc.exitCode !== null || proc.signalCode !== null) resolve();
      else {
        proc.once("exit", () => resolve());
        // A binary that cannot start emits `error` and no `exit`.
        proc.once("error", () => resolve());
      }
    });
  }

  /**
   * Sends one request and returns its answer, skipping every other line on stdout.
   *
   * @throws `Error` naming the op when the program answers `error` (with its `detail`, which
   *   the program redacts), ends, or does not answer within the wait.
   */
  #call(request: Answer, timeoutMs = CALL_WAIT_MS): Promise<Answer> {
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

  /** Reads a token file of the program and removes it, so no token stays on disk. */
  async #readToken(path: string): Promise<Secret> {
    const token = new Secret((await readFile(path, "utf8")).trim());
    await unlink(path);
    return token;
  }

  /**
   * Starts the server and reads Lotse's credentials: generates the DPoP key in the work
   * directory and sends `start` with {@link TIMINGS} and its thumbprint.
   *
   * @throws `Error` when the program refuses or does not answer.
   */
  async start(): Promise<void> {
    const key = await DpopKey.generate({ path: join(this.#dir, "dpop.pem") });
    const ready = await this.#call({
      op: "start",
      config: TIMINGS,
      agent_jkt: key.jkt,
      dir: this.#dir,
    });
    this.origin = text(ready, "origin");
    this.baseUrl = `${this.origin}/api/v1`;
    this.instanceId = text(ready, "instance_id");
    this.agentId = text(ready, "agent_id");
    this.otherAgentId = text(ready, "other_agent_id");
    this.credentials = {
      apiToken: await this.#readToken(text(ready, "token_file")),
      dpop: key,
    };
  }

  /**
   * Alex opens a thread in Support/Allgemein with `text`; Lotse's run is created.
   *
   * @param message - The first message of the thread.
   * @returns Run, thread and message IDs.
   */
  async trigger(message = "Hello @Lotse"): Promise<Triggered> {
    const a = await this.#call({ op: "trigger", text: message });
    return {
      runId: text(a, "run_id"),
      threadId: text(a, "thread_id"),
      messageId: text(a, "message_id"),
    };
  }

  /**
   * Returns a forged token of `variant` for `runId`.
   *
   * @param runId - The run.
   * @param variant - The forgery.
   * @returns The forged token.
   */
  async forgeToken(runId: string, variant: ForgedVariant): Promise<Secret> {
    const a = await this.#call({ op: "forge_token", run_id: runId, variant });
    return this.#readToken(text(a, "token_file"));
  }

  /**
   * Returns state, reason, version and the number of starts of a run.
   *
   * @param runId - The run.
   * @returns The server's view of the run.
   */
  async runState(runId: string): Promise<RunView> {
    const a = await this.#call({ op: "run", run_id: runId });
    const reason = a.state_reason;
    return {
      state: text(a, "state"),
      stateReason: typeof reason === "string" ? reason : undefined,
      version: integer(a, "version"),
      starts: integer(a, "starts"),
    };
  }

  /**
   * Restarts the server on the same database, rewinding `consumer` to `ackedSeq`. The program
   * requires both; an unknown consumer answers `error` after the new server runs.
   *
   * @param consumer - Lotse's consumer to rewind.
   * @param ackedSeq - Its new acknowledged `seq`.
   */
  async restart(consumer?: string, ackedSeq?: number): Promise<void> {
    await this.#call({
      op: "restart",
      consumer: consumer ?? null,
      acked_seq: ackedSeq ?? null,
    });
  }

  /**
   * Lets real time pass.
   *
   * @param ms - The time to pass in milliseconds.
   */
  async passTime(ms: number): Promise<void> {
    await this.clock.sleep(ms);
  }

  /**
   * Stops the program with `stop` and waits for its exit (the server stops, `pgtest` removes
   * its container); kills it by PID only if it does not exit. Then removes the work directory.
   */
  async stop(): Promise<void> {
    await this.#call({ op: "stop" }, STOP_WAIT_MS).catch(() => undefined);
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
    await rm(this.#dir, { recursive: true, force: true });
  }
}

/** The name the M2-30 work split used for {@link RealServer}; the Python name is `RealServer`. */
export { RealServer as SdkServer };

/**
 * Starts `sdkserver` like the session fixture `real` of the Python SDK: a work directory of
 * mode `0700` under the system temporary directory, the binary with `-test.run
 * '^TestSDKServer$'` and no test timeout, then `start`. On a failed start the process is
 * stopped and the directory removed before the error propagates.
 *
 * @param binary - The path `global-setup.ts` provides as `sdkserverBinary`.
 * @returns The started server; call {@link RealServer.stop} when done.
 * @throws `Error` when the program cannot start (no Docker-compatible runtime, for example).
 */
export async function startRealServer(binary: string): Promise<RealServer> {
  const workdir = await mkdtemp(join(tmpdir(), "bothy-sdkserver-run-"));
  await chmod(workdir, 0o700); // the token files live here
  const proc = spawn(
    binary,
    ["-test.run", "^TestSDKServer$", "-test.timeout", "0"],
    {
      cwd: join(repositoryRoot(), "services", "server", "test", "sdkserver"),
      stdio: ["pipe", "pipe", "ignore"],
    },
  );
  const server = new RealServer(proc, workdir);
  try {
    await server.start();
  } catch (err) {
    await server.stop();
    throw err;
  }
  return server;
}
