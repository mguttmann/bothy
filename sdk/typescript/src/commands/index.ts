// SPDX-License-Identifier: Apache-2.0
/**
 * Slash commands of an agent (docs/spec/09-agents.md 10; docs/spec/11-extensions.md 11.2 row
 * `commands`): a declared list of {@link CommandSpec}, its reconciliation
 * {@link reconcileCommands} after every `ready` with a replaceable {@link CommandStore}, and
 * autocomplete under the guard {@link guardedAutocomplete}, which never answers after the
 * request expired. A handler never calls a model (R-AG-14).
 *
 * @example
 * ```ts
 * const declared = [new CommandSpec("status", "Show the run status")];
 * const result = await reconcileCommands(api, agentId, declared, { store: new MemoryCommandStore() });
 * result.created; // ["status"] on the first run
 * ```
 *
 * @module
 */

export {
  AutocompleteTooManyChoicesError,
  autocompleteRequest,
  guardedAutocomplete,
  restAutocompleteSender,
  type AutocompleteHandler,
  type AutocompleteRequest,
  type AutocompleteSend,
  type GuardedAutocompleteOptions,
} from "./autocomplete.js";
export {
  CommandSpec,
  MemoryCommandStore,
  ownCommands,
  reconcileCommands,
  type CommandClient,
  type CommandSpecOptions,
  type CommandStore,
  type ReconcileOptions,
  type ReconcileResult,
} from "./registry.js";
