// SPDX-License-Identifier: Apache-2.0
/**
 * The FakeServer's events and bodies built from recorded fixtures (private; port of
 * `bothy_sdk/testing/_fake_fixtures.py`).
 *
 * Every fixture is a normalized object recorded from the real server (fixed IDs and times, `seq`
 * renumbered, no tokens; docs/DECISIONS.md, recorded fixtures). They are the same objects the
 * Python FakeServer loads from its package (`test/fake-server.test.ts` holds both equal), kept
 * here as data so the built package needs no file outside `dist`. The FakeServer takes one by
 * name, overrides the members it knows and substitutes every other occurrence of a replaced
 * fixture value generically, so a fixture recorded again with more members still yields
 * consistent IDs.
 *
 * @module
 */

import { isPlainObject, type JsonObject } from "./base.js";

/** The recorded fixtures by name (the Python file name without `.json`). */
export const FIXTURES = {
  "agent.followup": {
    actor: {
      id: "01900000-0000-7000-8000-000000000201",
      kind: "user",
    },
    audience: "self",
    id: "01900000-0000-7000-8000-000000000902",
    occurred_at: "2026-01-01T00:00:00.000Z",
    payload: {
      message_id: "01900000-0000-7000-8000-000000000106",
      run_id: "01900000-0000-7000-8000-000000000101",
      thread_id: "01900000-0000-7000-8000-000000000102",
    },
    permission_version: 1,
    resource: {
      id: "01900000-0000-7000-8000-000000000101",
      kind: "run",
    },
    schema_version: 1,
    scope: {
      channel_id: "01900000-0000-7000-8000-000000000103",
      space_id: "01900000-0000-7000-8000-000000000104",
      thread_id: "01900000-0000-7000-8000-000000000102",
    },
    seq: 2,
    type: "agent.followup",
  },
  "agent.invocation": {
    actor: {
      id: "01900000-0000-7000-8000-000000000001",
      kind: "user",
    },
    audience: "self",
    correlation_id: "01900000-0000-7000-8000-000000000002",
    id: "01900000-0000-7000-8000-000000000003",
    occurred_at: "2026-01-01T00:00:00.000Z",
    payload: {
      billing_mode: "organization",
      channel_id: "01900000-0000-7000-8000-000000000004",
      message_id: "01900000-0000-7000-8000-000000000005",
      run_id: "01900000-0000-7000-8000-000000000006",
      session_mode: "new",
      settings_version: 1,
      space_id: "01900000-0000-7000-8000-000000000007",
      thread_id: "01900000-0000-7000-8000-000000000008",
      trigger_kind: "message",
      triggered_by: {
        display_name: "Alex",
        handle: "alex",
        id: "01900000-0000-7000-8000-000000000001",
        kind: "user",
      },
    },
    permission_version: 16,
    resource: {
      id: "01900000-0000-7000-8000-000000000006",
      kind: "run",
    },
    schema_version: 1,
    scope: {
      channel_id: "01900000-0000-7000-8000-000000000004",
      space_id: "01900000-0000-7000-8000-000000000007",
      thread_id: "01900000-0000-7000-8000-000000000008",
    },
    seq: 1,
    type: "agent.invocation",
  },
  "command.autocomplete_requested": {
    actor: {
      id: "01900000-0000-7000-8000-000000000201",
      kind: "user",
    },
    audience: "self",
    id: "01900000-0000-7000-8000-000000000601",
    occurred_at: "2026-01-01T00:00:00.000Z",
    payload: {
      channel_id: "01900000-0000-7000-8000-000000000103",
      command_id: "01900000-0000-7000-8000-000000000501",
      expires_at: "2026-01-01T00:00:03.000Z",
      option: "target",
      options: [],
      request_id: "01900000-0000-7000-8000-000000000602",
      thread_id: "01900000-0000-7000-8000-000000000102",
      user_id: "01900000-0000-7000-8000-000000000201",
      value: "",
    },
    resource: {
      id: "01900000-0000-7000-8000-000000000501",
      kind: "command",
    },
    schema_version: 1,
    scope: {
      channel_id: "01900000-0000-7000-8000-000000000103",
      space_id: "01900000-0000-7000-8000-000000000104",
      thread_id: "01900000-0000-7000-8000-000000000102",
    },
    type: "command.autocomplete_requested",
  },
  "command.invoked": {
    actor: {
      id: "01900000-0000-7000-8000-000000000201",
      kind: "user",
    },
    audience: "self",
    id: "01900000-0000-7000-8000-000000000904",
    occurred_at: "2026-01-01T00:00:00.000Z",
    payload: {
      channel_id: "01900000-0000-7000-8000-000000000103",
      command_id: "01900000-0000-7000-8000-000000000501",
      command_name: "status",
      control: false,
      invocation_id: "01900000-0000-7000-8000-000000000502",
      invoked_by: {
        display_name: "Alex",
        handle: "alex",
        id: "01900000-0000-7000-8000-000000000201",
        kind: "user",
      },
      options: [],
      run_id: "01900000-0000-7000-8000-000000000101",
      thread_id: "01900000-0000-7000-8000-000000000102",
    },
    permission_version: 1,
    resource: {
      id: "01900000-0000-7000-8000-000000000501",
      kind: "command",
    },
    schema_version: 1,
    scope: {
      channel_id: "01900000-0000-7000-8000-000000000103",
      space_id: "01900000-0000-7000-8000-000000000104",
      thread_id: "01900000-0000-7000-8000-000000000102",
    },
    seq: 4,
    type: "command.invoked",
  },
  discovery: {
    api_versions: ["v1"],
    auth_providers: [
      {
        id: "01900000-0000-7000-8000-000000000001",
        kind: "local",
        name: "Local accounts",
      },
    ],
    branding: {
      accent_color: "spruce",
      product_name: "Example Chat",
    },
    features: [],
    gateway: {
      protocol_versions: [1],
      url: "/api/v1/gateway",
    },
    instance_id: "01900000-0000-7000-8000-000000000002",
    jwks_uri: "/api/v1/instance/jwks",
    local_password_enabled: true,
    min_client_versions: [],
    setup_required: false,
    spec_version: "0.2.0",
    webhook_keys_uri: "/api/v1/instance/webhook-keys",
  },
  "interaction.created": {
    actor: {
      id: "01900000-0000-7000-8000-000000000201",
      kind: "user",
    },
    audience: "self",
    id: "01900000-0000-7000-8000-000000000903",
    occurred_at: "2026-01-01T00:00:00.000Z",
    payload: {
      interaction: {
        client_interaction_id: "01900000-0000-7000-8000-000000000403",
        component_id: "01900000-0000-7000-8000-000000000402",
        created_at: "2026-01-01T00:00:00.000Z",
        custom_id: "approval.once",
        id: "01900000-0000-7000-8000-000000000401",
        message_id: "01900000-0000-7000-8000-000000000107",
        principal_id: "01900000-0000-7000-8000-000000000201",
        run_id: "01900000-0000-7000-8000-000000000101",
        state: "dispatched",
        thread_id: "01900000-0000-7000-8000-000000000102",
        values: {},
      },
    },
    permission_version: 1,
    resource: {
      id: "01900000-0000-7000-8000-000000000401",
      kind: "interaction",
    },
    schema_version: 1,
    scope: {
      channel_id: "01900000-0000-7000-8000-000000000103",
      space_id: "01900000-0000-7000-8000-000000000104",
      thread_id: "01900000-0000-7000-8000-000000000102",
    },
    seq: 3,
    type: "interaction.created",
  },
  run: {
    agent_id: "01900000-0000-7000-8000-000000000001",
    billing_mode_expected: "organization",
    billing_mode_used: "organization",
    channel_id: "01900000-0000-7000-8000-000000000002",
    cost: {
      amount_micros: 0,
      currency: "USD",
      is_estimate: true,
    },
    followups: {},
    id: "01900000-0000-7000-8000-000000000003",
    metrics: {},
    model: "model-test",
    queued_at: "2026-01-01T00:00:00.000Z",
    started_at: "2026-01-01T00:00:00.000Z",
    state: "running",
    steps: {},
    thread_id: "01900000-0000-7000-8000-000000000004",
    trigger_kind: "message",
    triggered_by: "01900000-0000-7000-8000-000000000005",
    triggering_message_id: "01900000-0000-7000-8000-000000000006",
    version: 3,
  },
  "run.cancel_requested": {
    actor: {
      id: "01900000-0000-7000-8000-000000000201",
      kind: "user",
    },
    audience: "self",
    id: "01900000-0000-7000-8000-000000000905",
    occurred_at: "2026-01-01T00:00:00.000Z",
    payload: {
      reason: "user_cancelled",
      requested_by: "01900000-0000-7000-8000-000000000201",
      run_id: "01900000-0000-7000-8000-000000000101",
      thread_id: "01900000-0000-7000-8000-000000000102",
    },
    permission_version: 1,
    resource: {
      id: "01900000-0000-7000-8000-000000000101",
      kind: "run",
    },
    schema_version: 1,
    scope: {
      channel_id: "01900000-0000-7000-8000-000000000103",
      space_id: "01900000-0000-7000-8000-000000000104",
      thread_id: "01900000-0000-7000-8000-000000000102",
    },
    seq: 5,
    type: "run.cancel_requested",
  },
  "run.state_changed": {
    actor: {
      kind: "system",
    },
    id: "01900000-0000-7000-8000-000000000906",
    occurred_at: "2026-01-01T00:00:00.000Z",
    payload: {
      run: {
        agent_id: "01900000-0000-7000-8000-000000000301",
        billing_mode_expected: "organization",
        billing_mode_used: "organization",
        channel_id: "01900000-0000-7000-8000-000000000103",
        cost: {
          amount_micros: 0,
          currency: "USD",
          is_estimate: true,
        },
        id: "01900000-0000-7000-8000-000000000101",
        queued_at: "2026-01-01T00:00:00.000Z",
        started_at: "2026-01-01T00:00:00.000Z",
        state: "running",
        step_count: 0,
        thread_id: "01900000-0000-7000-8000-000000000102",
        triggered_by: "01900000-0000-7000-8000-000000000201",
        version: 3,
      },
      trigger_state: "working",
      triggering_message_id: "01900000-0000-7000-8000-000000000105",
    },
    permission_version: 1,
    resource: {
      id: "01900000-0000-7000-8000-000000000101",
      kind: "run",
      version: 3,
    },
    schema_version: 1,
    scope: {
      channel_id: "01900000-0000-7000-8000-000000000103",
      space_id: "01900000-0000-7000-8000-000000000104",
      thread_id: "01900000-0000-7000-8000-000000000102",
    },
    seq: 6,
    type: "run.state_changed",
  },
  run_context: {
    channel_context: [],
    context_floor_seq: 0,
    run_id: "01900000-0000-7000-8000-000000000101",
    thread_context: [
      {
        attachments: [],
        author: {
          display_name: "Alex",
          handle: "alex",
          id: "01900000-0000-7000-8000-000000000201",
          kind: "user",
        },
        content_md: "Hello @Lotse",
        created_at: "2026-01-01T00:00:00.000Z",
        kind: "text",
        message_id: "01900000-0000-7000-8000-000000000105",
        origin: "human",
        role: "user",
        thread_seq: 1,
        trust: "trigger",
      },
    ],
    trigger_message: {
      attachments: [],
      author: {
        display_name: "Alex",
        handle: "alex",
        id: "01900000-0000-7000-8000-000000000201",
        kind: "user",
      },
      content_md: "Hello @Lotse",
      created_at: "2026-01-01T00:00:00.000Z",
      kind: "text",
      message_id: "01900000-0000-7000-8000-000000000105",
      origin: "human",
      role: "user",
      thread_seq: 1,
      trust: "trigger",
    },
  },
} as const satisfies Readonly<Record<string, JsonObject>>;

