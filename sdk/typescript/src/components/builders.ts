// SPDX-License-Identifier: Apache-2.0
/**
 * Builders of `component.v1` (docs/spec/09-agenten.md 11; port of `bothy_sdk/components.py`).
 *
 * {@link button}, {@link select} and {@link form} return generated `ComponentCreate` objects
 * for the `components` of a message with `kind = card`; {@link approvalCard} builds the approval
 * request of 09 11.5 and {@link questionCard} a choice question of 09 11.6. The builders refuse,
 * before anything is sent, what the server refuses with `400 validation_failed`: styles outside
 * {@link BUTTON_STYLES}, field types outside {@link FORM_FIELD_TYPES}, the limits
 * `component.custom_id_max_chars` and `component.form_max_fields`, the rules of each definition,
 * and display texts with control or bidi characters (SEC-IN-4). {@link checkComponents} holds a
 * whole card against `component.max_per_message` and unique custom IDs. The limits come from
 * {@link SdkSettings}, whose defaults are the server's; the server stays the authority. Errors
 * are `RangeError`s naming the field, never the text a caller passed.
 *
 * Browser-safe: no `node:` import.
 *
 * @module
 */

import { DEFAULTS, type SdkSettings } from "../config.js";
import type { components } from "../generated/openapi.js";
import { uuidv7 } from "../ids.js";

type ComponentCreate = components["schemas"]["ComponentCreate"];
type ComponentType = components["schemas"]["ComponentType"];
type Capability = components["schemas"]["Capability"];
type ComponentRestriction = components["schemas"]["ComponentRestriction"];
type MessageCreate = components["schemas"]["MessageCreate"];

/** Button styles: roles of the design system, never a free colour (09 11.1). */
export const BUTTON_STYLES = Object.freeze([
  "primary",
  "secondary",
  "success",
  "danger",
  "ghost",
] as const);

/** Types of form fields (09 11.1). */
export const FORM_FIELD_TYPES = Object.freeze([
  "text",
  "textarea",
  "number",
  "boolean",
  "choice",
] as const);

/** Values of the approval buttons (09 11.5); their custom IDs are `approval.<value>`. */
export const APPROVAL_VALUES = Object.freeze([
  "once",
  "session",
  "always",
  "deny",
] as const);

/** One style of {@link BUTTON_STYLES}. */
export type ButtonStyle = (typeof BUTTON_STYLES)[number];

/** One value of {@link APPROVAL_VALUES}. */
export type ApprovalValue = (typeof APPROVAL_VALUES)[number];

// `terminal` exists only on the target `host` (07 2.4); a component cannot require it. The
// record makes tsc hold the list equal to the generated `Capability` without `terminal`.
const CAPABILITIES: Readonly<Record<Exclude<Capability, "terminal">, true>> = {
  approve_actions: true,
  upload: true,
  export: true,
  manage: true,
};
const RESTRICTIONS: Readonly<Record<ComponentRestriction, true>> = {
  any: true,
  run_starter: true,
  owner: true,
};
const FIELD_NAME = /^[a-z][a-z0-9_]*$/;
const LENGTH_TYPES: ReadonlySet<string> = new Set(["text", "textarea"]);
// Control characters (category Cc) and the bidi controls of SEC-IN-4, as kernel/textcheck of
// the server lists them.
const UNSAFE_TEXT = /[\p{Cc}؜‎‏‪‫‬‭‮⁦⁧⁨⁩]/u;

// (label, style, required capability, restricted to) per approval value, the table of 09 11.5.
const APPROVAL: Readonly<
  Record<
    ApprovalValue,
    readonly [string, ButtonStyle, Capability | undefined, ComponentRestriction]
  >
> = {
  once: ["Allow once", "primary", "approve_actions", "run_starter"],
  session: [
    "Allow for this session",
    "secondary",
    "approve_actions",
    "run_starter",
  ],
  always: ["Always allow", "secondary", "approve_actions", "owner"],
  deny: ["Deny", "danger", undefined, "run_starter"],
};

/** One option of a {@link select}, or one answer of a {@link questionCard}. */
export interface SelectOption {
  /** Display text. */
  readonly label: string;
  /** Value the interaction reports; unique within the select. */
  readonly value: string;
  /** Optional display text under the label. */
  readonly description?: string | undefined;
}

