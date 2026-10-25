// SPDX-License-Identifier: Apache-2.0
/**
 * The live-box patcher of a run (11 11.2 row `runs`; port of `RunPatcher` of
 * `bothy_sdk/runs.py`).
 *
 * {@link RunPatcher} keeps the desired state per path of docs/spec/09-agents.md 5.3 and sends
 * at most one patch per `agent.run_update_min_interval`; a later value on the same path replaces
 * the earlier one in its place, and an {@link RunPatcher.addStep} followed by updates goes out as
 * one `add` (docs/DECISIONS.md D-M2-29-9). Its transport is REST `patchRun`, because the answer
 * carries the new version and a `409 base_version_conflict` carries the current document as the
 * new base (D-M2-30-7); the gateway op `run.update` is offered through
 * {@link RunPatcher.useGateway}. A `409 conflict` with `reason` `run_not_active` or a
 * `403 invocation_token_invalid` with `reason` `run_inactive` stops the patcher and ends the work
 * on the run. Every path and op is checked locally against {@link PATCH_PATHS} before a request
 * ({@link PatchPathError}), so `/state` never leaves the process.
 *
 * Log lines carry the run and the problem code only, never a value of the live box (SEC-LOG-2).
 *
 * @example
 * ```ts
 * const patcher = new RunPatcher(api, run.id, run.version);
 * patcher.setStatusLine("Reading the thread");
 * const step = patcher.addStep("tool_call", "search", { title: "Search" });
 * patcher.updateStep(step, { status: "completed", finishedAt: new Date() });
 * await patcher.flush();
 * ```
 *
 * @module
 */

import { type ApiClient, unwrap } from "../client.js";
import type { Clock } from "../clock.js";
import type { SdkSettings } from "../config.js";
import {
  BaseVersionConflict,
  ConflictError,
  type GatewayOpError,
  InvocationTokenInvalid,
  ProblemError,
  RateLimitedError,
  ServerError,
  TransportError,
} from "../errors.js";
import type { components } from "../generated/openapi.js";
import { uuidv7 } from "../ids.js";

type JsonPatchOperation = components["schemas"]["JsonPatchOperation"];
type PatchOp = JsonPatchOperation["op"];

/** Number members of `run_metrics.v1` an adapter may set (`/metrics/<field>`). */
export const METRIC_FIELDS = Object.freeze([
  "context_pct",
  "tokens_in",
  "tokens_out",
  "tokens_cache_read",
  "tokens_cache_write",
  "tokens_cache_write_1h",
  "subagents",
  "background_jobs",
  "api_retries",
  "progress_events",
] as const);

/** One member of {@link METRIC_FIELDS}. */
export type MetricField = (typeof METRIC_FIELDS)[number];

const STEP_FIELDS = ["status", "title", "detail", "finished_at"] as const;
const FOLLOWUP_ORDER: Readonly<Record<FollowupState, number>> = {
  delivered: 1,
  absorbed: 2,
};
const UUID_SEGMENT = "[0-9a-fA-F-]{36}";

/** The state of a follow-up the adapter reports (`/followups/<message_id>/state`). */
export type FollowupState = "delivered" | "absorbed";

function escapeRegExp(text: string): string {
  return text.replace(/[.*+?^${}()|[\]\\]/g, "\\$&");
}

function compile(pattern: string): RegExp {
  const parts = pattern.split("/").map((segment) => {
    if (segment === "<field>") return `(?:${METRIC_FIELDS.join("|")})`;
    if (segment.startsWith("<")) return UUID_SEGMENT;
    return escapeRegExp(segment);
  });
  return new RegExp(`^${parts.join("/")}$`);
}

/** One row of the path table of docs/spec/09-agents.md 5.3. */
export class PatchPath {
  /** The path with placeholders `<field>`, `<id>` and `<message_id>`. */
  readonly pattern: string;
  /** The RFC 6902 operations allowed on it. */
  readonly ops: ReadonlySet<PatchOp>;
  private readonly regex: RegExp;

  /**
   * Creates a row.
   *
   * @param pattern - The path with placeholders.
   * @param ops - The allowed operations.
   */
  constructor(pattern: string, ops: Iterable<PatchOp>) {
    this.pattern = pattern;
    this.ops = new Set(ops);
    this.regex = compile(pattern);
    Object.freeze(this);
  }

