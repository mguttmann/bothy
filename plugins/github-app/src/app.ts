// SPDX-License-Identifier: Apache-2.0
/**
 * Start and stop of the app (brief M2-49 Design notes 3, 4, 8 and 13).
 *
 * {@link startApp} checks the secret files and the state directory, opens the store (lock),
 * creates the SDK client with the app's token (read once), asks `getMe` and `getInstance`,
 * starts the worker and the HTTP listener, and returns the listening address with `stop()`.
 * `fetch`, the clock, the logger, the chat port and the store are injectable for tests. After a
 * `401 unauthenticated` the endpoints answer `503` until `getMe` succeeds again, tried once per
 * `sdk.retry_backoff_cap`. `stop()` stops accepting (later requests get `503`), lets the running
 * step finish and releases the lock.
 *
 * @example
 * ```ts
 * const app = await startApp(await loadConfig(file));
 * console.log(app.port);
 * await app.stop();
 * ```
 *
 * @module
 */

import {
  DEFAULTS,
  createApiClient,
  loadDpopKey,
  systemClock,
  type Clock,
  type Logger,
} from "bothy-sdk";

import type { ChatPort, InstanceLimits } from "./chat/port.js";
import {
  eventsRoute,
  githubWebhookRoute,
  interactionsRoute,
  type ReceiveContext,
} from "./chat/receive.js";
import { SdkChatPort } from "./chat/sdk-adapter.js";
import { checkStartFiles, readSecretFile, type AppConfig } from "./config.js";
import { APP_VERSION, GitHubClient } from "./github/client.js";
import { startHttpServer } from "./http/server.js";
import { createLogger, sdkLogger } from "./log.js";
import { FileStore } from "./store/file-store.js";
import type { Store } from "./store/store.js";
import { JobRunner } from "./worker/jobs.js";
import { Worker } from "./worker/worker.js";

/** The routes of the app. */
export const ROUTES = Object.freeze({
  /** GitHub deliveries. */
  github: "/github/webhook",
  /** `delivery.interactions_url` of the manifest. */
  interactions: "/chat/interactions",
  /** `delivery.events_url` of the manifest (mirroring only). */
  events: "/chat/events",
});

/** Injectable dependencies of {@link startApp}; everything defaults to production. */
export interface AppDeps {
  /** `fetch` of the SDK client and the GitHub client (tests pass the guarded one). */
  readonly fetch?: typeof globalThis.fetch | undefined;
  /** The clock (default the system clock). */
  readonly clock?: Clock | undefined;
  /** The logger (default JSON lines on stdout). */
  readonly logger?: Logger | undefined;
  /** The port to the server (default the SDK adapter over the app's token). */
  readonly chat?: ChatPort | undefined;
  /** The store (default `FileStore` in `github_app.state_dir`). */
  readonly store?: Store | undefined;
  /** Overrides of `sdk.http_timeout`, `sdk.retry_*` (tests use small values). */
  readonly settings?: Partial<typeof DEFAULTS> | undefined;
}

/** A running app. */
export interface RunningApp {
  /** The bound host of the listener. */
  readonly host: string;
  /** The bound port of the listener. */
  readonly port: number;
  /** The base URL of the listener, `http://<host>:<port>`. */
  readonly url: string;
  /**
   * Resolves once the queue is empty and the worker waits (tests).
   *
   * @returns A promise of the next idle moment.
   */
  idle(): Promise<void>;
  /** Stops accepting, lets the running step finish and releases the lock. */
  stop(): Promise<void>;
}

/**
 * Starts the app.
 *
 * @param config - The loaded configuration.
 * @param deps - Injectable dependencies.
 * @returns The running app.
 * @throws `ConfigError` for a secret file or state directory that fails its check,
 *   `StoreLockedError` when another process runs on the state directory, the SDK's errors when
 *   `getMe` or `getInstance` fail, and `Error` when the address cannot be bound.
 */
