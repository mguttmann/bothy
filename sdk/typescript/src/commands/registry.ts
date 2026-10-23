// SPDX-License-Identifier: Apache-2.0
/**
 * The declared commands of an agent and their reconciliation (docs/spec/09-agents.md 10.2;
 * port of `bothy_sdk/commands.py`).
 *
 * An adapter may declare its commands. When it does, it reconciles them after every `ready`
 * (docs/DECISIONS.md D-M2-29-12; 09 10.2 asks for one request, but the API has no such operation
 * and no list of own commands, a reported spec gap): own commands are those of
 * `listChannelCommands` over the channels of `listServedChannels` whose `owner_principal_id` is
 * the agent; missing ones are registered, changed ones updated with `If-Match` of their version,
 * undeclared ones removed. A replaceable {@link CommandStore} records the own commands of the
 * last reconciliation, so an undeclared command that is no longer listed in a served channel
 * (its channel was unbound) is removed as well. `undefined` (the default) means the adapter does
 * not manage commands; an empty list removes all own commands.
 *
 * @module
 */

import type { Client } from "openapi-fetch";

import { DEFAULTS, type SdkSettings } from "../config.js";
import { ConflictError, isNotFound, problemError } from "../errors.js";
import type { components, paths } from "../generated/openapi.js";
import { uuidv7 } from "../ids.js";
import { type Logger, defaultLogger } from "../logger.js";

type Command = components["schemas"]["Command"];
type CommandRegister = components["schemas"]["CommandRegister"];
type CommandOption = components["schemas"]["CommandOption"];
type CommandLevel = components["schemas"]["CommandLevel"];
type TargetKind = components["schemas"]["TargetKind"];

/** The part of the openapi-fetch client the reconciliation uses. */
export type CommandClient = Pick<
  Client<paths>,
  "GET" | "POST" | "PUT" | "DELETE"
>;

// The records make tsc hold each vocabulary equal to the generated type.
const TARGET_KINDS: Readonly<Record<TargetKind, true>> = {
  instance: true,
  space: true,
  channel: true,
  host: true,
};
const LEVELS: Readonly<Record<CommandLevel, true>> = {
  write: true,
  trigger: true,
};

/** The optional members of a {@link CommandSpec}. */
export interface CommandSpecOptions {
  /** The options (generated `CommandOption`). */
  readonly options?: readonly CommandOption[] | undefined;
  /** Where it applies; absent leaves the server default. */
  readonly scopeKind?: string | undefined;
  /** The scope target, if any. */
  readonly scopeId?: string | undefined;
  /** The required level, if not the default. */
  readonly requiredLevel?: string | undefined;
  /** `true` for a control command. */
  readonly control?: boolean | undefined;
}

/**
 * A command the adapter declares; maps to `CommandRegister`.
 *
 * @example
 * ```ts
 * new CommandSpec("status", "Show the run status").toRegister().name; // "status"
 * ```
 */
export class CommandSpec {
  /** The command name. */
  readonly name: string;
  /** At most `command.description_max_chars` characters. */
  readonly description: string;
  /** The options. */
  readonly options: readonly CommandOption[];
  /** Where it applies, if not the server default. */
  readonly scopeKind: string | undefined;
  /** The scope target, if any. */
  readonly scopeId: string | undefined;
  /** The required level, if not the default. */
  readonly requiredLevel: string | undefined;
  /** Whether it is a control command. */
  readonly control: boolean;

  /**
   * Declares a command.
   *
   * @param name - The command name.
   * @param description - At most `command.description_max_chars` characters.
   * @param options - Options, scope, required level and the control flag.
   */
  constructor(
    name: string,
    description: string,
    options: CommandSpecOptions = {},
  ) {
    this.name = name;
    this.description = description;
    this.options = Object.freeze([...(options.options ?? [])]);
    this.scopeKind = options.scopeKind;
    this.scopeId = options.scopeId;
    this.requiredLevel = options.requiredLevel;
    this.control = options.control ?? false;
    Object.freeze(this);
  }