/** One field of a {@link form}. */
export interface FormField {
  /** Key of the field in the submitted values, `[a-z][a-z0-9_]*`, unique in the form. */
  readonly name: string;
  /** Display text. */
  readonly label: string;
  /** One of {@link FORM_FIELD_TYPES}. */
  readonly type: string;
  /** Whether the field must be filled (default `false`). */
  readonly required?: boolean | undefined;
  /** Longest text, at least 1; only for `text` and `textarea`. */
  readonly maxLength?: number | undefined;
  /** Distinct answers; required for `choice` and only there. */
  readonly choices?: readonly string[] | undefined;
}

/** The members every builder shares; they map to the members of `ComponentCreate`. */
export interface ComponentOptions {
  /**
   * Whether using it disables the other single-use components of the message (09 11.3); the
   * server's default applies when left out.
   */
  readonly singleUse?: boolean | undefined;
  /** A capability the person needs, for example `approve_actions`. */
  readonly requiredCapability?: string | undefined;
  /** `any`, `run_starter` or `owner` (09 11.2). */
  readonly restrictedTo?: string | undefined;
  /** Expiry as Unix milliseconds or a `Date`; the server's default applies when left out. */
  readonly expiresAt?: number | Date | undefined;
  /** Source of the limits (default `DEFAULTS`). */
  readonly settings?: SdkSettings | undefined;
}

/** Options of {@link button}. */
export interface ButtonOptions extends ComponentOptions {
  /** One of {@link BUTTON_STYLES} (default `secondary`). */
  readonly style?: string | undefined;
  /** Optional semantic icon name of the design system; the server checks the name. */
  readonly icon?: string | undefined;
}

/** Options of {@link select}. */
export interface SelectOptions extends ComponentOptions {
  /** Optional display text. */
  readonly placeholder?: string | undefined;
  /** Fewest selected options; the server's default 1 when left out. */
  readonly minValues?: number | undefined;
  /**
   * Most selected options; the server's default 1 when left out. At least 1, at least
   * `minValues` and at most the number of options.
   */
  readonly maxValues?: number | undefined;
}

/** Options of {@link form}. */
export interface FormOptions extends ComponentOptions {
  /** Optional display text of the submit button. */
  readonly submitLabel?: string | undefined;
}

/** Options of {@link approvalCard}. */
export interface ApprovalCardOptions {
  /** `false` leaves out `approval.always` (an adapter setting of 09 11.5); default `true`. */
  readonly withAlways?: boolean | undefined;
  /** Source of the limits. */
  readonly settings?: SdkSettings | undefined;
}

/** Options of {@link questionCard}. */
export interface QuestionCardOptions {
  /** `true` builds one select instead of one button per answer. */
  readonly asSelect?: boolean | undefined;
  /** Display text of the select. */
  readonly placeholder?: string | undefined;
  /** Who may answer; the run's starter (and owners) by default. */
  readonly restrictedTo?: string | undefined;
  /** Source of the limits. */
  readonly settings?: SdkSettings | undefined;
}

/** Options of {@link cardMessage}. */
export interface CardMessageOptions {
  /** The run the card belongs to (an approval card carries it, 09 11.5). */
  readonly runId?: string | undefined;
  /** The message it answers, if any. */
  readonly replyToMessageId?: string | undefined;
  /** Source of the limits. */
  readonly settings?: SdkSettings | undefined;
}

/**
 * Builds a button.
 *
 * @param customId - ID of the component in its message, at most `component.custom_id_max_chars`
 *   characters.
 * @param label - Display text.
 * @param value - Value the interaction reports.
 * @param options - Style, icon and the shared members of {@link ComponentOptions}.
 * @returns The generated `ComponentCreate`.
 * @throws `RangeError` when a value breaks a rule of `component.v1`; the message names the field.
 *
 * @example
 * ```ts
 * button("refund", "Refund", "yes", { style: "danger" }).definition;
 * // { label: "Refund", style: "danger", value: "yes" }
 * ```
 */
export function button(
  customId: string,
  label: string,
  value: string,
  options: ButtonOptions = {},
): ComponentCreate {
  const definition: Record<string, unknown> = {
    label: text(label, "label"),
    style: style(options.style ?? "secondary"),
  };
  if (options.icon !== undefined) definition.icon = text(options.icon, "icon");
  definition.value = text(value, "value");
  return component("button", customId, definition, options);
}