  /**
   * Tells whether a concrete path is this row.
   *
   * @param path - A JSON pointer such as `/steps/<uuid>/status`.
   * @returns `true` for a match.
   */
  matches(path: string): boolean {
    return this.regex.test(path);
  }
}

const ADD_REPLACE: readonly PatchOp[] = ["add", "replace"];
const ALL: readonly PatchOp[] = ["add", "replace", "remove"];
const REPLACE: readonly PatchOp[] = ["replace"];

/**
 * The paths an adapter may patch, with their operations (09 5.3); `test/runs.test.ts` holds it
 * equal to the table of the specification.
 */
export const PATCH_PATHS: readonly PatchPath[] = Object.freeze([
  new PatchPath("/status_line", ALL),
  new PatchPath("/last_text_excerpt", ADD_REPLACE),
  new PatchPath("/model", REPLACE),
  new PatchPath("/metrics/<field>", ADD_REPLACE),
  new PatchPath("/metrics/last_api_retry", ALL),
  new PatchPath("/cost", REPLACE),
  new PatchPath("/plan", ALL),
  new PatchPath("/steps/<id>", ["add"]),
  ...STEP_FIELDS.map((f) => new PatchPath(`/steps/<id>/${f}`, REPLACE)),
  new PatchPath("/followups/<message_id>/state", REPLACE),
]);

function isAllowed(op: string, path: string): boolean {
  return PATCH_PATHS.some((p) => p.matches(path) && p.ops.has(op as PatchOp));
}

/**
 * A patch operation outside the path table of 09 5.3 (for example anything on `/state`),
 * refused before any request. A `RangeError`, as Python's `ValueError` of `send_ops`.
 */
export class PatchPathError extends RangeError {
  /** The refused operation. */
  readonly op: string;
  /** The refused path. */
  readonly path: string;

  /**
   * Creates the error.
   *
   * @param op - The refused operation.
   * @param path - The refused path.
   */
  constructor(op: string, path: string) {
    super(`patch path not allowed: ${op} ${path}`);
    this.name = "PatchPathError";
    this.op = op;
    this.path = path;
  }
}

/**
 * Tells whether an error says the run has ended (`run_not_active` or `run_inactive`).
 *
 * @param error - A problem of a run operation.
 * @returns `true` when the work on the run must end.
 */
export function isRunEnded(error: ProblemError): boolean {
  if (error instanceof ConflictError && error.code === "conflict")
    return error.reason === "run_not_active";
  return (
    error instanceof InvocationTokenInvalid && error.reason === "run_inactive"
  );
}

/** A refusal worth sending the same changes again: 5xx, 429 or an exhausted rebase. */
function isTransient(error: ProblemError): boolean {
  return (
    error instanceof ServerError ||
    error instanceof RateLimitedError ||
    error instanceof BaseVersionConflict
  );
}

/**
 * The gateway side of {@link RunPatcher.useGateway}: sends the op `run.update`
 * (docs/spec/04-sync-protocol.md 6.2). The gateway consumer of `gateway` fits.
 */
export interface RunUpdateGateway {
  /**
   * Sends one `run.update`.
   *
   * @param runId - The run.
   * @param baseVersion - The version the ops build on.
   * @param ops - The operations.
   * @param ref - The `ref` an `error` frame names.
   */
  sendRunUpdate(
    runId: string,
    baseVersion: number,
    ops: readonly JsonPatchOperation[],
    ref?: string,
  ): Promise<void>;
}

/** Options of {@link RunPatcher}. */
export interface RunPatcherOptions {
  /**
   * `agent.run_update_min_interval` and `sdk.retry_max_attempts` (default: the client's
   * settings).
   */
  readonly settings?: SdkSettings | undefined;
  /** The time source of the throttle (default: the client's clock). */
  readonly clock?: Clock | undefined;
  /** Called once when the server says the run has ended. */
  readonly onEnded?: (() => void) | undefined;
}

/** The members of a new step of {@link RunPatcher.addStep} besides kind and name. */
export interface AddStepOptions {
  /** The display title (default empty). */
  readonly title?: string | undefined;
  /** The step status (default `running`). */
  readonly status?: string | undefined;
  /** The parent step, if any. */
  readonly parentStepId?: string | undefined;
  /** A JSON object with details. */
  readonly detail?: Readonly<Record<string, unknown>> | undefined;
}

