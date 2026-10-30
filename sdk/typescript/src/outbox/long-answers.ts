// SPDX-License-Identifier: Apache-2.0
/**
 * Long answers stay one message (brief M4-47; docs/spec/01-product.md 6.4 and 6.13); the port
 * of `bothy_sdk.long_answers`.
 *
 * A reply is never split into several messages. {@link sendLongMessage} sends a text within
 * `message_max_content_bytes` as it is; a longer text goes out as one message that carries the
 * head ({@link splitForLimit}: never inside a character or a fenced code block) and the rest as
 * one text attachment {@link OVERFLOW_NAME}, uploaded through an upload session of mode `direct`
 * (docs/spec/12-files.md 3.2). When that attachment cannot be made, the head goes out with a
 * short note, still as one message. Every request goes through the given client, so a caller
 * inside a run's binding (07 SR9) uploads and sends under it.
 *
 * @example
 * ```ts
 * await sendLongMessage(api, outbox, threadId, channelId, answer, { runId });
 * ```
 *
 * @module
 */

import { createHash } from "node:crypto";

import { type ApiClient, IDEMPOTENCY_KEY_HEADER, unwrap } from "../client.js";
import { ProblemError } from "../errors.js";
import { uuidv7 } from "../ids.js";
import type { Outbox, SendMessageOptions } from "./outbox.js";
import type { OutboxEntry } from "./store.js";

/** The name of the attachment that carries the rest of a long answer (D-M4-47-3). */
export const OVERFLOW_NAME = "answer.txt";

const FENCE_MIN = 3;
const FENCE_INDENT_MAX = 3;
const encoder = new TextEncoder();

/** The head and the rest of a cut text; `rest` is absent when the text fits. */
export interface LimitSplit {
  /** The text of the message. */
  readonly head: string;
  /** The rest, sent as one attachment. */
  readonly rest: string | undefined;
}

/** Options of {@link sendLongMessage}. */
export interface SendLongMessageOptions extends SendMessageOptions {
  /** `message_max_content_bytes`; absent reads it once with `getInstance`. */
  readonly maxContentBytes?: number | undefined;
}

/**
 * The note after the head when the rest could not be attached.
 *
 * @param restBytes - The UTF-8 bytes of the rest that was not sent.
 * @returns A short English note.
 */
export function overflowNote(restBytes: number): string {
  return `\n\n(The rest of this answer, ${String(restBytes)} bytes, could not be attached.)`;
}

function fence(line: string): [string, number, string] | undefined {
  const stripped = line.replace(/^ +/, "");
  if (line.length - stripped.length > FENCE_INDENT_MAX || stripped === "")
    return undefined;
  const char = stripped[0] ?? "";
  if (char !== "`" && char !== "~") return undefined;
  let run = 0;
  while (stripped[run] === char) run++;
  if (run < FENCE_MIN) return undefined;
  return [char, run, stripped.slice(run)];
}

/** The offset of the line that opens a fenced block still open at the end of `head`. */
function openFenceStart(head: string): number | undefined {
  let open: [string, number, number] | undefined;
  let start = 0;
  for (const line of head.split("\n")) {
    const f = fence(line);
    if (open === undefined) {
      if (f !== undefined && !(f[0] === "`" && f[2].includes("`")))
        open = [f[0], f[1], start];
    } else if (
      f !== undefined &&
      f[0] === open[0] &&
      f[1] >= open[1] &&
      f[2].trim() === ""
    ) {
      open = undefined;
    }
    start += line.length + 1;
  }
  return open?.[2];
}

function utf8Size(codePoint: number): number {
  if (codePoint < 0x80) return 1;
  if (codePoint < 0x800) return 2;
  if (codePoint < 0x10000) return 3;
  return 4;
}

/**
 * Cuts `text` so the head has at most `maxBytes` UTF-8 bytes (D-M4-47-3).
 *
 * The head is the longest prefix within the limit that never splits a character, shortened to
 * its last line break when it has one. When the cut lies inside a fenced code block, the head
 * ends before the line that opens the block, so the block goes whole into the rest; a block that
 * starts the text stays cut, because the head would be empty. Head and rest together are always
 * exactly `text`.
 *
 * @param text - The answer.
 * @param maxBytes - `message_max_content_bytes` (at least 1).
 * @returns The head and the rest (absent when the text fits).
 *
 * @example
 * ```ts
 * splitForLimit("one\ntwo\nthree", 9); // { head: "one\ntwo\n", rest: "three" }
 * ```
 */
export function splitForLimit(text: string, maxBytes: number): LimitSplit {
  if (encoder.encode(text).length <= maxBytes)
    return { head: text, rest: undefined };
  let used = 0;
  let cut = 0;
  for (const char of text) {
    const size = utf8Size(char.codePointAt(0) ?? 0);
    if (used + size > maxBytes) break;
    used += size;
    cut += char.length;
  }
  if (cut > 0) {
    const newline = text.lastIndexOf("\n", cut - 1);
    if (newline >= 0) cut = newline + 1;
  }
  const opened = openFenceStart(text.slice(0, cut));
  if (opened !== undefined && opened > 0) cut = opened;
  return { head: text.slice(0, cut), rest: text.slice(cut) };
}

