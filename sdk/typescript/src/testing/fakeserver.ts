// SPDX-License-Identifier: Apache-2.0
/**
 * In-process FakeServer for adapter tests (docs/spec/09-agents.md 16.1; port of
 * `bothy_sdk/testing/fakeserver.py`).
 *
 * {@link FakeServer} serves the REST operations an agent adapter uses and the gateway on one
 * `node:http` listener on `127.0.0.1` with port 0. It plays the server rules of
 * docs/spec/09-agents.md 4.3 to 4.10, 5.1 and 5.3 and docs/spec/04-sync-protocol.md 6, 11.2
 * and 15.1 to 15.5 as the server code builds them: DPoP with nonces, consumers with cursors, a
 * fresh invocation token per delivery, redelivery after `agent.ack_timeout`, the start, renew
 * and finish verdicts and the patch rules. Every time decision goes through the shared
 * {@link FakeClock}; tests move time with {@link FakeServer.passTime}. Faults (`./faults.js`)
 * make one operation fail on purpose.
 *
 * Like the Python FakeServer it is created with `new FakeServer(options)` and opened with
 * {@link FakeServer.start}; {@link FakeServer.stop} (or `await using`) closes it. Recorded
 * requests ({@link FakeServer.requests}) hold method, path and status only, never a header
 * value or a body, so no token can leak through them (SEC-LOG-2).
 *
 * @example
 * ```ts
 * const fake = new FakeServer();
 * await fake.start();
 * const run = await fake.trigger();
 * (await fake.runState(run.runId)).state; // "queued"
 * await fake.stop();
 * ```
 *
 * @module
 */

import {
  generateKeyPairSync,
  randomBytes,
  sign,
  type KeyObject,
} from "node:crypto";
import {
  createServer,
  type IncomingMessage,
  type Server,
  type ServerResponse,
} from "node:http";
import type { AddressInfo } from "node:net";

import { Secret } from "../auth/secret.js";
import type { Credentials, DpopSigner } from "../auth/types.js";
import { DEFAULTS, resolveSettings, type SdkSettings } from "../config.js";
import type { components } from "../generated/openapi.js";
import { uuidv7 } from "../ids.js";
import { FakeClock, settle } from "./clock.js";
import {
  byCodeUnit,
  canonicalJson,
  canonicalUuid,
  FakeProblem,
  fieldError,
  isPlainObject,
  timingsWithDefaults,
  type Timings,
} from "./fake/base.js";
import { Authenticator, WWW_AUTHENTICATE } from "./fake/dpop.js";
import type { Envelope, LogEntry } from "./fake/events.js";
import * as fixtures from "./fake/fixtures.js";
import { Gateway } from "./fake/gateway.js";
import { Hub } from "./fake/hub.js";
import { FORGE_VARIANTS } from "./fake/tokens.js";
import { consumerOk, World, type RunRecord } from "./fake/world.js";
import { Fault, FaultKind } from "./faults.js";
import type {
  ForgedVariant,
  RunView,
  ScenarioServer,
  Triggered,
} from "./scenario-server.js";
import { signWebhook, webhookPublicKey } from "./webhooks.js";

export { TIMING_KEYS, type TimingKey, type Timings } from "./fake/base.js";

type S = components["schemas"];

const API = "/api/v1";
const GATEWAY_PATH = `${API}/gateway`;
const GATEWAY_OPS = new Set([
  "ack",
  "run.update",
  "heartbeat",
  "identify",
  "resume",
  "command.autocomplete_result",
]);
const PUBLIC = new Set(["getDiscovery", "getJwks", "getWebhookKeys"]);
// Consumer operations judge the agent by its own grants: the run binding is ignored there.
const DELIVERY = new Set([
  "pollAgentEvents",
  "ackAgentEvents",
  "getConsumerState",
]);
const IDEMPOTENT_KEY_OPS = new Set([
  "updateComponent",
  "respondToInteraction",
  "registerCommand",
  "updateCommand",
  "removeCommand",
]);
const ROUTES: readonly [string, string, string][] = [
  ["GET", "/instance/discovery", "getDiscovery"],
  ["GET", "/instance/jwks", "getJwks"],
  ["GET", "/instance/webhook-keys", "getWebhookKeys"],
  ["GET", "/agents/me/consumers/{consumer}/events", "pollAgentEvents"],
  ["POST", "/agents/me/consumers/{consumer}/ack", "ackAgentEvents"],
  ["GET", "/agents/me/consumers/{consumer}", "getConsumerState"],
  ["POST", "/agents/me/heartbeat", "sendHeartbeat"],
  ["GET", "/agents/me/triggerers", "getAgentTriggerers"],
  ["GET", "/agents/me/channels", "listServedChannels"],
  ["GET", "/agents/me/channels/{channel_id}/config", "getServedChannelConfig"],
  ["GET", "/runs/{run_id}", "getRun"],
  ["PATCH", "/runs/{run_id}", "patchRun"],
  ["GET", "/runs/{run_id}/context", "getRunContext"],
  ["POST", "/runs/{run_id}/start", "startRun"],
  ["POST", "/runs/{run_id}/resume", "resumeRun"],
  ["POST", "/runs/{run_id}/invocation-token", "renewInvocationToken"],
  ["POST", "/runs/{run_id}/await-input", "awaitRunInput"],
  ["POST", "/runs/{run_id}/finish", "finishRun"],
  ["POST", "/threads/{thread_id}/messages", "createMessage"],
  ["PATCH", "/components/{component_id}", "updateComponent"],
  ["POST", "/interactions/{interaction_id}/respond", "respondToInteraction"],
  ["POST", "/commands", "registerCommand"],
  ["PUT", "/commands/{command_id}", "updateCommand"],
  ["DELETE", "/commands/{command_id}", "removeCommand"],
  ["GET", "/channels/{channel_id}/commands", "listChannelCommands"],
  [
    "POST",
    "/command-autocompletions/{request_id}/answer",
    "answerAutocomplete",
  ],
];
const COMPILED = ROUTES.map(
  ([method, path, op]) =>
    [
      method,
      new RegExp(`^${path.replace(/\{(\w+)\}/g, "(?<$1>[^/]+)")}$`),
      op,
    ] as const,
);

