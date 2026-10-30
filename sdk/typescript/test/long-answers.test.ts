// SPDX-License-Identifier: Apache-2.0
//
// Long answers stay one message (brief M4-47; docs/spec/01-product.md 6.4 and 6.13); port of
// `sdk/python/tests/test_long_answers.py`. `splitForLimit` cuts a text so its head fits
// `message_max_content_bytes` without splitting a character or a fenced code block;
// `sendLongMessage` sends a text within the limit as it is and a longer one as one message whose
// rest is one text attachment, uploaded through an upload session of mode `direct` (12 3.2);
// when that attachment cannot be made the head goes out with a note, still one message. The
// scripted server refuses a `content_md` above the limit with `400 validation_failed`
// `too_long`, as the server does.

import { createHash } from "node:crypto";

import { describe, expect, it } from "vitest";

import { createApiClient, type ApiClient } from "../src/client.js";
import { silentLogger } from "../src/logger.js";
import {
  OVERFLOW_NAME,
  sendLongMessage,
  splitForLimit,
} from "../src/outbox/index.js";
import { Outbox, OutboxState } from "../src/outbox/index.js";

type Json = Record<string, unknown>;

const BASE = "https://example.com/api/v1";
const THREAD = "01900000-0000-7000-8000-00000000a001";
const CHANNEL = "01900000-0000-7000-8000-000000000103";
const AGENT = "01900000-0000-7000-8000-000000000301";

const bytes = (s: string): number => new TextEncoder().encode(s).length;

function problem(status: number, code: string, errors?: Json[]): Response {
  return Response.json(
    { type: "about:blank", title: code, status, code, errors },
    { status, headers: { "Content-Type": "application/problem+json" } },
  );
}

/** A scripted server of `getInstance`, the upload session and `createMessage`. */
class UploadServer {
  maxContent = 200;
  maxUpload = 1_000_000;
  partSize = 64;
  failParts = false;
  instanceReads = 0;
  readonly aborted: string[] = [];
  readonly messages: Json[] = [];
  readonly files = new Map<string, { name: string; data: Uint8Array[] }>();

