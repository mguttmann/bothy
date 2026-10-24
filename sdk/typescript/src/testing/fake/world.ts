// SPDX-License-Identifier: Apache-2.0
/**
 * The FakeServer's data and run rules (private; port of `bothy_sdk/testing/_fake_world.py`).
 *
 * One world after docs/spec/07-permissions.md "Examples": owner Robin, Alex who triggers, the agent
 * Lotse that operates the channel Allgemein of space Support, and the agent Texter. The run
 * rules follow the server code of M2-08 to M2-10 (docs/spec/09-agents.md 4.5, 4.5.1, 5.1 and
 * 5.3, docs/spec/04-sync-protocol.md 11.2 and 15.2 to 15.5): fresh tokens per delivery, the
 * start verdicts `superseded`, `expired` and `invalid`, consumed tokens, `already_started`,
 * mismatches that fail the run, renewals with `renewed_from_jti` and patches with
 * `base_version`. Every body is built with the generated types, so `tsc` holds its shape.
 *
 * @module
 */

import type { components } from "../../generated/openapi.js";
import { uuidv7 } from "../../ids.js";
import type { FakeClock } from "../clock.js";
import {
  byCodeUnit,
  canonicalUuid,
  FakeProblem,
  fieldError,
  iso,
  isPlainObject,
  type Timings,
} from "./base.js";
import type { Envelope, LogEntry } from "./events.js";
import * as fixtures from "./fixtures.js";
import { applyOps, parseOps, textError } from "./patch.js";
import {
  Issuer,
  TRIGGER_OBJECT,
  type InvocationClaims,
  type TriggerKind,
  type UnsignedClaims,
} from "./tokens.js";

type S = components["schemas"];
type RunDoc = S["Run"];
type RunSummaryDoc = S["RunSummary"];
type RunContextDoc = S["RunContext"];
type ContextItem = S["RunContextMessage"];
type MessageDoc = S["Message"];
type ComponentDoc = S["Component"];
type InteractionDoc = S["Interaction"];
type CommandDoc = S["Command"];
type PrincipalRefDoc = S["PrincipalRef"];
type RunStateName = S["RunState"];
type BillingModeName = S["BillingMode"];
type ToolClassName = S["ToolClass"];

/** Run states in which a run is still active. */
export const ACTIVE: ReadonlySet<string> = new Set([
  "queued",
  "running",
  "waiting_input",
]);
/** Run states in which a run has ended. */
export const ENDED: ReadonlySet<string> = new Set([
  "completed",
  "failed",
  "cancelled",
]);
/** Events whose deliveries carry an invocation token. */
export const TOKEN_EVENTS: ReadonlySet<string> = new Set([
  "agent.invocation",
  "agent.followup",
  "interaction.created",
  "command.invoked",
]);
const KEEP_FAILED = new Set([
  "billing_mode_mismatch",
  "tool_class_mismatch",
  "invalid_invocation_token",
  "runtime_error",
  "adapter_error",
  "runtime_not_authenticated",
]);
const CANCEL_REASONS = new Set([
  "user_cancelled",
  "access_revoked",
  "agent_disabled",
  "channel_read_only",
  "user_disabled",
]);
const TOOL_RANK: Readonly<Record<string, number>> = { chat: 0, scoped: 1 };
const CONSUMER = /^[a-z0-9][a-z0-9_-]*$/;
const COMMAND_NAME = /^[a-z0-9][a-z0-9_-]*$/;
// The server's limit of `last_text_excerpt` in characters (09 5.3), as the Python fake plays it.
const EXCERPT_MAX = 500;

// Every member of a schema, held complete by tsc: a missing member makes the argument `never`.
function members<T>() {
  return <const K extends readonly (keyof T & string)[]>(
    keys: K & ([Exclude<keyof T, K[number]>] extends [never] ? unknown : never),
  ): K => keys;
}

// Members the FakeServer produces itself; a fixture contributes only members beyond these.
const RUN_MEMBERS = members<RunDoc>()([
  "id",
  "thread_id",
  "channel_id",
  "agent_id",
  "agent_session_id",
  "trigger_kind",
  "triggering_message_id",
  "triggering_command_id",
  "triggering_interaction_id",
  "triggered_by",
  "state",
  "state_reason",
  "version",
  "billing_mode_expected",
  "billing_mode_used",
  "model",
  "status_line",
  "last_text_excerpt",
  "metrics",
  "cost",
  "summary",
  "steps",
  "followups",
  "queued_at",
  "started_at",
  "finished_at",
  "duration_ms",
  "plan",
]);
const CONTEXT_MEMBERS = members<RunContextDoc>()([
  "run_id",
  "trigger_message",
  "thread_context",
  "channel_context",
  "context_floor_seq",
  "truncated_before_seq",
]);
const SUMMARY_MEMBERS = members<RunSummaryDoc>()([
  "id",
  "thread_id",
  "channel_id",
  "agent_id",
  "triggered_by",
  "state",
  "state_reason",
  "version",
  "billing_mode_expected",
  "billing_mode_used",
  "status_line",
  "summary",
  "cost",
  "step_count",
  "duration_ms",
  "queued_at",
  "started_at",
  "finished_at",
]);

/**
 * Returns the rank of a tool class (`chat` < `scoped` < everything else).
 *
 * @param value - The tool class.
 * @returns The rank.
 */
export function toolRank(value: string): number {
  return TOOL_RANK[value] ?? 2;
}

/**
 * Tells whether `name` is a valid consumer name.
 *
 * @param name - The candidate.
 * @returns Whether it is valid.
 */
export function consumerOk(name: string): boolean {
  return CONSUMER.test(name);
}

/** A principal of the world. */
export class Person {
  /**
   * Creates the principal.
   *
   * @param id - The principal ID.
   * @param kind - `user` or `agent`.
   * @param handle - The handle.
   * @param displayName - The display name.
   */
  constructor(
    readonly id: string,
    readonly kind: "user" | "agent",
    readonly handle: string,
    readonly displayName: string,
  ) {}

  /**
   * Returns the `PrincipalRef`.
   *
   * @returns The reference.
   */
  ref(): PrincipalRefDoc {
    return {
      id: this.id,
      kind: this.kind,
      handle: this.handle,
      display_name: this.displayName,
    };
  }
}

