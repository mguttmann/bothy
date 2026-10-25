// SPDX-License-Identifier: Apache-2.0
/**
 * The heartbeat building block for `agent_heartbeat.v1` (docs/spec/09-agents.md 4.8; port of
 * `bothy_sdk/heartbeat.py`).
 *
 * {@link buildHeartbeat} returns the generated `Heartbeat` with exactly the members and `details`
 * keys the server's decoder accepts (M2-20, docs/DECISIONS.md D-M2-20-2, D-M2-29-13) and
 * refuses, before anything is sent, every value outside its form: names {@link NAME_FORM},
 * versions {@link VERSION_FORM}, codes {@link CODE_FORM}, no duplicates in lists. These forms
 * make paths, URLs, whitespace, control and bidi characters and most key formats
 * unrepresentable, so a heartbeat cannot carry a secret, content or a path. A refusal is a
 * `RangeError` that names the field and never the value.
 *
 * Browser-safe: no `node:` import.
 *
 * @module
 */

import type { components } from "../generated/openapi.js";

type Heartbeat = components["schemas"]["Heartbeat"];
type BillingEvidence = components["schemas"]["BillingEvidence"];
type BillingMode = components["schemas"]["BillingMode"];
type ToolClass = components["schemas"]["ToolClass"];
type ProjectReadiness = components["schemas"]["ProjectReadiness"];
type AgentRateLimitState = components["schemas"]["AgentRateLimitState"];

/** Form of `details.adapter.name` and `details.runtime.name`. */
export const NAME_FORM = "^[a-z0-9][a-z0-9._-]*$";

/** Form of `runtime_version` and `details.adapter.version`. */
export const VERSION_FORM = "^[0-9A-Za-z][0-9A-Za-z.+_-]*$";

/** Form of `billing_evidence.provider`, `billing_evidence.method` and each degraded reason. */
export const CODE_FORM = "^[a-z0-9][a-z0-9_.:-]*$";

/** The closed key set of `details`. */
export const DETAILS_KEYS = Object.freeze([
  "capabilities",
  "active_runs",
  "degraded_reasons",
  "adapter",
  "runtime",
  "protocol_version",
] as const);

/** The closed key set of `details.capabilities` (09 2.2). */
export const CAPABILITY_KEYS = Object.freeze([
  "followups",
  "components",
  "voice_input",
  "title",
  "files_out",
] as const);

/** The top-level members {@link buildHeartbeat} sets. */
export const HEARTBEAT_MEMBERS = Object.freeze([
  "state",
  "runtime_version",
  "reported_billing_mode",
  "billing_modes_supported",
  "billing_evidence",
  "reported_tool_class",
  "rate_limit",
  "details",
  "project_readiness",
] as const);

/** The options of {@link buildHeartbeat}; each maps to the member of the same name. */
export interface BuildHeartbeatOptions {
  /** `runtime_version`: version of the agent runtime, {@link VERSION_FORM}. */
  readonly runtimeVersion?: string | undefined;
  /**
   * `reported_billing_mode`: `personal` or `organization`; leave it out when the adapter
   * follows the configured mode per run.
   */
  readonly reportedBillingMode?: BillingMode | undefined;
  /** `billing_modes_supported`: distinct modes the adapter can prove. */
  readonly billingModesSupported?: readonly BillingMode[] | undefined;
  /**
   * `billing_evidence`: `key_type` (required), `provider` and `method` (codes); never a key value
   * or prefix.
   */
  readonly billingEvidence?: BillingEvidence | undefined;
  /** `reported_tool_class`: the tool class the runtime actually loaded. */
  readonly reportedToolClass?: ToolClass | undefined;
  /**
   * `rate_limit`: the rate-limit state of the runtime at its provider (M6-14), `state` (`ok`,
   * `near_limit`, `limited` or `unknown`) and optionally `resets_at` (an RFC 3339 time with a
   * time zone); never a quota, key or subscription name. Leaving it out clears the state the
   * server stored from an earlier heartbeat.
   */
  readonly rateLimit?: AgentRateLimitState | undefined;
  /**
   * The closed object `details`: `capabilities` (`followups` `steer` or `queue`; `components`,
   * `voice_input`, `title`, `files_out` booleans), `active_runs` (distinct run IDs),
   * `degraded_reasons` (distinct codes), `adapter` (`name` required, `version` optional),
   * `runtime` (`name` required) and `protocol_version` (integer, at least 1).
   */
  readonly details?: Readonly<Record<string, unknown>> | undefined;
  /** `project_readiness`: runners only, one entry per bound forum, distinct channels. */
  readonly projectReadiness?: readonly ProjectReadiness[] | undefined;
}

