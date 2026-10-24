// SPDX-License-Identifier: Apache-2.0
// Configuration (brief M2-49 Design note 3).

import { chmod, mkdir, mkdtemp, rm, writeFile } from "node:fs/promises";
import { tmpdir } from "node:os";
import { join } from "node:path";

import { DEFAULTS, SETTING_KEYS, parseDuration } from "bothy-sdk";
import { afterEach, beforeEach, describe, expect, it } from "vitest";

import {
  CONFIG_DEFAULTS,
  CONFIG_KEYS,
  ConfigError,
  checkStartFiles,
  loadConfig,
  parseConfig,
  parseConfigDuration,
  prepareStateDir,
  readSecretFile,
} from "../src/config.js";
import { appFile, repoFile } from "./helpers/repo.js";

const BASE = JSON.parse(appFile("config.example.json")) as {
  github_app: Record<string, unknown>;
};

function parse(
  members: Record<string, unknown>,
  drop: string[] = [],
): ReturnType<typeof parseConfig> {
  const github_app: Record<string, unknown> = {
    ...BASE.github_app,
    ...members,
  };
  for (const key of drop) delete github_app[key];
  return parseConfig(JSON.stringify({ github_app }), "config.json");
}

function failure(fn: () => unknown): ConfigError {
  try {
    fn();
  } catch (err) {
    if (err instanceof ConfigError) return err;
    throw err;
  }
  throw new Error("no ConfigError");
}

/** The "Default" column of docs/operations/configuration.md for a key. */
function referenceDefault(key: string): string {
  const row = repoFile("docs/operations/configuration.md")
    .split("\n")
    .find((line) => line.startsWith(`| \`${key}\` |`));
  if (row === undefined) throw new Error(`${key} is not in the reference`);
  return (row.split("|")[5] ?? "").trim();
}

let dir = "";

beforeEach(async () => {
  dir = await mkdtemp(join(tmpdir(), "github-app-config-"));
});

afterEach(async () => {
  await rm(dir, { recursive: true, force: true });
});