  readonly fetch = async (request: Request): Promise<Response> => {
    const path = new URL(request.url).pathname.replace("/api/v1", "");
    const method = request.method;
    if (method === "GET" && path === "/instance") {
      this.instanceReads++;
      return Response.json({
        instance_id: "01900000-0000-7000-8000-000000000001",
        branding: { product_name: "Team Chat", version: 1 },
        locale_default: "en",
        spec_version: "1.0.0",
        features: [],
        limits: {
          message_max_content_bytes: this.maxContent,
          message_max_attachments: 10,
          file_max_upload_bytes: this.maxUpload,
          thread_title_max_chars: 200,
          voice_max_duration_ms: 900000,
        },
        app_lock: { required: false, after_ms: 300000 },
      });
    }
    if (method === "POST" && path === `/channels/${CHANNEL}/uploads`) {
      const body = (await request.json()) as Json;
      const size = Number(body.size_bytes);
      if (size > this.maxUpload) return problem(413, "payload_too_large");
      const id = `01900000-0000-7000-8000-${String(this.files.size + 1).padStart(12, "0")}`;
      this.files.set(id, { name: String(body.name), data: [] });
      const count = Math.max(1, Math.ceil(size / this.partSize));
      return Response.json(
        {
          file_id: id,
          mode: "direct",
          part_size_bytes: this.partSize,
          parts: Array.from({ length: count }, (_, i) => ({
            part_number: i + 1,
            url: `${BASE}/uploads/${id}/parts/${String(i + 1)}`,
            headers: [
              { name: "Content-Type", value: "application/octet-stream" },
            ],
          })),
          expires_at: "2026-01-02T00:00:00Z",
        },
        { status: 201 },
      );
    }
    const part = /^\/uploads\/([^/]+)\/parts\/(\d+)$/.exec(path);
    if (method === "PUT" && part !== null && this.failParts)
      return problem(503, "unavailable");
    if (method === "PUT" && part !== null) {
      const data = new Uint8Array(await request.arrayBuffer());
      const digest = createHash("sha256").update(data).digest("base64");
      expect(request.headers.get("Content-Digest")).toBe(`sha-256=:${digest}:`);
      this.files.get(part[1] ?? "")?.data.push(data);
      return Response.json({
        part_number: Number(part[2]),
        etag: `e${part[2] ?? ""}`,
      });
    }
    const abort = /^\/uploads\/([^/]+)$/.exec(path);
    if (method === "DELETE" && abort !== null) {
      this.aborted.push(abort[1] ?? "");
      return new Response(null, { status: 204 });
    }
    const done = /^\/uploads\/([^/]+)\/complete$/.exec(path);
    if (method === "POST" && done !== null) {
      const id = done[1] ?? "";
      return Response.json({
        id,
        channel_id: CHANNEL,
        original_name: this.files.get(id)?.name,
        mime_type: "text/plain",
        size_bytes: 1,
        sha256: "0".repeat(64),
        uploaded_by: AGENT,
        upload_state: "complete",
        scan_status: "pending",
        created_at: "2026-01-01T00:00:00Z",
      });
    }
    if (method === "POST" && path === `/threads/${THREAD}/messages`) {
      const body = (await request.json()) as Json;
      if (bytes(String(body.content_md)) > this.maxContent)
        return problem(400, "validation_failed", [
          { pointer: "/content_md", code: "too_long" },
        ]);
      this.messages.push(body);
      return Response.json(
        {
          id: "01900000-0000-7000-8000-00000000c001",
          thread_id: THREAD,
          channel_id: CHANNEL,
          thread_seq: 1,
          author: {
            id: AGENT,
            kind: "agent",
            handle: "lotse",
            display_name: "Lotse",
          },
          client_msg_id: body.client_msg_id,
          origin: "agent",
          kind: "text",
          content_md: body.content_md,
          content_format: 1,
          silent: false,
          attachments: [],
          mention_ids: [],
          reactions: [],
          pinned: false,
          components: [],
          version: 1,
          created_at: "2026-01-01T00:00:00Z",
        },
        { status: 201 },
      );
    }
    return problem(404, "not_found");
  };

  text(id: string): string {
    const parts = this.files.get(id)?.data ?? [];
    return Buffer.concat(parts.map((p) => Buffer.from(p))).toString("utf8");
  }
}

function client(server: UploadServer): ApiClient {
  return createApiClient({
    baseUrl: BASE,
    token: "example-api-token",
    fetch: server.fetch,
    settings: { retryMaxAttempts: 0 },
    logger: silentLogger,
  });
}

describe("splitForLimit", () => {
  it("leaves a text within the limit", () => {
    expect(splitForLimit("short", 10)).toEqual({
      head: "short",
      rest: undefined,
    });
  });

  it("cuts at the last line break within the limit and loses nothing", () => {
    const text = Array.from({ length: 50 }, (_, i) => `line ${String(i)}`).join(
      "\n",
    );
    const { head, rest } = splitForLimit(text, 100);
    expect(bytes(head)).toBeLessThanOrEqual(100);
    expect(head.endsWith("\n")).toBe(true);
    expect(head + (rest ?? "")).toBe(text);
  });

  it("never splits a character or a surrogate pair", () => {
    expect(splitForLimit("é".repeat(100), 51).head).toBe("é".repeat(25));
    expect(splitForLimit("\u{1F600}".repeat(20), 10).head).toBe(
      "\u{1F600}\u{1F600}",
    );
  });

  it("moves a fenced block whole into the rest", () => {
    const code = Array.from({ length: 40 }, (_, i) => `x = ${String(i)}`).join(
      "\n",
    );
    const { head, rest } = splitForLimit(
      `Intro line\n\`\`\`python\n${code}\n\`\`\`\nAfter`,
      120,
    );
    expect(head).toBe("Intro line\n");
    expect(rest?.startsWith("```python\n")).toBe(true);
  });

  it("property: every cut fits and loses nothing", () => {
    let seed = 4747;
    const next = (n: number): number => {
      seed = (seed * 1103515245 + 12345) % 2147483648;
      return seed % n;
    };
    const parts = ["a", "b", " ", "\n", "é", "\u{1F600}", "\n```\n"];
    for (let round = 0; round < 300; round++) {
      let text = "";
      const length = next(300);
      for (let i = 0; i < length; i++) text += parts[next(parts.length)] ?? "";
      const limit = 1 + next(200);
      const { head, rest } = splitForLimit(text, limit);
      expect(bytes(head)).toBeLessThanOrEqual(limit);
      expect(head + (rest ?? "")).toBe(text);
      expect(rest === undefined).toBe(bytes(text) <= limit);
    }
  });
});

