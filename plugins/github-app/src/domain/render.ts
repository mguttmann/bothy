// SPDX-License-Identifier: Apache-2.0
/**
 * Foreign text in the chat and in GitHub (docs/spec/03-data-model.md content format rules 2
 * and 3, SEC-IN-4, SEC-OUT-1; brief M2-49 Design notes 9 and 10).
 *
 * GitHub text never resolves a mention or becomes a link in the chat: logins go into a code
 * span ({@link codeSpan}) and bodies into a fenced code block ({@link fence}), each with a
 * delimiter one backtick longer than the longest backtick run of its text (CommonMark 0.31.2),
 * so the text cannot close it. A link is written only for an absolute `https` URL without
 * whitespace, angle brackets or parentheses ({@link safeLink}). Messages are cut to
 * `limits.message_max_content_bytes` in UTF-8 bytes at a code point boundary with the fence
 * closed and the line {@link CUT_LINE}; titles are cleaned of control and bidi characters and
 * cut to `limits.thread_title_max_chars` code points ({@link cleanTitle}).
 *
 * @example
 * ```ts
 * const content = renderEvent(event, limits);
 * ```
 *
 * @module
 */

import type { components } from "bothy-sdk";

import type { GitHubEvent } from "../github/events.js";

/** The limits of `getInstance` the rendering needs. */
export type RenderLimits = Pick<
  components["schemas"]["InstanceLimits"],
  "message_max_content_bytes" | "thread_title_max_chars"
>;

/** The line that ends a cut message. */
export const CUT_LINE = "(cut, full text on GitHub)";

// Bidi controls of SEC-IN-4: LRM, RLM, ALM, LRE to RLO, LRI to PDI.
const BIDI = /[\u200e\u200f\u061c\u202a-\u202e\u2066-\u2069]/gu;
// Unicode category Cc (C0, DEL and C1 controls).
const CONTROL = /\p{Cc}/gu;
const LINE_BREAK = /\r\n|[\n\r\u2028\u2029\u0085]/gu;

function longestRun(text: string, char: string): number {
  let longest = 0;
  let run = 0;
  for (const c of text) {
    run = c === char ? run + 1 : 0;
    if (run > longest) longest = run;
  }
  return longest;
}

/**
 * Wraps text in a code span whose delimiter is one backtick longer than the longest backtick run
 * of the text, so the text cannot end the span; line breaks become spaces.
 *
 * @param text - Foreign text such as a login.
 * @returns The code span.
 */
export function codeSpan(text: string): string {
  const flat = text.replace(LINE_BREAK, " ");
  const ticks = "`".repeat(longestRun(flat, "`") + 1);
  // A space on both sides keeps a leading or trailing backtick apart from the delimiter;
  // CommonMark removes exactly one on each side.
  const pad =
    flat.startsWith("`") ||
    flat.endsWith("`") ||
    flat.startsWith(" ") ||
    flat.endsWith(" ")
      ? " "
      : "";
  return `${ticks}${pad}${flat}${pad}${ticks}`;
}

/**
 * Returns the fence of a fenced code block for `text`: at least three backticks and one more
 * than its longest backtick run, so no line of the text can close it.
 *
 * @param text - The content of the block.
 * @returns The fence string.
 */
export function fenceFor(text: string): string {
  return "`".repeat(Math.max(3, longestRun(text, "`") + 1));
}

/**
 * Wraps text in a fenced code block that the text cannot close.
 *
 * @param text - Foreign text such as an issue body.
 * @param fence - The fence; default {@link fenceFor} of the text.
 * @returns The block, ending with the closing fence.
 */
export function fence(text: string, fence = fenceFor(text)): string {
  const body = text.endsWith("\n") ? text : `${text}\n`;
  return `${fence}\n${body}${fence}`;
}

/**
 * Returns `url` when it may become a link: an absolute `https` URL without whitespace, angle
 * brackets or parentheses.
 *
 * @param url - A URL from a payload.
 * @returns The URL, or `undefined` when no link may be written.
 */
export function safeLink(url: string): string | undefined {
  if (/[\s<>()]/u.test(url) || /\p{Cc}/u.test(url)) return undefined;
  try {
    return new URL(url).protocol === "https:" && url.startsWith("https://")
      ? url
      : undefined;
  } catch {
    return undefined;
  }
}

/**
 * Writes `label` as a link to `url`, or as plain text when the URL may not be linked.
 *
 * @param label - The visible text the app chose (never foreign text).
 * @param url - The target.
 * @returns Markdown.
 */
export function link(label: string, url: string): string {
  const target = safeLink(url);
  return target === undefined ? label : `[${label}](${target})`;
}

function utf8Length(text: string): number {
  return Buffer.byteLength(text, "utf8");
}

/**
 * Cuts text to at most `maxBytes` UTF-8 bytes at a code point boundary.
 *
 * @param text - The text.
 * @param maxBytes - The bound.
 * @returns The longest prefix within the bound.
 */
