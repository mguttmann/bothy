// SPDX-License-Identifier: Apache-2.0
// The chat port on the SDK client (brief M2-49 Design note 9): request bodies without JSON null,
// the stored client_msg_id, Authorization, and ProblemError with status and code.

import { randomUUID } from "node:crypto";

import { ProblemError, createApiClient, uuidv7 } from "bothy-sdk";
import { webhookPublicKey } from "bothy-sdk/testing";
import { describe, expect, it } from "vitest";

import { SdkChatPort } from "../src/chat/sdk-adapter.js";

interface Seen {
  method: string;
  path: string;
  headers: Headers;
  body: string;
}

const PRINCIPAL = {
  id: randomUUID(),
  kind: "app",
  handle: "github",
  display_name: "GitHub",
};

function message(
  threadId: string,
  clientMsgId: string,
): Record<string, unknown> {
  return {
    id: randomUUID(),
    thread_id: threadId,
    channel_id: randomUUID(),
    thread_seq: 1,
    author: PRINCIPAL,
    client_msg_id: clientMsgId,
    origin: "app",
    kind: "text",
    content_md: "x",
    content_format: 1,
    silent: false,
    attachments: [],
    mention_ids: [],
    reactions: [],
    pinned: false,
    components: [],
    version: 1,
    created_at: new Date().toISOString(),
  };
}

function setup(answer: (seen: Seen) => Response) {
  const seen: Seen[] = [];
  const fetch = async (request: Request): Promise<Response> => {
    const entry = {
      method: request.method,
      path: new URL(request.url).pathname,
      headers: request.headers,
      body: request.body === null ? "" : await request.text(),
    };
    seen.push(entry);
    return answer(entry);
  };
  const api = createApiClient({
    baseUrl: "http://127.0.0.1:9/api/v1",
    token: "app-token-value",
    fetch,
    retry: { maxAttempts: 1 },
  });
  const publicApi = createApiClient({
    baseUrl: "http://127.0.0.1:9/api/v1",
    fetch,
    retry: { maxAttempts: 1 },
  });
  return { port: new SdkChatPort(api, publicApi), seen };
}

const json = (status: number, body: unknown): Response =>
  new Response(JSON.stringify(body), {
    status,
    headers: { "Content-Type": "application/json" },
  });

describe("SdkChatPort", () => {
  it("sends the writes without JSON null, with the stored identifiers and Authorization", async () => {
    const threadId = randomUUID();
    const { port, seen } = setup((s) => {
      if (s.path.endsWith("/threads"))
        return json(201, {
          thread: { id: threadId },
          first_message: message(threadId, "x"),
        });
      if (s.path.endsWith("/messages")) {
        const body = JSON.parse(s.body) as { client_msg_id: string };
        return json(201, message(threadId, body.client_msg_id));
      }
      return new Response(null, { status: 204 });
    });
    const msgId = uuidv7();
    const idem = uuidv7();
    expect(
      await port.createForumThread({
        channelId: randomUUID(),
        title: "#12 Title",
        firstMessage: { client_msg_id: msgId, content_md: "x" },
        idempotencyKey: idem,
      }),
    ).toEqual({ threadId });
    const posted = await port.postMessage(threadId, {
      client_msg_id: msgId,
      content_md: "y",
    });
    expect(posted.client_msg_id).toBe(msgId);
    await port.respondInteraction(
      randomUUID(),
      { state: "acknowledged", response: { text: "ok" } },
      idem,
    );
    for (const s of seen) {
      expect(s.body).not.toContain("null");
      expect(s.headers.get("authorization")).toBe("Bearer app-token-value");
      expect(s.headers.get("user-agent")).toContain("bothy-sdk/");
    }
    expect(JSON.parse(seen[0]?.body ?? "")).toEqual({
      title: "#12 Title",
      first_message: { client_msg_id: msgId, content_md: "x" },
    });
    expect(seen[0]?.headers.get("idempotency-key")).toBe(idem);
    expect(seen[2]?.headers.get("idempotency-key")).toBe(idem);
    expect(seen.map((s) => `${s.method} ${s.path}`)).toEqual([
      expect.stringMatching(/^POST \/api\/v1\/channels\/.+\/threads$/),
      `POST /api/v1/threads/${threadId}/messages`,
      expect.stringMatching(/^POST \/api\/v1\/interactions\/.+\/respond$/),
    ]);
  });

  it("reads me, the limits, a message and the webhook keys (those without the token)", async () => {
    const threadId = randomUUID();
    const key = webhookPublicKey(new Uint8Array(32).fill(7));
    const { port, seen } = setup((s) => {
      if (s.path.endsWith("/me"))
        return json(200, {
          principal: PRINCIPAL,
          is_owner: false,
          permission_version: 1,
        });
      if (s.path.endsWith("/instance"))
        return json(200, {
          instance_id: randomUUID(),
          branding: {},
          locale_default: "en",
          spec_version: "0.2.0",
          features: [],
          limits: {
            message_max_content_bytes: 100,
            message_max_attachments: 1,
            file_max_upload_bytes: 1,
            thread_title_max_chars: 10,
            voice_max_duration_ms: 1,
          },
        });
      if (s.path.endsWith("/webhook-keys"))
        return json(200, {
          keys: [
            {
              kid: "k",
              state: "active",
              public_key: key,
              not_before: new Date().toISOString(),
            },
          ],
        });
      return json(200, message(threadId, uuidv7()));
    });
    expect((await port.me()).id).toBe(PRINCIPAL.id);
    expect((await port.limits()).thread_title_max_chars).toBe(10);
    expect((await port.getMessage(randomUUID())).thread_id).toBe(threadId);
    expect(await port.webhookKeys()).toHaveLength(1);
    const keys = seen.find((s) => s.path.endsWith("/webhook-keys"));
    expect(keys?.headers.get("authorization")).toBeNull();
  });

  it("passes a problem answer on as ProblemError with status and code", async () => {
    const { port } = setup(() =>
      json(403, {
        type: "about:blank",
        title: "Forbidden",
        status: 403,
        code: "scope_missing",
      }),
    );
    const err = await port
      .postMessage(randomUUID(), { client_msg_id: uuidv7(), content_md: "x" })
      .catch((e: unknown) => e);
    expect(err).toBeInstanceOf(ProblemError);
    expect(err).toMatchObject({ status: 403, code: "scope_missing" });
  });
});
