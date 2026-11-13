// SPDX-License-Identifier: Apache-2.0
/**
 * The handle of one run (11 11.2 row `runs`; port of `RunHandle` of `bothy_sdk/runs.py`).
 *
 * The adapter loop creates a {@link RunHandle} before `prepare` (the run is still `queued`) and
 * calls {@link RunHandle.started} once `startRun` or `resumeRun` answered `200`; from then on
 * the handler patches the live box through {@link RunHandle.patch}, reads the context, waits for
 * input, reports its agent session, replies through the outbox and ends the run with
 * {@link RunHandle.finish}. Inside the handler every request of the client is bound to the run
 * (07 SR9, `ApiClient.bindRun`); the handle itself never binds.
 *
 * @example
 * ```ts
 * async function handleRun(run: RunHandle): Promise<void> {
 *   const { system, trigger } = renderContext(await run.loadContext());
 *   run.patch.setStatusLine("Thinking");
 *   const answer = await runtime.ask(system, trigger, { signal: run.cancelled });
 *   await run.send(answer);
 *   await run.finish("completed", { summary: "Answered" });
 * }
 * ```
 *
 * @module
 */

import { type ApiClient, unwrap } from "../client.js";
import type { SdkSettings } from "../config.js";
import { ProblemError, TransportError } from "../errors.js";
import type { components } from "../generated/openapi.js";
import { uuidv7 } from "../ids.js";
import { isRunEnded, RunPatcher } from "./patcher.js";

type S = components["schemas"];

const SERVER_ERROR = 500;
// Refused before the run was touched: the request never reached the run.
const NOT_STORED: ReadonlySet<number> = new Set([401, 429]);

/**
 * Tells whether a failed `finishRun` left the end unstored: no answer arrived, or the server
 * answered `5xx`, `401` or `429`. Any other answer is the server's decision about the end.
 */
function endNotStored(error: unknown): boolean {
  if (error instanceof TransportError) return true;
  return (
    error instanceof ProblemError &&
    (error.status >= SERVER_ERROR || NOT_STORED.has(error.status))
  );
}

/**
 * The outbox a {@link RunHandle} replies through; the outbox of `outbox` fits. The handle adds
 * `runId` to the options.
 */
export interface RunOutbox {
  /**
   * Queues a message in a thread and sends it.
   *
   * @param threadId - The thread.
   * @param contentMd - The Markdown text.
   * @param options - Further options of the outbox, with `runId` set by the handle.
   * @returns The outbox entry.
   */
  sendMessage(
    threadId: string,
    contentMd: string,
    options: Readonly<Record<string, unknown>> & { readonly runId: string },
  ): Promise<unknown>;
}

/** Options of {@link RunHandle}. */
export interface RunHandleOptions {
  /** The outbox for replies ({@link RunHandle.send}). */
  readonly outbox?: RunOutbox | undefined;
  /** Settings of the patcher (default: the client's settings). */
  readonly settings?: SdkSettings | undefined;
  /** Called after `awaitRunInput` succeeded (the adapter marks its inbox entry waiting). */
  readonly onWaiting?: (() => Promise<void>) | undefined;
}

/** The optional members of `RunFinish` for {@link RunHandle.finish}. */
export interface FinishOptions {
  /** The reason code (for example `runtime_error`); required for `failed` and `cancelled`. */
  readonly reason?: string | undefined;
  /** A short summary. */
  readonly summary?: string | undefined;
  /** Final metrics. */
  readonly metrics?: S["RunMetrics"] | undefined;
  /** Usage per model (09 4.10 "usage", `costs` in openapi). */
  readonly costs?: readonly S["CostReport"][] | undefined;
}

/** The options of {@link RunHandle.reportSession}. */
export interface ReportSessionOptions {
  /**
   * The `version` of the session row (`agent_session.version` of the invocation, or of the
   * last answer); absent when the run has no row yet and the report creates it.
   */
  readonly ifMatch?: number | string | undefined;
}

/**
 * What a handler gets for one run: patches, context, input, the end and replies.
 *
 * The adapter creates it before `prepare` (the run is still `queued`) and starts the patcher
 * once `startRun` or `resumeRun` answered; inside the handler every request of the client is
 * bound to the run (07 SR9).
 */
export class RunHandle {
  /** The client of the adapter. */
  readonly client: ApiClient;
  /** The run. */
  readonly runId: string;
  /** The run's thread. */
  readonly threadId: string;
  /** The `jti` of the token the run started or resumed with (`undefined` before). */
  tokenJti: string | undefined;
  /** The outbox for replies. */
  readonly outbox: RunOutbox | undefined;
  /**
   * `true` once {@link RunHandle.finish} ran; `false` again when the server did not store the
   * end (see {@link RunHandle.finishPending}).
   */
  finished = false;
  /** `true` while the run waits for input. */
  waiting = false;