/** The run document's live members the patches change. */
export interface LiveBox {
  /** The metrics. */
  metrics: S["RunMetrics"];
  /** The steps by ID. */
  steps: Record<string, S["RunStep"]>;
  /** The estimated cost. */
  cost?: S["Money"];
  /** The model. */
  model?: string;
  /** The status line. */
  status_line?: string;
  /** The last text excerpt. */
  last_text_excerpt?: string;
  /** The plan. */
  plan?: S["RunPlan"];
}

/** A run row with the columns the rules need. */
export class RunRecord {
  /** The state. */
  state: RunStateName = "queued";
  /** The state reason. */
  stateReason: string | undefined;
  /** The version. */
  version = 1;
  /** The `jti` the next start must present. */
  invocationJti: string = uuidv7();
  /** The `jti` that started the run. */
  startedJti: string | undefined;
  /** The `jti` the current token was renewed from. */
  renewedFromJti: string | undefined;
  /** The billing mode the start reported. */
  billingModeUsed: BillingModeName | undefined;
  /** The summary of `finishRun`. */
  summary: string | undefined;
  /** When the run first started, Unix milliseconds. */
  startedAt: number | undefined;
  /** When the run ended, Unix milliseconds. */
  finishedAt: number | undefined;
  /** How often the run went to `running`. */
  starts = 0;
  /** The live box. */
  live: LiveBox = { metrics: {}, steps: {} };
  /** Every token issued for the run by `jti`. */
  readonly issued = new Map<string, InvocationClaims>();
  /** The `jti` of the newest token. */
  lastJti: string | undefined;
  /** Whether a cancel waits for the adapter. */
  cancelRequested = false;

  /**
   * Creates the row.
   *
   * @param id - The run.
   * @param threadId - The thread.
   * @param triggeredBy - The person who triggered it.
   * @param triggeringMessageId - The triggering message.
   * @param queuedAt - When it was queued, Unix milliseconds.
   * @param billingModeExpected - The configured billing mode.
   */
  constructor(
    readonly id: string,
    readonly threadId: string,
    readonly triggeredBy: string,
    readonly triggeringMessageId: string,
    readonly queuedAt: number,
    public billingModeExpected: BillingModeName,
  ) {}
}

/** A component with the message it belongs to. */
type StoredComponent = ComponentDoc & { message_id: string };

/** An autocomplete request waiting for its answer. */
interface PendingAutocomplete {
  /** When it expires, Unix milliseconds. */
  readonly expires: number;
  /** The event payload. */
  readonly payload: S["CommandAutocompleteRequestedPayload"];
}

/** The members of one {@link World.envelope} call besides type and payload. */
interface EnvelopeOptions {
  readonly resource: S["EventResource"];
  readonly threadId: string | undefined;
  readonly actor?: Person;
  readonly audience?: boolean;
  readonly drop?: readonly string[];
}

/** The members of {@link World} the transport layers set. */
export interface WorldOptions {
  /** Lotse's configured billing mode. */
  readonly billingMode: BillingModeName;
  /** Lotse's configured tool class. */
  readonly toolClass: ToolClassName;
  /** Appends a durable event to the log and wakes the deliverers. */
  readonly append: (type: string, envelope: Envelope) => LogEntry;
}

function closed(
  body: unknown,
  required: readonly string[] = [],
  optional: readonly string[] = [],
  enums: Readonly<Record<string, readonly string[]>> = {},
): asserts body is Record<string, unknown> {
  if (!isPlainObject(body)) throw fieldError("", "required");
  const errors = Object.keys(body)
    .filter((n) => !required.includes(n) && !optional.includes(n))
    .sort(byCodeUnit)
    .map((n) => ({ pointer: `/${n}`, code: "unknown_member" }));
  for (const n of required)
    if (!(n in body)) errors.push({ pointer: `/${n}`, code: "required" });
  for (const [name, values] of Object.entries(enums))
    if (name in body && !values.includes(body[name] as string))
      errors.push({ pointer: `/${name}`, code: "invalid_enum" });
  if (errors.length) throw new FakeProblem("validation_failed", { errors });
}

function checkIfMatch(ifMatch: string | undefined, version: number): void {
  if (ifMatch === undefined) throw new FakeProblem("precondition_required");
  if (![`"${version}"`, `W/"${version}"`].includes(ifMatch.trim()))
    throw new FakeProblem("version_conflict");
}

function getOr404<T>(table: ReadonlyMap<string, T>, key: string): T {
  const canonical = canonicalUuid(key);
  const found = canonical === undefined ? undefined : table.get(canonical);
  if (found === undefined) throw new FakeProblem("not_found");
  return found;
}

/** Data and rules; the transport layers call into it. */
export class World {
  /** The instance. */
  readonly instanceId = uuidv7();
  /** The sync epoch. */
  readonly syncEpoch = uuidv7();
  /** The space Support. */
  readonly spaceId = uuidv7();
  /** The channel Allgemein. */
  readonly channelId = uuidv7();
  /** The owner. */
  readonly robin = new Person(uuidv7(), "user", "robin", "Robin");
  /** The person who triggers. */
  readonly alex = new Person(uuidv7(), "user", "alex", "Alex");
  /** The agent under test. */
  readonly lotse = new Person(uuidv7(), "agent", "lotse", "Lotse");
  /** Another agent. */
  readonly texter = new Person(uuidv7(), "agent", "texter", "Texter");
  /** The permission version. */
  permissionVersion = 1;
  /** The settings version of the channel. */
  settingsVersion = 1;
  /** The invocation key. */
  readonly issuer: Issuer;
  /** The runs by ID. */
  readonly runs = new Map<string, RunRecord>();
  /** The message IDs of every thread, in order. */
  readonly threads = new Map<string, string[]>();
  /** The messages by ID. */
  readonly messages = new Map<string, MessageDoc>();
  /** Lotse's messages by `client_msg_id`. */
  readonly byClientMsgId = new Map<string, string>();
  /** The stored `createMessage` bodies, in order. */
  readonly messageBodies: Record<string, unknown>[] = [];
  /** The components by ID. */
  readonly components = new Map<string, StoredComponent>();
  /** The interactions by ID. */
  readonly interactions = new Map<string, InteractionDoc>();
  /** The commands by ID. */
  readonly commands = new Map<string, CommandDoc>();
  /** Open autocomplete requests by ID. */
  readonly autocomplete = new Map<string, PendingAutocomplete>();
  /** The answers of autocomplete requests by ID. */
  readonly autocompleteAnswers = new Map<string, S["CommandChoice"][]>();
  /** The heartbeat bodies received, in order. */
  readonly heartbeats: Record<string, unknown>[] = [];
  /** Lotse's configured billing mode. */
  billingMode: BillingModeName;
  /** Lotse's configured tool class. */
  toolClass: ToolClassName;
  private readonly append: WorldOptions["append"];

