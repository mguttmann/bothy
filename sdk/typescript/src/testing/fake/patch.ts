// SPDX-License-Identifier: Apache-2.0
/**
 * Run patches of the FakeServer: the path table of docs/spec/09-agenten.md 5.3 (private; port
 * of `bothy_sdk/testing/_fake_patch.py`).
 *
 * Static checks run over every op first (all field errors in one `400 validation_failed`),
 * then the ops apply in order on a copy of the live box (the first failure is the only error).
 * Set semantics: `add` and `replace` set a member also when it is absent, `remove` of an absent
 * member changes nothing. Follow-up ops always miss their target, as in the server until M2-11.
 *
 * @module
 */

import {
  canonicalUuid,
  FakeProblem,
  isPlainObject,
  isUuid7,
  type FieldErrorBody,
} from "./base.js";

const FORBIDDEN = "this path is not patchable with this operation";
const INT_METRICS = new Set([
  "tokens_in",
  "tokens_out",
  "tokens_cache_read",
  "tokens_cache_write",
  "tokens_cache_write_1h",
  "subagents",
  "background_jobs",
  "api_retries",
]);
const STEP_KINDS = new Set([
  "tool_call",
  "status",
  "subagent",
  "workflow",
  "background_job",
]);
const STEP_STATUS = new Set([
  "pending",
  "running",
  "completed",
  "failed",
  "cancelled",
]);
const PLAN_STATUS = new Set(["pending", "in_progress", "completed"]);
const STEP_MEMBERS = new Set([
  "id",
  "kind",
  "name",
  "title",
  "status",
  "detail",
  "parent_step_id",
  "started_at",
  "finished_at",
]);
const BIDI = new Set([
  "؜",
  "‎",
  "‏",
  "‪",
  "‫",
  "‬",
  "‭",
  "‮",
  "⁦",
  "⁧",
  "⁨",
  "⁩",
]);
const CURRENCY = /^[A-Z]{3}$/;
// An RFC 3339 date-time as Python's `datetime.fromisoformat` reads it, with the `T` separator.
const RFC3339 =
  /^\d{4}-\d{2}-\d{2}T\d{2}:\d{2}(:\d{2}(\.\d{1,6})?)?(Z|[+-]\d{2}:\d{2})?$/;

/** The kind of a patchable path. */
type PathKind =
  | "text"
  | "excerpt"
  | "money"
  | "plan"
  | "retry"
  | "metric"
  | "step"
  | "step_member"
  | "followup";

/** One op after the static checks. */
export interface ParsedOp {
  /** `add`, `replace` or `remove`. */
  readonly op: "add" | "replace" | "remove";
  /** The decoded pointer tokens. */
  readonly tokens: readonly string[];
  /** The kind of the path. */
  readonly kind: PathKind;
  /** The value; `undefined` for `remove`. */
  readonly value: unknown;
}

/**
 * Returns the code of a display text violation, or `undefined`.
 *
 * @param value - The candidate text.
 * @returns `invalid_type`, `control_character`, `bidi_control` or `undefined`.
 */
export function textError(value: unknown): string | undefined {
  if (typeof value !== "string") return "invalid_type";
  for (const ch of value) {
    const code = ch.codePointAt(0) ?? 0;
    if (code < 0x20 || (code >= 0x7f && code <= 0x9f))
      return "control_character";
    if (BIDI.has(ch)) return "bidi_control";
  }
  return undefined;
}

function pointer(path: string): string[] | undefined {
  if (path === "") return [];
  if (!path.startsWith("/")) return undefined;
  const tokens: string[] = [];
  for (const raw of path.slice(1).split("/")) {
    if (/~(?![01])/.test(raw)) return undefined;
    tokens.push(raw.replaceAll("~1", "/").replaceAll("~0", "~"));
  }
  return tokens;
}

const SIMPLE: readonly [readonly string[], PathKind, readonly string[]][] = [
  [["status_line"], "text", ["add", "replace", "remove"]],
  [["last_text_excerpt"], "excerpt", ["add", "replace"]],
  [["model"], "text", ["replace"]],
  [["cost"], "money", ["replace"]],
  [["plan"], "plan", ["add", "replace", "remove"]],
  [["metrics", "last_api_retry"], "retry", ["add", "replace", "remove"]],
];