/** The changes of {@link RunPatcher.updateStep}; absent members stay as they are. */
export interface UpdateStepOptions {
  /** The new status. */
  readonly status?: string | undefined;
  /** The new title. */
  readonly title?: string | undefined;
  /** The new detail object. */
  readonly detail?: Readonly<Record<string, unknown>> | undefined;
  /** The end time. */
  readonly finishedAt?: Date | undefined;
}

/** The optional members of `/metrics/last_api_retry` ({@link RunPatcher.setLastApiRetry}). */
export interface LastApiRetryOptions {
  /** The runtime's limit, if known. */
  readonly maxRetries?: number | undefined;
  /** The wait before the next attempt, if known. */
  readonly retryDelayMs?: number | undefined;
  /** A short error text, if any. */
  readonly error?: string | undefined;
}

/** One plan item of `/plan` (`run_plan.v1`). */
export interface PlanItem {
  /** The item ID. */
  readonly id: string;
  /** The item text. */
  readonly text: string;
  /** `pending`, `in_progress` or `completed`. */
  readonly status: string;
}

interface Change {
  readonly op: PatchOp;
  readonly value: unknown;
}

/** A queue that runs one task at a time, like Python's `asyncio.Lock`. */
class Mutex {
  private tail: Promise<void> = Promise.resolve();

  async run<T>(fn: () => Promise<T>): Promise<T> {
    const previous = this.tail;
    let release!: () => void;
    this.tail = new Promise<void>((resolve) => {
      release = resolve;
    });
    await previous;
    try {
      return await fn();
    } finally {
      release();
    }
  }
}

/**
 * Collects live-box changes of one run and sends them throttled.
 *
 * Every setter only records the change and schedules a send; the throttle timer sends the
 * pending changes as one patch once `agent.run_update_min_interval` passed since the last sent
 * patch. {@link RunPatcher.flush} sends at once. After the end of the run every setter is a
 * no-op.
 */
export class RunPatcher {
  /** The REST client (bound to the run by the caller, 07 SR9). */
  readonly client: ApiClient;
  /** The run. */
  readonly runId: string;
  /** The version the next patch builds on. */
  baseVersion: number;
  /** `agent.run_update_min_interval` and `sdk.retry_max_attempts`. */
  readonly settings: SdkSettings;
  /** The time source of the throttle. */
  readonly clock: Clock;
  /** `true` once the server said the run has ended; nothing is sent after that. */
  ended = false;
  /** The number of patches the server accepted (or the gateway took). */
  patchesSent = 0;

  private readonly onEnded: (() => void) | undefined;
  private readonly pending = new Map<string, Change>();
  private readonly followups = new Map<string, FollowupState>();
  private readonly lock = new Mutex();
  private lastSent: number | undefined;
  private timer: Promise<void> | undefined;
  private timerAbort: AbortController | undefined;
  private gateway: RunUpdateGateway | undefined;
  private gatewayRefs = 0;
  private sentOps: readonly JsonPatchOperation[] = [];

  /**
   * Creates the patcher.
   *
   * @param client - The REST client (bound to the run by the caller, 07 SR9).
   * @param runId - The run.
   * @param baseVersion - The version of the `Run` that `startRun` or `resumeRun` answered.
   * @param options - Settings, clock and the `onEnded` callback.
   */
  constructor(
    client: ApiClient,
    runId: string,
    baseVersion: number,
    options: RunPatcherOptions = {},
  ) {
    this.client = client;
    this.runId = runId;
    this.baseVersion = baseVersion;
    this.settings = options.settings ?? client.settings;
    this.clock = options.clock ?? client.clock;
    this.onEnded = options.onEnded;
  }

  // ---- setters -------------------------------------------------------------------------

  private set(path: string, op: PatchOp, value?: unknown): void {
    if (this.ended) return;
    this.pending.set(path, { op, value });
    this.schedule();
  }

  /**
   * Sets `/status_line`.
   *
   * @param text - The status line.
   */
  setStatusLine(text: string): void {
    this.set("/status_line", "replace", text);
  }

  /** Removes `/status_line`. */
  clearStatusLine(): void {
    this.set("/status_line", "remove");
  }