  /**
   * Creates the world.
   *
   * @param clock - The shared FakeClock.
   * @param timings - The server keys the FakeServer plays.
   * @param options - Billing mode, tool class and the log.
   */
  constructor(
    readonly clock: FakeClock,
    readonly timings: Timings,
    options: WorldOptions,
  ) {
    this.billingMode = options.billingMode;
    this.toolClass = options.toolClass;
    this.append = options.append;
    this.issuer = new Issuer(this.instanceId);
  }

  // Time ---------------------------------------------------------------------------------

  /**
   * Returns the clock's wall time.
   *
   * @returns Unix milliseconds.
   */
  now(): number {
    return this.clock.now();
  }

  /**
   * Returns the wall time as the server writes it.
   *
   * @returns The timestamp.
   */
  nowIso(): string {
    return iso(this.now());
  }

  // Documents ----------------------------------------------------------------------------

  /**
   * Returns the `Run` body as `getRun` answers it.
   *
   * @param run - The run.
   * @returns The document.
   */
  runDoc(run: RunRecord): RunDoc {
    const doc: RunDoc = {
      id: run.id,
      thread_id: run.threadId,
      channel_id: this.channelId,
      agent_id: this.lotse.id,
      triggered_by: run.triggeredBy,
      trigger_kind: "message",
      triggering_message_id: run.triggeringMessageId,
      state: run.state,
      version: run.version,
      billing_mode_expected: run.billingModeExpected,
      metrics: { ...run.live.metrics },
      cost: run.live.cost ?? {
        amount_micros: 0,
        currency: "USD",
        is_estimate: true,
      },
      steps: { ...run.live.steps },
      followups: {},
      queued_at: iso(run.queuedAt),
    };
    if (run.live.model !== undefined) doc.model = run.live.model;
    if (run.live.status_line !== undefined)
      doc.status_line = run.live.status_line;
    if (run.live.last_text_excerpt !== undefined)
      doc.last_text_excerpt = run.live.last_text_excerpt;
    if (run.live.plan !== undefined) doc.plan = run.live.plan;
    if (run.stateReason !== undefined) doc.state_reason = run.stateReason;
    if (run.billingModeUsed !== undefined)
      doc.billing_mode_used = run.billingModeUsed;
    if (run.summary !== undefined) doc.summary = run.summary;
    if (run.startedAt !== undefined) doc.started_at = iso(run.startedAt);
    if (run.finishedAt !== undefined) doc.finished_at = iso(run.finishedAt);
    if (run.startedAt !== undefined && run.finishedAt !== undefined)
      doc.duration_ms = Math.trunc(run.finishedAt - run.startedAt);
    return fixtures.build("run", doc, { replace: RUN_MEMBERS });
  }

  private summaryDoc(run: RunRecord): RunSummaryDoc {
    const doc = this.runDoc(run) as unknown as Record<string, unknown>;
    const out = Object.fromEntries(
      SUMMARY_MEMBERS.filter((n) => n in doc).map((n) => [n, doc[n]]),
    );
    return {
      ...(out as Omit<RunSummaryDoc, "step_count">),
      step_count: Object.keys(run.live.steps).length,
    };
  }

  /**
   * Returns an envelope of type `type` built from its fixture (without `seq`).
   *
   * @param type - The event type (also the fixture name).
   * @param payload - The payload.
   * @param options - Resource, thread, actor, audience and paths to drop.
   * @returns The envelope.
   */
  envelope(
    type: fixtures.FixtureName,
    payload: object,
    options: EnvelopeOptions,
  ): Envelope {
    const scope: S["EventScope"] = {
      space_id: this.spaceId,
      channel_id: this.channelId,
    };
    if (options.threadId !== undefined) scope.thread_id = options.threadId;
    const actor: S["EventActor"] =
      options.actor !== undefined
        ? { id: options.actor.id, kind: options.actor.kind }
        : { kind: "system" };
    const audience = options.audience ?? true;
    const override = {
      id: uuidv7(),
      type,
      schema_version: 1,
      occurred_at: this.nowIso(),
      scope,
      resource: options.resource,
      permission_version: this.permissionVersion,
      payload,
      actor,
      ...(audience ? { audience: "self" as const } : {}),
    } satisfies S["EventEnvelope"] & { type: string; payload: object };
    const doc = fixtures.build(type, override, {
      replace: ["payload", "scope", "resource", "actor"],
      drop: options.drop ?? [],
    }) as Envelope;
    if (!audience) delete doc.audience;
    return doc;
  }

  private stateChanged(run: RunRecord): void {
    let triggerState: S["TriggerState"] | undefined =
      run.state === "running"
        ? "working"
        : run.state === "completed"
          ? "done"
          : undefined;
    if (run.state === "failed" || run.state === "cancelled")
      triggerState = run.state;
    const payload: S["RunStateChangedPayload"] = {
      run: this.summaryDoc(run),
      triggering_message_id: run.triggeringMessageId,
    };
    if (triggerState !== undefined) payload.trigger_state = triggerState;
    const envelope = this.envelope("run.state_changed", payload, {
      resource: { kind: "run", id: run.id, version: run.version },
      threadId: run.threadId,
      audience: false,
    });
    this.append("run.state_changed", envelope);
  }

  /**
   * Changes the state: version + 1, times, reason and a `run.state_changed` event.
   *
   * @param run - The run.
   * @param state - The new state.
   * @param reason - The state reason.
   */
  transition(run: RunRecord, state: RunStateName, reason?: string): void {
    const now = this.now();
    run.state = state;
    run.version += 1;
    run.stateReason = reason;
    if (state === "running") {
      run.starts += 1;
      run.startedAt ??= now;
    }
    if (ENDED.has(state)) run.finishedAt = now;
    this.stateChanged(run);
  }

  // Lookups ------------------------------------------------------------------------------

