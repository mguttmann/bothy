// SPDX-License-Identifier: Apache-2.0
/**
 * Configuration of the app (docs/spec/11-extensions.md 16, place "GitHub app"; brief M2-49
 * Design note 3).
 *
 * A JSON file holds one object `github_app` whose members are the keys `github_app.*` without
 * their prefix. Unknown members, wrong types, bad durations and missing required keys abort the
 * start; every message names the key and the file and never a value. Durations follow
 * docs/operations/configuration.md section 2 (`(0|[1-9][0-9]*)(ms|s|m|h|d)`). The reused keys
 * of the SDK (`webhook.receiver_tolerance`, `sdk.http_timeout`, `sdk.retry_*`) are not part of
 * the file; their values are the SDK's `DEFAULTS`.
 *
 * Secret files (`token_file`, `dpop_key_file`, `webhook_secret_file`, `github_token_file`) must
 * have mode `0400` or `0600` and belong to the process user ({@link readSecretFile}); the state
 * directory is created with mode `0700`, and an existing one with group or other access is
 * refused ({@link prepareStateDir}).
 *
 * @example
 * ```ts
 * const config = await loadConfig("/etc/github-app/config.json");
 * await checkStartFiles(config);
 * ```
 *
 * @module
 */

import { constants } from "node:fs";
import { mkdir, open, readFile, stat } from "node:fs/promises";
import { isIP } from "node:net";

/** One configured repository and its forum. */
export interface RepositoryMapping {
  /** `<owner>/<name>` as configured; compared without case. */
  readonly repository: string;
  /** The forum channel the events of this repository go to. */
  readonly channelId: string;
}

/** The loaded configuration; durations in milliseconds. */
export interface AppConfig {
  /** The file the configuration was read from, for messages. */
  readonly file: string;
  /** `github_app.listen`: host of the HTTP listener. */
  readonly listenHost: string;
  /** `github_app.listen`: port of the HTTP listener (0 lets the system choose). */
  readonly listenPort: number;
  /** `github_app.server_url`: base URL of the instance, without the API prefix. */
  readonly serverUrl: string;
  /** `github_app.token_file`: file with the app's API token. */
  readonly tokenFile: string;
  /** `github_app.dpop_key_file`: key file of a DPoP-bound token; empty for none. */
  readonly dpopKeyFile: string;
  /** `github_app.webhook_secret_file`: file with the secret of the GitHub webhook. */
  readonly webhookSecretFile: string;
  /** `github_app.github_token_file`: file with the token for the GitHub REST API. */
  readonly githubTokenFile: string;
  /** `github_app.github_api_url`: base URL of the GitHub REST API. */
  readonly githubApiUrl: string;
  /** `github_app.repositories`: the repositories and their forums. */
  readonly repositories: readonly RepositoryMapping[];
  /** `github_app.state_dir`: the state directory. */
  readonly stateDir: string;
  /** `github_app.max_body_bytes`: largest request body on each endpoint. */
  readonly maxBodyBytes: number;
  /** `github_app.dedupe_window`: how long identifiers, created objects and drafts are kept. */
  readonly dedupeWindowMs: number;
  /** `github_app.max_queued_jobs`: open jobs above which the endpoints answer `503`. */
  readonly maxQueuedJobs: number;
}

/**
 * The defaults of the optional keys, the "Default" column of docs/operations/configuration.md
 * (held equal by `test/config.test.ts`).
 */
export const CONFIG_DEFAULTS: Readonly<Record<string, string>> = Object.freeze(
  {
    "github_app.dpop_key_file": "",
    "github_app.github_api_url": "https://api.github.com",
    "github_app.max_body_bytes": "1048576",
    "github_app.dedupe_window": "72h",
    "github_app.max_queued_jobs": "10000",
  },
);

/** Every key of the file, without the prefix `github_app.`. */
export const CONFIG_KEYS: readonly string[] = Object.freeze([
  "listen",
  "server_url",
  "token_file",
  "dpop_key_file",
  "webhook_secret_file",
  "github_token_file",
  "github_api_url",
  "repositories",
  "state_dir",
  "max_body_bytes",
  "dedupe_window",
  "max_queued_jobs",
]);

