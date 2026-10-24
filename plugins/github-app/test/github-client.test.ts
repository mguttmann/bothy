// SPDX-License-Identifier: Apache-2.0
// The GitHub REST client (brief M2-49 Design note 14).

import { afterEach, beforeEach, describe, expect, it } from "vitest";

import {
  APP_VERSION,
  GITHUB_API_VERSION,
  GitHubClient,
} from "../src/github/client.js";
import { FakeGitHub } from "./fakes/github-api.js";
import { PATIENCE_MS } from "./helpers/patience.js";
import { guardedFetch } from "./setup/no-network.js";

let github: FakeGitHub;

beforeEach(async () => {
  github = await FakeGitHub.start({
    token: "t0ken",
    secret: "s",
    repositories: ["example-org/widgets"],
  });
});

afterEach(async () => {
  await github.stop();
});

function client(
  options: { timeoutMs?: number; token?: () => Promise<string> } = {},
): GitHubClient {
  return new GitHubClient({
    apiUrl: github.url,
    token: options.token ?? (() => Promise.resolve("t0ken")),
    timeoutMs: options.timeoutMs ?? PATIENCE_MS,
    fetch: guardedFetch,
  });
}

describe("GitHubClient", () => {
  it("sends the headers of the GitHub REST API and reads the created issue", async () => {
    const result = await client().createIssue("example-org/widgets", "T", "B");
    expect(result).toEqual({
      kind: "created",
      number: 100,
      id: 9_000_001,
      htmlUrl: "https://github.com/example-org/widgets/issues/100",
    });
    const [request] = github.requests;
    expect(request?.headers).toMatchObject({
      accept: "application/vnd.github+json",
      authorization: "Bearer t0ken",
      "x-github-api-version": GITHUB_API_VERSION,
      "user-agent": `github-app/${APP_VERSION}`,
      "content-type": "application/json",
    });
    expect(request?.body).toEqual({ title: "T", body: "B" });
  });

  it("creates a comment on the number", async () => {
    const result = await client().createComment("example-org/widgets", 7, "C");
    expect(result).toMatchObject({ kind: "created", number: 7 });
    expect(github.requests[0]?.path).toBe(
      "/repos/example-org/widgets/issues/7/comments",
    );
  });

  it("treats a 302 as refused and does not follow it", async () => {
    github.fail({
      status: 302,
      location: `${github.url}/repos/example-org/widgets/issues`,
    });
    expect(await client().createIssue("example-org/widgets", "T", "B")).toEqual(
      { kind: "refused", status: 302 },
    );
    expect(github.requests).toHaveLength(1);
  });

  it("treats 4xx as refused and 5xx as unknown, without reading the error body", async () => {
    github.fail({ status: 422 }, { status: 500 });
    expect(await client().createIssue("example-org/widgets", "T", "B")).toEqual(
      { kind: "refused", status: 422 },
    );
    expect(await client().createIssue("example-org/widgets", "T", "B")).toEqual(
      { kind: "unknown", status: 500 },
    );
    expect(await client().createIssue("example-org/other", "T", "B")).toEqual({
      kind: "refused",
      status: 404,
    });
  });

  it("ends a request after sdk.http_timeout", async () => {
    // The fake never answers, so only the bound ends the call, however late it fires; a short
    // bound may even fire before the request arrives, with the same outcome.
    github.fail({ hang: true });
    expect(
      // never-answers: the only answer is the bound itself.
      await client({ timeoutMs: 50 }).createIssue(
        "example-org/widgets",
        "T",
        "B",
      ),
    ).toEqual({ kind: "unknown", reason: "timeout" });
  });

  it("reports a dropped connection as a transport error", async () => {
    github.fail({ drop: true });
    expect(await client().createIssue("example-org/widgets", "T", "B")).toEqual(
      {
        kind: "unknown",
        reason: "transport",
      },
    );
    expect(github.pendingFaults()).toBe(0);
  });

  it("sends nothing when the token cannot be read", async () => {
    const result = await client({
      token: () => Promise.reject(new Error("mode")),
    }).createIssue("example-org/widgets", "T", "B");
    expect(result).toEqual({ kind: "not_sent", reason: "secret_file" });
    expect(github.requests).toHaveLength(0);
  });

  it("treats an unreadable 201 as unknown", async () => {
    const odd = new GitHubClient({
      apiUrl: github.url,
      token: () => Promise.resolve("t0ken"),
      timeoutMs: PATIENCE_MS,
      fetch: () =>
        Promise.resolve(
          new Response(JSON.stringify({ id: "x" }), { status: 201 }),
        ),
    });
    expect(await odd.createIssue("example-org/widgets", "T", "B")).toEqual({
      kind: "unknown",
      status: 201,
      reason: "invalid_answer",
    });
  });
});