  /**
   * Returns the run, or throws `404 not_found` (also for a malformed ID).
   *
   * @param runId - The run.
   * @returns The run.
   */
  runOr404(runId: string): RunRecord {
    return getOr404(this.runs, runId);
  }

  // Trigger and deliveries ---------------------------------------------------------------

  private message(
    threadId: string,
    author: Person,
    text: string,
    options: {
      clientMsgId?: string;
      runId?: string | undefined;
      kind?: S["MessageKind"];
    } = {},
  ): MessageDoc {
    let thread = this.threads.get(threadId);
    if (thread === undefined) {
      thread = [];
      this.threads.set(threadId, thread);
    }
    const message: MessageDoc = {
      id: uuidv7(),
      thread_id: threadId,
      channel_id: this.channelId,
      thread_seq: thread.length + 1,
      author: author.ref(),
      client_msg_id: options.clientMsgId ?? uuidv7(),
      origin: author.kind === "agent" ? "agent" : "human",
      kind: options.kind ?? "text",
      content_md: text,
      content_format: 1,
      silent: false,
      attachments: [],
      mention_ids: [],
      reactions: [],
      pinned: false,
      components: [],
      version: 1,
      created_at: this.nowIso(),
    };
    if (options.runId !== undefined) message.run_id = options.runId;
    thread.push(message.id);
    this.messages.set(message.id, message);
    return message;
  }

  /**
   * Creates a new thread with Alex's message, a queued run and its `agent.invocation`.
   *
   * @param text - The message content.
   * @returns The run.
   */
  trigger(text: string): RunRecord {
    const threadId = uuidv7();
    const message = this.message(threadId, this.alex, text);
    const run = new RunRecord(
      uuidv7(),
      threadId,
      this.alex.id,
      message.id,
      this.now(),
      this.billingMode,
    );
    this.runs.set(run.id, run);
    const payload: S["AgentInvocationPayload"] = {
      run_id: run.id,
      thread_id: threadId,
      channel_id: this.channelId,
      space_id: this.spaceId,
      trigger_kind: "message",
      message_id: message.id,
      triggered_by: this.alex.ref(),
      billing_mode: this.billingMode,
      settings_version: this.settingsVersion,
    };
    const envelope = this.envelope("agent.invocation", payload, {
      resource: { kind: "run", id: run.id },
      threadId,
      actor: this.alex,
    });
    this.append("agent.invocation", envelope);
    return run;
  }

  /**
   * Posts Alex's follow-up message in the run's thread and its `agent.followup`.
   *
   * @param runId - The run.
   * @param text - The message content.
   * @returns The message ID.
   */
  followup(runId: string, text: string): string {
    const run = this.runOr404(runId);
    const message = this.message(run.threadId, this.alex, text);
    const payload: S["AgentFollowupPayload"] = {
      run_id: run.id,
      thread_id: run.threadId,
      message_id: message.id,
    };
    const envelope = this.envelope("agent.followup", payload, {
      resource: { kind: "run", id: run.id },
      threadId: run.threadId,
      actor: this.alex,
    });
    this.append("agent.followup", envelope);
    return message.id;
  }

  /**
   * Lets Alex operate a component of the run: `interaction.created`.
   *
   * @param runId - The run.
   * @param customId - The component's custom ID (created on a card of the run if missing).
   * @param values - The submitted values.
   * @returns The interaction ID.
   */
  interaction(
    runId: string,
    customId: string,
    values: Readonly<Record<string, unknown>>,
  ): string {
    const run = this.runOr404(runId);
    let component = [...this.components.values()].find(
      (c) => c.custom_id === customId && c.run_id === run.id,
    );
    if (component === undefined) {
      const message = this.message(run.threadId, this.lotse, "Approve?", {
        runId: run.id,
        kind: "card",
      });
      component = this.component(
        message,
        { custom_id: customId, type: "button", definition: {} },
        0,
      );
    }
    const interaction: InteractionDoc = {
      id: uuidv7(),
      component_id: component.id,
      message_id: component.message_id,
      principal_id: this.alex.id,
      client_interaction_id: uuidv7(),
      custom_id: customId,
      values: { ...values },
      state: "dispatched",
      thread_id: run.threadId,
      run_id: run.id,
      created_at: this.nowIso(),
    };
    this.interactions.set(interaction.id, interaction);
    const payload: S["InteractionCreatedPayload"] = { interaction };
    const envelope = this.envelope("interaction.created", payload, {
      resource: { kind: "interaction", id: interaction.id },
      threadId: run.threadId,
      actor: this.alex,
    });
    this.append("interaction.created", envelope);
    return interaction.id;
  }

  /**
   * Lets Alex run a command of Lotse: `command.invoked`.
   *
   * @param command - The command.
   * @param runId - The run whose thread is used, if any.
   * @param threadId - The thread without a run, if any.
   * @returns The invocation ID.
   */
  commandInvoked(
    command: CommandDoc,
    runId: string | undefined,
    threadId: string | undefined,
  ): string {
    const thread =
      runId !== undefined ? this.runOr404(runId).threadId : threadId;
    const payload: S["CommandInvokedPayload"] = {
      invocation_id: uuidv7(),
      command_id: command.id,
      command_name: command.name,
      control: command.control,
      channel_id: this.channelId,
      invoked_by: this.alex.ref(),
      options: [],
    };
    if (thread !== undefined) payload.thread_id = thread;
    if (runId !== undefined) payload.run_id = runId;
    const envelope = this.envelope("command.invoked", payload, {
      resource: { kind: "command", id: command.id },
      threadId: thread,
      actor: this.alex,
    });
    this.append("command.invoked", envelope);
    return payload.invocation_id;
  }

  private claims(
    sub: string,
    kind: TriggerKind,
    obj: string,
    threadId: string | undefined,
  ): UnsignedClaims {
    const iat = Math.floor(this.now() / 1000);
    const base = {
      aud: this.lotse.id,
      sub,
      iat,
      exp: iat + Math.trunc(this.timings["agent.invocation_token_ttl"] / 1000),
      trigger_kind: kind,
      thread_id: threadId ?? this.channelId,
      channel_id: this.channelId,
      space_id: this.spaceId,
      level: "trigger" as const,
      billing_mode: this.billingMode,
      tool_class: this.toolClass,
      permission_version: this.permissionVersion,
    };
    return { ...base, [TRIGGER_OBJECT[kind]]: obj } as UnsignedClaims;
  }