/**
 * Builds a select.
 *
 * @param customId - ID of the component in its message.
 * @param choices - At least one option with distinct values.
 * @param options - Placeholder, bounds and the shared members of {@link ComponentOptions}.
 * @returns The generated `ComponentCreate`.
 * @throws `RangeError` when a value breaks a rule of `component.v1`; the message names the field.
 *
 * @example
 * ```ts
 * select("room", [{ label: "Room A", value: "a" }, { label: "Room B", value: "b" }]);
 * ```
 */
export function select(
  customId: string,
  choices: readonly SelectOption[],
  options: SelectOptions = {},
): ComponentCreate {
  const definition: Record<string, unknown> = {};
  if (options.placeholder !== undefined)
    definition.placeholder = text(options.placeholder, "placeholder");
  definition.options = selectOptions(choices, "options");
  const { minValues, maxValues } = options;
  const low = minValues === undefined ? 1 : integer(minValues, "min_values");
  const high = maxValues === undefined ? 1 : integer(maxValues, "max_values");
  if (high < 1 || high > choices.length)
    throw new RangeError(
      "max_values: must be at least 1 and at most the number of options",
    );
  if (low < 0 || low > high)
    throw new RangeError("min_values: must be between 0 and max_values");
  if (minValues !== undefined) definition.min_values = low;
  if (maxValues !== undefined) definition.max_values = high;
  return component("select", customId, definition, options);
}

/**
 * Builds a form.
 *
 * @param customId - ID of the component in its message.
 * @param title - Display text.
 * @param fields - At least one and at most `component.form_max_fields` fields with distinct
 *   names.
 * @param options - Submit label and the shared members of {@link ComponentOptions}.
 * @returns The generated `ComponentCreate`.
 * @throws `RangeError` when a value breaks a rule of `component.v1`; the message names the field.
 *
 * @example
 * ```ts
 * form("trip", "Trip details", [{ name: "destination", label: "Destination", type: "text" }]);
 * ```
 */
export function form(
  customId: string,
  title: string,
  fields: readonly FormField[],
  options: FormOptions = {},
): ComponentCreate {
  const settings = options.settings ?? DEFAULTS;
  if (fields.length === 0 || fields.length > settings.componentFormMaxFields)
    throw new RangeError(
      "fields: between one field and component.form_max_fields fields",
    );
  const definition: Record<string, unknown> = { title: text(title, "title") };
  if (options.submitLabel !== undefined)
    definition.submit_label = text(options.submitLabel, "submit_label");
  const names = new Set<string>();
  definition.fields = fields.map((item, index) => {
    const rendered = formField(item, `fields[${index}]`);
    if (names.has(item.name))
      throw new RangeError(`fields[${index}].name: repeated`);
    names.add(item.name);
    return rendered;
  });
  return component("form", customId, definition, options);
}

/**
 * Builds the approval buttons of 09 11.5.
 *
 * The buttons are `approval.once`, `approval.session`, `approval.always` and `approval.deny`,
 * all single use, so one decision disables the rest (09 11.3). Every button but `deny` requires
 * `approve_actions`; `always` is for owners only, because it changes the runtime's lasting allow
 * list for every future trigger; the others are for the run's starter. Send them with
 * {@link cardMessage} or the outbox as a message with `kind = card` and the run's `run_id`.
 *
 * @param options - `withAlways: false` leaves out `approval.always`; the settings.
 * @returns The buttons in the order of the table.
 *
 * @example
 * ```ts
 * approvalCard({ withAlways: false }).map((c) => c.custom_id);
 * // ["approval.once", "approval.session", "approval.deny"]
 * ```
 */
export function approvalCard(
  options: ApprovalCardOptions = {},
): ComponentCreate[] {
  const withAlways = options.withAlways ?? true;
  const card = APPROVAL_VALUES.filter((v) => withAlways || v !== "always").map(
    (value) => {
      const [label, buttonStyle, capability, restricted] = APPROVAL[value];
      return button(`approval.${value}`, label, value, {
        style: buttonStyle,
        singleUse: true,
        requiredCapability: capability,
        restrictedTo: restricted,
        settings: options.settings,
      });
    },
  );
  checkComponents(card, options.settings);
  return card;
}