/** One answer of an operation: status, body and extra headers. */
interface Answer {
  readonly status: number;
  readonly body: unknown;
  readonly headers: Readonly<Record<string, string>>;
}

/** The options of {@link FakeServer}; every member is optional. */
export interface FakeServerOptions {
  /** The clock shared with the SDK under test; a new {@link FakeClock} when absent. */
  readonly clock?: FakeClock;
  /**
   * Server keys to override (`agent.invocation_token_ttl`, `auth.clock_leeway`,
   * `agent.ack_timeout`, `agent.ack_persist_interval`, `sync.heartbeat_interval`,
   * `sync.resume_window`, `agent.max_inflight_events`, `command.autocomplete_timeout`,
   * `agent.jwks_max_age`, `agent.cancel_grace`, `auth.dpop_nonce_ttl`,
   * `auth.dpop_proof_max_age`): durations in milliseconds, `agent.max_inflight_events` a count.
   */
  readonly timings?: Readonly<Record<string, number>>;
  /** Shortcut for the timing `agent.ack_timeout` in milliseconds. */
  readonly ackTimeoutMs?: number;
  /** Shortcut for the timing `agent.invocation_token_ttl` in milliseconds. */
  readonly tokenTtlMs?: number;
  /** Bind Lotse's API token to a DPoP key (`credentials.dpop`); else a bearer token. Default `true`. */
  readonly dpop?: boolean;
  /** Lotse's configured billing mode. Default `organization`. */
  readonly billingMode?: S["BillingMode"];
  /** Lotse's configured tool class. Default `scoped`. */
  readonly toolClass?: S["ToolClass"];
}

/** One REST request as the FakeServer received it: method, path (without query) and status. */
export interface RecordedRequest {
  /** The HTTP method. */
  readonly method: string;
  /** The path without query. */
  readonly path: string;
  /** The answered status; `0` when the connection was dropped without an answer. */
  readonly status: number;
}

/** One gateway frame as the FakeServer received it: the op only. */
export interface RecordedFrame {
  /** The op of the frame. */
  readonly op: string;
}

/** A consumer's stored acknowledgement and the head of the log. */
export interface ConsumerView {
  /** The stored acknowledgement. */
  readonly ackedSeq: number;
  /** The highest `seq` of the log. */
  readonly headSeq: number;
}

/** The options of {@link FakeServer.sendWebhook}. */
export interface SendWebhookOptions {
  /** `whsec_` secrets (or raw keys) for `v1` entries, newest generation first. */
  readonly hmacSecrets?: readonly (string | Uint8Array)[];
}

/** An ES256 key of `node:crypto` behind {@link DpopSigner} (64-byte `R || S` signatures). */
class FakeDpopKey implements DpopSigner {
  readonly alg = "ES256" as const;
  readonly publicJwk: Readonly<Record<string, string>>;
  private readonly key: KeyObject;

  constructor() {
    const { privateKey, publicKey } = generateKeyPairSync("ec", {
      namedCurve: "P-256",
    });
    this.key = privateKey;
    const jwk = publicKey.export({ format: "jwk" });
    this.publicJwk = Object.freeze({
      kty: "EC",
      crv: "P-256",
      x: String(jwk.x),
      y: String(jwk.y),
    });
  }

  sign(data: Uint8Array): Promise<Uint8Array> {
    return Promise.resolve(
      new Uint8Array(
        sign("sha256", data, { key: this.key, dsaEncoding: "ieee-p1363" }),
      ),
    );
  }
}

/** The drop of the disconnect fault: the socket closes without an answer. */
class Disconnected extends Error {}

function headerValues(request: IncomingMessage, name: string): string[] {
  return request.headersDistinct[name] ?? [];
}

function readBody(request: IncomingMessage): Promise<Buffer> {
  return new Promise<Buffer>((resolve, reject) => {
    const chunks: Buffer[] = [];
    request.on("data", (chunk: Buffer) => chunks.push(chunk));
    request.on("end", () => resolve(Buffer.concat(chunks)));
    request.on("error", reject);
  });
}

/**
 * The in-process server the SDK's tests and an adapter's tests run against.
 *
 * Create it with `new FakeServer(options)`, open it with {@link FakeServer.start} and close it
 * with {@link FakeServer.stop} (the shape of the Python FakeServer). Members that depend on the
 * port ({@link FakeServer.origin}, {@link FakeServer.baseUrl}, {@link FakeServer.gatewayUrl})
 * hold their value after the start.
 */
