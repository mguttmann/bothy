// SPDX-License-Identifier: Apache-2.0
/**
 * The logging interface of the hand layer.
 *
 * Log lines carry operations, statuses, problem codes, correlation IDs and run IDs; never a
 * header value, a body, a token, a proof or a message content (SEC-LOG-2). Any logger with the
 * four methods fits (for example `pino` or `console`). Browser-safe.
 *
 * @example
 * ```ts
 * const api = createApiClient({ baseUrl: "https://example.com/api/v1", logger: console });
 * ```
 *
 * @module
 */

/** Structured fields of a log line. */
export type LogFields = Readonly<Record<string, unknown>>;

/** A logger with four levels. */
export interface Logger {
  /**
   * Logs a detail for debugging.
   *
   * @param msg - The message.
   * @param fields - Structured fields.
   */
  debug(msg: string, fields?: LogFields): void;
  /**
   * Logs a normal event.
   *
   * @param msg - The message.
   * @param fields - Structured fields.
   */
  info(msg: string, fields?: LogFields): void;
  /**
   * Logs a condition the operator should look at.
   *
   * @param msg - The message.
   * @param fields - Structured fields.
   */
  warn(msg: string, fields?: LogFields): void;
  /**
   * Logs a failure.
   *
   * @param msg - The message.
   * @param fields - Structured fields.
   */
  error(msg: string, fields?: LogFields): void;
}

const noop = (): void => undefined;

/** A logger that drops every line. */
export const silentLogger: Logger = Object.freeze({
  debug: noop,
  info: noop,
  warn: noop,
  error: noop,
});

/**
 * The default logger: drops `debug` and `info`, writes `warn` and `error` to `console` like the
 * default level of Python's `logging`.
 */
export const defaultLogger: Logger = Object.freeze({
  debug: noop,
  info: noop,
  warn: (msg: string, fields?: LogFields) => {
    if (fields) console.warn(msg, fields);
    else console.warn(msg);
  },
  error: (msg: string, fields?: LogFields) => {
    if (fields) console.error(msg, fields);
    else console.error(msg);
  },
});
