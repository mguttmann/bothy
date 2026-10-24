// SPDX-License-Identifier: Apache-2.0
/**
 * What the shared scenarios need from a server: the FakeServer or the real server of
 * `services/server/test/sdkserver` (ADR 0026 "Confirmation": the same scenarios against both).
 *
 * @module
 */

import type { Secret } from "../auth/secret.js";
import type { Credentials } from "../auth/types.js";
import type { Clock } from "../clock.js";
import type { SdkSettings } from "../config.js";

/** Forged token variants both servers build (09 16.2 AG-08). */
export const FORGED_VARIANTS = [
  "bad_signature",
  "alg_hs256",
  "aud_other_agent",
  "typ_access",
] as const;

/** One of {@link FORGED_VARIANTS}. */
export type ForgedVariant = (typeof FORGED_VARIANTS)[number];

/** A run a scenario triggered: Alex mentions Lotse in a new thread. */
export interface Triggered {
  /** The run the trigger created. */
  readonly runId: string;
  /** The new thread. */
  readonly threadId: string;
  /** The triggering message. */
  readonly messageId: string;
}

/** The server's view of a run for the checks; `starts` counts transitions to `running`. */
export interface RunView {
  /** The run state. */
  readonly state: string;
  /** The state reason, `undefined` when none. */
  readonly stateReason: string | undefined;
  /** The run version. */
  readonly version: number;
  /** How often the run changed into `running`. */
  readonly starts: number;
}

/** What the scenarios need from a server (FakeServer or the real server). */
export interface ScenarioServer {
  /** The origin, for example `http://127.0.0.1:<port>`. */
  readonly origin: string;
  /** The API base, `origin + "/api/v1"`. */
  readonly baseUrl: string;
  /** The instance (`iss` of the invocation tokens). */
  readonly instanceId: string;
  /** Lotse, the agent under test. */
  readonly agentId: string;
  /** Texter, another agent. */
  readonly otherAgentId: string;
  /** Lotse's API token and DPoP key. */
  readonly credentials: Credentials;
  /** The clock the adapter under test must use. */
  readonly clock: Clock;
  /** The server keys the server plays (`agent.ack_timeout`, ...), in milliseconds. */
  readonly timings: Readonly<Record<string, number>>;
  /** The SDK settings the adapter under test uses. */
  readonly sdkSettings: SdkSettings;
  /**
   * Lets Alex mention Lotse in a new thread.
   *
   * @param text - The first message of the thread.
   * @returns Run, thread and message IDs.
   */
  trigger(text?: string): Promise<Triggered>;
  /**
   * Returns a forged token of `variant` for `runId`.
   *
   * @param runId - The run.
   * @param variant - The forgery.
   */
  forgeToken(runId: string, variant: ForgedVariant): Promise<Secret>;
  /**
   * Returns state, reason, version and the number of starts of a run.
   *
   * @param runId - The run.
   */
  runState(runId: string): Promise<RunView>;
  /**
   * Restarts the server on the same data, optionally rewinding a consumer cursor.
   *
   * @param consumer - The consumer to rewind.
   * @param ackedSeq - The new acknowledged `seq` of that consumer.
   */
  restart(consumer?: string, ackedSeq?: number): Promise<void>;
  /**
   * Lets `ms` milliseconds pass (fake or real).
   *
   * @param ms - The time to pass.
   */
  passTime(ms: number): Promise<void>;
}