export class FakeServer implements ScenarioServer {
  /** The clock shared with the SDK under test. */
  readonly clock: FakeClock;
  /** The server keys in effect: durations in milliseconds, `agent.max_inflight_events` a count. */
  readonly timings: Timings;
  /** Lotse's API token and, with `dpop`, its key. */
  readonly credentials: Credentials;
  /** Settings an adapter under test should use (short waits, a leeway within `auth.clock_leeway`). */
  readonly sdkSettings: SdkSettings;
  /** `http://127.0.0.1:<port>` after the start; empty before. */
  origin = "";
  private readonly hub: Hub;
  private readonly world: World;
  private readonly auth: Authenticator;
  private readonly gateway: Gateway;
  private readonly recorded: {
    method: string;
    path: string;
    status: number;
  }[] = [];
  private readonly frameOps: RecordedFrame[] = [];
  private readonly faults = new Map<string, Fault[]>();
  private readonly deprecated = new Map<string, string>();
  private readonly idempotency = new Map<
    string,
    [string, Answer | undefined]
  >();
  private readonly fixedTokens = new Map<number, string>();
  private tasks = new AbortController();
  private readonly webhookSeed = new Uint8Array(randomBytes(32));
  private server: Server | undefined;

  /**
   * Creates the server; {@link FakeServer.start} opens the listener.
   *
   * @param options - Clock, timings, DPoP, billing mode and tool class.
   * @throws `RangeError` on an unknown timing key.
   */
  constructor(options: FakeServerOptions = {}) {
    this.clock = options.clock ?? new FakeClock();
    const overrides: Record<string, number> = { ...options.timings };
    if (options.ackTimeoutMs !== undefined)
      overrides["agent.ack_timeout"] = options.ackTimeoutMs;
    if (options.tokenTtlMs !== undefined)
      overrides["agent.invocation_token_ttl"] = options.tokenTtlMs;
    this.timings = timingsWithDefaults(overrides);
    this.hub = new Hub(this.clock, this.timings, (entry) =>
      this.deliver(entry),
    );
    this.world = new World(this.clock, this.timings, {
      billingMode: options.billingMode ?? "organization",
      toolClass: options.toolClass ?? "scoped",
      append: (type, envelope) => this.hub.append(type, envelope),
    });
    const key = (options.dpop ?? true) ? new FakeDpopKey() : undefined;
    const apiToken = new Secret(
      `bothy_${randomBytes(32).toString("base64url")}`,
    );
    this.credentials =
      key === undefined ? { apiToken } : { apiToken, dpop: key };
    this.auth = new Authenticator(
      apiToken.reveal(),
      key?.publicJwk,
      this.clock,
      this.timings,
    );
    this.gateway = new Gateway({
      world: this.world,
      hub: this.hub,
      auth: this.auth,
      htu: () => this.gatewayHtu,
      takeFault: (op) => this.takeGatewayFault(op),
      record: (op) => this.frameOps.push({ op }),
    });
    // The retry and reconnect waits of the SDK are cut to a tenth of a second and below, so
    // tests run fast; the leeway never exceeds the server's `auth.clock_leeway`.
    this.sdkSettings = resolveSettings({
      retryBackoffBaseMs: 50,
      retryBackoffCapMs: 500,
      reconnectBackoffBaseMs: 50,
      reconnectBackoffCapMs: 500,
      invocationTokenLeewayMs: Math.min(
        DEFAULTS.invocationTokenLeewayMs,
        this.timings["auth.clock_leeway"],
      ),
    });
  }

  // Identity -----------------------------------------------------------------------------

  /** The instance (`iss` of every token). */
  get instanceId(): string {
    return this.world.instanceId;
  }

  /** Lotse's principal ID, the agent under test. */
  get agentId(): string {
    return this.world.lotse.id;
  }

  /** Texter's principal ID, a second agent. */
  get otherAgentId(): string {
    return this.world.texter.id;
  }

  /** The REST base URL, `origin + "/api/v1"`. */
  get baseUrl(): string {
    return this.origin + API;
  }

  /** The gateway's WebSocket URL. */
  get gatewayUrl(): string {
    return `ws${this.origin.slice("http".length)}${GATEWAY_PATH}`;
  }

  /** The `htu` of a gateway proof: always HTTPS with the server's host and port. */
  get gatewayHtu(): string {
    return `https${this.origin.slice("http".length)}${GATEWAY_PATH}`;
  }

  /** The stored `createMessage` bodies (with `client_msg_id`), in order. */
  get messages(): readonly Readonly<Record<string, unknown>>[] {
    return this.world.messageBodies;
  }

  /** The registered commands by ID. */
  get commands(): ReadonlyMap<string, S["Command"]> {
    return this.world.commands;
  }

  /** The heartbeat bodies received (REST and the gateway member `agent`), in order. */
  get heartbeats(): readonly Readonly<Record<string, unknown>>[] {
    return this.world.heartbeats;
  }

  /**
   * Returns every REST request received, in order: method, path and status only.
   *
   * @returns Copies of the records.
   */
  requests(): RecordedRequest[] {
    return this.recorded.map((r) => ({
      method: r.method,
      path: r.path,
      status: r.status,
    }));
  }

  /**
   * Returns the op of every gateway frame received from a client, in order.
   *
   * @returns Copies of the records.
   */
  frames(): RecordedFrame[] {
    return this.frameOps.map((f) => ({ op: f.op }));
  }

  // Life cycle ---------------------------------------------------------------------------