const OPTION_KEYS: ReadonlySet<string> = new Set([
  "runtimeVersion",
  "reportedBillingMode",
  "billingModesSupported",
  "billingEvidence",
  "reportedToolClass",
  "rateLimit",
  "details",
  "projectReadiness",
]);
// The records make tsc hold each vocabulary equal to the generated type.
const STATES: Readonly<Record<Heartbeat["state"], true>> = {
  online: true,
  degraded: true,
};
const BILLING_MODES: Readonly<Record<BillingMode, true>> = {
  personal: true,
  organization: true,
};
const TOOL_CLASSES: Readonly<Record<ToolClass, true>> = {
  chat: true,
  scoped: true,
  host: true,
};
const KEY_TYPES: Readonly<Record<BillingEvidence["key_type"], true>> = {
  api_key: true,
  oauth: true,
  cloud: true,
  none: true,
  unknown: true,
};
// The option names are camelCase identifiers; they are echoed only in that form.
const OPTION_NAME = /^[a-zA-Z][a-zA-Z0-9]*$/;
const FOLLOWUP_MODES: ReadonlySet<unknown> = new Set(["steer", "queue"]);
const EVIDENCE_KEYS = ["provider", "key_type", "method"] as const;
const RATE_LIMIT_KEYS = ["state", "resets_at"] as const;
const RATE_LIMIT_STATES: Readonly<Record<AgentRateLimitState["state"], true>> =
  {
    ok: true,
    near_limit: true,
    limited: true,
    unknown: true,
  };
// RFC 3339 with a time zone (Z or an offset), as the server parses `resets_at`.
const RFC3339 =
  /^\d{4}-\d{2}-\d{2}T\d{2}:\d{2}:\d{2}(\.\d+)?(Z|[+-]\d{2}:\d{2})$/;
const READINESS_KEYS = ["channel_id", "ready"] as const;
// `protocol_version` is a 32-bit integer on the server; a larger one is refused there.
const PROTOCOL_VERSION_MAX = 2 ** 31 - 1;
const UUID = /^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$/i;

// `$` of a JavaScript regex without the `m` flag matches only at the very end, so the forms of
// the specification match the whole value as written.
const NAME = new RegExp(NAME_FORM);
const VERSION = new RegExp(VERSION_FORM);
const CODE = new RegExp(CODE_FORM);

/**
 * Builds a heartbeat of schema `agent_heartbeat.v1`.
 *
 * Every heartbeat is the whole reported state: an option left out is "not reported" (09 4.8
 * "Full state"), except `projectReadiness`, where absent means "unchanged". The adapter fills
 * `details.active_runs` and sends the result as member `agent` of every gateway heartbeat, or
 * through `sendHeartbeat` with the polling source ({@link HeartbeatSender}).
 *
 * @param state - `online` (default) or `degraded`.
 * @param options - The other members.
 * @returns The generated `Heartbeat`.
 * @throws `RangeError` for an unknown key, a missing required member, a value of the wrong
 *   type, outside its vocabulary or form, or a duplicate. The message names the field, never
 *   the value.
 *
 * @example
 * ```ts
 * buildHeartbeat("online", {
 *   runtimeVersion: "2.3.1",
 *   billingEvidence: { key_type: "api_key", provider: "anthropic" },
 *   details: { adapter: { name: "example-adapter" }, protocol_version: 1 },
 * });
 * buildHeartbeat("online", { runtimeVersion: "/opt/runtime" });
 * // RangeError: runtime_version: does not match the version form
 * ```
 */