const REQUIRED = new Set([
  "listen",
  "server_url",
  "token_file",
  "webhook_secret_file",
  "github_token_file",
  "repositories",
  "state_dir",
]);

const DURATION = /^(0|[1-9][0-9]*)(ms|s|m|h|d)$/;
const UNIT_MS: Readonly<Record<string, number>> = {
  ms: 1,
  s: 1_000,
  m: 60_000,
  h: 3_600_000,
  d: 86_400_000,
};
const INTEGER = /^(0|[1-9][0-9]*)$/;
const PORT = /^(0|[1-9][0-9]{0,4})$/;
const REPOSITORY = /^[A-Za-z0-9](?:[A-Za-z0-9-]*[A-Za-z0-9])?\/[A-Za-z0-9._-]+$/;
const UUID =
  /^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$/i;
const SECRET_MODES = new Set([0o400, 0o600]);

/** A configuration fault; the message names the key and the file, never a value. */
export class ConfigError extends Error {
  /** The key, with its prefix, or `github_app` for the file as a whole. */
  readonly key: string;

  /**
   * Creates the error.
   *
   * @param key - The key, for example `github_app.listen`.
   * @param file - The configuration file or the file the key names.
   * @param rule - What is wrong, without the value.
   */
  constructor(key: string, file: string, rule: string) {
    super(`${key} in ${file}: ${rule}`);
    this.name = "ConfigError";
    this.key = key;
  }
}

/**
 * Parses a duration of docs/operations/configuration.md section 2.
 *
 * @param text - For example `72h` or `250ms`.
 * @returns Milliseconds, or `undefined` when the text is no duration.
 */
export function parseConfigDuration(text: string): number | undefined {
  const match = DURATION.exec(text);
  if (!match) return undefined;
  const value = Number(match[1]) * (UNIT_MS[match[2] ?? ""] ?? 0);
  return Number.isSafeInteger(value) ? value : undefined;
}

/**
 * Reports whether `host` is a loopback name or address.
 *
 * @param host - A host without brackets.
 * @returns `true` for `localhost`, `127.0.0.0/8` and `::1`.
 */
export function isLoopbackHost(host: string): boolean {
  const bare = host.replace(/^\[(.*)\]$/, "$1").toLowerCase();
  if (bare === "localhost" || bare === "::1") return true;
  return isIP(bare) === 4 && bare.startsWith("127.");
}

function key(name: string): string {
  return `github_app.${name}`;
}

function text(
  raw: Record<string, unknown>,
  name: string,
  file: string,
): string | undefined {
  const value = raw[name];
  if (value === undefined) {
    if (REQUIRED.has(name))
      throw new ConfigError(key(name), file, "required key is missing");
    return CONFIG_DEFAULTS[key(name)];
  }
  if (typeof value !== "string")
    throw new ConfigError(key(name), file, "must be a string");
  return value;
}

function requiredText(
  raw: Record<string, unknown>,
  name: string,
  file: string,
): string {
  const value = text(raw, name, file);
  if (value === undefined || value === "")
    throw new ConfigError(key(name), file, "must not be empty");
  return value;
}

function url(
  raw: Record<string, unknown>,
  name: string,
  file: string,
): string {
  const value = requiredText(raw, name, file);
  let parsed: URL;
  try {
    parsed = new URL(value);
  } catch {
    throw new ConfigError(key(name), file, "must be an absolute URL");
  }
  if (parsed.username || parsed.password || parsed.hash || parsed.search)
    throw new ConfigError(
      key(name),
      file,
      "must have no user part, query or fragment",
    );
  const plain =
    parsed.protocol === "http:" && isLoopbackHost(parsed.hostname);
  if (parsed.protocol !== "https:" && !plain)
    throw new ConfigError(
      key(name),
      file,
      "must use https (http only for a loopback host)",
    );
  return value.replace(/\/+$/, "");
}