  /**
   * Opens the listener on `127.0.0.1` with a free port.
   *
   * @returns A promise that resolves when the server listens.
   */
  async start(): Promise<void> {
    this.tasks = new AbortController();
    const server = createServer((request, response) => {
      void this.http(request, response);
    });
    server.on("upgrade", (request: IncomingMessage, socket, head: Buffer) => {
      if ((request.url ?? "").split("?")[0] === GATEWAY_PATH)
        this.gateway.upgrade(request, socket, head);
      else socket.destroy();
    });
    await new Promise<void>((resolve, reject) => {
      server.once("error", reject);
      server.listen(0, "127.0.0.1", () => resolve());
    });
    const address = server.address() as AddressInfo;
    this.origin = `http://${address.address}:${address.port}`;
    this.server = server;
  }

  /**
   * Closes every connection and the listener.
   *
   * @returns A promise that resolves when everything is closed.
   */
  async stop(): Promise<void> {
    this.tasks.abort();
    await this.gateway.closeAll(1001, "going_away");
    await this.gateway.shutdown();
    const server = this.server;
    this.server = undefined;
    if (server === undefined) return;
    await new Promise<void>((resolve) => {
      server.close(() => resolve());
      server.closeAllConnections();
    });
  }

  /**
   * Stops the server at the end of an `await using` block.
   *
   * @returns A promise that resolves when the server stopped.
   */
  [Symbol.asyncDispose](): Promise<void> {
    return this.stop();
  }

  // Scenario helpers ---------------------------------------------------------------------

  /**
   * Lets Alex post `text` in a new thread: a queued run and its `agent.invocation`.
   *
   * @param text - The message content.
   * @returns The run, thread and message.
   */
  async trigger(text = "Hello @Lotse"): Promise<Triggered> {
    const run = this.world.trigger(text);
    await settle();
    return {
      runId: run.id,
      threadId: run.threadId,
      messageId: run.triggeringMessageId,
    };
  }

  /**
   * Lets Alex post a follow-up to the run: `agent.followup`.
   *
   * @param runId - The run.
   * @param text - The message content.
   * @returns The follow-up message ID.
   */
  async deliverFollowup(
    runId: string,
    text = "One more thing",
  ): Promise<string> {
    const messageId = this.world.followup(runId, text);
    await settle();
    return messageId;
  }

  /**
   * Lets Alex operate a component of the run: `interaction.created`.
   *
   * @param runId - The run.
   * @param customId - The component's custom ID (created on a card of the run if missing).
   * @param values - The submitted values.
   * @returns The interaction ID.
   */
  async deliverInteraction(
    runId: string,
    customId = "approval.once",
    values: Readonly<Record<string, unknown>> = {},
  ): Promise<string> {
    const interactionId = this.world.interaction(runId, customId, values);
    await settle();
    return interactionId;
  }

  /**
   * Registers a command of Lotse in the channel (as `registerCommand` would).
   *
   * @param name - The command name.
   * @returns The command ID.
   */
  registerCommand(name: string): Promise<string> {
    const command = this.world.registerCommand({
      name,
      description: `The ${name} command`,
      scope_kind: "channel",
      scope_id: this.world.channelId,
    });
    return Promise.resolve(command.id);
  }

  /**
   * Lets Alex run Lotse's command `name` (registered first if missing): `command.invoked`.
   *
   * @param name - The command name.
   * @param options - The run of the thread (its thread is used) or a thread without a run.
   * @returns The invocation ID of the payload.
   */
  async invokeCommand(
    name: string,
    options: { runId?: string; threadId?: string } = {},
  ): Promise<string> {
    let command = [...this.world.commands.values()].find(
      (c) => c.name === name,
    );
    if (command === undefined)
      command = this.world.commands.get(await this.registerCommand(name));
    const invocationId = this.world.commandInvoked(
      command as S["Command"],
      options.runId,
      options.threadId,
    );
    await settle();
    return invocationId;
  }

  /**
   * Sends `command.autocomplete_requested` to a live gateway connection of Lotse.
   *
   * @param commandId - A registered command.
   * @param value - What Alex has typed so far.
   * @param options - `expiresInMs` until `expires_at`; `command.autocomplete_timeout` when absent.
   * @returns The request ID.
   * @throws `RangeError` when the command is not registered, `Error` when no gateway connection
   *   of Lotse is live.
   */
  async requestAutocomplete(
    commandId: string,
    value = "",
    options: { expiresInMs?: number } = {},
  ): Promise<string> {
    const command = this.world.commands.get(canonicalUuid(commandId) ?? "");
    if (command === undefined) throw new RangeError("unknown command");
    const timeout =
      options.expiresInMs ?? this.timings["command.autocomplete_timeout"];
    const envelope = this.world.autocompleteEvent(command, value, timeout);
    if (!(await this.hub.ephemeral(envelope)))
      throw new Error("no live gateway connection");
    return (envelope.payload as S["CommandAutocompleteRequestedPayload"])
      .request_id;
  }

  /**
   * Returns what the adapter answered before `expires_at`, if anything.
   *
   * @param requestId - The request.
   * @returns The choices, or `undefined` when no answer arrived in time.
   */
  autocompleteAnswer(requestId: string): S["CommandChoice"][] | undefined {
    const choices = this.world.autocompleteAnswers.get(
      canonicalUuid(requestId) ?? "",
    );
    return choices === undefined
      ? undefined
      : choices.map((c) => ({ name: c.name, value: c.value }));
  }