  /**
   * Returns the claims an `agent.invocation` of the run would carry now (not recorded).
   *
   * @param run - The run.
   * @returns Full claims with `iss` and a fresh `jti`.
   */
  invocationClaims(run: RunRecord): InvocationClaims {
    const claims = this.claims(
      run.triggeredBy,
      "message",
      run.triggeringMessageId,
      run.threadId,
    );
    return { iss: this.instanceId, jti: uuidv7(), ...claims, run_id: run.id };
  }

  /**
   * The delivery hook: decides, issues a fresh token and records it as the server does.
   *
   * @param entry - The log entry.
   * @returns The envelope with its delivery fields, or `undefined` when the event is skipped.
   */
  deliver(entry: LogEntry): Envelope | undefined {
    const envelope: Envelope = { ...entry.envelope };
    if (!TOKEN_EVENTS.has(entry.type)) return envelope;
    const payload = envelope.payload as Record<string, unknown>;
    const interaction = payload.interaction as
      Record<string, unknown> | undefined;
    const runId = (payload.run_id ?? interaction?.run_id) as string | undefined;
    const run = runId ? this.runs.get(runId) : undefined;
    if (run !== undefined && ENDED.has(run.state)) return undefined;
    const record =
      run !== undefined &&
      ((entry.type === "agent.invocation" && run.state === "queued") ||
        ((entry.type === "agent.followup" ||
          entry.type === "interaction.created") &&
          run.state === "waiting_input"));
    let kind: TriggerKind;
    let obj: string;
    let sub: string;
    if (entry.type === "agent.invocation") {
      [kind, obj, sub] = [
        "message",
        payload.message_id as string,
        run?.triggeredBy ?? this.alex.id,
      ];
    } else if (entry.type === "agent.followup") {
      [kind, obj, sub] = [
        "followup",
        payload.message_id as string,
        this.alex.id,
      ];
    } else if (entry.type === "interaction.created") {
      [kind, obj, sub] = [
        "interaction",
        interaction?.id as string,
        interaction?.principal_id as string,
      ];
    } else {
      [kind, obj, sub] = [
        "command",
        payload.command_id as string,
        (payload.invoked_by as PrincipalRefDoc).id,
      ];
    }
    const thread =
      typeof payload.thread_id === "string" ? payload.thread_id : run?.threadId;
    const claims = this.claims(sub, kind, obj, thread);
    if (run !== undefined) claims.run_id = run.id;
    const [token, full] = this.issuer.issue(claims);
    if (run !== undefined) {
      run.issued.set(full.jti, full);
      run.lastJti = full.jti;
      const changed = run.billingModeExpected !== this.billingMode;
      run.billingModeExpected = this.billingMode;
      if (record) {
        run.invocationJti = full.jti;
        run.renewedFromJti = undefined;
      }
      if (record || changed) run.version += 1;
    }
    envelope.invocation_token = token;
    return envelope;
  }

  /**
   * Returns an ephemeral `command.autocomplete_requested` with its token (no `seq`).
   *
   * @param command - The command.
   * @param value - What Alex has typed so far.
   * @param expiresInMs - Milliseconds until `expires_at`.
   * @returns The envelope.
   */
  autocompleteEvent(
    command: CommandDoc,
    value: string,
    expiresInMs: number,
  ): Envelope {
    const requestId = uuidv7();
    const expires = this.now() + expiresInMs;
    const payload: S["CommandAutocompleteRequestedPayload"] = {
      request_id: requestId,
      command_id: command.id,
      channel_id: this.channelId,
      user_id: this.alex.id,
      option: "target",
      value,
      options: [],
      expires_at: iso(expires),
    };
    const envelope = this.envelope("command.autocomplete_requested", payload, {
      resource: { kind: "command", id: command.id },
      threadId: undefined,
      actor: this.alex,
      drop: ["seq", "permission_version"],
    });
    [envelope.invocation_token] = this.issuer.issue(
      this.claims(this.alex.id, "autocomplete", command.id, undefined),
    );
    this.autocomplete.set(requestId, { expires, payload });
    return envelope;
  }

  // Invocation-Token header (SR9) --------------------------------------------------------

  /**
   * Checks the `Invocation-Token` header of a request.
   *
   * @param values - Every value of the header.
   * @param operationId - The operation.
   * @throws {@link FakeProblem} `403 invocation_token_invalid` with `invalid` or `run_inactive`.
   */
  bind(values: readonly string[], operationId: string): void {
    if (!values.length) return;
    const invalid = new FakeProblem("invocation_token_invalid", {
      reason: "invalid",
    });
    const value = values[0];
    if (values.length !== 1 || !value) throw invalid;
    const checked = this.issuer.check(value, {
      aud: this.lotse.id,
      nowMs: this.now(),
      leewayMs: this.timings["auth.clock_leeway"],
      ignoreExpiry: true,
    });
    const claims = checked.claims;
    if (
      checked.verdict !== "valid" ||
      claims === undefined ||
      !("run_id" in claims)
    )
      throw invalid;
    const run = this.runs.get(claims.run_id as string);
    if (run === undefined || claims.sub !== run.triggeredBy) throw invalid;
    if (claims.thread_id !== run.threadId) throw invalid;
    if (
      ENDED.has(run.state) &&
      !["startRun", "resumeRun", "renewInvocationToken"].includes(operationId)
    )
      throw new FakeProblem("invocation_token_invalid", {
        reason: "run_inactive",
      });
  }

  // Run operations -----------------------------------------------------------------------

  private checkToken(run: RunRecord, token: unknown): InvocationClaims {
    if (typeof token !== "string" || !token)
      throw new FakeProblem("invocation_token_invalid", { reason: "invalid" });
    const checked = this.issuer.check(token, {
      aud: this.lotse.id,
      nowMs: this.now(),
      leewayMs: this.timings["auth.clock_leeway"],
      runId: run.id,
      ignoreExpiry: true,
    });
    if (checked.verdict !== "valid" || checked.claims === undefined)
      throw new FakeProblem("invocation_token_invalid", { reason: "invalid" });
    return checked.claims as unknown as InvocationClaims;
  }

