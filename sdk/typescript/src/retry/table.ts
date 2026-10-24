// SPDX-License-Identifier: Apache-2.0
/**
 * The operations of docs/api/openapi.yaml that write without `Idempotency-Key` and may still be
 * repeated (docs/spec/06-api.md 7 point 3, docs/decisions.d/M2-30.md D-M2-30-3).
 *
 * @module
 */

/**
 * The `x-idempotency-exempt` reasons that make a request repeatable: `agent_runtime` (monotone
 * or compare-and-swap writes of the agent runtime), `client_id` (the ID is in the body, so a
 * repeat gives the first answer) and `read_only` (no state change). The other reasons
 * (`auth_flow`, `telemetry`, `origin_key`) promise nothing for a repeat.
 */
export const RETRY_EXEMPT_REASONS: readonly string[] = Object.freeze([
  "agent_runtime",
  "client_id",
  "read_only",
]);

/**
 * Every operation of docs/api/openapi.yaml whose `x-idempotency-exempt` reason is one of
 * {@link RETRY_EXEMPT_REASONS}, keyed `METHOD /path/template` (the path below `/api/v1`), with
 * its reason. `test/retry.test.ts` holds it equal to the specification.
 */
export const IDEMPOTENT_EXEMPT_OPERATIONS: ReadonlyMap<string, string> =
  new Map([
    ["PATCH /runs/{run_id}", "agent_runtime"], // patchRun
    ["POST /agents/me/consumers/{consumer}/ack", "agent_runtime"], // ackAgentEvents
    ["POST /agents/me/heartbeat", "agent_runtime"], // sendHeartbeat
    ["POST /app-manifests/validate", "read_only"], // validateManifest
    ["POST /audit/verify", "read_only"], // verifyAuditChain
    [
      "POST /channels/{channel_id}/commands/{command_id}/autocomplete",
      "read_only",
    ], // autocompleteCommand
    ["POST /command-autocompletions/{request_id}/answer", "agent_runtime"], // answerAutocomplete
    ["POST /components/{component_id}/interactions", "client_id"], // createInteraction
    ["POST /files/{file_id}/tickets", "read_only"], // createFileTicket
    ["POST /grants/preview", "read_only"], // previewGrant
    ["POST /hooks/{webhook_id}", "client_id"], // postIncomingWebhookWithBearer
    ["POST /hooks/{webhook_id}/{token}", "client_id"], // postIncomingWebhook
    ["POST /runs/{run_id}/await-input", "agent_runtime"], // awaitRunInput
    ["POST /runs/{run_id}/finish", "agent_runtime"], // finishRun
    ["POST /runs/{run_id}/invocation-token", "agent_runtime"], // renewInvocationToken
    ["POST /runs/{run_id}/resume", "agent_runtime"], // resumeRun
    ["POST /runs/{run_id}/start", "agent_runtime"], // startRun
    ["POST /threads/{thread_id}/messages", "client_id"], // createMessage
  ]);

/** The path prefix of every operation (`servers` of docs/api/openapi.yaml). */
export const API_PREFIX = "/api/v1";

interface CompiledRoute {
  readonly key: string;
  readonly method: string;
  readonly pattern: RegExp;
  readonly templated: number;
}

const ROUTES: readonly CompiledRoute[] = [
  ...IDEMPOTENT_EXEMPT_OPERATIONS.keys(),
]
  .map((key) => {
    const [method = "", path = ""] = key.split(" ");
    const source = path
      .split(/\{[^}]+\}/)
      .map((part) => part.replace(/[.*+?^${}()|[\]\\]/g, "\\$&"))
      .join("[^/]+");
    return {
      key,
      method,
      pattern: new RegExp(`^${source}$`),
      templated: path.split("{").length - 1,
    };
  })
  // Literal routes win over templated ones of the same shape (for example `/agents/me/...`).
  .sort((a, b) => a.templated - b.templated);

/**
 * Returns the path of a request below {@link API_PREFIX}, without origin, query and fragment.
 *
 * @param pathOrUrl - An absolute URL, a path with the prefix or a path below it.
 * @returns The path below the prefix.
 */
export function apiPath(pathOrUrl: string): string {
  let path = pathOrUrl;
  if (/^[a-z][a-z0-9+.-]*:\/\//i.test(path)) path = new URL(path).pathname;
  path = path.replace(/[?#].*$/s, "");
  const at = path.indexOf(`${API_PREFIX}/`);
  return at >= 0 ? path.slice(at + API_PREFIX.length) : path;
}

/**
 * Returns the key of {@link IDEMPOTENT_EXEMPT_OPERATIONS} a request matches, if any.
 *
 * @param method - The HTTP method.
 * @param pathOrUrl - The request URL, its path or a path template.
 * @returns The key `METHOD /path/template`, or `undefined`.
 *
 * @example
 * ```ts
 * exemptOperationOf("POST", "https://example.com/api/v1/runs/r1/start");
 * // "POST /runs/{run_id}/start"
 * ```
 */
export function exemptOperationOf(
  method: string,
  pathOrUrl: string,
): string | undefined {
  const upper = method.toUpperCase();
  const path = apiPath(pathOrUrl);
  return ROUTES.find((r) => r.method === upper && r.pattern.test(path))?.key;
}