  /**
   * Lets a person cancel the run: `run.cancel_requested`.
   *
   * A queued run ends `cancelled` at once; an active run waits `agent.cancel_grace` for the
   * adapter's `cancelled` and then ends with `cancel_timeout`.
   *
   * @param runId - The run.
   * @param reason - The payload's reason.
   * @throws `Error` when the run has completed or failed.
   */
  async cancel(
    runId: string,
    reason: S["RunCancelRequestedPayload"]["reason"] = "user_cancelled",
  ): Promise<void> {
    const run = this.runRecord(runId);
    if (this.world.cancel(run, reason)) void this.cancelTimeout(run);
    await settle();
  }

  private async cancelTimeout(run: RunRecord): Promise<void> {
    try {
      await this.clock.sleep(
        this.timings["agent.cancel_grace"],
        this.tasks.signal,
      );
    } catch {
      return;
    }
    if (run.state === "running" || run.state === "waiting_input")
      this.world.transition(run, "cancelled", "cancel_timeout");
  }

  /**
   * Delivers every unacknowledged event again now, with fresh tokens and `redelivered`.
   *
   * @param consumer - Only this consumer; every consumer when absent.
   */
  async redeliver(consumer?: string): Promise<void> {
    const pick = (name: string): boolean =>
      consumer === undefined || name === consumer;
    const windows = [...this.hub.rest]
      .filter(([name]) => pick(name))
      .map(([, w]) => w);
    const conns = [...this.hub.connections]
      .filter(([name]) => pick(name))
      .map(([, c]) => c);
    for (const window of [...windows, ...conns.map((c) => c.window)])
      window.restartDelivery();
    for (const conn of conns) conn.wake.set();
    await settle();
  }

  /**
   * Delivers an existing event's payload again under a new event ID with `token`.
   *
   * The replay case of AZ-P6: the same token on a second event.
   *
   * @param envelopeId - The ID of an event in the log.
   * @param token - The token the new delivery carries (not recorded on the run).
   * @returns The new event ID.
   * @throws `RangeError` when no event has that ID.
   */
  async deliverWithToken(envelopeId: string, token: Secret): Promise<string> {
    const entry = this.hub.log.find((e) => e.envelope.id === envelopeId);
    if (entry === undefined) throw new RangeError("no such event");
    const envelope: Envelope = { ...entry.envelope, id: uuidv7() };
    delete envelope.seq;
    const added = this.hub.append(entry.type, envelope);
    this.fixedTokens.set(added.seq, token.reveal());
    await settle();
    return envelope.id;
  }

  /**
   * Returns a forged token for the run, built from its newest delivered token.
   *
   * A run without a delivery yet gets the claims its `agent.invocation` would carry; nothing is
   * recorded on the run, so it stays as it is (AG-08 sends these to a queued run).
   *
   * @param runId - The run.
   * @param variant - `bad_signature`, `alg_hs256`, `typ_access` or `aud_other_agent`.
   * @returns The forged token.
   * @throws `RangeError` on an unknown variant or run.
   */
  forgeToken(runId: string, variant: ForgedVariant): Promise<Secret> {
    try {
      if (!FORGE_VARIANTS.includes(variant))
        throw new RangeError(`unknown forge variant: ${String(variant)}`);
      const run = this.runRecord(runId);
      const claims =
        run.issued.get(run.lastJti ?? "") ?? this.world.invocationClaims(run);
      return Promise.resolve(
        this.world.issuer.forge(claims, variant, this.otherAgentId),
      );
    } catch (error) {
      return Promise.reject(error as Error);
    }
  }

  /**
   * Returns the run's state, reason, version and start count.
   *
   * @param runId - The run.
   * @returns The view.
   * @throws `RangeError` on an unknown run.
   */
  runState(runId: string): Promise<RunView> {
    try {
      const run = this.runRecord(runId);
      return Promise.resolve({
        state: run.state,
        stateReason: run.stateReason,
        version: run.version,
        starts: run.starts,
      });
    } catch (error) {
      return Promise.reject(error as Error);
    }
  }

  /**
   * Returns the `getRun` body of the run.
   *
   * @param runId - The run.
   * @returns The Run document.
   * @throws `RangeError` on an unknown run.
   */
  run(runId: string): S["Run"] {
    return this.world.runDoc(this.runRecord(runId));
  }

  /**
   * Returns the consumer's stored acknowledgement and the head.
   *
   * @param consumer - The consumer name.
   * @returns The view.
   * @throws `RangeError` when the consumer never connected.
   */
  consumerState(consumer: string): ConsumerView {
    const cursor = this.hub.cursors.get(consumer);
    if (cursor === undefined) throw new RangeError("unknown consumer");
    return { ackedSeq: cursor.acked, headSeq: this.hub.head };
  }

  /**
   * Restarts the server process and keeps its data.
   *
   * Every gateway connection closes with `4013` (as the server's shutdown does), sessions and
   * delivery windows in memory are forgotten, and one consumer's cursor may be set; the next
   * delivery of every unacknowledged event carries a new token (AG-28).
   *
   * @param consumer - The consumer whose cursor is set.
   * @param ackedSeq - The cursor value (for example the `seq` before an event, AG-28).
   */
  async restart(consumer?: string, ackedSeq?: number): Promise<void> {
    this.gateway.draining = true;
    try {
      await this.gateway.closeAll(4013, "server_restart");
      this.gateway.forgetSessions();
      this.hub.restart(consumer, ackedSeq);
    } finally {
      this.gateway.draining = false;
    }
    await settle();
  }

