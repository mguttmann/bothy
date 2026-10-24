// SPDX-License-Identifier: Apache-2.0
/**
 * JSON log lines of the app (SEC-LOG-2, SEC-LOG-4, SEC-SEC-3; brief M2-49 Design note 15).
 *
 * Every line is one JSON object with `time`, `level`, `msg` (a fixed code such as
 * `github.delivery.accepted`) and only the attributes of {@link LOG_ATTRIBUTES}; any other field
 * is dropped before the line is written, so a caller cannot leak a title, a body, a login, a URL,
 * a header value, a token or a secret by passing it. The SDK client gets {@link sdkLogger}, which
 * replaces the SDK's free-text messages by the code `sdk.client`.
 *
 * @example
 * ```ts
 * const log = createLogger({ write: (line) => process.stdout.write(line) });
 * log.info("github.delivery.accepted", { event: "issues", action: "opened", status: 202 });
 * ```
 *
 * @module
 */

import type { LogFields, Logger } from "bothy-sdk";

/** The only attributes a log line may carry besides `time`, `level` and `msg`. */
export const LOG_ATTRIBUTES: readonly string[] = Object.freeze([
  "event",
  "action",
  "delivery_id",
  "webhook_id",
  "job_id",
  "job_kind",
  "step",
  "operation",
  "status",
  "outcome",
  "reason",
  "code",
  "number",
  "thread_id",
  "channel_id",
]);

const ALLOWED: ReadonlySet<string> = new Set(LOG_ATTRIBUTES);

/** The levels in rising order. */
export type LogLevel = "debug" | "info" | "warn" | "error";

const RANK: Readonly<Record<LogLevel, number>> = {
  debug: 0,
  info: 1,
  warn: 2,
  error: 3,
};

/** Options of {@link createLogger}. */
export interface LoggerOptions {
  /** Receives each line including its trailing newline (default: `process.stdout.write`). */
  readonly write?: ((line: string) => void) | undefined;
  /** Lowest level that is written (default `info`). */
  readonly level?: LogLevel | undefined;
  /** Wall time in Unix milliseconds (default `Date.now`). */
  readonly now?: (() => number) | undefined;
}

/**
 * Returns `fields` reduced to the allowed attributes with a string, number or boolean value.
 *
 * @param fields - The fields a caller passed.
 * @returns A new object with the allowed members only.
 */
export function allowedFields(
  fields: LogFields | undefined,
): Record<string, string | number | boolean> {
  const out: Record<string, string | number | boolean> = {};
  if (fields === undefined) return out;
  for (const [key, value] of Object.entries(fields)) {
    if (!ALLOWED.has(key)) continue;
    if (
      typeof value === "string" ||
      typeof value === "number" ||
      typeof value === "boolean"
    )
      out[key] = value;
  }
  return out;
}

/**
 * Creates the logger of the app: one JSON line per call at or above the level.
 *
 * @param options - Sink, level and clock.
 * @returns A logger with the four methods of the SDK's `Logger`.
 */
export function createLogger(options: LoggerOptions = {}): Logger {
  const write =
    options.write ??
    ((line: string) => {
      process.stdout.write(line);
    });
  const min = RANK[options.level ?? "info"];
  const now = options.now ?? Date.now;
  const emit =
    (level: LogLevel) =>
    (msg: string, fields?: LogFields): void => {
      if (RANK[level] < min) return;
      const line = {
        time: new Date(now()).toISOString(),
        level,
        msg,
        ...allowedFields(fields),
      };
      write(`${JSON.stringify(line)}\n`);
    };
  return {
    debug: emit("debug"),
    info: emit("info"),
    warn: emit("warn"),
    error: emit("error"),
  };
}

/**
 * Wraps the app's logger for the SDK client: the SDK's message text (which names the operation
 * and may quote a `Deprecation` header) becomes the code `sdk.client`; the allowed fields such
 * as `operation` and `status` stay.
 *
 * @param logger - The app's logger.
 * @returns The logger to pass to `createApiClient`.
 */
export function sdkLogger(logger: Logger): Logger {
  return {
    debug: (_msg, fields) => logger.debug("sdk.client", fields),
    info: (_msg, fields) => logger.info("sdk.client", fields),
    warn: (_msg, fields) => logger.warn("sdk.client", fields),
    error: (_msg, fields) => logger.error("sdk.client", fields),
  };
}
