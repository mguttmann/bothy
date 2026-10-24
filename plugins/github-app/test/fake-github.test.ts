// SPDX-License-Identifier: Apache-2.0
// The Fake-GitHub API itself (brief M2-49 Design note 17).

import { createServer } from "node:http";
import type { AddressInfo } from "node:net";

import { afterEach, describe, expect, it } from "vitest";

import { verifyGitHubSignature } from "../src/github/signature.js";
import { FakeGitHub } from "./fakes/github-api.js";
import { fixture } from "./helpers/fixtures.js";

let github: FakeGitHub | undefined;
const servers: ReturnType<typeof createServer>[] = [];

afterEach(async () => {
  await github?.stop();
  for (const s of servers.splice(0)) await new Promise((r) => s.close(r));
});

/** A receiver that checks each delivery with the app's signature check. */
async function receiver(
  secret: string,
): Promise<{ url: string; results: unknown[] }> {
  const results: unknown[] = [];
  const server = createServer((req, res) => {
    const chunks: Buffer[] = [];
    req.on("data", (c: Buffer) => chunks.push(c));
    req.on("end", () => {
      results.push({
        check: verifyGitHubSignature(
          secret,
          new Uint8Array(Buffer.concat(chunks)),
          req.headers["x-hub-signature-256"],
        ),
        event: req.headers["x-github-event"],
        agent: req.headers["user-agent"],
      });
      res.writeHead(204).end();
    });
  });
  servers.push(server);
  await new Promise<void>((r) => server.listen(0, "127.0.0.1", r));
  return {
    url: `http://127.0.0.1:${String((server.address() as AddressInfo).port)}/`,
    results,
  };
}

describe("FakeGitHub", () => {
  it("signs deliveries so that the app's check accepts them, and alters them on request", async () => {
    github = await FakeGitHub.start({
      token: "t",
      secret: "sec",
      repositories: ["example-org/widgets"],
    });
    const target = await receiver("sec");
    await github.deliver(target.url, "issues", fixture("issues", "opened"));
    await github.deliver(target.url, "issues", fixture("issues", "opened"), {
      signature: "alter",
    });
    await github.deliver(target.url, "issues", fixture("issues", "opened"), {
      signature: "truncate",
    });
    await github.deliver(target.url, "issues", fixture("issues", "opened"), {
      signature: "omit",
    });
    expect(target.results).toEqual([
      {
        check: { ok: true },
        event: "issues",
        agent: expect.stringMatching(/^GitHub-Hookshot\//) as unknown,
      },
      expect.objectContaining({ check: { ok: false, reason: "mismatch" } }),
      expect.objectContaining({ check: { ok: false, reason: "malformed" } }),
      expect.objectContaining({ check: { ok: false, reason: "missing" } }),
    ]);
  });

  it("checks the token and the headers, records requests, and echoes created objects", async () => {
    github = await FakeGitHub.start({
      token: "t",
      secret: "sec",
      repositories: ["example-org/widgets"],
    });
    const target = await receiver("sec");
    github.autoEcho = { url: target.url };
    const post = (
      path: string,
      headers: Record<string, string>,
      body: unknown,
    ) =>
      fetch(`${github?.url ?? ""}${path}`, {
        method: "POST",
        headers,
        body: JSON.stringify(body),
      });
    const good = {
      Authorization: "Bearer t",
      Accept: "application/vnd.github+json",
      "X-GitHub-Api-Version": "2022-11-28",
      "User-Agent": "github-app/0.0.0",
    };
    expect(
      (
        await post(
          "/repos/example-org/widgets/issues",
          { ...good, Authorization: "Bearer x" },
          {},
        )
      ).status,
    ).toBe(401);
    expect(
      (
        await post(
          "/repos/example-org/widgets/issues",
          { ...good, Accept: "text/plain" },
          {},
        )
      ).status,
    ).toBe(400);
    expect(
      (await post("/repos/example-org/widgets/issues", good, { title: "" }))
        .status,
    ).toBe(422);
    expect(
      (await post("/repos/example-org/other/issues", good, { title: "x" }))
        .status,
    ).toBe(404);
    expect(
      (
        await post("/repos/example-org/widgets/issues", good, {
          title: "x",
          body: "y",
        })
      ).status,
    ).toBe(201);
    expect(
      (
        await post("/repos/example-org/widgets/issues/100/comments", good, {
          body: "z",
        })
      ).status,
    ).toBe(201);
    await github.echoesDone();
    expect(target.results.map((r) => (r as { event: string }).event)).toEqual([
      "issues",
      "issue_comment",
    ]);
    expect(github.issues()).toHaveLength(5);
    expect(github.comments()).toHaveLength(1);
    expect(github.requests[0]?.headerNames).toContain("authorization");
  });
});