  /**
   * Builds the generated `CommandRegister`.
   *
   * @param settings - The limit `command.description_max_chars` (default `DEFAULTS`).
   * @returns The registration body.
   * @throws `RangeError` when the description is longer than the limit or the scope kind or the
   *   level is outside its vocabulary.
   */
  toRegister(settings: SdkSettings = DEFAULTS): CommandRegister {
    if ([...this.description].length > settings.commandDescriptionMaxChars)
      throw new RangeError(
        `description of ${this.name} is longer than the limit command.description_max_chars`,
      );
    const body: CommandRegister = {
      name: this.name,
      description: this.description,
    };
    if (this.options.length > 0) body.options = [...this.options];
    if (this.scopeKind !== undefined)
      body.scope_kind = member(TARGET_KINDS, this.scopeKind, "scope_kind");
    if (this.scopeId !== undefined) body.scope_id = this.scopeId;
    if (this.requiredLevel !== undefined)
      body.required_level = member(
        LEVELS,
        this.requiredLevel,
        "required_level",
      );
    if (this.control) body.control = true;
    return body;
  }
}

/** What a reconciliation changed (command names). */
export interface ReconcileResult {
  /** Registered now. */
  readonly created: readonly string[];
  /** Updated now. */
  readonly updated: readonly string[];
  /** Removed now. */
  readonly removed: readonly string[];
}

/**
 * Durable record of the agent's own commands after the last reconciliation: command name to
 * command ID. Replaceable, like the inbox and outbox stores.
 */
export interface CommandStore {
  /**
   * Returns the recorded commands.
   *
   * @returns Name to command ID; empty before the first reconciliation.
   */
  load(): Promise<Readonly<Record<string, string>>>;
  /**
   * Replaces the record.
   *
   * @param commands - Name to command ID of every own command now.
   */
  save(commands: Readonly<Record<string, string>>): Promise<void>;
}

/** A {@link CommandStore} in memory (tests, or an adapter without a state directory). */
export class MemoryCommandStore implements CommandStore {
  private commands: Readonly<Record<string, string>> = {};

  /**
   * Returns the recorded commands.
   *
   * @returns Name to command ID.
   */
  load(): Promise<Readonly<Record<string, string>>> {
    return Promise.resolve({ ...this.commands });
  }

  /**
   * Replaces the record.
   *
   * @param commands - Name to command ID.
   */
  save(commands: Readonly<Record<string, string>>): Promise<void> {
    this.commands = Object.freeze({ ...commands });
    return Promise.resolve();
  }
}

/** Options of {@link reconcileCommands}. */
export interface ReconcileOptions {
  /** The limits `command.description_max_chars` and `command.max_per_principal`. */
  readonly settings?: SdkSettings | undefined;
  /** The record of own commands; without one only listed commands are removed. */
  readonly store?: CommandStore | undefined;
  /** The logger (default `defaultLogger`). */
  readonly logger?: Logger | undefined;
}

/**
 * Returns the agent's own commands by name over every served channel.
 *
 * @param api - An openapi-fetch client of the public API.
 * @param agentId - The agent's principal ID.
 * @returns Own commands by name; an instance-wide command listed in several channels counts once.
 * @throws `ProblemError` when a list is refused.
 */
export async function ownCommands(
  api: CommandClient,
  agentId: string,
): Promise<Map<string, Command>> {
  const served = await api.GET("/agents/me/channels");
  if (!served.response.ok)
    throw problemError(served.response.status, served.error, {
      operation: "listServedChannels",
    });
  const found = new Map<string, Command>();
  for (const channel of served.data?.items ?? []) {
    const listed = await api.GET("/channels/{channel_id}/commands", {
      params: { path: { channel_id: channel.channel_id } },
    });
    if (!listed.response.ok)
      throw problemError(listed.response.status, listed.error, {
        operation: "listChannelCommands",
      });
    for (const command of listed.data?.items ?? [])
      if (command.owner_principal_id.toLowerCase() === agentId.toLowerCase())
        found.set(command.id, command);
  }
  return new Map([...found.values()].map((c) => [c.name, c]));
}

/**
 * Makes the agent's own commands equal to `declared` (D-M2-29-12).
 *
 * @param api - An openapi-fetch client of the public API.
 * @param agentId - The agent's principal ID.
 * @param declared - The declared commands; `undefined` does nothing.
 * @param options - Settings, the {@link CommandStore} and the logger.
 * @returns What changed.
 * @throws `RangeError` before any request when a description is too long or more commands than
 *   `command.max_per_principal` are declared; `ProblemError` for a refusal other than a
 *   `409 conflict` on register (logged and skipped: another principal owns the name in the
 *   scope).
 *
 * @example
 * ```ts
 * await reconcileCommands(api, agentId, [new CommandSpec("status", "Show the run status")]);
 * ```
 */
