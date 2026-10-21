// SPDX-License-Identifier: Apache-2.0
/**
 * Autocomplete of command options (docs/spec/09-agenten.md 10.5, R-AG-14; port of
 * `guarded_autocomplete` in `bothy_sdk/commands.py`).
 *
 * {@link guardedAutocomplete} runs the handler of a `command.autocomplete_requested` event under
 * a guard that ends at the request's `expires_at`: on timeout the handler's `AbortSignal` fires,
 * nothing is sent (the server answers the person with an empty list) and a late result of the
 * handler is dropped; more choices than `command.autocomplete_max_choices` throw and nothing is
 * sent. The answer goes out through a sender callback: the gateway op
 * `command.autocomplete_result`, or {@link restAutocompleteSender} (`answerAutocomplete`)
 * without a gateway.
 *
 * A handler never calls a model: autocomplete must answer within `command.autocomplete_timeout`
 * from data the adapter already has, and the SDK never calls a model here either (R-AG-14).
 *
 * @module
 */

import type { Client } from "openapi-fetch";

import { type Clock, systemClock } from "../clock.js";
import { DEFAULTS, type SdkSettings } from "../config.js";
import { problemError } from "../errors.js";
import type { components, paths } from "../generated/openapi.js";
import { type Logger, defaultLogger } from "../logger.js";

type CommandOptionValue = components["schemas"]["CommandOptionValue"];
type AutocompletePayload =
  components["schemas"]["CommandAutocompleteRequestedPayload"];

/** One `command.autocomplete_requested` event, as the handler sees it. */
export interface AutocompleteRequest {
  /** The request (the answer names it). */
  readonly requestId: string;
  /** The command. */
  readonly commandId: string;
  /** The option being typed. */
  readonly option: string;
  /** What the person typed so far. */
  readonly value: string;
  /** The other option values so far. */
  readonly options: readonly CommandOptionValue[];
  /** The channel. */
  readonly channelId: string;
  /** The thread, if any. */
  readonly threadId: string | undefined;
  /** The person typing. */
  readonly userId: string;
  /** The deadline of the answer, Unix milliseconds. */
  readonly expiresAt: number;
}

/**
 * The handler of an autocomplete request: returns the choices for what was typed. It must not
 * call a model (R-AG-14) and should stop when `signal` fires at `expires_at`.
 */
export type AutocompleteHandler = (
  request: AutocompleteRequest,
  signal: AbortSignal,
) => Promise<readonly components["schemas"]["CommandChoice"][]>;

/** Sends the choices of a request (gateway `command.autocomplete_result` or REST). */
export type AutocompleteSend = (
  requestId: string,
  choices: readonly components["schemas"]["CommandChoice"][],
) => Promise<void>;

/** Options of {@link guardedAutocomplete}. */
export interface GuardedAutocompleteOptions {
  /** The time source of the guard (default `systemClock`). */
  readonly clock?: Clock | undefined;
  /** The limit `command.autocomplete_max_choices` (default `DEFAULTS`). */
  readonly settings?: SdkSettings | undefined;
  /** The logger (default `defaultLogger`). */
  readonly logger?: Logger | undefined;
}

/** The handler returned more than `command.autocomplete_max_choices` choices; nothing was sent. */
export class AutocompleteTooManyChoicesError extends RangeError {
  /** Creates the error. */
  constructor() {
    super("more choices than command.autocomplete_max_choices");
    this.name = "AutocompleteTooManyChoicesError";
  }
}

/**
 * Reads the payload of `command.autocomplete_requested` into an {@link AutocompleteRequest}.
 *
 * @param payload - The generated payload of the event.
 * @returns The request.
 * @throws `RangeError` when `expires_at` is not a time.
 */
export function autocompleteRequest(
  payload: AutocompletePayload,
): AutocompleteRequest {
  const expiresAt = Date.parse(payload.expires_at);
  if (Number.isNaN(expiresAt)) throw new RangeError("expires_at: not a time");
  return {
    requestId: payload.request_id,
    commandId: payload.command_id,
    option: payload.option,
    value: payload.value,
    options: Array.isArray(payload.options) ? [...payload.options] : [],
    channelId: payload.channel_id,
    threadId: payload.thread_id,
    userId: payload.user_id,
    expiresAt,
  };
}

/**
 * Returns an {@link AutocompleteSend} over the REST operation `answerAutocomplete` (for an
 * adapter without a gateway connection).
 *
 * @param api - An openapi-fetch client of the public API.
 * @returns The send function.
 * @throws (from the returned function) `ProblemError` when the server refuses, for
 *   example `410` after `expires_at`.
 */
export function restAutocompleteSender(
  api: Pick<Client<paths>, "POST">,
): AutocompleteSend {
  return async (requestId, choices) => {
    const result = await api.POST(
      "/command-autocompletions/{request_id}/answer",
      {
        params: { path: { request_id: requestId } },
        body: { choices: [...choices] },
      },
    );
    if (!result.response.ok)
      throw problemError(result.response.status, result.error, {
        operation: "answerAutocomplete",
      });
  };
}

const TIMED_OUT: unique symbol = Symbol("timed out");

/**
 * Runs the handler until the request expires and sends its choices.
 *
 * @param handler - The handler.
 * @param request - The request.
 * @param send - Sends the choices (gateway or {@link restAutocompleteSender}).
 * @param options - Clock, settings and logger.
 * @returns `true` when an answer was sent, `false` after a timeout or for an expired request
 *   (the handler is then not called).
 * @throws {@link AutocompleteTooManyChoicesError} for too many choices (nothing was sent); an
 *   error of the handler or of `send` as thrown.
 *
 * @example
 * ```ts
 * await guardedAutocomplete(
 *   async (req) => [{ name: "Staging", value: "staging" }].filter((c) => c.value.startsWith(req.value)),
 *   autocompleteRequest(event.payload),
 *   restAutocompleteSender(api),
 * );
 * ```
 */
export async function guardedAutocomplete(
  handler: AutocompleteHandler,
  request: AutocompleteRequest,
  send: AutocompleteSend,
  options: GuardedAutocompleteOptions = {},
): Promise<boolean> {
  const clock = options.clock ?? systemClock;
  const limit = (options.settings ?? DEFAULTS).autocompleteMaxChoices;
  const logger = options.logger ?? defaultLogger;
  const remaining = request.expiresAt - clock.now();
  if (remaining <= 0) return false;
  const cancel = new AbortController();
  const stopGuard = new AbortController();
  const work = Promise.resolve().then(() => handler(request, cancel.signal));
  // A result or an error after the timeout is dropped, never unhandled.
  work.catch(() => undefined);
  const guard = clock.sleep(remaining, stopGuard.signal).then(
    (): typeof TIMED_OUT => TIMED_OUT,
    (): typeof TIMED_OUT => TIMED_OUT,
  );
  let outcome:
    readonly components["schemas"]["CommandChoice"][] | typeof TIMED_OUT;
  try {
    outcome = await Promise.race([work, guard]);
  } finally {
    stopGuard.abort();
  }
  if (outcome === TIMED_OUT || clock.now() >= request.expiresAt) {
    cancel.abort();
    logger.info(`autocomplete ${request.requestId} timed out; nothing sent`);
    return false;
  }
  const choices = [...outcome];
  if (choices.length > limit) throw new AutocompleteTooManyChoicesError();
  await send(request.requestId, choices);
  return true;
}
