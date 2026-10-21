// SPDX-License-Identifier: Apache-2.0
/**
 * The agent's two writes on a card (docs/spec/09-agenten.md 11.4): {@link respond} answers an
 * interaction (`respondToInteraction`), {@link update} disables or changes an own component
 * (`updateComponent`).
 *
 * Each checks the precondition its operation defines before anything is sent: `respond` a
 * final state of `InteractionRespond`; `update` the component version for the required
 * `If-Match` (without it the server answers `428`) and only the members of `ComponentUpdate`.
 * Both send an `Idempotency-Key` (UUIDv7, docs/spec/06-api.md 7), so the client may repeat them
 * after a network error. A refusal of the server is the typed `ProblemError`, for example
 * `PreconditionFailedError` for a stale version.
 *
 * Browser-safe: the functions take any openapi-fetch client of the public API and import no
 * `node:` module.
 *
 * @module
 */

import type { Client } from "openapi-fetch";

import { problemError } from "../errors.js";
import type { components, paths } from "../generated/openapi.js";
import { uuidv7 } from "../ids.js";

type Interaction = components["schemas"]["Interaction"];
type InteractionRespond = components["schemas"]["InteractionRespond"];
type Component = components["schemas"]["Component"];
type ComponentUpdate = components["schemas"]["ComponentUpdate"];

/** The final states an agent may give an interaction (`InteractionRespond.state`). */
export const RESPOND_STATES = Object.freeze([
  "acknowledged",
  "failed",
  "rejected",
] as const);

/** The states `updateComponent` may set (`ComponentUpdate.state`). */
export const UPDATE_STATES = Object.freeze(["active", "disabled"] as const);

const UPDATE_MEMBERS: ReadonlySet<string> = new Set(["state", "definition"]);

/** The part of the openapi-fetch client {@link respond} and {@link update} use. */
export type InteractionClient = Pick<Client<paths>, "POST" | "PATCH">;

/** Options of {@link respond}. */
export interface RespondOptions {
  /** A short answer shown with the card (`response.text`); content, so not text-checked. */
  readonly text?: string | undefined;
  /** The `Idempotency-Key`; a new UUIDv7 when absent. */
  readonly idempotencyKey?: string | undefined;
}

/** Options of {@link update}. */
export interface UpdateOptions {
  /** The `Idempotency-Key`; a new UUIDv7 when absent. */
  readonly idempotencyKey?: string | undefined;
}

/**
 * Answers an interaction of an own card (`respondToInteraction`).
 *
 * From `received` or `dispatched` the server stores the state and the response; the same final
 * state again answers the current interaction; every other transition is
 * `409 invalid_state_transition` (09 11.4). No event follows: the card shows the result through
 * {@link update}.
 *
 * @param api - An openapi-fetch client of the public API (for example `createApiClient`).
 * @param interactionId - The interaction of `interaction.created`.
 * @param state - `acknowledged`, `failed` or `rejected`.
 * @param options - The response text and the `Idempotency-Key`.
 * @returns The interaction as the server stored it.
 * @throws `RangeError` before sending for a state outside {@link RESPOND_STATES} or a text that
 *   is not a string; `ProblemError` when the server refuses.
 *
 * @example
 * ```ts
 * await respond(api, interactionId, "acknowledged", { text: "Refund approved" });
 * ```
 */
export async function respond(
  api: InteractionClient,
  interactionId: string,
  state: InteractionRespond["state"],
  options: RespondOptions = {},
): Promise<Interaction> {
  if (!(RESPOND_STATES as readonly unknown[]).includes(state))
    throw new RangeError("state: not one of the final interaction states");
  const body: InteractionRespond = { state };
  if (options.text !== undefined) {
    if (typeof options.text !== "string")
      throw new RangeError("response.text: must be a string");
    body.response = { text: options.text };
  }
  const result = await api.POST("/interactions/{interaction_id}/respond", {
    params: {
      path: { interaction_id: interactionId },
      header: { "Idempotency-Key": options.idempotencyKey ?? uuidv7() },
    },
    body,
  });
  if (!result.response.ok)
    throw problemError(result.response.status, result.error, {
      operation: "respondToInteraction",
    });
  return result.data as Interaction;
}

/**
 * Disables, reactivates or changes an own component (`updateComponent`).
 *
 * `If-Match` carries `version`; a stale one is `412 version_conflict` with the current component
 * in `current`. `used` and `expired` are final, `active -> disabled` is allowed, `disabled ->
 * active` only before `expires_at` (09 11.4). A new `definition` follows 09 11.1 for the
 * component's type, for example the `definition` of a builder of this module; the server checks
 * it. Type, custom ID and the access members cannot change.
 *
 * @param api - An openapi-fetch client of the public API.
 * @param componentId - The component (`Interaction.component_id` or a `Component.id`).
 * @param version - The component version the change is based on, a positive integer.
 * @param change - `state` and/or `definition`.
 * @param options - The `Idempotency-Key`.
 * @returns The component as the server stored it.
 * @throws `RangeError` before sending for a version that is not a positive integer, a member
 *   outside `ComponentUpdate`, a state outside {@link UPDATE_STATES} or a definition that is not
 *   an object; `ProblemError` when the server refuses.
 *
 * @example
 * ```ts
 * const interaction = await respond(api, interactionId, "acknowledged");
 * await update(api, interaction.component_id, 1, { state: "disabled" });
 * ```
 */
export async function update(
  api: InteractionClient,
  componentId: string,
  version: number,
  change: ComponentUpdate,
  options: UpdateOptions = {},
): Promise<Component> {
  if (!Number.isInteger(version) || version < 1)
    throw new RangeError("version: must be a positive integer for If-Match");
  for (const key of Object.keys(change))
    if (!UPDATE_MEMBERS.has(key))
      throw new RangeError(
        /^[a-z_]+$/.test(key)
          ? `${key}: cannot change through updateComponent`
          : "change: unknown member",
      );
  const body: ComponentUpdate = {};
  if (change.state !== undefined) {
    if (!(UPDATE_STATES as readonly unknown[]).includes(change.state))
      throw new RangeError("state: must be active or disabled");
    body.state = change.state;
  }
  if (change.definition !== undefined) {
    const { definition } = change;
    if (
      typeof definition !== "object" ||
      definition === null ||
      Array.isArray(definition)
    )
      throw new RangeError("definition: must be an object");
    body.definition = definition;
  }
  const result = await api.PATCH("/components/{component_id}", {
    params: {
      path: { component_id: componentId },
      header: {
        "If-Match": `"${version}"`,
        "Idempotency-Key": options.idempotencyKey ?? uuidv7(),
      },
    },
    body,
  });
  if (!result.response.ok)
    throw problemError(result.response.status, result.error, {
      operation: "updateComponent",
    });
  return result.data as Component;
}