  /**
   * Sets `/last_text_excerpt` (the server keeps its last `run.excerpt_max_chars`).
   *
   * @param text - The draft text so far.
   */
  setLastTextExcerpt(text: string): void {
    this.set("/last_text_excerpt", "replace", text);
  }

  /**
   * Sets `/model`.
   *
   * @param model - The model name of the runtime.
   */
  setModel(model: string): void {
    this.set("/model", "replace", model);
  }

  /**
   * Sets `/metrics/<name>`.
   *
   * @param name - One of {@link METRIC_FIELDS} ({@link MetricField}); a plain string is checked
   *   at run time.
   * @param value - The number.
   * @throws `RangeError` when `name` is not a metric of `run_metrics.v1`.
   */
  setMetric(name: string, value: number): void {
    if (!(METRIC_FIELDS as readonly string[]).includes(name))
      throw new RangeError(`unknown metric: ${name}`);
    this.set(`/metrics/${name}`, "replace", value);
  }

  /**
   * Sets `/metrics/last_api_retry`.
   *
   * @param attempt - The attempt number.
   * @param options - `maxRetries`, `retryDelayMs` and `error`, each if known.
   */
  setLastApiRetry(attempt: number, options: LastApiRetryOptions = {}): void {
    const value: Record<string, unknown> = { attempt };
    if (options.maxRetries !== undefined)
      value.max_retries = options.maxRetries;
    if (options.retryDelayMs !== undefined)
      value.retry_delay_ms = options.retryDelayMs;
    if (options.error !== undefined) value.error = options.error;
    this.set("/metrics/last_api_retry", "replace", value);
  }

  /** Removes `/metrics/last_api_retry`. */
  clearLastApiRetry(): void {
    this.set("/metrics/last_api_retry", "remove");
  }

  /**
   * Sets `/cost`; always an estimate (`is_estimate: true`).
   *
   * @param amountMicros - The amount in millionths of the currency unit.
   * @param currency - The ISO 4217 code.
   */
  setCost(amountMicros: number, currency: string): void {
    this.set("/cost", "replace", {
      amount_micros: amountMicros,
      currency,
      is_estimate: true,
    });
  }

  /**
   * Sets `/plan`.
   *
   * @param items - Plan items with `id`, `text` and `status`.
   */
  setPlan(items: readonly PlanItem[]): void {
    this.set("/plan", "replace", { items: items.map((i) => ({ ...i })) });
  }

  /** Removes `/plan`. */
  clearPlan(): void {
    this.set("/plan", "remove");
  }

  /**
   * Adds a step; its ID is a UUIDv7 of the adapter, the server assigns `ordinal`.
   *
   * @param kind - `tool_call`, `subagent`, `background_job`, `workflow` or `status`.
   * @param name - The step name.
   * @param options - `title`, `status`, `parentStepId` and `detail`.
   * @returns The step ID.
   */
  addStep(kind: string, name: string, options: AddStepOptions = {}): string {
    const stepId = uuidv7();
    const value: Record<string, unknown> = {
      id: stepId,
      kind,
      name,
      title: options.title ?? "",
      status: options.status ?? "running",
    };
    if (options.parentStepId !== undefined)
      value.parent_step_id = options.parentStepId;
    if (options.detail !== undefined) value.detail = { ...options.detail };
    this.set(`/steps/${stepId}`, "add", value);
    return stepId;
  }

  /**
   * Changes a step; an unsent `add` absorbs the change.
   *
   * @param stepId - The step.
   * @param changes - `status`, `title`, `detail` and `finishedAt`; absent members stay.
   */
  updateStep(stepId: string, changes: UpdateStepOptions): void {
    const members: Record<string, unknown> = {};
    if (changes.status !== undefined) members.status = changes.status;
    if (changes.title !== undefined) members.title = changes.title;
    if (changes.detail !== undefined) members.detail = { ...changes.detail };
    if (changes.finishedAt !== undefined)
      members.finished_at = changes.finishedAt.toISOString();
    const addPath = `/steps/${stepId}`;
    const pendingAdd = this.pending.get(addPath);
    if (pendingAdd?.op === "add") {
      Object.assign(pendingAdd.value as Record<string, unknown>, members);
      this.schedule();
      return;
    }
    for (const [name, value] of Object.entries(members))
      this.set(`${addPath}/${name}`, "replace", value);
  }

