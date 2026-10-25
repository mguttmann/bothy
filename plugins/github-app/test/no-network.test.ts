// SPDX-License-Identifier: Apache-2.0
// No test reaches a real GitHub service (brief M2-49 Design note 18; PLAN row "no call to a real
// GitHub service in the test").

import { describe, expect, it } from "vitest";

import { NetworkGuardError, guardedFetch } from "./setup/no-network.js";
import { testConfig } from "./helpers/world.js";

const BASE = {
  listen: "127.0.0.1:0",
  server_url: "http://127.0.0.1:9",
  token_file: "/run/secrets/token",
  webhook_secret_file: "/run/secrets/webhook",
  github_token_file: "/run/secrets/github",
  github_api_url: "http://127.0.0.1:9",
  repositories: [
    {
      repository: "example-org/widgets",
      channel_id: "0198a4f2-6c1e-7a3b-9d2e-4f5a6b7c8d90",
    },
  ],
  state_dir: "/var/lib/github-app",
};

describe("no network", () => {
  it("replaces the global fetch with the guard", () => {
    expect(globalThis.fetch).toBe(guardedFetch);
  });

  it.each([
    "https://api.github.com/",
    "https://api.github.com/repos/example-org/widgets/issues",
    "https://github.com/",
    "https://chat.example.com/api/v1/me",
    "http://192.0.2.10:8080/",
  ])("refuses %s before any connection", async (url) => {
    await expect(fetch(url)).rejects.toBeInstanceOf(NetworkGuardError);
    await expect(fetch(new Request(url))).rejects.toBeInstanceOf(
      NetworkGuardError,
    );
  });

  it("follows no redirect away from the loopback host", async () => {
    const { createServer } = await import("node:http");
    const server = createServer((_req, res) => {
      res.writeHead(302, { Location: "https://api.github.com/" }).end();
    });
    await new Promise<void>((resolve) =>
      server.listen(0, "127.0.0.1", resolve),
    );
    const { port } = server.address() as import("node:net").AddressInfo;
    try {
      await expect(
        fetch(`http://127.0.0.1:${String(port)}/`),
      ).rejects.toThrow();
      const manual = await fetch(`http://127.0.0.1:${String(port)}/`, {
        redirect: "manual",
      });
      expect(manual.status).toBe(302);
    } finally {
      await new Promise((resolve) => server.close(resolve));
    }
  });

  it("refuses a configuration with a GitHub or server address off the loopback host", () => {
    expect(() =>
      testConfig({ ...BASE, github_api_url: "https://api.github.com" }),
    ).toThrow(/loopback github_api_url/);
    expect(() =>
      testConfig({ ...BASE, server_url: "https://chat.example.com" }),
    ).toThrow(/loopback server_url/);
    expect(testConfig(BASE).githubApiUrl).toBe("http://127.0.0.1:9");
  });
});