// The path kind for `op` on `tokens`, or undefined when forbidden.
function route(op: string, tokens: readonly string[]): PathKind | undefined {
  const found = SIMPLE.find(
    ([path]) =>
      path.length === tokens.length && path.every((t, i) => t === tokens[i]),
  );
  if (found !== undefined) return found[2].includes(op) ? found[1] : undefined;
  if (tokens.length === 2 && tokens[0] === "metrics") {
    if (INT_METRICS.has(tokens[1] ?? "") || tokens[1] === "context_pct")
      return op === "add" || op === "replace" ? "metric" : undefined;
    return undefined;
  }
  if (tokens[0] === "steps") {
    if (tokens.length === 2 && op === "add") return "step";
    if (
      tokens.length === 3 &&
      op === "replace" &&
      ["status", "title", "detail", "finished_at"].includes(tokens[2] ?? "")
    )
      return "step_member";
    return undefined;
  }
  if (tokens.length === 3 && tokens[0] === "followups" && tokens[2] === "state")
    return op === "replace" ? "followup" : undefined;
  return undefined;
}

function firstNull(value: unknown, at: string): string | undefined {
  if (value === null) return at;
  if (Array.isArray(value)) {
    for (const [i, item] of value.entries()) {
      const found = firstNull(item, `${at}/${i}`);
      if (found) return found;
    }
  } else if (isPlainObject(value)) {
    for (const key of Object.keys(value).sort()) {
      const found = firstNull(value[key], `${at}/${key}`);
      if (found) return found;
    }
  }
  return undefined;
}

function rfc3339(value: unknown): boolean {
  return (
    typeof value === "string" &&
    RFC3339.test(value) &&
    !Number.isNaN(Date.parse(value))
  );
}

function integral(value: unknown): value is number {
  return (
    typeof value === "number" &&
    Number.isFinite(value) &&
    Number.isInteger(value)
  );
}

function checkValue(
  kind: PathKind,
  member: string,
  value: unknown,
  at: string,
): FieldErrorBody[] {
  const errs: FieldErrorBody[] = [];
  const err = (pointerText: string, code: string): FieldErrorBody[] => {
    errs.push({ pointer: pointerText, code });
    return errs;
  };
  switch (kind) {
    case "text": {
      const code = textError(value);
      return code ? err(at, code) : errs;
    }
    case "excerpt":
      return typeof value === "string" ? errs : err(at, "invalid_type");
    case "metric": {
      if (typeof value !== "number") return err(at, "invalid_type");
      if (member === "context_pct") {
        if (value < 0) return err(at, "too_small");
        return value > 100 ? err(at, "too_large") : errs;
      }
      if (!integral(value)) return err(at, "invalid_type");
      if (value < 0) return err(at, "too_small");
      return value > Number.MAX_SAFE_INTEGER ? err(at, "too_large") : errs;
    }
    case "retry": {
      if (!isPlainObject(value)) return err(at, "invalid_type");
      const known = ["attempt", "max_retries", "retry_delay_ms", "error"];
      for (const name of Object.keys(value).sort())
        if (!known.includes(name)) err(`${at}/${name}`, "unknown_member");
      if (!("attempt" in value)) err(`${at}/attempt`, "required");
      for (const name of ["attempt", "max_retries", "retry_delay_ms"])
        if (name in value && !integral(value[name]))
          err(`${at}/${name}`, "invalid_type");
      const code = "error" in value ? textError(value.error) : undefined;
      if (code) err(`${at}/error`, code);
      return errs;
    }
    case "money": {
      if (!isPlainObject(value)) return err(at, "invalid_type");
      const known = ["amount_micros", "currency", "is_estimate"];
      for (const name of Object.keys(value).sort())
        if (!known.includes(name)) err(`${at}/${name}`, "unknown_member");
      for (const name of known)
        if (!(name in value)) err(`${at}/${name}`, "required");
      if (
        "amount_micros" in value &&
        (!integral(value.amount_micros) || value.amount_micros < 0)
      )
        err(`${at}/amount_micros`, "invalid_type");
      if ("currency" in value && !CURRENCY.test(String(value.currency)))
        err(`${at}/currency`, "pattern_mismatch");
      if ("is_estimate" in value && value.is_estimate !== true)
        err(`${at}/is_estimate`, "invalid");
      return errs;
    }
    case "plan": {
      const items = isPlainObject(value) ? value.items : undefined;
      if (!Array.isArray(items)) return err(`${at}/items`, "invalid_type");
      const seen: unknown[] = [];
      for (const [j, item] of items.entries()) {
        const ip = `${at}/items/${j}`;
        if (!isPlainObject(item)) {
          err(ip, "invalid_type");
          continue;
        }
        for (const name of ["id", "text", "status"])
          if (!(name in item)) err(`${ip}/${name}`, "required");
        if (seen.includes(item.id)) err(`${ip}/id`, "invalid");
        seen.push(item.id);
        const code = "text" in item ? textError(item.text) : undefined;
        if (code) err(`${ip}/text`, code);
        if ("status" in item && !PLAN_STATUS.has(item.status as string))
          err(`${ip}/status`, "invalid_enum");
      }
      return errs;
    }
    case "step":
      return checkStep(value, at, member);
    case "step_member": {
      if (member === "status" && !STEP_STATUS.has(value as string))
        return err(at, "invalid_enum");
      const code = member === "title" ? textError(value) : undefined;
      if (code) return err(at, code);
      if (member === "detail" && !isPlainObject(value))
        return err(at, "invalid_type");
      if (member === "finished_at" && !rfc3339(value))
        return err(at, "invalid_format");
      return errs;
    }
    case "followup":
      return value === "delivered" || value === "absorbed"
        ? errs
        : err(at, "invalid_enum");
  }
}

