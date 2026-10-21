// SPDX-License-Identifier: Apache-2.0
/**
 * The trust list of `v1a` public keys (docs/api/webhooks.md 3 point 3).
 *
 * @module
 */

import type { KeyObject } from "node:crypto";

import type { Client } from "openapi-fetch";

import { systemClock, type Clock } from "../clock.js";
import { DEFAULTS, type SdkSettings } from "../config.js";
import type { components, paths } from "../generated/openapi.js";
import { parseWebhookPublicKey } from "./keys.js";

type WebhookKeyState = components["schemas"]["WebhookKey"]["state"];

// Every state the server lists is trusted: `next` signs soon, `retiring` still signs until
// `not_after`. A state of a newer server is not trusted, since its meaning is unknown.
const TRUSTED_STATES: ReadonlySet<string> = new Set<WebhookKeyState>([
  "next",
  "active",
  "retiring",
]);

/** The input of {@link WebhookKeyCache}. */
export interface WebhookKeyCacheOptions {
  /** A client of the public API, for example of `createApiClient`; only `GET` is used. */
  readonly client: Pick<Client<paths>, "GET">;
  /** Time source of the cache age; default the system clock. */
  readonly clock?: Clock;
  /**
   * Settings: `agent.jwks_max_age` without a `max-age`, `adapter.jwks_refresh_min_interval`
   * as the shortest age; default {@link DEFAULTS}.
   */
  readonly settings?: SdkSettings;
}

function maxAgeMs(cacheControl: string | null): number | undefined {
  for (const directive of (cacheControl ?? "").split(",")) {
    const [name, value] = directive.trim().split("=", 2);
    if (name?.toLowerCase() !== "max-age" || value === undefined) continue;
    const seconds = value.trim().replace(/^"(.*)"$/, "$1");
    return /^\d+$/.test(seconds) ? Number(seconds) * 1000 : undefined;
  }
  return undefined;
}

/**
 * Holds the `v1a` public keys of `getWebhookKeys` as a trust list.
 *
 * The list is fetched on first use and again only after the `max-age` of its response's
 * `Cache-Control` (without one: `agent.jwks_max_age`; never sooner than
 * `adapter.jwks_refresh_min_interval`). A new key appears in it beforehand as `next`, so a
 * failed verification is never a reason to fetch: a forged request cannot make the app call
 * the server.
 *
 * @example
 * ```ts
 * const cache = new WebhookKeyCache({ client: createApiClient({ baseUrl }) });
 * const verified = verifyWebhook({ body, headers, keys: await cache.keys() });
 * ```
 */
export class WebhookKeyCache {
  private readonly client: Pick<Client<paths>, "GET">;
  private readonly clock: Clock;
  private readonly settings: SdkSettings;
  private cached: KeyObject[] | undefined;
  private expires = 0;
  private pending: Promise<KeyObject[]> | undefined;

  /**
   * Creates the cache; nothing is fetched before {@link WebhookKeyCache.keys}.
   *
   * @param options - The client, clock and settings.
   */
  constructor(options: WebhookKeyCacheOptions) {
    this.client = options.client;
    this.clock = options.clock ?? systemClock;
    this.settings = options.settings ?? DEFAULTS;
  }

  /**
   * Returns the trusted keys, fetching them when the list is missing or older than its
   * age. Concurrent calls share one fetch.
   *
   * @returns Every usable listed key in the states `next`, `active` and `retiring`.
   * @throws `Error` naming `getWebhookKeys` and the status when the fetch fails; the next
   *   call tries again.
   */
  keys(): Promise<KeyObject[]> {
    if (this.cached !== undefined && this.clock.monotonic() < this.expires) {
      return Promise.resolve(this.cached);
    }
    this.pending ??= this.fetch().finally(() => {
      this.pending = undefined;
    });
    return this.pending;
  }

  private async fetch(): Promise<KeyObject[]> {
    const { data, response } = await this.client.GET("/instance/webhook-keys");
    if (data === undefined) {
      throw new Error(
        `getWebhookKeys failed with status ${String(response.status)}`,
      );
    }
    const keys: KeyObject[] = [];
    for (const listed of data.keys) {
      if (!TRUSTED_STATES.has(listed.state)) continue;
      try {
        keys.push(parseWebhookPublicKey(listed.public_key));
      } catch {
        // A key this SDK cannot read signs nothing it could verify; the others stay usable.
      }
    }
    // agent.jwks_max_age without a max-age; adapter.jwks_refresh_min_interval as the floor.
    const age = Math.max(
      maxAgeMs(response.headers.get("cache-control")) ??
        this.settings.jwksMaxAgeMs,
      this.settings.jwksRefreshMinIntervalMs,
    );
    this.cached = keys;
    this.expires = this.clock.monotonic() + age;
    return keys;
  }
}