  /**
   * `startRun` or `resumeRun`.
   *
   * @param runId - The run.
   * @param body - The request body.
   * @param resume - Whether the operation is `resumeRun`.
   * @returns The Run body and whether the answer is an idempotent replay.
   */
  start(runId: string, body: unknown, resume: boolean): [RunDoc, boolean] {
    closed(
      body,
      ["invocation_token", "billing_mode_used", "tool_class_used"],
      ["external_session_ref", "model", "billing_evidence"],
      {
        billing_mode_used: ["personal", "organization"],
        tool_class_used: ["chat", "scoped", "host"],
      },
    );
    if ("external_session_ref" in body)
      throw fieldError("/external_session_ref", "not_enabled");
    const run = this.runOr404(runId);
    const claims = this.checkToken(run, body.invocation_token);
    const jti = claims.jti;
    if (run.startedJti !== undefined && jti === run.startedJti) {
      if (run.state === "running") return [this.runDoc(run), true];
      if (run.state === "waiting_input")
        throw new FakeProblem("invocation_token_invalid", {
          reason: "superseded",
        });
    }
    if (run.state !== "queued" && run.state !== "waiting_input")
      throw new FakeProblem("already_started", {
        detail: "the run has already started or ended",
      });
    if (jti !== run.invocationJti)
      throw new FakeProblem("invocation_token_invalid", {
        reason: "superseded",
      });
    if (
      this.now() / 1000 >
      claims.exp + this.timings["auth.clock_leeway"] / 1000
    )
      throw new FakeProblem("invocation_token_invalid", { reason: "expired" });
    if ((run.state === "waiting_input") !== resume)
      throw new FakeProblem("invalid_state_transition");
    const evidence = isPlainObject(body.billing_evidence)
      ? body.billing_evidence
      : {};
    let mismatch: string | undefined;
    if (
      evidence.key_type === "unknown" ||
      body.billing_mode_used !== run.billingModeExpected
    )
      mismatch = "billing_mode_mismatch";
    else if (
      toolRank(body.tool_class_used as string) > toolRank(claims.tool_class)
    )
      mismatch = "tool_class_mismatch";
    if (mismatch !== undefined) {
      this.transition(run, "failed", mismatch);
      throw new FakeProblem(mismatch, { reason: mismatch });
    }
    run.startedJti = jti;
    run.renewedFromJti = undefined;
    run.billingModeUsed = body.billing_mode_used as BillingModeName;
    if (typeof body.model === "string" && body.model)
      run.live.model = body.model;
    this.transition(run, "running");
    return [this.runDoc(run), false];
  }

  /**
   * `renewInvocationToken`: a new token for a queued or waiting run.
   *
   * @param runId - The run.
   * @param body - The request body.
   * @returns The grant.
   */
  renew(runId: string, body: unknown): S["InvocationTokenGrant"] {
    closed(body, ["invocation_token"]);
    const run = this.runOr404(runId);
    const claims = this.checkToken(run, body.invocation_token);
    const jti = claims.jti;
    if (run.state !== "queued" && run.state !== "waiting_input")
      throw new FakeProblem("already_started", {
        detail: "the run has already started or ended",
      });
    const known =
      jti === run.invocationJti ||
      (run.renewedFromJti !== undefined && jti === run.renewedFromJti);
    if (!known || run.startedJti === jti)
      throw new FakeProblem("invocation_token_invalid", {
        reason: "superseded",
      });
    if (claims.trigger_kind !== "message" && claims.trigger_kind !== "followup")
      throw new FakeProblem("internal_error");
    const obj = claims[TRIGGER_OBJECT[claims.trigger_kind]] ?? "";
    const fresh = this.claims(
      claims.sub,
      claims.trigger_kind,
      obj,
      claims.thread_id,
    );
    fresh.run_id = run.id;
    const [token, full] = this.issuer.issue(fresh);
    if (jti === run.invocationJti) run.renewedFromJti = jti;
    run.invocationJti = full.jti;
    run.billingModeExpected = this.billingMode;
    run.issued.set(full.jti, full);
    run.lastJti = full.jti;
    run.version += 1;
    return { invocation_token: token, expires_at: iso(full.exp * 1000) };
  }

  /**
   * `awaitRunInput`: `running` to `waiting_input`.
   *
   * @param runId - The run.
   * @param body - The request body.
   * @returns The Run body.
   */
  awaitInput(runId: string, body: unknown): RunDoc {
    closed(body, ["status_line"]);
    if (textError(body.status_line))
      throw fieldError("/status_line", "invalid");
    const run = this.runOr404(runId);
    if (run.state === "waiting_input") return this.runDoc(run);
    if (run.state !== "running")
      throw new FakeProblem("invalid_state_transition");
    if (body.status_line) run.live.status_line = body.status_line as string;
    this.transition(run, "waiting_input");
    return this.runDoc(run);
  }

  /**
   * `finishRun` with the server's reason rules.
   *
   * @param runId - The run.
   * @param body - The request body.
   * @returns The Run body.
   */
  finish(runId: string, body: unknown): RunDoc {
    closed(body, ["state"], ["reason", "summary", "metrics", "costs"], {
      state: ["completed", "failed", "cancelled"],
    });
    if ("costs" in body) throw fieldError("/costs", "not_enabled");
    const state = body.state as "completed" | "failed" | "cancelled";
    let reason = (body.reason as string | undefined) || undefined;
    if ((state === "failed" || state === "cancelled") && !reason)
      throw fieldError("/reason", "required");
    if (state === "cancelled" && !CANCEL_REASONS.has(reason ?? ""))
      throw fieldError("/reason", "invalid_enum");
    if ("summary" in body && textError(body.summary))
      throw fieldError("/summary", "invalid");
    const run = this.runOr404(runId);
    if (run.state === state) return this.runDoc(run);
    if (
      ENDED.has(run.state) ||
      (state === "completed" && run.state !== "running")
    )
      throw new FakeProblem("invalid_state_transition");
    if (state === "completed") reason = undefined;
    else if (state === "failed" && !KEEP_FAILED.has(reason ?? "")) {
      this.reasonStep(run, String(reason));
      reason = "adapter_error";
    }
    if (typeof body.summary === "string" && body.summary)
      run.summary = body.summary;
    if ("metrics" in body)
      run.live.metrics = { ...(body.metrics as S["RunMetrics"]) };
    this.transition(run, state, reason);
    return this.runDoc(run);
  }