function count(
  raw: Record<string, unknown>,
  name: string,
  file: string,
): number {
  const value = raw[name];
  if (value === undefined) {
    const fallback = CONFIG_DEFAULTS[key(name)] ?? "";
    if (!INTEGER.test(fallback))
      throw new ConfigError(key(name), file, "required key is missing");
    return Number(fallback);
  }
  if (typeof value !== "number" || !Number.isSafeInteger(value) || value <= 0)
    throw new ConfigError(key(name), file, "must be a positive whole number");
  return value;
}

function duration(
  raw: Record<string, unknown>,
  name: string,
  file: string,
): number {
  const value = text(raw, name, file) ?? "";
  const ms = parseConfigDuration(value);
  if (ms === undefined || ms <= 0)
    throw new ConfigError(key(name), file, "must be a positive duration");
  return ms;
}

function listen(
  raw: Record<string, unknown>,
  file: string,
): { host: string; port: number } {
  const value = requiredText(raw, "listen", file);
  const match = /^(\[[^\]]*\]|[^:[\]]*):([^:]*)$/.exec(value);
  const host = match?.[1]?.replace(/^\[(.*)\]$/, "$1") ?? "";
  const port = match?.[2] ?? "";
  if (!match || !PORT.test(port) || Number(port) > 65535)
    throw new ConfigError(key("listen"), file, "must be host:port");
  if (host !== "" && isIP(host) === 0)
    throw new ConfigError(
      key("listen"),
      file,
      "the host must be empty or an IP address",
    );
  return { host, port: Number(port) };
}

function repositories(
  raw: Record<string, unknown>,
  file: string,
): RepositoryMapping[] {
  const value = raw.repositories;
  const name = key("repositories");
  if (value === undefined)
    throw new ConfigError(name, file, "required key is missing");
  if (!Array.isArray(value) || value.length === 0)
    throw new ConfigError(name, file, "must be a non-empty list");
  const seen = new Set<string>();
  return value.map((entry: unknown, index) => {
    const at = `${name}[${String(index)}]`;
    if (typeof entry !== "object" || entry === null || Array.isArray(entry))
      throw new ConfigError(at, file, "must be an object");
    const members = entry as Record<string, unknown>;
    for (const member of Object.keys(members))
      if (member !== "repository" && member !== "channel_id")
        throw new ConfigError(`${at}.${member}`, file, "unknown member");
    const repository = members.repository;
    const channelId = members.channel_id;
    if (typeof repository !== "string" || !REPOSITORY.test(repository))
      throw new ConfigError(`${at}.repository`, file, "must be <owner>/<name>");
    if (typeof channelId !== "string" || !UUID.test(channelId))
      throw new ConfigError(`${at}.channel_id`, file, "must be a UUID");
    const folded = repository.toLowerCase();
    if (seen.has(folded))
      throw new ConfigError(`${at}.repository`, file, "is listed twice");
    seen.add(folded);
    return { repository, channelId: channelId.toLowerCase() };
  });
}

/**
 * Parses the configuration object of a file's text.
 *
 * @param source - The JSON text of the file.
 * @param file - The file name for messages.
 * @returns The configuration with every default applied.
 * @throws {@link ConfigError} naming the key and the file.
 */
export function parseConfig(source: string, file: string): AppConfig {
  let document: unknown;
  try {
    document = JSON.parse(source);
  } catch {
    throw new ConfigError("github_app", file, "the file is no JSON");
  }
  if (typeof document !== "object" || document === null || Array.isArray(document))
    throw new ConfigError("github_app", file, "the file must hold one object");
  for (const member of Object.keys(document))
    if (member !== "github_app")
      throw new ConfigError(member, file, "unknown member");
  const raw = (document as Record<string, unknown>).github_app;
  if (typeof raw !== "object" || raw === null || Array.isArray(raw))
    throw new ConfigError("github_app", file, "must be an object");
  const values = raw as Record<string, unknown>;
  for (const member of Object.keys(values))
    if (!CONFIG_KEYS.includes(member))
      throw new ConfigError(key(member), file, "unknown key");
  const { host, port } = listen(values, file);
  return {
    file,
    listenHost: host,
    listenPort: port,
    serverUrl: url(values, "server_url", file),
    tokenFile: requiredText(values, "token_file", file),
    dpopKeyFile: text(values, "dpop_key_file", file) ?? "",
    webhookSecretFile: requiredText(values, "webhook_secret_file", file),
    githubTokenFile: requiredText(values, "github_token_file", file),
    githubApiUrl: url(
      { github_api_url: CONFIG_DEFAULTS[key("github_api_url")], ...values },
      "github_api_url",
      file,
    ),
    repositories: repositories(values, file),
    stateDir: requiredText(values, "state_dir", file),
    maxBodyBytes: count(values, "max_body_bytes", file),
    dedupeWindowMs: duration(
      { dedupe_window: CONFIG_DEFAULTS[key("dedupe_window")], ...values },
      "dedupe_window",
      file,
    ),
    maxQueuedJobs: count(values, "max_queued_jobs", file),
  };
}