describe("configuration", () => {
  it("loads config.example.json", async () => {
    const config = await loadConfig(
      join(import.meta.dirname, "..", "config.example.json"),
    );
    expect(config.listenPort).toBe(8090);
    expect(config.repositories).toEqual([
      {
        repository: "example-org/widgets",
        channelId: "0198a4f2-6c1e-7a3b-9d2e-4f5a6b7c8d90",
      },
    ]);
    expect(config.dedupeWindowMs).toBe(72 * 3_600_000);
  });

  it.each([
    "listen",
    "server_url",
    "token_file",
    "webhook_secret_file",
    "github_token_file",
    "repositories",
    "state_dir",
  ])("requires %s", (key) => {
    const err = failure(() => parse({}, [key]));
    expect(err.key).toBe(`github_app.${key}`);
    expect(err.message).toContain("config.json");
  });

  it("applies the defaults of the optional keys", () => {
    const config = parse({}, [
      "dpop_key_file",
      "github_api_url",
      "max_body_bytes",
      "dedupe_window",
      "max_queued_jobs",
    ]);
    expect(config).toMatchObject({
      dpopKeyFile: "",
      githubApiUrl: "https://api.github.com",
      maxBodyBytes: 1_048_576,
      dedupeWindowMs: 259_200_000,
      maxQueuedJobs: 10_000,
    });
  });

  it.each([
    [{ extra: 1 }, "github_app.extra", "unknown key"],
    [{ dedupe_window: "1h30m" }, "github_app.dedupe_window", "duration"],
    [{ dedupe_window: "0s" }, "github_app.dedupe_window", "duration"],
    [{ max_body_bytes: "1024" }, "github_app.max_body_bytes", "whole number"],
    [{ max_queued_jobs: 1.5 }, "github_app.max_queued_jobs", "whole number"],
    [{ listen: 8090 }, "github_app.listen", "string"],
    [{ listen: "localhost:8090" }, "github_app.listen", "IP address"],
    [{ listen: "127.0.0.1" }, "github_app.listen", "host:port"],
    [
      { server_url: "http://chat.example.com" },
      "github_app.server_url",
      "https",
    ],
    [
      { server_url: "https://user:pw@chat.example.com" },
      "github_app.server_url",
      "user part",
    ],
    [
      { github_api_url: "not a url" },
      "github_app.github_api_url",
      "absolute URL",
    ],
    [{ repositories: [] }, "github_app.repositories", "non-empty"],
    [
      {
        repositories: [
          {
            repository: "widgets",
            channel_id: "0198a4f2-6c1e-7a3b-9d2e-4f5a6b7c8d90",
          },
        ],
      },
      "github_app.repositories[0].repository",
      "owner",
    ],
    [
      { repositories: [{ repository: "a/b", channel_id: "x" }] },
      "github_app.repositories[0].channel_id",
      "UUID",
    ],
    [
      {
        repositories: [
          {
            repository: "a/b",
            channel_id: "0198a4f2-6c1e-7a3b-9d2e-4f5a6b7c8d90",
            x: 1,
          },
        ],
      },
      "github_app.repositories[0].x",
      "unknown",
    ],
    [{ repositories: ["a/b"] }, "github_app.repositories[0]", "object"],
    [
      {
        repositories: [
          {
            repository: "A/B",
            channel_id: "0198a4f2-6c1e-7a3b-9d2e-4f5a6b7c8d90",
          },
          {
            repository: "a/b",
            channel_id: "0198a4f2-6c1e-7a3b-9d2e-4f5a6b7c8d91",
          },
        ],
      },
      "github_app.repositories[1].repository",
      "twice",
    ],
  ])(
    "refuses %j naming the key and the file but no value",
    (members, key, rule) => {
      const err = failure(() => parse(members));
      expect(err.key).toBe(key);
      expect(err.message).toContain("config.json");
      expect(err.message).toContain(rule);
      for (const value of Object.values(members)) {
        if (typeof value === "string" && value.length > 2)
          expect(err.message).not.toContain(value);
      }
    },
  );

  it("refuses a file that is no JSON object or has another top-level member", () => {
    expect(failure(() => parseConfig("[1]", "c.json")).message).toContain(
      "one object",
    );
    expect(failure(() => parseConfig("{", "c.json")).message).toContain(
      "no JSON",
    );
    expect(failure(() => parseConfig('{"other": {}}', "c.json")).key).toBe(
      "other",
    );
    expect(failure(() => parseConfig('{"github_app": 1}', "c.json")).key).toBe(
      "github_app",
    );
  });

  it("accepts http only for a loopback host and an empty or IP listen host", () => {
    expect(
      parse({ server_url: "http://127.0.0.1:8080", listen: ":8090" })
        .listenHost,
    ).toBe("");
    expect(
      parse({ server_url: "http://[::1]:8080", listen: "[::1]:0" }).listenHost,
    ).toBe("::1");
    expect(parse({ server_url: "http://localhost:8080/" }).serverUrl).toBe(
      "http://localhost:8080",
    );
  });

  it("parses durations of configuration.md section 2", () => {
    expect(parseConfigDuration("250ms")).toBe(250);
    expect(parseConfigDuration("2d")).toBe(172_800_000);
    expect(parseConfigDuration("01h")).toBeUndefined();
    expect(parseConfigDuration("1.5h")).toBeUndefined();
  });

  it("aborts on a secret file with mode 0644, of another type or empty", async () => {
    const file = join(dir, "secret");
    await writeFile(file, "value", { mode: 0o644 });
    await expect(readSecretFile(file, "github_app.token_file")).rejects.toThrow(
      /0400 or 0600/,
    );
    await chmod(file, 0o600);
    await expect(readSecretFile(file, "github_app.token_file")).resolves.toBe(
      "value",
    );
    await chmod(file, 0o400);
    await expect(readSecretFile(file, "github_app.token_file")).resolves.toBe(
      "value",
    );
    await writeFile(join(dir, "empty"), "  \n", { mode: 0o600 });
    await expect(readSecretFile(join(dir, "empty"), "k")).rejects.toThrow(
      /empty/,
    );
    await expect(readSecretFile(join(dir, "missing"), "k")).rejects.toThrow(
      /cannot be opened/,
    );
    await mkdir(join(dir, "sub"), { mode: 0o700 });
    await expect(readSecretFile(join(dir, "sub"), "k")).rejects.toThrow(
      ConfigError,
    );
    const err = await readSecretFile(file, "github_app.token_file").catch(
      () => undefined,
    );
    expect(err).toBe("value");
  });

  it("refuses a state directory with group access and creates a new one with 0700", async () => {
    const open = join(dir, "open");
    await mkdir(open, { mode: 0o750 });
    await chmod(open, 0o750);
    await expect(prepareStateDir(open, "config.json")).rejects.toThrow(
      /group or other access/,
    );
    const fresh = join(dir, "fresh");
    await prepareStateDir(fresh, "config.json");
    const { stat } = await import("node:fs/promises");
    expect((await stat(fresh)).mode & 0o777).toBe(0o700);
    await writeFile(join(dir, "file"), "", { mode: 0o600 });
    await expect(
      prepareStateDir(join(dir, "file"), "config.json"),
    ).rejects.toThrow(/no directory/);
    await expect(
      prepareStateDir(join(dir, "no", "such"), "config.json"),
    ).rejects.toThrow(/cannot be created/);
  });

  it("checks every secret file at start", async () => {
    for (const name of ["token", "webhook", "github", "dpop"])
      await writeFile(join(dir, name), "v", { mode: 0o600 });
    const config = parse({
      token_file: join(dir, "token"),
      webhook_secret_file: join(dir, "webhook"),
      github_token_file: join(dir, "github"),
      dpop_key_file: join(dir, "dpop"),
      state_dir: join(dir, "state"),
    });
    await checkStartFiles(config);
    await chmod(join(dir, "dpop"), 0o644);
    await expect(checkStartFiles(config)).rejects.toThrow(
      /github_app.dpop_key_file/,
    );
  });

  it("holds every default equal to the reference and reuses the SDK's keys", () => {
    for (const key of CONFIG_KEYS) {
      const full = `github_app.${key}`;
      const reference = referenceDefault(full);
      const own = CONFIG_DEFAULTS[full];
      if (own === undefined) expect(reference).toBe("no default, required");
      else if (own === "") expect(reference).toBe("empty");
      else expect(reference).toBe(`\`${own}\``);
    }
    for (const [field, key] of Object.entries(SETTING_KEYS)) {
      if (
        ![
          "sdk.http_timeout",
          "sdk.retry_backoff_cap",
          "webhook.receiver_tolerance",
        ].includes(key)
      )
        continue;
      const reference = referenceDefault(key).replaceAll("`", "");
      expect(DEFAULTS[field as keyof typeof DEFAULTS]).toBe(
        parseDuration(reference),
      );
    }
  });
});
