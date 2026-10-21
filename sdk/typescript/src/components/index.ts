// SPDX-License-Identifier: Apache-2.0
/**
 * Message components of `component.v1` (docs/spec/09-agenten.md 11; docs/spec/11-erweiterungen.md
 * 11.2 row `components`): the builders {@link button}, {@link select}, {@link form},
 * {@link approvalCard} (09 11.5) and {@link questionCard} (09 11.6), the limit check
 * {@link checkComponents}, the card body {@link cardMessage}, and the agent's writes
 * {@link respond} and {@link update}.
 *
 * Browser-safe (part of the `./core` entry): no `node:` import and no import of `auth`,
 * `invocation`, `gateway`, `webhooks` or `outbox`.
 *
 * @example
 * ```ts
 * const card = approvalCard({ withAlways: false });
 * card.map((c) => c.custom_id); // ["approval.once", "approval.session", "approval.deny"]
 * button("refund", "Refund", "yes", { style: "danger" }).definition;
 * // { label: "Refund", style: "danger", value: "yes" }
 * ```
 *
 * @module
 */

export {
  APPROVAL_VALUES,
  BUTTON_STYLES,
  FORM_FIELD_TYPES,
  approvalCard,
  button,
  cardMessage,
  checkComponents,
  form,
  questionCard,
  select,
  type ApprovalCardOptions,
  type ApprovalValue,
  type ButtonOptions,
  type ButtonStyle,
  type CardMessageOptions,
  type ComponentOptions,
  type FormField,
  type FormOptions,
  type QuestionCardOptions,
  type SelectOption,
  type SelectOptions,
} from "./builders.js";
export {
  RESPOND_STATES,
  UPDATE_STATES,
  respond,
  update,
  type InteractionClient,
  type RespondOptions,
  type UpdateOptions,
} from "./interactions.js";