export function cutBytes(text: string, maxBytes: number): string {
  if (utf8Length(text) <= maxBytes) return text;
  let out = "";
  let used = 0;
  for (const c of text) {
    const size = utf8Length(c);
    if (used + size > maxBytes) break;
    out += c;
    used += size;
  }
  return out;
}

/**
 * Builds a message of a head line and an optional fenced block, cut to `maxBytes`: the fenced
 * text is shortened and {@link CUT_LINE} added, the fence always closed.
 *
 * @param head - The head line(s) the app wrote.
 * @param text - Foreign text for the fenced block; empty for none.
 * @param maxBytes - `limits.message_max_content_bytes`.
 * @returns The message content.
 */
export function withBlock(
  head: string,
  text: string,
  maxBytes: number,
): string {
  if (text === "") return cutBytes(head, maxBytes);
  const full = `${head}\n\n${fence(text)}`;
  if (utf8Length(full) <= maxBytes) return full;
  const fenceText = fenceFor(text);
  const frame = `${head}\n\n${fenceText}\n\n${fenceText}\n${CUT_LINE}`;
  const room = maxBytes - utf8Length(frame);
  if (room <= 0) return cutBytes(head, maxBytes);
  const cut = cutBytes(text, room);
  return `${head}\n\n${fence(cut, fenceText)}\n${CUT_LINE}`;
}

/**
 * Cleans a title for a thread (SEC-IN-4): control characters of category `Cc` and the bidi
 * controls are removed, line breaks become spaces, the result is trimmed and cut to `maxChars`
 * code points.
 *
 * @param title - Foreign title text.
 * @param maxChars - `limits.thread_title_max_chars`.
 * @returns The cleaned title; may be empty.
 */
export function cleanTitle(title: string, maxChars: number): string {
  const cleaned = title
    .replace(LINE_BREAK, " ")
    .replace(BIDI, "")
    .replace(CONTROL, "")
    .trim();
  return [...cleaned].slice(0, maxChars).join("").trim();
}

/**
 * Returns the bare title `#<n>` or `PR #<n>` of an item.
 *
 * @param item - Issue or pull request.
 * @param number - The number.
 * @returns The bare title.
 */
export function bareTitle(item: GitHubEvent["item"], number: number): string {
  return item === "pull_request"
    ? `PR #${String(number)}`
    : `#${String(number)}`;
}

/**
 * Returns the thread title of an item: `#<n> <title>` or `PR #<n> <title>`, cleaned and cut;
 * the bare title when nothing of the title is left.
 *
 * @param item - Issue or pull request.
 * @param number - The number.
 * @param title - The foreign title.
 * @param maxChars - `limits.thread_title_max_chars`.
 * @returns The title.
 */
export function threadTitle(
  item: GitHubEvent["item"],
  number: number,
  title: string,
  maxChars: number,
): string {
  const bare = bareTitle(item, number);
  const cleaned = cleanTitle(title, maxChars);
  if (cleaned === "") return [...bare].slice(0, maxChars).join("");
  return cleanTitle(`${bare} ${cleaned}`, maxChars);
}

/**
 * Renders the chat message of a GitHub event (Design note 9, English templates).
 *
 * @param event - The parsed event.
 * @param limits - The limits of `getInstance`.
 * @returns The message content.
 */
export function renderEvent(event: GitHubEvent, limits: RenderLimits): string {
  const max = limits.message_max_content_bytes;
  const ref = link(`#${String(event.number)}`, event.htmlUrl);
  const who = codeSpan(event.actor);
  switch (event.event) {
    case "issues":
      if (event.action === "opened")
        return withBlock(`**Issue ${ref} opened** by ${who}`, event.text, max);
      return cutBytes(`**Issue ${ref} ${event.action}** by ${who}`, max);
    case "issue_comment":
      return withBlock(`${who} commented on ${ref}:`, event.text, max);
    case "pull_request": {
      if (event.action === "opened")
        return withBlock(
          `**Pull request ${ref} opened** by ${who}`,
          event.text,
          max,
        );
      const verb =
        event.action === "closed" && event.merged === true
          ? "merged"
          : event.action;
      return cutBytes(`**Pull request ${ref} ${verb}** by ${who}`, max);
    }
    case "pull_request_review":
      return withBlock(
        `${who} reviewed ${ref}: ${codeSpan(event.reviewState ?? "")}`,
        event.text,
        max,
      );
  }
}

/**
 * Renders the GitHub comment of a mirrored chat message (Design note 12): the display name in a
 * code span and the content in a fenced block, so chat mentions never become GitHub mentions or
 * links.
 *
 * @param displayName - The author's display name.
 * @param content - The message content.
 * @returns The comment body.
 */
export function renderMirror(displayName: string, content: string): string {
  return `${codeSpan(displayName)} wrote in chat:\n\n${fence(content)}`;
}