describe("sendLongMessage", () => {
  it("sends an answer within the limit as one message without files", async () => {
    const server = new UploadServer();
    const api = client(server);
    const entry = await sendLongMessage(
      api,
      new Outbox(api),
      THREAD,
      CHANNEL,
      "Done.",
      {
        maxContentBytes: 200,
      },
    );
    expect(entry.state).toBe(OutboxState.SENT);
    expect(server.messages).toHaveLength(1);
    expect(server.messages[0]?.content_md).toBe("Done.");
    expect(server.messages[0]?.attachment_file_ids).toBeUndefined();
    expect(server.files.size).toBe(0);
    expect(server.instanceReads).toBe(0);
  });

  it("sends a long answer as one message with its rest attached", async () => {
    const server = new UploadServer();
    const api = client(server);
    const text = Array.from(
      { length: 80 },
      (_, i) => `Step ${String(i)}: done`,
    ).join("\n");
    const entry = await sendLongMessage(
      api,
      new Outbox(api),
      THREAD,
      CHANNEL,
      text,
      {
        runId: "01900000-0000-7000-8000-000000000101",
      },
    );
    expect(entry.state).toBe(OutboxState.SENT);
    expect(server.instanceReads).toBe(1);
    expect(server.messages).toHaveLength(1);
    const body = server.messages[0] ?? {};
    const head = String(body.content_md);
    expect(bytes(head)).toBeLessThanOrEqual(200);
    const [id] = [...server.files.keys()];
    expect(server.files.get(id ?? "")?.name).toBe(OVERFLOW_NAME);
    expect(body.attachment_file_ids).toEqual([id]);
    expect(head + server.text(id ?? "")).toBe(text);
  });

  it("sends the head with a note when the rest cannot be attached", async () => {
    const server = new UploadServer();
    server.maxUpload = 10;
    const api = client(server);
    const text = Array.from(
      { length: 80 },
      (_, i) => `Step ${String(i)}: done`,
    ).join("\n");
    const entry = await sendLongMessage(
      api,
      new Outbox(api),
      THREAD,
      CHANNEL,
      text,
    );
    expect(entry.state).toBe(OutboxState.SENT);
    expect(server.messages).toHaveLength(1);
    const head = String(server.messages[0]?.content_md);
    expect(bytes(head)).toBeLessThanOrEqual(200);
    expect(head).toContain("could not be attached");
    expect(server.messages[0]?.attachment_file_ids).toBeUndefined();
  });

  it("aborts an upload that failed before the note goes out", async () => {
    const server = new UploadServer();
    server.failParts = true;
    const api = client(server);
    const text = Array.from(
      { length: 80 },
      (_, i) => `Step ${String(i)}: done`,
    ).join("\n");
    const entry = await sendLongMessage(
      api,
      new Outbox(api),
      THREAD,
      CHANNEL,
      text,
    );
    expect(entry.state).toBe(OutboxState.SENT);
    expect(server.aborted).toEqual([...server.files.keys()]);
    expect(server.aborted).toHaveLength(1);
    expect(String(server.messages[0]?.content_md)).toContain(
      "could not be attached",
    );
  });
});