function checkStep(
  value: unknown,
  at: string,
  stepId: string,
): FieldErrorBody[] {
  if (!isPlainObject(value)) return [{ pointer: at, code: "invalid_type" }];
  const errs: FieldErrorBody[] = [];
  if ("ordinal" in value)
    errs.push({ pointer: `${at}/ordinal`, code: "patch_path_forbidden" });
  for (const name of Object.keys(value).sort())
    if (!STEP_MEMBERS.has(name) && name !== "ordinal")
      errs.push({ pointer: `${at}/${name}`, code: "unknown_member" });
  if ("id" in value && value.id !== stepId)
    errs.push({ pointer: `${at}/id`, code: "invalid" });
  if (!STEP_KINDS.has(value.kind as string))
    errs.push({
      pointer: `${at}/kind`,
      code: "kind" in value ? "invalid_enum" : "required",
    });
  if (!("name" in value))
    errs.push({ pointer: `${at}/name`, code: "required" });
  for (const name of ["name", "title"]) {
    const code = name in value ? textError(value[name]) : undefined;
    if (code) errs.push({ pointer: `${at}/${name}`, code });
  }
  if ("status" in value && !STEP_STATUS.has(value.status as string))
    errs.push({ pointer: `${at}/status`, code: "invalid_enum" });
  if ("detail" in value && !isPlainObject(value.detail))
    errs.push({ pointer: `${at}/detail`, code: "invalid_type" });
  const parent = value.parent_step_id;
  if (parent !== undefined && !(typeof parent === "string" && isUuid7(parent)))
    errs.push({ pointer: `${at}/parent_step_id`, code: "invalid_format" });
  for (const name of ["started_at", "finished_at"])
    if (name in value && !rfc3339(value[name]))
      errs.push({ pointer: `${at}/${name}`, code: "invalid_format" });
  return errs;
}

/**
 * Runs the static checks of every op.
 *
 * @param ops - The `ops` member of the request.
 * @returns Per op: the op, the pointer tokens, the path kind and the value.
 * @throws {@link FakeProblem} `400 validation_failed` with every field error in op order.
 */
export function parseOps(ops: unknown): ParsedOp[] {
  if (!Array.isArray(ops))
    throw new FakeProblem("validation_failed", {
      errors: [{ pointer: "/ops", code: "invalid_type" }],
    });
  const errors: FieldErrorBody[] = [];
  const parsed: ParsedOp[] = [];
  for (const [i, raw] of ops.entries()) {
    const at = `/ops/${i}`;
    if (!isPlainObject(raw) || typeof raw.path !== "string") {
      errors.push({ pointer: at, code: "invalid_type" });
      continue;
    }
    const op = raw.op;
    if (op !== "add" && op !== "replace" && op !== "remove") {
      errors.push({ pointer: `${at}/op`, code: "invalid_enum" });
      continue;
    }
    const tokens = pointer(raw.path);
    if (tokens === undefined) {
      errors.push({ pointer: `${at}/path`, code: "invalid_format" });
      continue;
    }
    if (
      tokens.length >= 2 &&
      ["steps", "followups"].includes(tokens[0] ?? "") &&
      !isUuid7(tokens[1] ?? "")
    ) {
      errors.push({ pointer: `${at}/path`, code: "invalid_format" });
      continue;
    }
    const kind = route(op, tokens);
    if (kind === undefined) {
      errors.push({
        pointer: at,
        code: "patch_path_forbidden",
        detail: FORBIDDEN,
      });
      continue;
    }
    if (op === "remove") {
      parsed.push({ op, tokens, kind, value: undefined });
      continue;
    }
    if (!("value" in raw)) {
      errors.push({ pointer: `${at}/value`, code: "required" });
      continue;
    }
    const value = raw.value;
    const nullAt = firstNull(value, `${at}/value`);
    if (nullAt) {
      errors.push({ pointer: nullAt, code: "invalid_type" });
      continue;
    }
    const member =
      (kind === "metric" || kind === "step" ? tokens[1] : tokens.at(-1)) ?? "";
    const found = checkValue(kind, member, value, `${at}/value`);
    if (found.length) {
      errors.push(...found);
      continue;
    }
    parsed.push({ op, tokens, kind, value });
  }
  if (errors.length)
    throw new FakeProblem("validation_failed", {
      detail: "the patch has invalid operations",
      errors,
    });
  return parsed;
}