  /**
   * Moves the shared clock forward ({@link FakeClock.advance}).
   *
   * @param ms - How far, in milliseconds.
   */
  async passTime(ms: number): Promise<void> {
    await this.clock.advance(ms);
    await settle();
  }

  /**
   * Lets the next `times` calls of `target` fail with `fault`.
   *
   * @param target - An `operationId` or a gateway op (`ack`, `run.update`, `heartbeat`, ...);
   *   on a gateway op every fault drops the connection.
   * @param fault - The fault, built with the static methods of {@link Fault}.
   * @param times - How many calls it hits; a positive integer.
   * @throws `TypeError` when `fault` is not a {@link Fault}, `RangeError` on a bad `times`.
   */
  inject(target: string, fault: Fault, times = 1): void {
    if (!(fault instanceof Fault))
      throw new TypeError("fault must be built with the methods of Fault");
    if (!Number.isInteger(times) || times < 1)
      throw new RangeError("times must be a positive integer");
    const queue = this.faults.get(target) ?? [];
    for (let i = 0; i < times; i++) queue.push(fault);
    this.faults.set(target, queue);
  }

  /**
   * Closes every gateway connection with `code`.
   *
   * @param code - A close code of docs/api/asyncapi.yaml.
   */
  async closeConnections(code: number): Promise<void> {
    await this.gateway.closeAll(code, "closed");
    await settle();
  }

  /**
   * Marks an operation deprecated: its answers carry `Deprecation: true` and `Sunset`.
   *
   * @param operationId - The operation.
   * @param sunset - The HTTP date of the `Sunset` header.
   */
  deprecate(
    operationId: string,
    sunset = "Wed, 30 Jun 2027 00:00:00 GMT",
  ): void {
    this.deprecated.set(operationId, sunset);
  }

  /** Starts a new DPoP nonce window now; the previous nonce stays valid one window. */
  rotateNonce(): void {
    this.auth.nonces.rotate();
  }

  /**
   * Posts a webhook signed with the FakeServer's own key (see `getWebhookKeys`) and the given
   * HMAC secrets (webhooks.md 2): `webhook-id` is the envelope's `id`, `webhook-timestamp` the
   * clock's Unix seconds, `webhook-signature` the `v1a` entry first and then the `v1` entries.
   *
   * @param url - The receiver.
   * @param envelope - The event envelope; its `id` is the `webhook-id`.
   * @param options - The HMAC secrets of the `v1` entries.
   * @returns The receiver's response.
   */
  sendWebhook(
    url: string,
    envelope: { readonly id: string },
    options: SendWebhookOptions = {},
  ): Promise<Response> {
    const body = JSON.stringify(envelope);
    const timestamp = Math.floor(this.clock.now() / 1000);
    const signature = signWebhook({
      id: envelope.id,
      timestamp,
      body,
      seeds: [this.webhookSeed],
      secrets: options.hmacSecrets ?? [],
    });
    return fetch(url, {
      method: "POST",
      headers: {
        "content-type": "application/json",
        "webhook-id": envelope.id,
        "webhook-timestamp": String(timestamp),
        "webhook-signature": signature,
      },
      body,
    });
  }

  // Internals ----------------------------------------------------------------------------

  private runRecord(runId: string): RunRecord {
    const run = this.world.runs.get(canonicalUuid(runId) ?? "");
    if (run === undefined) throw new RangeError("unknown run");
    return run;
  }

  private deliver(entry: LogEntry): Envelope | undefined {
    const fixed = this.fixedTokens.get(entry.seq);
    if (fixed !== undefined)
      return { ...entry.envelope, invocation_token: fixed };
    return this.world.deliver(entry);
  }

  private takeGatewayFault(op: string): boolean {
    const queue = this.faults.get(op);
    if (GATEWAY_OPS.has(op) && queue?.length) {
      queue.shift();
      return true;
    }
    return false;
  }

  private takeFault(operationId: string): Fault | undefined {
    return this.faults.get(operationId)?.shift();
  }

  private async http(
    request: IncomingMessage,
    response: ServerResponse,
  ): Promise<void> {
    const method = request.method ?? "GET";
    const path = (request.url ?? "/").split("?")[0] ?? "/";
    const query = new URL(request.url ?? "/", "http://127.0.0.1").searchParams;
    let operationId = "";
    let params: Record<string, string> = {};
    if (path.startsWith(API)) {
      for (const [routeMethod, pattern, op] of COMPILED) {
        const match = pattern.exec(path.slice(API.length));
        if (match && routeMethod === method) {
          operationId = op;
          params = { ...match.groups };
          break;
        }
      }
    }
    const record = { method, path, status: 0 };
    this.recorded.push(record);
    let body: unknown;
    let malformed = false;
    let fault: Fault | undefined;
    let answer: Answer;
    try {
      const raw = await readBody(request);
      if (raw.length) {
        try {
          body = JSON.parse(raw.toString("utf8"));
        } catch {
          malformed = true;
        }
      }
      if (!operationId) throw new FakeProblem("not_found");
      if (malformed) throw new FakeProblem("malformed_request");
      await this.authenticate(operationId, request);
      // Faults hit an authenticated call, so a nonce challenge does not use one up.
      fault = this.takeFault(operationId);
      if (fault?.kind === FaultKind.DISCONNECT) throw new Disconnected();
      if (fault?.kind === FaultKind.RATE_LIMITED)
        throw new FakeProblem("rate_limited", {
          headers: { "Retry-After": String(fault.retryAfter) },
        });
      if (fault?.kind === FaultKind.STORED_LATE)
        await this.clock.sleep(fault.delayMs, this.tasks.signal);
      answer = await this.serve(operationId, params, query, request, body);
      if (fault?.kind === FaultKind.STORED_THEN_502)
        answer = this.problem(
          new FakeProblem("bad_gateway", { status: 502 }),
          path,
        );
    } catch (error) {
      if (error instanceof Disconnected || this.tasks.signal.aborted) {
        request.socket.destroy();
        return;
      }
      answer =
        error instanceof FakeProblem
          ? this.problem(error, path)
          : this.problem(
              new FakeProblem("internal_error", { status: 500 }),
              path,
            );
    }
    record.status = answer.status;
    this.respond(response, answer, operationId);
  }