/**
 * Builds a choice question of 09 11.6.
 *
 * The run waits for the answer; whichever comes first, a use of the card or a follow-up, counts,
 * so every component is single use. With buttons, answer `i` gets the custom ID
 * `<customId>.<i>` and its value; with `asSelect` one select of exactly one choice.
 *
 * @param customId - ID of the select, or the prefix of the button IDs.
 * @param answers - At least one answer with distinct values.
 * @param options - `asSelect`, the placeholder, who may answer and the settings.
 * @returns The components of the card.
 * @throws `RangeError` for no answers, a broken rule of `component.v1` or more components than
 *   `component.max_per_message`.
 *
 * @example
 * ```ts
 * questionCard("when", [{ label: "Friday", value: "fri" }, { label: "Monday", value: "mon" }]);
 * ```
 */
export function questionCard(
  customId: string,
  answers: readonly SelectOption[],
  options: QuestionCardOptions = {},
): ComponentCreate[] {
  const settings = options.settings ?? DEFAULTS;
  const restrictedTo = options.restrictedTo ?? "run_starter";
  if (answers.length === 0)
    throw new RangeError("answers: at least one answer is required");
  selectOptions(answers, "answers");
  let card: ComponentCreate[];
  if (options.asSelect) {
    card = [
      select(customId, answers, {
        placeholder: options.placeholder,
        minValues: 1,
        maxValues: 1,
        singleUse: true,
        restrictedTo,
        settings,
      }),
    ];
  } else {
    if (answers.length > settings.componentMaxPerMessage)
      throw new RangeError(
        "components: more than component.max_per_message components",
      );
    card = answers.map((answer, index) =>
      button(`${customId}.${index}`, answer.label, answer.value, {
        singleUse: true,
        restrictedTo,
        settings,
      }),
    );
  }
  checkComponents(card, settings);
  return card;
}

/**
 * Holds the components of one message against the limits.
 *
 * @param list - The components of one message.
 * @param settings - Source of `component.max_per_message` and `component.custom_id_max_chars`.
 * @throws `RangeError` for more than `component.max_per_message` components, a repeated custom
 *   ID, or a custom ID outside its rules.
 */
export function checkComponents(
  list: readonly ComponentCreate[],
  settings: SdkSettings = DEFAULTS,
): void {
  if (list.length > settings.componentMaxPerMessage)
    throw new RangeError(
      "components: more than component.max_per_message components",
    );
  const seen = new Set<string>();
  list.forEach((item, index) => {
    customId(item.custom_id, settings, `components[${index}].custom_id`);
    if (seen.has(item.custom_id))
      throw new RangeError(
        `components: custom ID of components[${index}] repeated`,
      );
    seen.add(item.custom_id);
  });
}

/**
 * Builds the `MessageCreate` body of a card: `kind: card`, a new `client_msg_id` (UUIDv7) and the
 * checked components. Use it where a message is posted without the outbox (for example from a
 * browser); an adapter sends cards through `Outbox.sendMessage` with `kind: "card"`, which stores
 * the ID before the first send.
 *
 * @param contentMd - The Markdown text shown with the card.
 * @param list - The components, for example of {@link approvalCard}.
 * @param options - The run, the message it answers and the settings.
 * @returns The request body of `createMessage`.
 * @throws `RangeError` for no components or a card outside the limits of
 *   {@link checkComponents}.
 *
 * @example
 * ```ts
 * const body = cardMessage("Allow the refund?", approvalCard(), { runId });
 * await api.POST("/threads/{thread_id}/messages", { params: { path: { thread_id } }, body });
 * ```
 */
export function cardMessage(
  contentMd: string,
  list: readonly ComponentCreate[],
  options: CardMessageOptions = {},
): MessageCreate {
  if (list.length === 0)
    throw new RangeError("components: a card needs at least one component");
  checkComponents(list, options.settings);
  const body: MessageCreate = {
    client_msg_id: uuidv7(),
    kind: "card",
    content_md: contentMd,
    components: [...list],
  };
  if (options.runId !== undefined) body.run_id = options.runId;
  if (options.replyToMessageId !== undefined)
    body.reply_to_message_id = options.replyToMessageId;
  return body;
}

