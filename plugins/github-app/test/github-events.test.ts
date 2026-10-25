// SPDX-License-Identifier: Apache-2.0
// The closed event list (brief M2-49 Design note 6) on the fixtures of the GitHub documentation.

import { describe, expect, it } from "vitest";

import type { RepositoryMapping } from "../src/config.js";
import { parseGitHubEvent } from "../src/github/events.js";
import { fixture } from "./helpers/fixtures.js";
import { FORUM } from "./helpers/world.js";

const REPOS: RepositoryMapping[] = [
  { repository: "Example-Org/Widgets", channelId: FORUM },
];

function parse(event: string, payload: unknown) {
  return parseGitHubEvent(event, payload, REPOS);
}

describe("parseGitHubEvent", () => {
  it("parses an opened issue", () => {
    const parsed = parse("issues", fixture("issues", "opened", { number: 12 }));
    expect(parsed).toEqual({
      kind: "event",
      event: {
        event: "issues",
        action: "opened",
        repository: "Example-Org/Widgets",
        channelId: FORUM,
        key: "example-org/widgets#12",
        item: "issue",
        number: 12,
        title: "Spelling error in the README file",
        htmlUrl: "https://github.com/example-org/widgets/issues/12",
        actor: "Codertocat",
        text: "It looks like you accidently spelled 'commit' with two 't's.",
      },
    });
  });

  it.each([
    ["issues", "closed", { item: "issue", number: 1 }],
    ["issues", "reopened", { item: "issue", number: 1 }],
    ["issue_comment", "created", { item: "issue", commentId: 492700400 }],
    [
      "issue_comment",
      "created.pull_request",
      { item: "pull_request", commentId: 492700401 },
    ],
    [
      "pull_request",
      "opened",
      { item: "pull_request", number: 2, merged: false },
    ],
    ["pull_request", "closed", { item: "pull_request", merged: false }],
    ["pull_request", "closed.merged", { item: "pull_request", merged: true }],
    ["pull_request", "reopened", { item: "pull_request" }],
    [
      "pull_request_review",
      "submitted",
      { item: "pull_request", reviewState: "commented" },
    ],
  ])("parses %s %s", (event, action, expected) => {
    const parsed = parse(event, fixture(event, action));
    expect(parsed.kind).toBe("event");
    if (parsed.kind !== "event") return;
    expect(parsed.event).toMatchObject(expected);
    expect(parsed.event.action).toBe(action.split(".")[0]);
    expect(parsed.event.htmlUrl).toMatch(
      /^https:\/\/github\.com\/example-org\/widgets\//,
    );
  });

  it("links a comment and a review to their own html_url", () => {
    const comment = parse(
      "issue_comment",
      fixture("issue_comment", "created", { commentId: 5 }),
    );
    expect(comment.kind === "event" && comment.event.htmlUrl).toBe(
      "https://github.com/example-org/widgets/issues/1#issuecomment-5",
    );
    const review = parse(
      "pull_request_review",
      fixture("pull_request_review", "submitted"),
    );
    expect(review.kind === "event" && review.event.htmlUrl).toContain(
      "#pullrequestreview-",
    );
  });

  it("reads a null body as empty text", () => {
    const payload = fixture("issues", "opened");
    (payload.issue as Record<string, unknown>).body = null;
    const parsed = parse("issues", payload);
    expect(parsed.kind === "event" && parsed.event.text).toBe("");
  });

  it("ignores ping, unsupported actions and events, and unknown repositories", () => {
    expect(parse("ping", fixture("ping", ""))).toEqual({ kind: "ping" });
    expect(
      parse("issues", { ...fixture("issues", "opened"), action: "labeled" }),
    ).toEqual({
      kind: "ignored",
      reason: "unsupported",
    });
    expect(
      parse("pull_request_review", fixture("pull_request_review", "dismissed")),
    ).toEqual({
      kind: "ignored",
      reason: "unsupported",
    });
    expect(parse("star", fixture("issues", "opened"))).toEqual({
      kind: "ignored",
      reason: "unsupported",
    });
    expect(
      parse(
        "issues",
        fixture("issues", "opened", { repository: "example-org/other" }),
      ),
    ).toEqual({
      kind: "ignored",
      reason: "repository_not_configured",
    });
  });

  it.each([
    [
      "no number",
      (p: Record<string, unknown>) =>
        delete (p.issue as Record<string, unknown>).number,
    ],
    [
      "no title",
      (p: Record<string, unknown>) =>
        delete (p.issue as Record<string, unknown>).title,
    ],
    ["no sender", (p: Record<string, unknown>) => delete p.sender],
    ["no repository", (p: Record<string, unknown>) => delete p.repository],
    [
      "a number of zero",
      (p: Record<string, unknown>) =>
        ((p.issue as Record<string, unknown>).number = 0),
    ],
  ])("refuses a payload with %s as invalid", (_name, change) => {
    const payload = fixture("issues", "opened");
    change(payload);
    expect(parse("issues", payload)).toEqual({ kind: "invalid" });
  });

  it("refuses a payload that is no object", () => {
    expect(parse("issues", [])).toEqual({ kind: "invalid" });
    expect(parse("issues", null)).toEqual({ kind: "invalid" });
  });
});