  private problem(problem: FakeProblem, path: string): Answer {
    return {
      status: problem.status,
      body: problem.body(path),
      headers: {
        "Content-Type": "application/problem+json",
        ...problem.headers,
      },
    };
  }

  private respond(
    response: ServerResponse,
    answer: Answer,
    operationId: string,
  ): void {
    const headers: Record<string, string> = {
      ...answer.headers,
      "Correlation-Id": uuidv7(),
    };
    if (this.credentials.dpop !== undefined)
      headers["DPoP-Nonce"] = this.auth.nonces.current();
    const sunset = this.deprecated.get(operationId);
    if (sunset !== undefined) {
      headers.Deprecation = "true";
      headers.Sunset = sunset;
    }
    if (answer.body === undefined) {
      delete headers["Content-Type"];
      response.writeHead(answer.status, headers).end();
      return;
    }
    headers["Content-Type"] ??= "application/json";
    response.writeHead(answer.status, headers).end(JSON.stringify(answer.body));
  }

  private async authenticate(
    operationId: string,
    request: IncomingMessage,
  ): Promise<void> {
    if (PUBLIC.has(operationId)) return;
    const binding = headerValues(request, "invocation-token");
    const authorization = headerValues(request, "authorization");
    if (binding.length && !authorization.length)
      throw new FakeProblem("unauthenticated", {
        headers: { "WWW-Authenticate": WWW_AUTHENTICATE },
      });
    await this.auth.authenticate({
      method: request.method ?? "GET",
      htu: this.origin + ((request.url ?? "/").split("?")[0] ?? "/"),
      authorization,
      proofs: headerValues(request, "dpop"),
    });
    if (!DELIVERY.has(operationId)) this.world.bind(binding, operationId);
  }

  private async serve(
    operationId: string,
    params: Record<string, string>,
    query: URLSearchParams,
    request: IncomingMessage,
    body: unknown,
  ): Promise<Answer> {
    const key = headerValues(request, "idempotency-key")[0];
    if (key && IDEMPOTENT_KEY_OPS.has(operationId)) {
      const path = (request.url ?? "/").split("?")[0] ?? "/";
      const signature = canonicalJson([request.method, path, body ?? null]);
      const slot = `${operationId}\u0000${key}`;
      const stored = this.idempotency.get(slot);
      if (stored !== undefined) {
        if (stored[0] !== signature)
          throw new FakeProblem("idempotency_key_reuse");
        if (stored[1] === undefined)
          throw new FakeProblem("request_in_progress");
        return {
          ...stored[1],
          headers: { ...stored[1].headers, "Idempotent-Replay": "true" },
        };
      }
      this.idempotency.set(slot, [signature, undefined]);
      try {
        const answer = await this.operation(
          operationId,
          params,
          query,
          request,
          body,
        );
        this.idempotency.set(slot, [signature, answer]);
        return answer;
      } catch (error) {
        this.idempotency.delete(slot);
        throw error;
      }
    }
    return this.operation(operationId, params, query, request, body);
  }