/**
 * Uploads `data` as the file `name` to `channelId` (12 3.2, mode `direct`).
 *
 * @param api - The REST client (inside the run's binding when called from a run).
 * @param channelId - The channel of the message the file will be attached to.
 * @param name - The file name.
 * @param data - The bytes (at least one).
 * @returns The `file_id` of the completed upload.
 * @throws {@link ProblemError} when the server refused a step (`413` above
 *   `file_max_upload_bytes`, `403` when uploads are off); `TypeError` for a session of another
 *   mode than `direct`; a transport error when no answer arrived.
 */
export async function uploadText(
  api: ApiClient,
  channelId: string,
  name: string,
  data: Uint8Array,
): Promise<string> {
  const session = await unwrap(
    api.POST("/channels/{channel_id}/uploads", {
      params: {
        path: { channel_id: channelId },
        header: { [IDEMPOTENCY_KEY_HEADER]: uuidv7() },
      },
      body: {
        name,
        size_bytes: data.length,
        sha256: createHash("sha256").update(data).digest("hex"),
        declared_mime_type: "text/plain",
      },
    }),
    "createUpload",
  );
  try {
    return await sendParts(api, session, data);
  } catch (error) {
    await abortUpload(api, session.file_id);
    throw error;
  }
}

/** The parts and the completion of an upload session of mode `direct`. */
async function sendParts(
  api: ApiClient,
  session: {
    file_id: string;
    mode: string;
    part_size_bytes: number;
    parts: {
      part_number: number;
      headers: { name: string; value: string }[];
    }[];
  },
  data: Uint8Array,
): Promise<string> {
  if (session.mode !== "direct")
    throw new TypeError(`upload mode ${session.mode} is not supported here`);
  const size = session.part_size_bytes;
  const parts = [];
  for (const part of session.parts) {
    const start = (part.part_number - 1) * size;
    const slice = data.subarray(start, start + size);
    const digest = createHash("sha256").update(slice).digest("base64");
    const headers: Record<string, string> = {};
    for (const h of part.headers) headers[h.name] = h.value;
    parts.push(
      await unwrap(
        api.PUT("/uploads/{file_id}/parts/{part_number}", {
          params: {
            path: { file_id: session.file_id, part_number: part.part_number },
            header: { "Content-Digest": `sha-256=:${digest}:` },
          },
          headers,
          // The part is raw bytes, not JSON.
          body: slice as unknown as string,
          bodySerializer: (body: string) => body,
        }),
        "uploadPart",
      ),
    );
  }
  const file = await unwrap(
    api.POST("/uploads/{file_id}/complete", {
      params: {
        path: { file_id: session.file_id },
        header: { [IDEMPOTENCY_KEY_HEADER]: uuidv7() },
      },
      body: { parts },
    }),
    "completeUpload",
  );
  return file.id;
}

/**
 * `abortUpload` of a session that will not complete (best effort), so it holds none of the
 * principal's open upload sessions until it expires.
 */
async function abortUpload(api: ApiClient, fileId: string): Promise<void> {
  try {
    await unwrap(
      api.DELETE("/uploads/{file_id}", {
        params: { path: { file_id: fileId } },
      }),
      "abortUpload",
    );
  } catch (error) {
    const code = error instanceof ProblemError ? error.code : "transport";
    api.logger.warn("long answer: upload not aborted", { code });
  }
}

async function readLimit(api: ApiClient): Promise<number> {
  const info = await unwrap(api.GET("/instance"), "getInstance");
  return info.limits.message_max_content_bytes;
}

/**
 * Sends `contentMd` as one message; its rest above the limit goes as one attachment.
 *
 * @param api - The REST client.
 * @param outbox - The outbox the message goes through.
 * @param threadId - The thread.
 * @param channelId - Its channel (the upload goes there).
 * @param contentMd - The whole answer.
 * @param options - `maxContentBytes` and the options of `Outbox.sendMessage`; the rest of a
 *   long text comes after the caller's `attachmentFileIds`.
 * @returns The outbox entry of the one message.
 */
export async function sendLongMessage(
  api: ApiClient,
  outbox: Outbox,
  threadId: string,
  channelId: string,
  contentMd: string,
  options: SendLongMessageOptions = {},
): Promise<OutboxEntry> {
  const { maxContentBytes, ...send } = options;
  const limit = maxContentBytes ?? (await readLimit(api));
  const files = [...(send.attachmentFileIds ?? [])];
  const split = splitForLimit(contentMd, limit);
  if (split.rest === undefined)
    return outbox.sendMessage(threadId, contentMd, send);
  let fileId: string;
  try {
    fileId = await uploadText(
      api,
      channelId,
      OVERFLOW_NAME,
      encoder.encode(split.rest),
    );
  } catch (error) {
    const code = error instanceof ProblemError ? error.code : "upload_failed";
    api.logger.warn("long answer: rest not attached", { code });
    // The note for the whole text bounds the note of any rest, so head and note fit together.
    const room = encoder.encode(
      overflowNote(encoder.encode(contentMd).length),
    ).length;
    const short = splitForLimit(contentMd, Math.max(1, limit - room));
    const note = overflowNote(encoder.encode(short.rest ?? "").length);
    return outbox.sendMessage(threadId, short.head + note, {
      ...send,
      attachmentFileIds: files,
    });
  }
  return outbox.sendMessage(threadId, split.head, {
    ...send,
    attachmentFileIds: [...files, fileId],
  });
}