  private readonly settings: SdkSettings | undefined;
  private readonly onWaiting: (() => Promise<void>) | undefined;
  private readonly cancelController = new AbortController();
  private patcher: RunPatcher | undefined;
  // The first end asked for; it is the run's end, also when it has to be sent again.
  private end:
    | {
        readonly state: S["RunFinish"]["state"];
        readonly options: FinishOptions;
      }
    | undefined;

  /**
   * Creates the handle of a run that has not started yet.
   *
   * @param client - The client of the adapter.
   * @param runId - The run.
   * @param threadId - The run's thread.
   * @param options - Outbox, settings and the `onWaiting` callback.
   */
  constructor(
    client: ApiClient,
    runId: string,
    threadId: string,
    options: RunHandleOptions = {},
  ) {
    this.client = client;
    this.runId = runId;
    this.threadId = threadId;
    this.outbox = options.outbox;
    this.settings = options.settings;
    this.onWaiting = options.onWaiting;
    this.tokenJti = undefined;
  }

  /**
   * Aborted when the run was cancelled or ended on the server; hand it to the runtime so its
   * work stops (`run.cancel_requested`, `run_not_active`, `run_inactive`).
   */
  get cancelled(): AbortSignal {
    return this.cancelController.signal;
  }

  /**
   * Marks the run as cancelled and calls the cancel callbacks (once).
   *
   * The adapter calls it for `run.cancel_requested`; the patcher calls it when the server says
   * the run has ended.
   */
  setCancelled(): void {
    this.cancelController.abort();
  }

  /**
   * Registers a callback for the cancellation; after it, the callback runs at once.
   *
   * @param callback - Called once when the run is cancelled or ended on the server.
   * @returns A function that removes the callback.
   */
  onCancel(callback: () => void): () => void {
    const signal = this.cancelController.signal;
    if (signal.aborted) {
      callback();
      return () => undefined;
    }
    const listener = (): void => callback();
    signal.addEventListener("abort", listener, { once: true });
    return () => signal.removeEventListener("abort", listener);
  }

  /**
   * Notes that `startRun` or `resumeRun` answered `200` with `run`.
   *
   * @param run - The answered run (its `version` is the patch base).
   * @param tokenJti - The `jti` the run started or resumed with.
   */
  started(run: S["Run"], tokenJti: string): void {
    this.tokenJti = tokenJti;
    this.waiting = false;
    if (this.patcher === undefined)
      this.patcher = new RunPatcher(this.client, this.runId, run.version, {
        settings: this.settings,
        onEnded: () => this.setCancelled(),
      });
    else this.patcher.baseVersion = run.version;
  }

  /** The patcher once the run started, else `undefined`. */
  get startedPatcher(): RunPatcher | undefined {
    return this.patcher;
  }

  /**
   * The live-box patcher.
   *
   * @throws `Error` when the run has not started yet (inside `prepare`).
   */
  get patch(): RunPatcher {
    if (this.patcher === undefined)
      throw new Error(
        "the run has not started; patches are possible from execute on",
      );
    return this.patcher;
  }

  /** The run document version the patcher works on (`0` before the start). */
  get version(): number {
    return this.patcher?.baseVersion ?? 0;
  }

  /**
   * Sends a reply in the run's thread through the outbox, with `runId` set.
   *
   * @param contentMd - The Markdown text.
   * @param options - Further options of the outbox's `sendMessage`.
   * @returns The outbox entry.
   * @throws `Error` when the handle has no outbox.
   */
  async send(
    contentMd: string,
    options: Readonly<Record<string, unknown>> = {},
  ): Promise<unknown> {
    if (this.outbox === undefined)
      throw new Error("the run handle has no outbox");
    return this.outbox.sendMessage(this.threadId, contentMd, {
      ...options,
      runId: this.runId,
    });
  }

  /**
   * Loads the run context (`getRunContext`); render it with `renderContext`.
   *
   * @returns The context.
   * @throws {@link ProblemError} when the server refused.
   */
  async loadContext(): Promise<S["RunContext"]> {
    return unwrap(
      this.client.GET("/runs/{run_id}/context", {
        params: { path: { run_id: this.runId } },
      }),
      "getRunContext",
    );
  }

  /**
   * Waits for a person (`awaitRunInput`); the run goes to `waiting_input`.
   *
   * Pending patches are sent first. The adapter resumes the run with the token of the next
   * follow-up or interaction.
   *
   * @param statusLine - What the run waits for.
   * @returns A promise that resolves when the run waits.
   * @throws {@link ProblemError} when the patch or `awaitRunInput` was refused.
   */
  async awaitInput(statusLine: string): Promise<void> {
    if (this.patcher !== undefined) await this.patcher.flush();
    await unwrap(
      this.client.POST("/runs/{run_id}/await-input", {
        params: { path: { run_id: this.runId } },
        body: { status_line: statusLine },
      }),
      "awaitRunInput",
    );
    this.waiting = true;
    if (this.onWaiting !== undefined) await this.onWaiting();
  }