/** The name of a recorded fixture. */
export type FixtureName = keyof typeof FIXTURES;

/** How {@link build} applies an override. */
export interface BuildOptions {
  /**
   * Top-level members whose fixture value is replaced as a whole by the override instead of
   * merged (members the FakeServer does not produce vanish).
   */
  readonly replace?: readonly string[];
  /** Slash paths (`"payload/message_id"`) removed after the merge. */
  readonly drop?: readonly string[];
}

/**
 * Returns a fresh copy of the fixture `name`.
 *
 * @param name - The fixture.
 * @returns The copy.
 */
export function load(name: FixtureName): Record<string, unknown> {
  return structuredClone(FIXTURES[name]) as Record<string, unknown>;
}

// Pairs every fixture string with the override value at the same place.
function collect(
  fixture: unknown,
  override: unknown,
  pairs: Map<string, string>,
): void {
  if (isPlainObject(fixture) && isPlainObject(override)) {
    for (const [key, value] of Object.entries(override))
      if (key in fixture) collect(fixture[key], value, pairs);
  } else if (
    typeof fixture === "string" &&
    typeof override === "string" &&
    fixture !== override &&
    isIdentity(fixture) &&
    !pairs.has(fixture)
  ) {
    pairs.set(fixture, override);
  }
}