export function buildHeartbeat(
  state: Heartbeat["state"] = "online",
  options: BuildHeartbeatOptions = {},
): Heartbeat {
  closed(options, OPTION_KEYS, "options", OPTION_NAME);
  const beat: Heartbeat = { state: member(STATES, state, "state") };
  const o = options;
  if (o.runtimeVersion !== undefined)
    beat.runtime_version = form(
      o.runtimeVersion,
      VERSION,
      "version",
      "runtime_version",
    );
  if (o.reportedBillingMode !== undefined)
    beat.reported_billing_mode = member(
      BILLING_MODES,
      o.reportedBillingMode,
      "reported_billing_mode",
    );
  if (o.billingModesSupported !== undefined) {
    const modes = list(o.billingModesSupported, "billing_modes_supported").map(
      (m) => member(BILLING_MODES, m, "billing_modes_supported"),
    );
    distinct(modes, "billing_modes_supported");
    beat.billing_modes_supported = modes;
  }
  if (o.billingEvidence !== undefined)
    beat.billing_evidence = evidence(o.billingEvidence);
  if (o.reportedToolClass !== undefined)
    beat.reported_tool_class = member(
      TOOL_CLASSES,
      o.reportedToolClass,
      "reported_tool_class",
    );
  if (o.rateLimit !== undefined) beat.rate_limit = rateLimit(o.rateLimit);
  if (o.details !== undefined) beat.details = details(o.details);
  if (o.projectReadiness !== undefined)
    beat.project_readiness = readiness(o.projectReadiness);
  return beat;
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

/** `value` when it is a string of `pattern`. */
function form(
  value: unknown,
  pattern: RegExp,
  name: string,
  field: string,
): string {
  if (typeof value !== "string")
    throw new RangeError(`${field}: must be a string`);
  if (!pattern.test(value))
    throw new RangeError(`${field}: does not match the ${name} form`);
  return value;
}

/** Refuses a list with a repeated entry. */
function distinct(values: readonly unknown[], field: string): void {
  if (new Set(values).size !== values.length)
    throw new RangeError(`${field}: entries must be distinct`);
}

/**
 * `value` as a plain object with no key outside `keys`.
 *
 * An unknown key is named only when it has the name form, because anything else could be the
 * very text a heartbeat must not carry.
 */
function closed(
  value: unknown,
  keys: Iterable<string>,
  field: string,
  nameForm: RegExp = NAME,
): Readonly<Record<string, unknown>> {
  if (typeof value !== "object" || value === null || Array.isArray(value))
    throw new RangeError(`${field}: must be an object`);
  const known = new Set(keys);
  for (const key of Object.keys(value))
    if (!known.has(key)) {
      if (nameForm.test(key))
        throw new RangeError(`${field}.${key}: unknown key`);
      throw new RangeError(`${field}: unknown key`);
    }
  return value as Readonly<Record<string, unknown>>;
}

/** `value` as an array; a string is no list of entries. */
function list(value: unknown, field: string): unknown[] {
  if (!Array.isArray(value)) throw new RangeError(`${field}: must be a list`);
  return [...(value as unknown[])];
}

/** `billing_evidence` with `key_type` from its vocabulary and codes. */
function evidence(value: unknown): BillingEvidence {
  const raw = closed(value, EVIDENCE_KEYS, "billing_evidence");
  if (!("key_type" in raw))
    throw new RangeError("billing_evidence.key_type: required");
  const out: BillingEvidence = {
    key_type: member(KEY_TYPES, raw.key_type, "billing_evidence.key_type"),
  };
  if ("provider" in raw)
    out.provider = form(
      raw.provider,
      CODE,
      "code",
      "billing_evidence.provider",
    );
  if ("method" in raw)
    out.method = form(raw.method, CODE, "code", "billing_evidence.method");
  return out;
}

/** `rate_limit` with `state` from its vocabulary and an RFC 3339 `resets_at`. */
function rateLimit(value: unknown): AgentRateLimitState {
  const raw = closed(value, RATE_LIMIT_KEYS, "rate_limit");
  if (!("state" in raw)) throw new RangeError("rate_limit.state: required");
  const out: AgentRateLimitState = {
    state: member(RATE_LIMIT_STATES, raw.state, "rate_limit.state"),
  };
  if ("resets_at" in raw) {
    const at = raw.resets_at;
    if (
      typeof at !== "string" ||
      !RFC3339.test(at) ||
      Number.isNaN(Date.parse(at))
    )
      throw new RangeError(
        "rate_limit.resets_at: must be an RFC 3339 time with a time zone",
      );
    out.resets_at = at;
  }
  return out;
}

/** The closed object `details` as JSON. */
function details(value: unknown): Record<string, unknown> {
  const raw = closed(value, DETAILS_KEYS, "details");
  const out: Record<string, unknown> = {};
  if ("capabilities" in raw) out.capabilities = capabilities(raw.capabilities);
  if ("active_runs" in raw) out.active_runs = runs(raw.active_runs);
  if ("degraded_reasons" in raw) {
    const field = "details.degraded_reasons";
    const reasons = list(raw.degraded_reasons, field).map((r) =>
      form(r, CODE, "code", field),
    );
    distinct(reasons, field);
    out.degraded_reasons = reasons;
  }
  if ("adapter" in raw) {
    const adapter = closed(raw.adapter, ["name", "version"], "details.adapter");
    const rendered: Record<string, string> = {
      name: name(adapter, "details.adapter.name"),
    };
    if ("version" in adapter)
      rendered.version = form(
        adapter.version,
        VERSION,
        "version",
        "details.adapter.version",
      );
    out.adapter = rendered;
  }
  if ("runtime" in raw) {
    const runtime = closed(raw.runtime, ["name"], "details.runtime");
    out.runtime = { name: name(runtime, "details.runtime.name") };
  }
  if ("protocol_version" in raw)
    out.protocol_version = protocolVersion(raw.protocol_version);
  return out;
}

/** The required member `name` in the name form. */
function name(value: Readonly<Record<string, unknown>>, field: string): string {
  if (!("name" in value)) throw new RangeError(`${field}: required`);
  return form(value.name, NAME, "name", field);
}

/** `details.capabilities` (09 2.2). */
function capabilities(value: unknown): Record<string, unknown> {
  const raw = closed(value, CAPABILITY_KEYS, "details.capabilities");
  const out: Record<string, unknown> = {};
  for (const key of CAPABILITY_KEYS) {
    if (!(key in raw)) continue;
    const field = `details.capabilities.${key}`;
    const entry = raw[key];
    if (key === "followups") {
      if (!FOLLOWUP_MODES.has(entry))
        throw new RangeError(`${field}: not one of the allowed values`);
    } else if (typeof entry !== "boolean") {
      throw new RangeError(`${field}: must be a boolean`);
    }
    out[key] = entry;
  }
  return out;
}

/** `details.active_runs`: distinct run IDs in canonical (lower-case) form. */
function runs(value: unknown): string[] {
  const field = "details.active_runs";
  const out = list(value, field).map((entry) => {
    if (typeof entry !== "string" || !UUID.test(entry))
      throw new RangeError(`${field}: entries must be run IDs`);
    return entry.toLowerCase();
  });
  distinct(out, field);
  return out;
}

/** `details.protocol_version`: an integer of at least 1. */
function protocolVersion(value: unknown): number {
  if (typeof value !== "number" || !Number.isInteger(value))
    throw new RangeError("details.protocol_version: must be an integer");
  if (value < 1 || value > PROTOCOL_VERSION_MAX)
    throw new RangeError("details.protocol_version: out of range");
  return value;
}

/** `project_readiness`: closed entries with distinct channels. */
function readiness(value: unknown): ProjectReadiness[] {
  const field = "project_readiness";
  const out = list(value, field).map((entry) => {
    const raw = closed(entry, READINESS_KEYS, field);
    if (typeof raw.channel_id !== "string" || !UUID.test(raw.channel_id))
      throw new RangeError(`${field}: entries need a channel_id`);
    if (typeof raw.ready !== "boolean")
      throw new RangeError(`${field}: ready must be a boolean`);
    return { channel_id: raw.channel_id.toLowerCase(), ready: raw.ready };
  });
  distinct(
    out.map((r) => r.channel_id),
    field,
  );
  return out;
}