  private followup(messageId: string, state: FollowupState): void {
    const current = this.followups.get(messageId);
    if (
      current !== undefined &&
      FOLLOWUP_ORDER[current] >= FOLLOWUP_ORDER[state]
    )
      throw new RangeError("follow-up states only move forward");
    this.followups.set(messageId, state);
    this.set(`/followups/${messageId}/state`, "replace", state);
  }

  /**
   * Marks a follow-up as delivered to the runtime.
   *
   * @param messageId - The follow-up message.
   * @throws `RangeError` when the follow-up is already further.
   */
  followupDelivered(messageId: string): void {
    this.followup(messageId, "delivered");
  }

  /**
   * Marks a follow-up as absorbed by the runtime.
   *
   * @param messageId - The follow-up message.
   * @throws `RangeError` when the follow-up is already absorbed.
   */
  followupAbsorbed(messageId: string): void {
    this.followup(messageId, "absorbed");
  }

  // ---- sending -------------------------------------------------------------------------

  /**
   * Sends patches with the gateway op `run.update` instead of REST.
   *
   * The gateway has no success reply, so the next base is assumed as `baseVersion + 1`; route
   * an `error` frame of a `run.update` of this run to {@link RunPatcher.onGatewayError}, which
   * loads `getRun` after `base_version_conflict` and sends again.
   *
   * @param gateway - The gateway consumer.
   */
  useGateway(gateway: RunUpdateGateway): void {
    this.gateway = gateway;
  }

  private schedule(): void {
    if (this.timer !== undefined) return;
    const abort = new AbortController();
    this.timerAbort = abort;
    this.timer = this.flushLater(abort.signal);
  }

  private async flushLater(signal: AbortSignal): Promise<void> {
    let sent = false;
    try {
      const wait =
        this.lastSent === undefined
          ? 0
          : this.lastSent +
            this.settings.runUpdateMinIntervalMs -
            this.clock.monotonic();
      // Even without a wait the send happens on a later turn, so the setters of one
      // synchronous stretch go out as one patch.
      await this.clock.sleep(Math.max(0, wait), signal);
      // A clock may finish a zero wait without looking at the signal.
      if (signal.aborted) return;
      await this.flush();
      sent = true;
    } catch (error) {
      if (!signal.aborted) this.logFailure(error);
    } finally {
      this.timer = undefined;
      this.timerAbort = undefined;
    }
    // Changes made while the patch was on its way go out in the next round. After a failed
    // patch nothing is rescheduled: the next change or flush sends again, so a server that
    // keeps refusing is not hammered.
    if (sent && !signal.aborted && this.pending.size > 0 && !this.ended)
      this.schedule();
  }

  private logFailure(error: unknown): void {
    const fields: Record<string, unknown> = { run: this.runId };
    if (error instanceof ProblemError) fields.code = error.code;
    else fields.error = error instanceof Error ? error.name : typeof error;
    this.client.logger.info(`run ${this.runId}: patch failed`, fields);
  }

  private ops(): JsonPatchOperation[] {
    return [...this.pending].map(([path, { op, value }]) =>
      op === "remove" ? { op, path } : { op, path, value },
    );
  }

  /**
   * Sends the pending changes now (one patch), rebasing after a version conflict.
   *
   * A patch that finds the run ended stops the patcher and returns. After a transient refusal
   * (5xx, `429`, a conflict that outlasted the rebases) or a transport error the changes stay
   * pending, values set meanwhile win; a permanent refusal (for example `400
   * validation_failed`) drops the patch, so one bad value never blocks the later updates.
   *
   * @returns A promise that resolves when the patch was sent (or there was nothing to send).
   * @throws {@link ProblemError} for a refusal other than the end of the run, or the version
   *   conflict after `sdk.retry_max_attempts` rounds; {@link TransportError} when no answer
   *   arrived.
   */
  async flush(): Promise<void> {
    await this.lock.run(async () => {
      if (this.ended || this.pending.size === 0) return;
      const ops = this.ops();
      const sent = new Map(this.pending);
      this.pending.clear();
      try {
        await this.send(ops);
      } catch (error) {
        if (error instanceof ProblemError) {
          if (isRunEnded(error)) {
            this.end();
            return;
          }
          if (isTransient(error)) this.restore(sent);
        } else if (error instanceof TransportError) {
          this.restore(sent);
        }
        throw error;
      }
    });
  }

