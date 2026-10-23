// SPDX-License-Identifier: Apache-2.0
/**
 * Named values the hand layer works with (docs/spec/11-extensions.md 16).
 *
 * Every limit, window and wait of the SDK is a member of {@link SdkSettings}; each member names
 * its configuration key in {@link SETTING_KEYS} and its default in {@link DEFAULTS} is the
 * "Default" column of `docs/operations/configuration.md` (a test holds both equal). Durations
 * are milliseconds. The server-side limits (`component.*`, `command.*`,
 * `agent.run_update_min_interval`) are only the defaults of the SDK's own early checks: the
 * server stays the authority and its `400 validation_failed` reaches the caller as usual
 * (docs/DECISIONS.md D-M2-29-2). Every option can be overridden by the caller.
 *
 * @example
 * ```ts
 * const s = settingsFromMapping({ "sdk.http_timeout": "5s", "sdk.retry_max_attempts": "3" });
 * s.httpTimeoutMs; // 5000
 * ```
 *
 * @module
 */

/** The named values of the SDK; durations in milliseconds, counts and sizes as integers. */
export interface SdkSettings {
  /** `sdk.http_timeout`: bound of one REST request attempt. */
  readonly httpTimeoutMs: number;
  /** `sdk.retry_max_attempts`: attempts per idempotent operation, the first one included. */
  readonly retryMaxAttempts: number;
  /** `sdk.retry_backoff_base`: base and lower bound of the retry wait. */
  readonly retryBackoffBaseMs: number;
  /** `sdk.retry_backoff_cap`: largest retry wait. */
  readonly retryBackoffCapMs: number;
  /** `sdk.poll_interval`: wait between polls after an empty page. */
  readonly pollIntervalMs: number;
  /** `adapter.invocation_token_leeway`: clock tolerance of the invocation token check. */
  readonly invocationTokenLeewayMs: number;
  /** `adapter.jwks_refresh_min_interval`: at most one forced JWKS reload per interval. */
  readonly jwksRefreshMinIntervalMs: number;
  /** `webhook.receiver_tolerance`: largest distance of `webhook-timestamp` to the own clock. */
  readonly webhookReceiverToleranceMs: number;
  /** `agent.jwks_max_age`: cache age of a key list without a `Cache-Control` `max-age`. */
  readonly jwksMaxAgeMs: number;
  /** `agent.run_update_min_interval`: throttle of run patches. */
  readonly runUpdateMinIntervalMs: number;
  /** `sync.reconnect_backoff_base`: base of the gateway reconnect backoff. */
  readonly reconnectBackoffBaseMs: number;
  /** `sync.reconnect_backoff_cap`: largest gateway reconnect wait. */
  readonly reconnectBackoffCapMs: number;
  /** `sync.ws_max_frame_bytes`: largest gateway frame received. */
  readonly wsMaxFrameBytes: number;
  /** `component.max_per_message`: components per message. */
  readonly componentMaxPerMessage: number;
  /** `component.custom_id_max_chars`: length of a `custom_id`. */
  readonly componentCustomIdMaxChars: number;
  /** `component.form_max_fields`: fields per form. */
  readonly componentFormMaxFields: number;
  /** `command.autocomplete_max_choices`: choices per autocomplete answer. */
  readonly autocompleteMaxChoices: number;
  /** `command.description_max_chars`: length of a command description. */
  readonly commandDescriptionMaxChars: number;
  /** `command.max_per_principal`: commands per agent or app. */
  readonly commandMaxPerPrincipal: number;
}

/** The configuration key of every member of {@link SdkSettings}. */
export const SETTING_KEYS: Readonly<Record<keyof SdkSettings, string>> =
  Object.freeze({
    httpTimeoutMs: "sdk.http_timeout",
    retryMaxAttempts: "sdk.retry_max_attempts",
    retryBackoffBaseMs: "sdk.retry_backoff_base",
    retryBackoffCapMs: "sdk.retry_backoff_cap",
    pollIntervalMs: "sdk.poll_interval",
    invocationTokenLeewayMs: "adapter.invocation_token_leeway",
    jwksRefreshMinIntervalMs: "adapter.jwks_refresh_min_interval",
    webhookReceiverToleranceMs: "webhook.receiver_tolerance",
    jwksMaxAgeMs: "agent.jwks_max_age",
    runUpdateMinIntervalMs: "agent.run_update_min_interval",
    reconnectBackoffBaseMs: "sync.reconnect_backoff_base",
    reconnectBackoffCapMs: "sync.reconnect_backoff_cap",
    wsMaxFrameBytes: "sync.ws_max_frame_bytes",
    componentMaxPerMessage: "component.max_per_message",
    componentCustomIdMaxChars: "component.custom_id_max_chars",
    componentFormMaxFields: "component.form_max_fields",
    autocompleteMaxChoices: "command.autocomplete_max_choices",
    commandDescriptionMaxChars: "command.description_max_chars",
    commandMaxPerPrincipal: "command.max_per_principal",
  });

