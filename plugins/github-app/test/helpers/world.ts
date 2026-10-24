// SPDX-License-Identifier: Apache-2.0
/**
 * The test world of the unit tests: a temporary state directory with secret files of mode
 * `0600`, a configuration on loopback addresses only, the Fake-GitHub API, the in-memory chat
 * port, and the app started with the guarded `fetch`, a captured logger and small SDK waits.
 *
 * @module
 */

import { randomUUID } from "node:crypto";
import { mkdtemp, rm, writeFile } from "node:fs/promises";
import { tmpdir } from "node:os";
import { join } from "node:path";

import { uuidv7, type Clock } from "bothy-sdk";

import { startApp, type RunningApp } from "../../src/app.js";
import { isLoopbackHost, parseConfig, type AppConfig } from "../../src/config.js";
import { createLogger } from "../../src/log.js";
import type { Store } from "../../src/store/store.js";
import { FakeChat } from "../fakes/chat.js";
import { FakeGitHub, type DeliverOptions } from "../fakes/github-api.js";
import { guardedFetch } from "../setup/no-network.js";
import { REPOSITORY } from "./fixtures.js";
import { PATIENCE_MS } from "./patience.js";

/** The forum of {@link REPOSITORY}. */
export const FORUM = "0198a4f2-6c1e-7a3b-9d2e-4f5a6b7c8d90";

/** Values the logs test looks for; every one is a secret or foreign text. */
export const SECRETS = Object.freeze({
  githubToken: `ghp-test-${randomUUID()}`,
  webhookSecret: `whsec-github-${randomUUID()}`,
  appToken: `app-token-${randomUUID()}`,
});

/**
 * Builds a configuration from members of `github_app` and refuses a server or GitHub address
 * outside the loopback host (brief M2-49 Design note 18).
 *
 * @param members - The members.
 * @returns The configuration.
 * @throws `Error` for a non-loopback `server_url` or `github_api_url`.
 */
export function testConfig(members: Record<string, unknown>): AppConfig {
  for (const key of ["server_url", "github_api_url"]) {
    const value = members[key];
    if (typeof value !== "string" || !isLoopbackHost(new URL(value).hostname))
      throw new Error(`the test configuration needs a loopback ${key}`);
  }
  return parseConfig(JSON.stringify({ github_app: members }), "test-config.json");
}

/** Options of {@link startWorld}. */
export interface WorldOptions {
  /** Extra repositories besides {@link REPOSITORY} (`[repository, forum]`). */
  readonly repositories?: readonly (readonly [string, string])[];
  /** Overrides of `github_app` members. */
  readonly members?: Record<string, unknown>;
  /** The store (default `FileStore` in the world's directory). */
  readonly store?: Store;
  /** The clock. */
  readonly clock?: Clock;
  /** An existing directory (a restart); default a new one. */
  readonly dir?: string;
  /** An existing chat fake (a restart). */
  readonly chat?: FakeChat;
  /** An existing GitHub fake (a restart). */
  readonly github?: FakeGitHub;
  /** Overrides of the SDK settings (`sdk.http_timeout` of a time-out test). */
  readonly settings?: { readonly httpTimeoutMs?: number };
}

/** A running world. */
export interface World {
  /** The temporary directory. */
  readonly dir: string;
  /** The configuration. */
  readonly config: AppConfig;
  /** The in-memory chat port. */
  readonly chat: FakeChat;
  /** The Fake-GitHub API. */
  readonly github: FakeGitHub;
  /** The app. */
  readonly app: RunningApp;
  /** Every log line, parsed. */
  readonly logs: Record<string, unknown>[];
  /** Every log line as written. */
  readonly rawLogs: string[];
  /**
   * Sends a GitHub delivery to the app.
   *
   * @returns The status.
   */
  deliver(event: string, payload: unknown, options?: DeliverOptions): Promise<number>;
  /**
   * Sends a signed request of the server to the app.
   *
   * @returns The response.
   */
  serverRequest(path: string, event: Record<string, unknown>, id?: string): Promise<Response>;
  /** Stops the app (the fakes keep running). */
  stopApp(): Promise<void>;
  /** Stops the app and the fakes and removes the directory. */
  close(): Promise<void>;
}