/** The generated component with the optional members that were given. */
function component(
  kind: ComponentType,
  id: string,
  definition: Record<string, unknown>,
  options: ComponentOptions,
): ComponentCreate {
  const settings = options.settings ?? DEFAULTS;
  const out: ComponentCreate = {
    custom_id: customId(id, settings, "custom_id"),
    type: kind,
    definition,
  };
  if (options.singleUse !== undefined) {
    if (typeof options.singleUse !== "boolean")
      throw new RangeError("single_use: must be a boolean");
    out.single_use = options.singleUse;
  }
  if (options.requiredCapability !== undefined)
    out.required_capability = member(
      CAPABILITIES,
      options.requiredCapability,
      "required_capability",
    );
  if (options.restrictedTo !== undefined)
    out.restricted_to = member(
      RESTRICTIONS,
      options.restrictedTo,
      "restricted_to",
    );
  if (options.expiresAt !== undefined)
    out.expires_at = timestamp(options.expiresAt);
  return out;
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

/** An RFC 3339 time in UTC of Unix milliseconds or a `Date`. */
function timestamp(value: unknown): string {
  const ms =
    value instanceof Date
      ? value.getTime()
      : typeof value === "number"
        ? value
        : Number.NaN;
  if (!Number.isFinite(ms))
    throw new RangeError("expires_at: must be a valid time");
  return new Date(ms).toISOString();
}

/** A custom ID: a display-safe text of at most `component.custom_id_max_chars` characters. */
function customId(
  value: unknown,
  settings: SdkSettings,
  field: string,
): string {
  const checked = text(value, field);
  // Characters, as the server counts them, not UTF-16 code units.
  if ([...checked].length > settings.componentCustomIdMaxChars)
    throw new RangeError(`${field}: longer than component.custom_id_max_chars`);
  return checked;
}

/** A non-empty display text without control or bidi characters (SEC-IN-4). */
function text(value: unknown, field: string): string {
  if (typeof value !== "string" || value === "")
    throw new RangeError(`${field}: must be a non-empty string`);
  if (UNSAFE_TEXT.test(value))
    throw new RangeError(`${field}: contains a control or bidi character`);
  return value;
}

/** A style of {@link BUTTON_STYLES}. */
function style(value: unknown): ButtonStyle {
  if (!(BUTTON_STYLES as readonly unknown[]).includes(value))
    throw new RangeError("style: not one of the design system roles");
  return value as ButtonStyle;
}

/** An integer that is not a boolean. */
function integer(value: unknown, field: string): number {
  if (typeof value !== "number" || !Number.isInteger(value))
    throw new RangeError(`${field}: must be an integer`);
  return value;
}

/** Options as JSON: at least one, distinct values. */
function selectOptions(
  list: readonly SelectOption[],
  field: string,
): Record<string, string>[] {
  if (list.length === 0)
    throw new RangeError(`${field}: at least one option is required`);
  const values = new Set<string>();
  return list.map((item, index) => {
    const where = `${field}[${index}]`;
    const out: Record<string, string> = {
      label: text(item.label, `${where}.label`),
      value: text(item.value, `${where}.value`),
    };
    if (item.description !== undefined)
      out.description = text(item.description, `${where}.description`);
    if (values.has(item.value))
      throw new RangeError(`${where}.value: repeated`);
    values.add(item.value);
    return out;
  });
}

/** One form field as JSON. */
function formField(item: FormField, where: string): Record<string, unknown> {
  if (typeof item.name !== "string" || !FIELD_NAME.test(item.name))
    throw new RangeError(`${where}.name: must match [a-z][a-z0-9_]*`);
  if (!(FORM_FIELD_TYPES as readonly unknown[]).includes(item.type))
    throw new RangeError(`${where}.type: not one of the field types`);
  const out: Record<string, unknown> = {
    name: item.name,
    label: text(item.label, `${where}.label`),
    type: item.type,
  };
  const required = item.required ?? false;
  if (typeof required !== "boolean")
    throw new RangeError(`${where}.required: must be a boolean`);
  out.required = required;
  if (item.maxLength !== undefined) {
    if (!LENGTH_TYPES.has(item.type))
      throw new RangeError(`${where}.max_length: only for text and textarea`);
    if (integer(item.maxLength, `${where}.max_length`) < 1)
      throw new RangeError(`${where}.max_length: must be at least 1`);
    out.max_length = item.maxLength;
  }
  const choices = item.choices ?? [];
  if (item.type === "choice") {
    if (choices.length === 0)
      throw new RangeError(`${where}.choices: required for a choice field`);
    const checked = choices.map((c) => text(c, `${where}.choices`));
    if (new Set(checked).size !== checked.length)
      throw new RangeError(`${where}.choices: entries must be distinct`);
    out.choices = checked;
  } else if (choices.length > 0) {
    throw new RangeError(`${where}.choices: only for a choice field`);
  }
  return out;
}