  /**
   * Reports the agent session of this run in its thread (`reportAgentSession`, 09 9.1 and
   * 9.5).
   *
   * `If-Match` is the compare-and-swap over the session row's `version`: absent when the run has
   * no row yet (the report creates it), otherwise the version of the row. Each call carries a
   * fresh `Idempotency-Key`, so the client may repeat it after a lost answer.
   *
   * @param report - The `AgentSessionReport` (`external_session_ref`, `model`, `started`,
   *   `bind_failed`).
   * @param options - `ifMatch`, the row version.
   * @returns The session row; its `version` is the next `ifMatch`.
   * @throws {@link ProblemError}, for example `412 version_conflict` or `409 conflict` with
   *   `session_bound_elsewhere`, `session_changed` or `session_not_bound`.
   */
  async reportSession(
    report: S["AgentSessionReport"],
    options: ReportSessionOptions = {},
  ): Promise<S["AgentSession"]> {
    const header: { "Idempotency-Key": string; "If-Match"?: string } = {
      "Idempotency-Key": uuidv7(),
    };
    if (options.ifMatch !== undefined)
      header["If-Match"] = `"${String(options.ifMatch)}"`;
    return unwrap(
      this.client.POST("/threads/{thread_id}/agent-sessions/report", {
        params: { path: { thread_id: this.threadId }, header },
        body: report,
      }),
      "reportAgentSession",
    );
  }

  /**
   * `true` while {@link RunHandle.finish} was called and the server did not store the end: no
   * answer arrived, or it answered `5xx`, `401` or `429`. The run is then still running on the
   * server; the adapter keeps it and sends the end again ({@link RunHandle.retryFinish}).
   */
  get finishPending(): boolean {
    return this.end !== undefined && !this.finished;
  }

  /**
   * Ends the run (`finishRun`) after sending pending patches.
   *
   * A final flush refused by the server or without an answer is dropped, and a `finishRun` answered with `run_inactive` or
   * `run_not_active` counts as already ended, not as an error; the server answers a repeated
   * equal end state with `200`. A second call does nothing.
   *
   * When the server did not store the end (no answer, `5xx`, `401` or `429`), the call throws
   * and the handle stays unfinished with {@link RunHandle.finishPending} set. The first end
   * asked for is the run's end: a later call sends that end again, whatever it asks for.
   *
   * @param state - `completed`, `failed` or `cancelled`.
   * @param options - `reason`, `summary`, `metrics` and `costs`.
   * @returns A promise that resolves when the run has ended.
   * @throws {@link ProblemError} for another refusal of the server; `TransportError` when no
   *   answer arrived.
   */
  async finish(
    state: S["RunFinish"]["state"],
    options: FinishOptions = {},
  ): Promise<void> {
    if (this.finished) return;
    this.finished = true;
    const again = this.end !== undefined;
    this.end ??= { state, options };
    const end = this.end;
    // The patcher was flushed and stopped by the first call.
    if (!again && this.patcher !== undefined) {
      try {
        await this.patcher.flush();
      } catch (error) {
        // Losing the last live-box update is acceptable, losing finishRun is not.
        if (!(error instanceof ProblemError || error instanceof TransportError))
          throw error;
      }
      await this.patcher.aclose();
    }
    const body: S["RunFinish"] = { state: end.state };
    if (end.options.reason !== undefined) body.reason = end.options.reason;
    if (end.options.summary !== undefined) body.summary = end.options.summary;
    if (end.options.metrics !== undefined) body.metrics = end.options.metrics;
    if (end.options.costs?.length) body.costs = [...end.options.costs];
    try {
      await unwrap(
        this.client.POST("/runs/{run_id}/finish", {
          params: { path: { run_id: this.runId } },
          body,
        }),
        "finishRun",
      );
    } catch (error) {
      if (error instanceof ProblemError && isRunEnded(error)) {
        this.client.logger.info(
          `run ${this.runId} already ended on the server`,
          { run: this.runId, code: error.code },
        );
        return;
      }
      // Without a decision of the server the run is still running there: not finished.
      if (endNotStored(error)) this.finished = false;
      throw error;
    }
  }

  /**
   * Sends the end again that {@link RunHandle.finish} asked for and the server did not store;
   * does nothing when no end is pending.
   *
   * @returns A promise that resolves when the run has ended.
   * @throws {@link ProblemError} for a refusal of the server; `TransportError` when no answer
   *   arrived (the end stays pending).
   */
  async retryFinish(): Promise<void> {
    if (this.end !== undefined)
      await this.finish(this.end.state, this.end.options);
  }
}