  private restore(sent: ReadonlyMap<string, Change>): void {
    // The failed changes go back in front, so a step's `add` precedes a `replace` of one of its
    // members made while the patch was on its way; values set meanwhile are newer and win in
    // place.
    const merged = new Map(sent);
    for (const [path, change] of this.pending) merged.set(path, change);
    this.pending.clear();
    for (const [path, change] of merged) this.pending.set(path, change);
  }

  /**
   * Sends raw RFC 6902 operations at once, after checking them against {@link PATCH_PATHS}.
   *
   * @param ops - The operations.
   * @returns The new version.
   * @throws {@link PatchPathError} when an operation names a path or op outside 09 5.3 (before
   *   any request); {@link ProblemError} when the server refused.
   *
   * @example
   * ```ts
   * await patcher.sendOps([{ op: "replace", path: "/status_line", value: "Reading" }]);
   * ```
   */
  async sendOps(ops: readonly JsonPatchOperation[]): Promise<number> {
    for (const item of ops)
      if (!isAllowed(item.op, item.path))
        throw new PatchPathError(item.op, item.path);
    return this.lock.run(() => this.send([...ops]));
  }

  private async send(ops: readonly JsonPatchOperation[]): Promise<number> {
    if (this.gateway !== undefined) return this.sendGateway(ops);
    const rounds = Math.max(1, this.settings.retryMaxAttempts);
    for (let attempt = 0; ; attempt++) {
      try {
        const result = await unwrap(
          this.client.PATCH("/runs/{run_id}", {
            params: { path: { run_id: this.runId } },
            body: { base_version: this.baseVersion, ops: [...ops] },
          }),
          "patchRun",
        );
        this.baseVersion = result.version;
      } catch (error) {
        const current =
          error instanceof BaseVersionConflict ? error.run : undefined;
        if (current === undefined || attempt >= rounds - 1) throw error;
        this.baseVersion = current.version;
        continue;
      }
      this.lastSent = this.clock.monotonic();
      this.patchesSent += 1;
      return this.baseVersion;
    }
  }

  private async sendGateway(
    ops: readonly JsonPatchOperation[],
  ): Promise<number> {
    const gateway = this.gateway as RunUpdateGateway;
    this.gatewayRefs += 1;
    this.sentOps = ops;
    await gateway.sendRunUpdate(
      this.runId,
      this.baseVersion,
      ops,
      `${this.runId}:${this.gatewayRefs}`,
    );
    this.baseVersion += 1;
    this.lastSent = this.clock.monotonic();
    this.patchesSent += 1;
    return this.baseVersion;
  }

  /**
   * Handles an `error` frame of a `run.update` of this run.
   *
   * `base_version_conflict` loads `getRun` and sends the last ops again on its version;
   * `conflict` (`run_not_active`) ends the patcher.
   *
   * @param error - The error of the frame.
   * @returns A promise that resolves when the reaction is done.
   * @throws {@link ProblemError} when `getRun` fails.
   */
  async onGatewayError(error: GatewayOpError): Promise<void> {
    if (error.code === "base_version_conflict") {
      const run = await unwrap(
        this.client.GET("/runs/{run_id}", {
          params: { path: { run_id: this.runId } },
        }),
        "getRun",
      );
      this.baseVersion = run.version;
      const ops = this.sentOps;
      if (ops.length) await this.lock.run(() => this.sendGateway(ops));
    } else if (
      error.code === "conflict" ||
      (error.detail ?? "").endsWith("run_not_active")
    ) {
      this.end();
    }
  }

  private end(): void {
    if (this.ended) return;
    this.ended = true;
    this.pending.clear();
    this.client.logger.info(
      `run ${this.runId} ended on the server; patcher stopped`,
      { run: this.runId },
    );
    this.onEnded?.();
  }

  /**
   * Stops the throttle timer without sending; a patch already on its way completes.
   *
   * @returns A promise that resolves when the timer has stopped.
   */
  async aclose(): Promise<void> {
    const timer = this.timer;
    this.timerAbort?.abort();
    if (timer !== undefined) await timer;
  }
}