const UUID = /^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$/i;

// Only IDs and times are substituted; enum values such as a state stay where they are.
function isIdentity(text: string): boolean {
  return UUID.test(text) || (text.endsWith("Z") && text.includes("T"));
}

function merge(
  base: Record<string, unknown>,
  override: Readonly<Record<string, unknown>>,
): void {
  for (const [key, value] of Object.entries(override)) {
    if (value === undefined) continue;
    const current = base[key];
    if (isPlainObject(value) && isPlainObject(current)) merge(current, value);
    else base[key] = structuredClone(value);
  }
}

function substitute(
  value: unknown,
  pairs: ReadonlyMap<string, string>,
): unknown {
  if (Array.isArray(value)) return value.map((v) => substitute(v, pairs));
  if (isPlainObject(value))
    return Object.fromEntries(
      Object.entries(value).map(([k, v]) => [k, substitute(v, pairs)]),
    );
  if (typeof value === "string") return pairs.get(value) ?? value;
  return value;
}

function dropPaths(
  doc: Record<string, unknown>,
  paths: readonly string[],
): void {
  for (const path of paths) {
    const parts = path.split("/");
    const last = parts.pop() ?? "";
    let node: unknown = doc;
    for (const part of parts)
      node = isPlainObject(node) ? (node[part] ?? {}) : {};
    if (isPlainObject(node)) delete node[last];
  }
}

/**
 * Returns the fixture `name` with `override` applied.
 *
 * The result carries the override's members with the override's type; members only the
 * fixture has stay in the JSON (with consistently substituted IDs) but are not typed.
 *
 * @param name - The fixture.
 * @param override - Members to set (deep merge); fixture strings they replace are substituted
 *   everywhere else in the document too.
 * @param options - Members replaced as a whole and paths dropped after the merge.
 * @returns The document.
 */
export function build<T extends object>(
  name: FixtureName,
  override: T,
  options: BuildOptions = {},
): T {
  const doc = load(name);
  const pairs = new Map<string, string>();
  collect(doc, override, pairs);
  for (const path of options.replace ?? []) delete doc[path];
  merge(doc, override as Readonly<Record<string, unknown>>);
  const merged = substitute(doc, pairs) as Record<string, unknown>;
  dropPaths(merged, options.drop ?? []);
  return merged as T;
}