  private reasonStep(run: RunRecord, original: string): void {
    const stepId = uuidv7();
    const now = this.nowIso();
    const ordinals = Object.values(run.live.steps).map((s) => s.ordinal);
    run.live.steps[stepId] = {
      id: stepId,
      ordinal: Math.max(0, ...ordinals) + 1,
      kind: "status",
      name: "adapter_reason",
      title: "",
      status: "failed",
      detail: textError(original) ? {} : { original_reason: original },
      started_at: now,
      finished_at: now,
    };
  }

  /**
   * `patchRun` and the gateway op `run.update`.
   *
   * @param runId - The run.
   * @param baseVersion - The version the ops build on.
   * @param ops - The ops.
   * @returns The new version.
   */
  patch(runId: string, baseVersion: unknown, ops: unknown): number {
    const run = this.runOr404(runId);
    const parsed = parseOps(ops);
    if (!ACTIVE.has(run.state))
      throw new FakeProblem("conflict", {
        reason: "run_not_active",
        detail: "the run is not active",
      });
    if (baseVersion !== run.version)
      throw new FakeProblem("base_version_conflict", {
        detail: "base_version is not the current version of the run",
        current: this.runDoc(run),
      });
    run.live = applyOps(
      run.live as unknown as Record<string, unknown>,
      parsed,
      {
        nowIso: () => this.nowIso(),
        excerptMax: EXCERPT_MAX,
      },
    ) as unknown as LiveBox;
    run.version += 1;
    return run.version;
  }

  /**
   * The server side of `cancelRun`.
   *
   * @param run - The run.
   * @param reason - The payload's reason.
   * @returns Whether the run still waits for the adapter's `cancelled` (grace timer).
   * @throws `Error` when the run has completed or failed.
   */
  cancel(
    run: RunRecord,
    reason: S["RunCancelRequestedPayload"]["reason"],
  ): boolean {
    if (run.state === "cancelled") return false;
    if (run.state === "completed" || run.state === "failed")
      throw new Error("the run has ended");
    const payload: S["RunCancelRequestedPayload"] = {
      run_id: run.id,
      thread_id: run.threadId,
      reason,
    };
    if (reason === "user_cancelled") payload.requested_by = this.alex.id;
    const envelope = this.envelope("run.cancel_requested", payload, {
      resource: { kind: "run", id: run.id },
      threadId: run.threadId,
      actor: this.alex,
    });
    this.append("run.cancel_requested", envelope);
    if (run.state === "queued") {
      this.transition(run, "cancelled", reason);
      return false;
    }
    run.cancelRequested = true;
    return true;
  }

  // Context, channels, heartbeat ---------------------------------------------------------

  /**
   * `getRunContext`: the trigger and the thread as marked items.
   *
   * @param runId - The run.
   * @returns The context.
   */
  context(runId: string): RunContextDoc {
    const run = this.runOr404(runId);
    const items = (this.threads.get(run.threadId) ?? []).map((id) =>
      this.contextItem(this.messages.get(id) as MessageDoc, run),
    );
    const trigger = items.find(
      (i) => i.message_id === run.triggeringMessageId,
    ) as ContextItem;
    const doc: RunContextDoc = {
      run_id: run.id,
      trigger_message: trigger,
      thread_context: items,
      channel_context: [],
      context_floor_seq: 0,
    };
    return fixtures.build("run_context", doc, { replace: CONTEXT_MEMBERS });
  }

  private contextItem(message: MessageDoc, run: RunRecord): ContextItem {
    const own = message.author.id === this.lotse.id;
    const trigger = message.id === run.triggeringMessageId;
    return {
      message_id: message.id,
      thread_seq: message.thread_seq,
      created_at: message.created_at,
      author: message.author,
      role: own ? "assistant" : "user",
      origin: message.origin,
      kind: message.kind,
      content_md: message.content_md,
      attachments: [],
      trust: trigger ? "trigger" : own ? "self" : "authorized",
    };
  }

  /**
   * Returns the `AgentChannelConfig` of Allgemein.
   *
   * @returns The configuration.
   */
  channelConfig(): S["AgentChannelConfig"] {
    return {
      channel_id: this.channelId,
      space_id: this.spaceId,
      behavior: {
        require_mention: true,
        thread_require_mention: false,
        free_response: false,
        auto_thread: true,
        allow_mentioned_agents: true,
        history_backfill_limit: 50,
        prompt: "",
        skills: [],
        settings_version: this.settingsVersion,
      },
    };
  }

  /**
   * `sendHeartbeat`: stores the report and answers the presence.
   *
   * @param body - The request body.
   * @returns The acknowledgement.
   */
  heartbeat(body: unknown): S["HeartbeatAck"] {
    if (
      !isPlainObject(body) ||
      (body.state !== "online" && body.state !== "degraded")
    )
      throw fieldError(
        "/state",
        !isPlainObject(body) || !("state" in body)
          ? "required"
          : "invalid_enum",
      );
    if ("rate_limit" in body) throw fieldError("/rate_limit", "not_enabled");
    this.heartbeats.push({ ...body });
    const presence: S["AgentRuntimeStatus"] = {
      state: body.state,
      last_heartbeat_at: this.nowIso(),
      billing_mode_matches:
        ("reported_billing_mode" in body
          ? body.reported_billing_mode
          : this.billingMode) === this.billingMode,
    };
    if ("reported_billing_mode" in body)
      presence.reported_billing_mode =
        body.reported_billing_mode as BillingModeName;
    if ("runtime_version" in body)
      presence.runtime_version = body.runtime_version as string;
    if ("reported_tool_class" in body)
      presence.reported_tool_class = body.reported_tool_class as ToolClassName;
    return {
      presence,
      interval_ms: Math.trunc(this.timings["sync.heartbeat_interval"]),
    };
  }

  // Messages, components, interactions, commands -----------------------------------------