  private async operation(
    op: string,
    p: Record<string, string>,
    query: URLSearchParams,
    request: IncomingMessage,
    body: unknown,
  ): Promise<Answer> {
    const world = this.world;
    const ok = (
      status: number,
      out: unknown,
      headers: Record<string, string> = {},
    ): Answer => ({
      status,
      body: out,
      headers,
    });
    const ifMatch = headerValues(request, "if-match")[0];
    const param = (name: string): string => p[name] ?? "";
    switch (op) {
      case "getDiscovery": {
        const override = {
          instance_id: world.instanceId,
          gateway: { url: GATEWAY_PATH, protocol_versions: [1] },
          jwks_uri: `${API}/instance/jwks`,
          webhook_keys_uri: `${API}/instance/webhook-keys`,
        } satisfies Partial<S["Discovery"]>;
        return ok(200, fixtures.build("discovery", override) as S["Discovery"]);
      }
      case "getJwks":
        return ok(200, world.issuer.jwks(), {
          "Content-Type": "application/jwk-set+json",
          "Cache-Control": this.cacheControl(),
        });
      case "getWebhookKeys": {
        const keys: S["WebhookKeys"] = {
          keys: [
            {
              kid: "whk-fake-1",
              public_key: webhookPublicKey(this.webhookSeed),
              state: "active",
            },
          ],
        };
        return ok(200, keys, {
          "Cache-Control": this.cacheControl(),
          "Access-Control-Allow-Origin": "*",
        });
      }
      case "pollAgentEvents":
      case "ackAgentEvents":
      case "getConsumerState": {
        const consumer = param("consumer");
        if (!consumerOk(consumer))
          throw fieldError("#path/consumer", "pattern_mismatch");
        if (op === "getConsumerState") return ok(200, this.hub.state(consumer));
        if (op === "ackAgentEvents") {
          if (
            !isPlainObject(body) ||
            Object.keys(body).some((k) => k !== "seq") ||
            !("seq" in body)
          )
            throw fieldError("/seq", "required");
          const seq = body.seq;
          if (typeof seq !== "number" || !Number.isInteger(seq) || seq < 0)
            throw fieldError("/seq", "too_small");
          return ok(200, this.hub.ack(consumer, seq));
        }
        const limit = query.get("limit");
        if (limit !== null && (!/^\d+$/.test(limit) || Number(limit) < 1))
          throw fieldError("#query/limit", "too_small");
        const page = await this.hub.poll(
          consumer,
          limit ? Number(limit) : undefined,
        );
        const out = {
          ...page,
          events: page.events as S["Event"][],
          sync_epoch: world.syncEpoch,
        } satisfies S["EventPage"];
        return ok(200, out);
      }
      case "sendHeartbeat":
        return ok(200, world.heartbeat(body));
      case "getAgentTriggerers": {
        const out: S["AgentTriggerers"] = {
          principal_ids: [world.alex.id, world.robin.id],
          permission_version: world.permissionVersion,
        };
        return ok(200, out);
      }
      case "listServedChannels": {
        const out: S["AgentChannelConfigList"] = {
          items: [world.channelConfig()],
        };
        return ok(200, out);
      }
      case "getServedChannelConfig":
        if (param("channel_id") !== world.channelId)
          throw new FakeProblem("not_found");
        return ok(200, world.channelConfig(), {
          ETag: `"${world.settingsVersion}"`,
        });
      case "getRun": {
        const run = world.runOr404(param("run_id"));
        return ok(200, world.runDoc(run), { ETag: `"${run.version}"` });
      }
      case "getRunContext":
        return ok(200, world.context(param("run_id")));
      case "patchRun": {
        if (!isPlainObject(body)) throw fieldError("", "required");
        const unknown = Object.keys(body)
          .filter((k) => k !== "base_version" && k !== "ops")
          .sort(byCodeUnit);
        if (unknown.length)
          throw fieldError(`/${unknown[0]}`, "unknown_member");
        const version = body.base_version;
        if (typeof version !== "number" || !Number.isInteger(version))
          throw fieldError(
            "/base_version",
            version === undefined || version === null
              ? "required"
              : "invalid_type",
          );
        if (!body.ops || (Array.isArray(body.ops) && !body.ops.length))
          throw fieldError("/ops", "too_short");
        const out: S["RunPatchResult"] = {
          version: world.patch(param("run_id"), version, body.ops),
        };
        return ok(200, out);
      }
      case "startRun":
      case "resumeRun": {
        const [doc, replay] = world.start(
          param("run_id"),
          body,
          op === "resumeRun",
        );
        await settle();
        return ok(200, doc, replay ? { "Idempotent-Replay": "true" } : {});
      }
      case "renewInvocationToken":
        return ok(200, world.renew(param("run_id"), body));
      case "awaitRunInput":
        return ok(200, world.awaitInput(param("run_id"), body));
      case "finishRun":
        return ok(200, world.finish(param("run_id"), body));
      case "createMessage": {
        const [message, created] = world.createMessage(
          param("thread_id"),
          body,
        );
        return created
          ? ok(201, message)
          : ok(200, message, { "Idempotent-Replay": "true" });
      }
      case "updateComponent": {
        const component = world.updateComponent(
          param("component_id"),
          body,
          ifMatch,
        );
        return ok(200, component, { ETag: `"${component.version}"` });
      }
      case "respondToInteraction":
        return ok(200, world.respond(param("interaction_id"), body));
      case "registerCommand": {
        const command = world.registerCommand(body);
        return ok(201, command, { ETag: `"${command.version}"` });
      }
      case "updateCommand": {
        const command = world.updateCommand(param("command_id"), body, ifMatch);
        return ok(200, command, { ETag: `"${command.version}"` });
      }
      case "removeCommand":
        world.removeCommand(param("command_id"));
        return ok(204, undefined);
      case "listChannelCommands":
        return ok(200, world.channelCommands(param("channel_id")));
      default:
        return this.answerAutocomplete(param("request_id"), body);
    }
  }

  private cacheControl(): string {
    return `public, max-age=${Math.trunc(this.timings["agent.jwks_max_age"] / 1000)}`;
  }

  private answerAutocomplete(requestId: string, body: unknown): Answer {
    const key = canonicalUuid(requestId);
    const pending =
      key === undefined ? undefined : this.world.autocomplete.get(key);
    if (key === undefined || pending === undefined)
      throw new FakeProblem("not_found");
    if (this.world.now() > pending.expires) throw new FakeProblem("gone");
    if (!isPlainObject(body) || !Array.isArray(body.choices))
      throw fieldError("/choices", "required");
    if (!this.world.autocompleteAnswers.has(key))
      this.world.autocompleteAnswers.set(key, [
        ...(body.choices as S["CommandChoice"][]),
      ]);
    return { status: 204, body: undefined, headers: {} };
  }
}