export async function reconcileCommands(
  api: CommandClient,
  agentId: string,
  declared: readonly CommandSpec[] | undefined,
  options: ReconcileOptions = {},
): Promise<ReconcileResult> {
  if (declared === undefined) return { created: [], updated: [], removed: [] };
  const settings = options.settings ?? DEFAULTS;
  const logger = options.logger ?? defaultLogger;
  const wanted = new Map(declared.map((s) => [s.name, s.toRegister(settings)]));
  if (wanted.size > settings.commandMaxPerPrincipal)
    throw new RangeError(
      "commands: more declared commands than command.max_per_principal",
    );
  const existing = await ownCommands(api, agentId);
  const recorded = (await options.store?.load()) ?? {};
  const now: Record<string, string> = {};
  const created: string[] = [];
  const updated: string[] = [];
  const removed: string[] = [];
  for (const [name, body] of wanted) {
    const current = existing.get(name);
    if (current === undefined) {
      const result = await api.POST("/commands", {
        params: { header: { "Idempotency-Key": uuidv7() } },
        body,
      });
      if (result.response.ok && result.data !== undefined) {
        created.push(name);
        now[name] = result.data.id;
        continue;
      }
      const error = problemError(result.response.status, result.error, {
        operation: "registerCommand",
      });
      if (!(error instanceof ConflictError)) throw error;
      logger.info(`command ${name} not registered`, { code: error.code });
    } else {
      now[name] = current.id;
      // `updateCommand` replaces the definition, and the stored command always carries
      // `options` and `control`: their absence in the body means "none" and `false`.
      const full: CommandRegister = {
        ...body,
        options: body.options ?? [],
        control: body.control ?? false,
      };
      if (!differs(current, full)) continue;
      const result = await api.PUT("/commands/{command_id}", {
        params: {
          path: { command_id: current.id },
          header: { "If-Match": `"${current.version}"` },
        },
        body: full,
      });
      if (!result.response.ok)
        throw problemError(result.response.status, result.error, {
          operation: "updateCommand",
        });
      updated.push(name);
    }
  }
  const listedIds = new Set([...existing.values()].map((c) => c.id));
  for (const [name, current] of existing)
    if (!wanted.has(name)) {
      await remove(api, current.id);
      removed.push(name);
    }
  for (const [name, commandId] of Object.entries(recorded)) {
    if (wanted.has(name) || listedIds.has(commandId)) continue;
    // Recorded earlier, no longer listed: remove it by ID; a 404 means it is gone already (or
    // no longer visible), so the record forgets it.
    try {
      await remove(api, commandId);
      removed.push(name);
    } catch (error) {
      if (!isNotFound(error)) throw error;
    }
  }
  await options.store?.save(now);
  return { created, updated, removed };
}

async function remove(api: CommandClient, commandId: string): Promise<void> {
  const result = await api.DELETE("/commands/{command_id}", {
    params: { path: { command_id: commandId } },
  });
  if (!result.response.ok)
    throw problemError(result.response.status, result.error, {
      operation: "removeCommand",
    });
}

/** Whether a member of the registration body differs from the stored command. */
function differs(existing: Command, wanted: CommandRegister): boolean {
  const theirs = existing as unknown as Readonly<Record<string, unknown>>;
  return Object.entries(wanted).some(
    ([key, value]) => canonical(theirs[key]) !== canonical(value),
  );
}

/** JSON with sorted keys, so two equal values compare equal. */
function canonical(value: unknown): string {
  return (
    JSON.stringify(value, (_key, v: unknown) =>
      typeof v === "object" && v !== null && !Array.isArray(v)
        ? Object.fromEntries(
            Object.entries(v as Record<string, unknown>).sort(([a], [b]) =>
              a < b ? -1 : a > b ? 1 : 0,
            ),
          )
        : v,
    ) ?? "undefined"
  );
}

/** A key of `members` (a closed vocabulary of the generated core). */
function member<K extends string>(
  members: Readonly<Record<K, true>>,
  value: unknown,
  field: string,
): K {
  if (typeof value === "string" && Object.hasOwn(members, value))
    return value as K;
  throw new RangeError(`${field}: not one of the allowed values`);
}