/**
 * The defaults: the "Default" column of `docs/operations/configuration.md` for each key of
 * {@link SETTING_KEYS} (held equal by `test/config-defaults.test.ts`).
 */
export const DEFAULTS: SdkSettings = Object.freeze({
  httpTimeoutMs: 30_000,
  retryMaxAttempts: 5,
  retryBackoffBaseMs: 500,
  retryBackoffCapMs: 30_000,
  pollIntervalMs: 2_000,
  invocationTokenLeewayMs: 10_000,
  jwksRefreshMinIntervalMs: 60_000,
  webhookReceiverToleranceMs: 300_000,
  jwksMaxAgeMs: 3_600_000,
  runUpdateMinIntervalMs: 500,
  reconnectBackoffBaseMs: 1_000,
  reconnectBackoffCapMs: 60_000,
  wsMaxFrameBytes: 1_048_576,
  componentMaxPerMessage: 25,
  componentCustomIdMaxChars: 100,
  componentFormMaxFields: 10,
  autocompleteMaxChoices: 25,
  commandDescriptionMaxChars: 100,
  commandMaxPerPrincipal: 100,
});

const DURATION = /^(\d+(?:\.\d+)?)(ms|s|m|h)$/;
const UNIT_MS: Readonly<Record<string, number>> = {
  ms: 1,
  s: 1_000,
  m: 60_000,
  h: 3_600_000,
};

/**
 * Parses a duration of the configuration reference into milliseconds.
 *
 * @param text - A number with one of the units `ms`, `s`, `m` or `h`, for example `"500ms"`.
 * @returns The duration in milliseconds.
 * @throws `RangeError` when the text is not such a duration.
 *
 * @example
 * ```ts
 * parseDuration("2m"); // 120000
 * ```
 */
export function parseDuration(text: string): number {
  const match = DURATION.exec(text.trim());
  if (!match) throw new RangeError(`not a duration: ${JSON.stringify(text)}`);
  return Number(match[1]) * (UNIT_MS[match[2] ?? "ms"] ?? 1);
}

/**
 * Builds settings from the defaults with the caller's overrides.
 *
 * @param overrides - Members to replace; absent members keep their default.
 * @returns The complete, frozen settings.
 */
export function resolveSettings(
  overrides: Partial<SdkSettings> = {},
): SdkSettings {
  return Object.freeze({ ...DEFAULTS, ...overrides });
}

/**
 * Builds settings from dotted configuration keys; absent keys keep their default.
 *
 * @param values - Dotted keys (`"sdk.http_timeout"`) to their text form; durations as
 *   `"500ms"`, `"30s"`, `"2m"`; counts and sizes as decimal integers.
 * @returns The settings.
 * @throws `RangeError` naming the key when a key is unknown or a value does not parse.
 */
export function settingsFromMapping(
  values: Readonly<Record<string, string>>,
): SdkSettings {
  const byKey = new Map<string, keyof SdkSettings>(
    (Object.entries(SETTING_KEYS) as [keyof SdkSettings, string][]).map(
      ([f, k]) => [k, f],
    ),
  );
  const out: Partial<Record<keyof SdkSettings, number>> = {};
  for (const [key, text] of Object.entries(values)) {
    const field = byKey.get(key);
    if (field === undefined)
      throw new RangeError(`unknown SDK setting: ${key}`);
    const isDuration = field.endsWith("Ms");
    let value: number;
    if (isDuration) {
      try {
        value = parseDuration(text);
      } catch {
        throw new RangeError(`invalid value for ${key}`);
      }
    } else {
      if (!/^\d+$/.test(text.trim()))
        throw new RangeError(`invalid value for ${key}`);
      value = Number(text.trim());
    }
    out[field] = value;
  }
  return resolveSettings(out);
}
