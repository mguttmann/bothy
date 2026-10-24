// SPDX-License-Identifier: Apache-2.0
// The command line entry (brief M2-49 Design note 2).

import { writeFile } from "node:fs/promises";
import { join } from "node:path";

import { afterEach, describe, expect, it } from "vitest";

import { configArgument, main, USAGE } from "../src/main.js";
import { FakeChat } from "./fakes/chat.js";
import { startWorld, type World } from "./helpers/world.js";
import { guardedFetch } from "./setup/no-network.js";

let world: World | undefined;

afterEach(async () => {
  await world?.close();
  world = undefined;
});

describe("main", () => {
  it("reads --config", () => {
    expect(configArgument(["--config", "a.json"])).toBe("a.json");
    expect(configArgument(["--config=a.json"])).toBe("a.json");
    expect(configArgument(["--config="])).toBeUndefined();
    expect(configArgument([])).toBeUndefined();
    expect(configArgument(["--other", "a"])).toBeUndefined();
  });

  it("exits 2 on wrong arguments and 1 on a configuration error without a value", async () => {
    const lines: string[] = [];
    const io = {
      error: (l: string) => lines.push(l),
      signal: () => Promise.resolve(),
    };
    expect(await main([], io)).toBe(2);
    expect(lines).toEqual([USAGE]);
    world = await startWorld();
    const file = join(world.dir, "bad.json");
    await writeFile(
      file,
      JSON.stringify({ github_app: { listen: "nope:1", secret: "hunter" } }),
    );
    expect(await main(["--config", file], io)).toBe(1);
    expect(lines[1]).toContain("github_app.secret");
    expect(lines[1]).not.toContain("hunter");
  });

  it("runs until the signal and exits 0 after a clean stop", async () => {
    world = await startWorld();
    await world.stopApp();
    const file = join(world.dir, "config.json");
    await writeFile(
      file,
      JSON.stringify({
        github_app: {
          listen: "127.0.0.1:0",
          server_url: "http://127.0.0.1:9",
          token_file: world.config.tokenFile,
          webhook_secret_file: world.config.webhookSecretFile,
          github_token_file: world.config.githubTokenFile,
          github_api_url: world.github.url,
          repositories: [
            {
              repository: "example-org/widgets",
              channel_id: "0198a4f2-6c1e-7a3b-9d2e-4f5a6b7c8d90",
            },
          ],
          state_dir: world.config.stateDir,
        },
      }),
    );
    let release = (): void => undefined;
    const signal = new Promise<void>((r) => {
      release = r;
    });
    let port = 0;
    const done = main(
      ["--config", file],
      {
        error: () => undefined,
        signal: () => signal,
        started: (app) => {
          port = app.port;
          release();
        },
      },
      {
        chat: new FakeChat(),
        fetch: guardedFetch,
        logger: {
          debug: () => undefined,
          info: () => undefined,
          warn: () => undefined,
          error: () => undefined,
        },
      },
    );
    expect(await done).toBe(0);
    expect(port).toBeGreaterThan(0);
  });

  it("exits 1 when the server refuses getMe, naming only the error class", async () => {
    world = await startWorld();
    await world.stopApp();
    const chat = new FakeChat();
    chat.fail("me", FakeChat.problem(401, "unauthenticated"));
    const lines: string[] = [];
    const file = join(world.dir, "config2.json");
    await writeFile(
      file,
      JSON.stringify({
        github_app: {
          listen: "127.0.0.1:0",
          server_url: "http://127.0.0.1:9",
          token_file: world.config.tokenFile,
          webhook_secret_file: world.config.webhookSecretFile,
          github_token_file: world.config.githubTokenFile,
          github_api_url: world.github.url,
          repositories: [
            {
              repository: "example-org/widgets",
              channel_id: "0198a4f2-6c1e-7a3b-9d2e-4f5a6b7c8d90",
            },
          ],
          state_dir: world.config.stateDir,
        },
      }),
    );
    const code = await main(
      ["--config", file],
      { error: (l) => lines.push(l), signal: () => Promise.resolve() },
      { chat, fetch: guardedFetch },
    );
    expect(code).toBe(1);
    expect(lines).toEqual(["start failed (UnauthenticatedError)"]);
  });
});
