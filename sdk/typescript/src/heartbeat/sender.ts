// SPDX-License-Identifier: Apache-2.0
/**
 * Sends the agent heartbeat at the interval the server asks for (docs/spec/09-agents.md 4.8;
 * the heartbeat loop of `PollingSource` in `bothy_sdk/sources.py`).
 *
 * {@link HeartbeatSender} builds a heartbeat with its `build` callback and hands it to `send`:
 * {@link restHeartbeatSend} for `sendHeartbeat` (the polling source) or a callback of the
 * gateway that puts it into the member `agent` of the next gateway `heartbeat`. Before the first
 * answer it waits `sdk.poll_interval`; after that it keeps the `interval_ms` of the last answer
 * (`HeartbeatAck.interval_ms`, the server's `sync.heartbeat_interval`). A failed send is logged
 * by its error name only and retried in the next interval.
 *
 * @module
 */

import type { Client } from "openapi-fetch";

import { type Clock, systemClock } from "../clock.js";
import { DEFAULTS, type SdkSettings } from "../config.js";
import { problemError } from "../errors.js";
import type { components, paths } from "../generated/openapi.js";
import { type Logger, defaultLogger } from "../logger.js";

/**
 * Sends one heartbeat and returns the interval in milliseconds the server asked for, or
 * `undefined` when the answer named none (the interval then stays).
 */
export type HeartbeatSend = (
  beat: components["schemas"]["Heartbeat"],
) => Promise<number | undefined>;

/** The options of {@link HeartbeatSender}. */
export interface HeartbeatSenderOptions {
  /**
   * Returns the heartbeat to send now (for example from {@link buildHeartbeat}), or `undefined`
   * to skip this interval.
   */
  readonly build: () => components["schemas"]["Heartbeat"] | undefined;
  /** Sends it: {@link restHeartbeatSend} or a gateway callback. */
  readonly send: HeartbeatSend;
  /** The time source of the waits (default `systemClock`). */
  readonly clock?: Clock | undefined;
  /** Settings; `pollIntervalMs` (`sdk.poll_interval`) is the wait before the first answer. */
  readonly settings?: Partial<SdkSettings> | undefined;
  /** The logger (default `defaultLogger`). */
  readonly logger?: Logger | undefined;
}

/**
 * Returns a {@link HeartbeatSend} over the REST operation `sendHeartbeat`.
 *
 * @param api - An openapi-fetch client of the public API (for example `createApiClient`).
 * @returns The send function; it resolves to `HeartbeatAck.interval_ms`.
 * @throws (from the returned function) `ProblemError` when the server refuses.
 *
 * @example
 * ```ts
 * const sender = new HeartbeatSender({ build: () => buildHeartbeat(), send: restHeartbeatSend(api) });
 * sender.start();
 * ```
 */
export function restHeartbeatSend(
  api: Pick<Client<paths>, "POST">,
): HeartbeatSend {
  return async (beat) => {
    const result = await api.POST("/agents/me/heartbeat", { body: beat });
    if (!result.response.ok)
      throw problemError(result.response.status, result.error, {
        operation: "sendHeartbeat",
      });
    return result.data?.interval_ms;
  };
}

/**
 * Sends the agent heartbeat in a loop and keeps the interval of the answers.
 *
 * @example
 * ```ts
 * const sender = new HeartbeatSender({
 *   build: () => buildHeartbeat("online", { details: { protocol_version: 1 } }),
 *   send: restHeartbeatSend(api),
 * });
 * sender.start();
 * // ...
 * await sender.stop();
 * ```
 */
export class HeartbeatSender {
  private readonly build: () => components["schemas"]["Heartbeat"] | undefined;
  private readonly send: HeartbeatSend;
  private readonly clock: Clock;
  private readonly logger: Logger;
  private interval: number;
  private abort: AbortController | undefined;
  private loop: Promise<void> | undefined;

  /**
   * Creates the sender; nothing is sent before {@link HeartbeatSender.start} or
   * {@link HeartbeatSender.beatOnce}.
   *
   * @param options - The build and send callbacks, clock, settings and logger.
   */
  constructor(options: HeartbeatSenderOptions) {
    this.build = options.build;
    this.send = options.send;
    this.clock = options.clock ?? systemClock;
    this.logger = options.logger ?? defaultLogger;
    this.interval = options.settings?.pollIntervalMs ?? DEFAULTS.pollIntervalMs;
  }

  /** The wait between two heartbeats in milliseconds: the last `interval_ms` of the server. */
  get intervalMs(): number {
    return this.interval;
  }

  /** Starts the loop: one heartbeat now, then one per interval; a second call does nothing. */
  start(): void {
    if (this.loop !== undefined) return;
    const abort = new AbortController();
    this.abort = abort;
    this.loop = this.run(abort.signal);
  }

  /**
   * Stops the loop after the heartbeat in flight, if any.
   *
   * @returns When the loop has ended.
   */
  async stop(): Promise<void> {
    this.abort?.abort();
    await this.loop;
    this.loop = undefined;
    this.abort = undefined;
  }

  /**
   * Builds and sends one heartbeat now and keeps the interval of the answer.
   *
   * @returns Whether a heartbeat was sent and accepted; `false` when `build` skipped it, or `build`
   *   or the send failed (logged, never thrown).
   */
  async beatOnce(): Promise<boolean> {
    try {
      // A build that throws (a value outside its form) is logged like a failed send, so the
      // loop keeps running and the process never sees an unhandled rejection.
      const beat = this.build();
      if (beat === undefined) return false;
      const next = await this.send(beat);
      // A missing or senseless interval keeps the current one, so the loop never spins.
      if (next !== undefined && Number.isFinite(next) && next > 0)
        this.interval = next;
      return true;
    } catch (error) {
      this.logger.info("heartbeat failed; retrying in the next interval", {
        error: error instanceof Error ? error.name : typeof error,
      });
      return false;
    }
  }

  private async run(signal: AbortSignal): Promise<void> {
    while (!signal.aborted) {
      await this.beatOnce();
      try {
        await this.clock.sleep(this.interval, signal);
      } catch {
        return;
      }
    }
  }
}