/** The inputs of {@link applyOps} besides box and ops. */
export interface ApplyOptions {
  /** Returns the wall time as the server writes it. */
  readonly nowIso: () => string;
  /** The longest `last_text_excerpt` kept, in characters. */
  readonly excerptMax: number;
}

/**
 * Applies checked ops to a copy of the live box.
 *
 * @param live - The live box of the run.
 * @param parsed - The ops of {@link parseOps}.
 * @param options - The time and the excerpt limit.
 * @returns The new live box.
 * @throws {@link FakeProblem} `400 validation_failed` with the first dynamic error.
 */
export function applyOps(
  live: Readonly<Record<string, unknown>>,
  parsed: readonly ParsedOp[],
  options: ApplyOptions,
): Record<string, unknown> {
  const box = structuredClone(live) as Record<string, unknown>;
  box.steps ??= {};
  const steps = box.steps as Record<string, Record<string, unknown>>;
  for (const [i, { op, tokens, kind, value }] of parsed.entries()) {
    const fail = (pointerText: string, code: string): FakeProblem =>
      new FakeProblem("validation_failed", {
        detail: "the patch does not apply to the run",
        errors: [{ pointer: pointerText.replace("{i}", String(i)), code }],
      });
    const head = tokens[0] ?? "";
    if (kind === "text" || kind === "excerpt") {
      let text = op === "remove" ? "" : (value as string);
      if (kind === "excerpt" && text)
        text =
          options.excerptMax > 0
            ? [...text].slice(-options.excerptMax).join("")
            : "";
      if (text) box[head] = text;
      else delete box[head];
    } else if (kind === "money") {
      const money = value as Record<string, unknown>;
      box.cost = {
        amount_micros: Math.trunc(money.amount_micros as number),
        currency: money.currency,
        is_estimate: true,
      };
    } else if (kind === "plan") {
      const items = (value as { items?: unknown[] } | undefined)?.items;
      if (op === "remove" || !items?.length) delete box.plan;
      else box.plan = { items };
    } else if (kind === "metric" || kind === "retry") {
      box.metrics ??= {};
      const metrics = box.metrics as Record<string, unknown>;
      if (op === "remove") delete metrics[tokens[1] ?? ""];
      else metrics[tokens[1] ?? ""] = value;
    } else if (kind === "step") {
      const stepId = canonicalUuid(tokens[1] ?? "") ?? "";
      if (stepId in steps) throw fail("/ops/{i}", "patch_target_exists");
      const spec = value as Record<string, unknown>;
      const parent =
        typeof spec.parent_step_id === "string"
          ? canonicalUuid(spec.parent_step_id)
          : undefined;
      if (parent !== undefined && !(parent in steps))
        throw fail("/ops/{i}/value/parent_step_id", "patch_target_missing");
      const ordinals = Object.values(steps).map((s) => s.ordinal as number);
      const step: Record<string, unknown> = {
        id: stepId,
        ordinal: Math.max(0, ...ordinals) + 1,
        kind: spec.kind,
        name: spec.name,
        title: spec.title ?? "",
        status: spec.status ?? "running",
        detail: spec.detail ?? {},
        started_at: spec.started_at ?? options.nowIso(),
      };
      if (parent !== undefined) step.parent_step_id = parent;
      if ("finished_at" in spec) step.finished_at = spec.finished_at;
      steps[stepId] = step;
    } else if (kind === "step_member") {
      const stepId = canonicalUuid(tokens[1] ?? "") ?? "";
      const step = steps[stepId];
      if (step === undefined) throw fail("/ops/{i}", "patch_target_missing");
      step[tokens[2] ?? ""] = value;
    } else {
      // followup: the server has no follow-up rows yet (M2-11)
      throw fail("/ops/{i}", "patch_target_missing");
    }
  }
  return box;
}