/**
 * Starts a world.
 *
 * @param options - Repositories, members, store, clock and the parts of a restart.
 * @returns The world.
 */
export async function startWorld(options: WorldOptions = {}): Promise<World> {
  const dir = options.dir ?? (await mkdtemp(join(tmpdir(), "github-app-test-")));
  const repositories = [[REPOSITORY, FORUM] as const, ...(options.repositories ?? [])];
  const github =
    options.github ??
    (await FakeGitHub.start({
      token: SECRETS.githubToken,
      secret: SECRETS.webhookSecret,
      repositories: repositories.map(([r]) => r),
    }));
  const chat = options.chat ?? new FakeChat();
  if (options.dir === undefined) {
    await writeFile(join(dir, "token"), SECRETS.appToken, { mode: 0o600 });
    await writeFile(join(dir, "webhook-secret"), SECRETS.webhookSecret, { mode: 0o600 });
    await writeFile(join(dir, "github-token"), SECRETS.githubToken, { mode: 0o600 });
  }
  const config = testConfig({
    listen: "127.0.0.1:0",
    server_url: "http://127.0.0.1:9",
    token_file: join(dir, "token"),
    webhook_secret_file: join(dir, "webhook-secret"),
    github_token_file: join(dir, "github-token"),
    github_api_url: github.url,
    repositories: repositories.map(([repository, channel_id]) => ({ repository, channel_id })),
    state_dir: join(dir, "state"),
    ...options.members,
  });
  const rawLogs: string[] = [];
  const logs: Record<string, unknown>[] = [];
  const logger = createLogger({
    level: "debug",
    write: (line) => {
      rawLogs.push(line);
      logs.push(JSON.parse(line) as Record<string, unknown>);
    },
  });
  const app = await startApp(config, {
    fetch: guardedFetch,
    chat,
    logger,
    ...(options.store === undefined ? {} : { store: options.store }),
    ...(options.clock === undefined ? {} : { clock: options.clock }),
    // No answer of the fakes comes close to the patience; the pause after a transient failure
    // is short, because the fakes recover at once.
    settings: {
      httpTimeoutMs: options.settings?.httpTimeoutMs ?? PATIENCE_MS,
      retryBackoffCapMs: 10,
      retryBackoffBaseMs: 1,
    },
  });
  github.autoEcho = { url: `${app.url}/github/webhook` };
  let appStopped = false;
  const stopApp = async (): Promise<void> => {
    if (appStopped) return;
    appStopped = true;
    await app.stop();
  };
  return {
    dir,
    config,
    chat,
    github,
    app,
    logs,
    rawLogs,
    deliver: (event, payload, deliverOptions) =>
      github.deliver(`${app.url}/github/webhook`, event, payload, deliverOptions),
    serverRequest: (path, event, id = uuidv7()) => {
      const body = JSON.stringify({ ...event, id });
      return guardedFetch(`${app.url}${path}`, {
        method: "POST",
        headers: chat.sign(body, id),
        body,
      });
    },
    stopApp,
    close: async () => {
      await stopApp();
      await github.stop();
      await rm(dir, { recursive: true, force: true });
    },
  };
}

/**
 * The envelope of a server event (`EventEnvelope` members the app reads, plus the delivery
 * member `sync_epoch`).
 *
 * @param type - The event type.
 * @param payload - The payload.
 * @returns The envelope without `id` (the request adds it).
 */
export function envelope(type: string, payload: Record<string, unknown>): Record<string, unknown> {
  return {
    type,
    schema_version: 1,
    occurred_at: new Date().toISOString(),
    scope: { kind: "channel", id: FORUM },
    resource: { kind: "message", id: uuidv7() },
    sync_epoch: uuidv7(),
    payload,
  };
}

/**
 * Logs whose `msg` or `reason` equals `value`.
 *
 * @param world - The world.
 * @param value - The code or reason.
 * @returns The matching lines.
 */
export function logsWith(world: World, value: string): Record<string, unknown>[] {
  return world.logs.filter((l) => l.msg === value || l.reason === value);
}