/**
 * Reads and parses the configuration file.
 *
 * @param file - The path given with `--config`.
 * @returns The configuration.
 * @throws {@link ConfigError} when the file cannot be read or a key is wrong.
 */
export async function loadConfig(file: string): Promise<AppConfig> {
  let source: string;
  try {
    source = await readFile(file, "utf8");
  } catch {
    throw new ConfigError("github_app", file, "the file cannot be read");
  }
  return parseConfig(source, file);
}

/**
 * Reads a secret file after the checks of SEC-SEC-1: a regular file of the process user with
 * mode `0400` or `0600`, read through the descriptor that was checked.
 *
 * @param file - The path.
 * @param name - The key that names the file, for messages.
 * @returns The content without surrounding whitespace.
 * @throws {@link ConfigError} naming the key and the file when a check fails or the file is
 *   empty.
 */
export async function readSecretFile(
  file: string,
  name: string,
): Promise<string> {
  let handle;
  try {
    handle = await open(file, constants.O_RDONLY | constants.O_NOFOLLOW);
  } catch {
    throw new ConfigError(name, file, "the secret file cannot be opened");
  }
  try {
    const info = await handle.stat();
    if (!info.isFile())
      throw new ConfigError(name, file, "the secret file is no regular file");
    if (!SECRET_MODES.has(info.mode & 0o777))
      throw new ConfigError(name, file, "the secret file needs mode 0400 or 0600");
    if (process.getuid !== undefined && info.uid !== process.getuid())
      throw new ConfigError(
        name,
        file,
        "the secret file must belong to the process user",
      );
    const value = (await handle.readFile("utf8")).trim();
    if (value === "")
      throw new ConfigError(name, file, "the secret file is empty");
    return value;
  } finally {
    await handle.close();
  }
}

/**
 * Creates the state directory with mode `0700`, or checks an existing one: a directory of the
 * process user without group or other access.
 *
 * @param dir - `github_app.state_dir`.
 * @param file - The configuration file, for messages.
 * @throws {@link ConfigError} when the directory cannot be created or is too open.
 */
export async function prepareStateDir(dir: string, file: string): Promise<void> {
  const name = key("state_dir");
  try {
    await mkdir(dir, { mode: 0o700 });
  } catch (err) {
    if ((err as NodeJS.ErrnoException).code !== "EEXIST")
      throw new ConfigError(name, file, "the directory cannot be created");
  }
  const info = await stat(dir);
  if (!info.isDirectory())
    throw new ConfigError(name, file, "is no directory");
  if ((info.mode & 0o077) !== 0)
    throw new ConfigError(name, file, "must have no group or other access");
  if (process.getuid !== undefined && info.uid !== process.getuid())
    throw new ConfigError(name, file, "must belong to the process user");
}

/**
 * The checks of a start: every secret file passes {@link readSecretFile} and the state
 * directory {@link prepareStateDir}.
 *
 * @param config - The loaded configuration.
 * @throws {@link ConfigError} for the first failed check.
 */
export async function checkStartFiles(config: AppConfig): Promise<void> {
  await readSecretFile(config.tokenFile, key("token_file"));
  if (config.dpopKeyFile !== "")
    await readSecretFile(config.dpopKeyFile, key("dpop_key_file"));
  await readSecretFile(config.webhookSecretFile, key("webhook_secret_file"));
  await readSecretFile(config.githubTokenFile, key("github_token_file"));
  await prepareStateDir(config.stateDir, config.file);
}
