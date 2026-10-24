// SPDX-License-Identifier: Apache-2.0
// Foreign text (brief M2-49 Design note 10): no mentions, no links, cutting, titles.

import { afterEach, describe, expect, it } from "vitest";

import {
  CUT_LINE,
  cleanTitle,
  codeSpan,
  cutBytes,
  fence,
  renderEvent,
  renderMirror,
  safeLink,
  threadTitle,
  withBlock,
} from "../src/domain/render.js";
import type { GitHubEvent } from "../src/github/events.js";
import { FakeChat } from "./fakes/chat.js";
import { fixture } from "./helpers/fixtures.js";
import { FORUM, startWorld, type World } from "./helpers/world.js";

const cp = (...points: number[]): string => String.fromCodePoint(...points);

const EVENT: GitHubEvent = {
  event: "issues",
  action: "opened",
  repository: "example-org/widgets",
  channelId: FORUM,
  key: "example-org/widgets#12",
  item: "issue",
  number: 12,
  title: "A title",
  htmlUrl: "https://github.com/example-org/widgets/issues/12",
  actor: "alex",
  text: "Ping @alex and [x](https://example.com)",
};
const LIMITS = {
  message_max_content_bytes: 16_384,
  thread_title_max_chars: 200,
};

/** The parts of a CommonMark text outside code spans and fenced blocks. */
function outsideCode(text: string): string {
  let rest = text.replace(/^(`{3,})\n[\s\S]*?\n\1$/gm, "");
  rest = rest.replace(/(`+)[\s\S]*?\1/g, "");
  return rest;
}

let world: World | undefined;

afterEach(async () => {
  await world?.close();
  world = undefined;
});

describe("render", () => {
  it("keeps mentions and links of GitHub text inside code", () => {
    const content = renderEvent(EVENT, LIMITS);
    expect(content).toContain("Ping @alex and [x](https://example.com)");
    const outside = outsideCode(content);
    expect(outside).not.toContain("@alex");
    expect(outside).not.toContain("example.com");
  });

  it("puts logins with backticks and at signs into a code span they cannot close", () => {
    expect(codeSpan("alex")).toBe("`alex`");
    expect(codeSpan("a`b")).toBe("``a`b``");
    expect(codeSpan("`@sam`")).toBe("`` `@sam` ``");
    expect(outsideCode(codeSpan("``@sam"))).not.toContain("@sam");
  });

  it("keeps the fence closed for a body with backtick runs", () => {
    const body = "```\n@robin\n````\nend";
    const block = fence(body);
    expect(block.startsWith("`````\n")).toBe(true);
    expect(block.endsWith("\n`````")).toBe(true);
    expect(outsideCode(block)).not.toContain("@robin");
  });

  it("cleans a title of a bidi override, a control character and a line break", () => {
    const title = `${cp(0x202e)}evil${cp(0x07)} title${cp(0x2066)}\nsecond${cp(0x200f)}`;
    expect(cleanTitle(title, 200)).toBe("evil title second");
    expect(threadTitle("issue", 12, `${cp(0x202e)}${cp(0x07)}\n`, 200)).toBe(
      "#12",
    );
    expect(threadTitle("pull_request", 7, "", 200)).toBe("PR #7");
  });

  it("cuts titles to limits.thread_title_max_chars code points", () => {
    const title = threadTitle("issue", 12, `${cp(0x1f600)}`.repeat(50), 10);
    expect([...title]).toHaveLength(10);
    expect(title.startsWith("#12 ")).toBe(true);
  });

  it("cuts a message at a code point boundary with the fence closed and the cut line", () => {
    const body = `${cp(0x1f600)}`.repeat(1000);
    const content = withBlock("**Head**", body, 300);
    expect(Buffer.byteLength(content, "utf8")).toBeLessThanOrEqual(300);
    expect(content.endsWith(`\n\`\`\`\n${CUT_LINE}`)).toBe(true);
    expect(content).not.toContain("�");
    expect(cutBytes("aé", 2)).toBe("a");
    expect(withBlock("head", "", 3)).toBe("hea");
    expect(withBlock("a head longer than the bound", "x".repeat(10), 10)).toBe(
      "a head lon",
    );
  });

  it.each([
    ["javascript:alert(1)"],
    ["http://github.com/example-org/widgets/issues/12"],
    ["https://github.com/example-org/widgets/issues/(12)"],
    ["https://github.com/a b"],
    ["https://github.com/<x>"],
    ["not a url"],
  ])("writes no link for %s", (url) => {
    expect(safeLink(url)).toBeUndefined();
    const content = renderEvent({ ...EVENT, htmlUrl: url }, LIMITS);
    expect(content.startsWith("**Issue #12 opened**")).toBe(true);
  });

  it("renders the templates of each event", () => {
    const at = (change: Partial<GitHubEvent>) =>
      renderEvent({ ...EVENT, ...change }, LIMITS).split("\n")[0];
    expect(at({ action: "closed" })).toBe(
      "**Issue [#12](https://github.com/example-org/widgets/issues/12) closed** by `alex`",
    );
    expect(at({ event: "issue_comment", action: "created" })).toBe(
      "`alex` commented on [#12](https://github.com/example-org/widgets/issues/12):",
    );
    expect(
      at({ event: "pull_request", item: "pull_request", action: "opened" }),
    ).toContain("**Pull request [#12]");
    expect(
      at({
        event: "pull_request",
        item: "pull_request",
        action: "closed",
        merged: true,
      }),
    ).toContain("merged**");
    expect(
      at({
        event: "pull_request",
        item: "pull_request",
        action: "closed",
        merged: false,
      }),
    ).toContain("closed**");
    expect(
      at({ event: "pull_request", item: "pull_request", action: "reopened" }),
    ).toContain("reopened**");
    expect(
      at({
        event: "pull_request_review",
        item: "pull_request",
        action: "submitted",
        reviewState: "approved",
      }),
    ).toBe(
      "`alex` reviewed [#12](https://github.com/example-org/widgets/issues/12): `approved`",
    );
  });

  it("renders a mirrored chat message without GitHub mentions or links", () => {
    const body = renderMirror("Sam `x`", "@robin [y](https://example.com)");
    expect(body.startsWith("`` Sam `x` `` wrote in chat:")).toBe(true);
    expect(outsideCode(body)).not.toContain("@robin");
    expect(outsideCode(body)).not.toContain("example.com");
  });

  it("retries once with #<number> after a 400 validation_failed on the title", async () => {
    world = await startWorld();
    world.chat.fail(
      "createForumThread",
      FakeChat.problem(400, "validation_failed", [
        { pointer: "/title", code: "invalid" },
      ]),
    );
    await world.deliver("issues", fixture("issues", "opened", { number: 12 }));
    await world.app.idle();
    const titles = world.chat
      .callsOf("createForumThread")
      .map((c) => (c.args[0] as { title: string }).title);
    expect(titles).toEqual(["#12 Spelling error in the README file", "#12"]);
  });
});