  /**
   * `createMessage`, idempotent by `client_msg_id`.
   *
   * @param threadId - The thread.
   * @param body - The request body.
   * @returns The Message and whether it was created now.
   */
  createMessage(threadId: string, body: unknown): [MessageDoc, boolean] {
    if (!isPlainObject(body) || !("client_msg_id" in body))
      throw fieldError("/client_msg_id", "required");
    const clientMsgId = canonicalUuid(String(body.client_msg_id));
    const thread = canonicalUuid(threadId);
    if (clientMsgId === undefined || thread === undefined)
      throw fieldError("/client_msg_id", "invalid_format");
    if (!this.threads.has(thread)) throw new FakeProblem("not_found");
    const existing = this.byClientMsgId.get(clientMsgId);
    if (existing !== undefined)
      return [this.messages.get(existing) as MessageDoc, false];
    let runId: string | undefined;
    if (body.run_id) {
      runId = canonicalUuid(asText(body.run_id, ""));
      if (runId === undefined) throw fieldError("/run_id", "invalid_format");
    }
    const message = this.message(
      thread,
      this.lotse,
      asText(body.content_md, ""),
      {
        clientMsgId,
        runId,
        kind: asText(body.kind, "text") as S["MessageKind"],
      },
    );
    const specs = Array.isArray(body.components)
      ? (body.components as Record<string, unknown>[])
      : [];
    for (const [position, spec] of specs.entries())
      this.component(message, spec, position);
    this.byClientMsgId.set(clientMsgId, message.id);
    this.messageBodies.push({ ...body });
    return [message, true];
  }

  private component(
    message: MessageDoc,
    spec: Readonly<Record<string, unknown>>,
    position: number,
  ): StoredComponent {
    const component: StoredComponent = {
      id: uuidv7(),
      custom_id: spec.custom_id as string,
      type: spec.type as S["ComponentType"],
      definition: {
        ...(spec.definition as Record<string, unknown> | undefined),
      },
      position,
      single_use: Boolean(spec.single_use ?? false),
      restricted_to: (spec.restricted_to ?? "any") as S["ComponentRestriction"],
      state: "active",
      version: 1,
      message_id: message.id,
    };
    if ("required_capability" in spec)
      component.required_capability =
        spec.required_capability as S["Capability"];
    if ("expires_at" in spec) component.expires_at = spec.expires_at as string;
    if (message.run_id !== undefined) component.run_id = message.run_id;
    this.components.set(component.id, component);
    message.components.push(withoutMessageId(component));
    return component;
  }

  /**
   * `updateComponent` with `If-Match`.
   *
   * @param componentId - The component.
   * @param body - The request body.
   * @param ifMatch - The `If-Match` header.
   * @returns The component.
   */
  updateComponent(
    componentId: string,
    body: unknown,
    ifMatch: string | undefined,
  ): ComponentDoc {
    const component = getOr404(this.components, componentId);
    checkIfMatch(ifMatch, component.version);
    closed(body, [], ["state", "definition"], {
      state: ["active", "disabled"],
    });
    Object.assign(component, body);
    component.version += 1;
    return withoutMessageId(component);
  }

  /**
   * `respondToInteraction`.
   *
   * @param interactionId - The interaction.
   * @param body - The request body.
   * @returns The interaction.
   */
  respond(interactionId: string, body: unknown): InteractionDoc {
    const interaction = getOr404(this.interactions, interactionId);
    closed(body, ["state"], ["response"], {
      state: ["acknowledged", "failed", "rejected"],
    });
    interaction.state = body.state as InteractionDoc["state"];
    if ("response" in body)
      interaction.response = body.response as S["InteractionResponse"];
    return interaction;
  }

  /**
   * `registerCommand` for Lotse.
   *
   * @param body - The request body.
   * @returns The command.
   */
  registerCommand(body: unknown): CommandDoc {
    closed(
      body,
      ["name", "description"],
      ["scope_kind", "scope_id", "options", "required_level", "control"],
    );
    if (!COMMAND_NAME.test(String(body.name)))
      throw fieldError("/name", "pattern_mismatch");
    const scopeKind = (body.scope_kind ?? "instance") as S["TargetKind"];
    const scopeId = body.scope_id as string | undefined;
    for (const command of this.commands.values())
      if (
        command.name === body.name &&
        command.scope_kind === scopeKind &&
        command.scope_id === scopeId
      )
        throw new FakeProblem("conflict", { reason: "command_exists" });
    const command: CommandDoc = {
      id: uuidv7(),
      owner_principal_id: this.lotse.id,
      name: body.name as string,
      description: body.description as string,
      scope_kind: scopeKind,
      options: Array.isArray(body.options)
        ? [...(body.options as S["CommandOption"][])]
        : [],
      required_level: (body.required_level ?? "trigger") as S["CommandLevel"],
      control: Boolean(body.control ?? false),
      version: 1,
    };
    if (scopeId !== undefined) command.scope_id = scopeId;
    this.commands.set(command.id, command);
    return command;
  }

  /**
   * `updateCommand` with `If-Match`.
   *
   * @param commandId - The command.
   * @param body - The request body.
   * @param ifMatch - The `If-Match` header.
   * @returns The command.
   */
  updateCommand(
    commandId: string,
    body: unknown,
    ifMatch: string | undefined,
  ): CommandDoc {
    const command = getOr404(this.commands, commandId);
    checkIfMatch(ifMatch, command.version);
    closed(
      body,
      ["name", "description"],
      ["scope_kind", "scope_id", "options", "required_level", "control"],
    );
    Object.assign(command, body);
    command.version += 1;
    return command;
  }

  /**
   * `removeCommand`.
   *
   * @param commandId - The command.
   */
  removeCommand(commandId: string): void {
    this.commands.delete(getOr404(this.commands, commandId).id);
  }

  /**
   * `listChannelCommands`: commands visible in the channel.
   *
   * @param channelId - The channel.
   * @returns The list.
   */
  channelCommands(channelId: string): S["CommandList"] {
    const visible = [...this.commands.values()].filter(
      (c) =>
        c.scope_kind === "instance" ||
        (c.scope_kind === "channel" && c.scope_id === channelId) ||
        (c.scope_kind === "space" && c.scope_id === this.spaceId),
    );
    if (channelId !== this.channelId) throw new FakeProblem("not_found");
    return { items: visible };
  }
}

function withoutMessageId(component: StoredComponent): ComponentDoc {
  const out: Partial<StoredComponent> = { ...component };
  delete out.message_id;
  return out as ComponentDoc;
}

// Python's `str()` of a JSON member: text stays, other scalars become their JSON text.
function asText(value: unknown, fallback: string): string {
  if (value === undefined) return fallback;
  return typeof value === "string" ? value : JSON.stringify(value);
}