export async function startApp(
  config: AppConfig,
  deps: AppDeps = {},
): Promise<RunningApp> {
  const clock = deps.clock ?? systemClock;
  const logger = deps.logger ?? createLogger({ now: () => clock.now() });
  const settings = { ...DEFAULTS, ...deps.settings };
  await checkStartFiles(config);
  const store =
    deps.store ??
    new FileStore({
      dir: config.stateDir,
      clock,
      windowMs: config.dedupeWindowMs,
      logger,
    });
  await store.open();
  let worker: Worker | undefined;
  let stopTimer: AbortController | undefined;
  try {
    const clientOptions = {
      baseUrl: `${config.serverUrl}/api/v1`,
      ...(deps.fetch === undefined ? {} : { fetch: deps.fetch }),
      logger: sdkLogger(logger),
      userAgent: `github-app/${APP_VERSION}`,
      clock,
      settings,
    };
    const chat =
      deps.chat ??
      new SdkChatPort(
        createApiClient({
          ...clientOptions,
          token: await readSecretFile(
            config.tokenFile,
            "github_app.token_file",
          ),
          dpopKey:
            config.dpopKeyFile === ""
              ? undefined
              : await loadDpopKey(config.dpopKeyFile),
        }),
        createApiClient(clientOptions),
      );
    const me = await chat.me();
    let limits: InstanceLimits = await chat.limits();
    let authenticated = true;
    let stopping = false;
    stopTimer = new AbortController();
    const signal = stopTimer.signal;
    // After a 401: getMe once per sdk.retry_backoff_cap until it succeeds.
    const recheck = async (): Promise<void> => {
      while (!authenticated && !signal.aborted) {
        await clock
          .sleep(settings.retryBackoffCapMs, signal)
          .catch(() => undefined);
        if (signal.aborted) return;
        try {
          await chat.me();
          authenticated = true;
          logger.info("app.authenticated", { reason: "get_me" });
        } catch {
          // Still refused or unreachable; try again after the pause.
        }
      }
    };
    const github = new GitHubClient({
      apiUrl: config.githubApiUrl,
      token: () =>
        readSecretFile(config.githubTokenFile, "github_app.github_token_file"),
      timeoutMs: settings.httpTimeoutMs,
      fetch: deps.fetch,
    });
    const runner = new JobRunner({
      config,
      store,
      chat,
      github,
      logger,
      now: () => clock.now(),
      limits: () => limits,
      refreshLimits: async () => {
        limits = await chat.limits();
      },
      onUnauthenticated: () => {
        if (!authenticated) return;
        authenticated = false;
        void recheck();
      },
    });
    worker = new Worker({
      store,
      run: (job) => runner.run(job),
      clock,
      pauseMs: settings.retryBackoffCapMs,
      logger,
    });
    const running = worker;
    const context: ReceiveContext = {
      config,
      store,
      chat,
      webhookSecret: () =>
        readSecretFile(
          config.webhookSecretFile,
          "github_app.webhook_secret_file",
        ),
      ownPrincipalId: () => me.id,
      wake: () => {
        running.wake();
      },
      clock,
      logger,
    };
    const http = await startHttpServer({
      host: config.listenHost,
      port: config.listenPort,
      maxBodyBytes: config.maxBodyBytes,
      routes: {
        [ROUTES.github]: githubWebhookRoute(context),
        [ROUTES.interactions]: interactionsRoute(context),
        [ROUTES.events]: eventsRoute(context),
      },
      ready: () => authenticated && !stopping,
      logger,
    });
    running.start();
    const host = http.host.includes(":") ? `[${http.host}]` : http.host;
    logger.info("app.started", { status: 0 });
    let stopped: Promise<void> | undefined;
    return {
      host: http.host,
      port: http.port,
      url: `http://${host}:${String(http.port)}`,
      idle: () => running.idle(),
      stop: () => {
        stopped ??= (async () => {
          stopping = true;
          stopTimer?.abort();
          await running.stop();
          await http.close();
          await store.close();
          logger.info("app.stopped", { status: 0 });
        })();
        return stopped;
      },
    };
  } catch (err) {
    stopTimer?.abort();
    await worker?.stop();
    await store.close();
    throw err;
  }
}
