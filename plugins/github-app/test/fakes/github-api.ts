// SPDX-License-Identifier: Apache-2.0
/**
 * The Fake-GitHub API (brief M2-49 Design note 17): an HTTP server on `127.0.0.1` port 0 with
 * the two endpoints of the app's GitHub client, and `deliver` for webhook deliveries signed like
 * GitHub signs them.
 *
 * The endpoints check `Authorization` (`401` without the test token), `Accept`,
 * `X-GitHub-Api-Version` and `User-Agent` (`400` otherwise), answer `404` for an unknown
 * repository and `422` for an empty title, assign increasing numbers and IDs, and record method,
 * path, JSON body and header names in memory. Faults are queued per call: a status (`500`,
 * `422`, `302`), an answer that never comes before the client gives up (`hang`), a dropped
 * connection (`drop`). With `autoEcho`, every issue or comment created through the fake is
 * followed by its `issues` `opened` or `issue_comment` `created` delivery.
 *
 * Nothing of a request is ever printed; tests compare in memory.
 *
 * @module
 */

import { createHmac, randomUUID } from "node:crypto";
import {
  createServer,
  type IncomingMessage,
  type Server,
  type ServerResponse,
} from "node:http";
import type { AddressInfo, Socket } from "node:net";

import { fixture } from "../helpers/fixtures.js";

/** A recorded request to the fake. */
export interface GitHubRequest {
  /** The method. */
  readonly method: string;
  /** The path. */
  readonly path: string;
  /** The JSON body. */
  readonly body: Record<string, unknown>;
  /** The header names in lower case. */
  readonly headerNames: readonly string[];
  /** The headers (tests compare values in memory only). */
  readonly headers: Readonly<Record<string, string>>;
  /** The status the fake answered with (0 for none). */
  readonly status: number;
}

/** A queued fault of the next call. */
export type GitHubFault =
  | { readonly status: number; readonly location?: string }
  | { readonly hang: true }
  | { readonly drop: true };

/** Options of {@link FakeGitHub.deliver}. */
export interface DeliverOptions {
  /** `omit`, `alter` (last digit changed), `truncate`, or an exact header value. */
  readonly signature?: "omit" | "alter" | "truncate" | string;
  /** The `X-GitHub-Delivery`; default a new UUID. */
  readonly deliveryId?: string;
  /** Leaves out `X-GitHub-Event`. */
  readonly omitEvent?: boolean;
  /** The `Content-Type`; default `application/json`. */
  readonly contentType?: string;
  /** Another secret than the fake's. */
  readonly secret?: string;
  /** A raw body instead of the JSON of the payload. */
  readonly rawBody?: Uint8Array;
}

/** Where `autoEcho` delivers. */
export interface EchoTarget {
  /** The app's `POST /github/webhook` URL. */
  readonly url: string;
}

/** Options of {@link FakeGitHub.start}. */
export interface FakeGitHubOptions {
  /** The token the endpoints accept. */
  readonly token: string;
  /** The webhook secret deliveries are signed with. */
  readonly secret: string;
  /** The repositories that exist (`<owner>/<name>`). */
  readonly repositories: readonly string[];
}

/** The Fake-GitHub API. */
export class FakeGitHub {
  /** Every request in arrival order. */
  readonly requests: GitHubRequest[] = [];
  /** The status of every delivery in order. */
  readonly deliveries: { readonly event: string; readonly status: number }[] = [];
  /** The base URL, `http://127.0.0.1:<port>`. */
  url = "";
  /** Where created objects echo to; `undefined` for no echo. */
  autoEcho: EchoTarget | undefined;
  readonly #options: FakeGitHubOptions;
  readonly #faults: GitHubFault[] = [];
  readonly #sockets = new Set<Socket>();
  readonly #echoes: Promise<unknown>[] = [];
  #server: Server | undefined;
  #nextNumber = 100;
  #nextId = 9_000_001;

  private constructor(options: FakeGitHubOptions) {
    this.#options = options;
  }

  /**
   * Starts the fake on `127.0.0.1` port 0.
   *
   * @param options - Token, secret and repositories.
   * @returns The running fake.
   */
  static async start(options: FakeGitHubOptions): Promise<FakeGitHub> {
    const fake = new FakeGitHub(options);
    const server = createServer((req, res) => {
      void fake.#handle(req, res);
    });
    server.on("connection", (socket) => {
      fake.#sockets.add(socket);
      socket.once("close", () => fake.#sockets.delete(socket));
    });
    await new Promise<void>((resolve) => server.listen(0, "127.0.0.1", resolve));
    fake.#server = server;
    fake.url = `http://127.0.0.1:${String((server.address() as AddressInfo).port)}`;
    return fake;
  }

  /** Stops the fake and ends every connection. */
  async stop(): Promise<void> {
    await Promise.allSettled(this.#echoes);
    for (const socket of this.#sockets) socket.destroy();
    await new Promise<void>((resolve) => this.#server?.close(() => resolve()) ?? resolve());
  }

  /**
   * Queues faults; each later call takes the first.
   *
   * @param faults - The faults.
   */
  fail(...faults: GitHubFault[]): void {
    this.#faults.push(...faults);
  }

  /** Waits for every echo delivery started so far. */
  async echoesDone(): Promise<void> {
    await Promise.allSettled(this.#echoes);
  }

  /** The requests that created an issue. */
  issues(): GitHubRequest[] {
    return this.requests.filter((r) => /\/issues$/.test(r.path));
  }

  /** The requests that created a comment. */
  comments(): GitHubRequest[] {
    return this.requests.filter((r) => /\/comments$/.test(r.path));
  }

  async #handle(req: IncomingMessage, res: ServerResponse): Promise<void> {
    const chunks: Buffer[] = [];
    for await (const chunk of req) chunks.push(chunk as Buffer);
    let body: Record<string, unknown> = {};
    try {
      body = JSON.parse(Buffer.concat(chunks).toString("utf8")) as Record<string, unknown>;
    } catch {
      body = {};
    }
    const headers: Record<string, string> = {};
    for (const [k, v] of Object.entries(req.headers))
      if (typeof v === "string") headers[k] = v;
    const path = req.url ?? "";
    const record = (status: number): void => {
      this.requests.push({
        method: req.method ?? "",
        path,
        body,
        headerNames: Object.keys(req.headers).sort(),
        headers,
        status,
      });
    };
    const fault = this.#faults.shift();
    if (fault !== undefined) {
      if ("hang" in fault) {
        record(0);
        return; // never answers; the client gives up and closes
      }
      if ("drop" in fault) {
        record(0);
        req.socket.destroy();
        return;
      }
      record(fault.status);
      if (fault.location !== undefined) res.setHeader("Location", fault.location);
      res.writeHead(fault.status, { "Content-Type": "application/json" });
      res.end(JSON.stringify({ message: "fault" }));
      return;
    }
    const answer = (status: number, json: unknown): void => {
      record(status);
      res.writeHead(status, { "Content-Type": "application/json" });
      res.end(JSON.stringify(json));
    };
    if (req.method !== "POST") return answer(404, { message: "Not Found" });
    if (headers.authorization !== `Bearer ${this.#options.token}`)
      return answer(401, { message: "Bad credentials" });
    if (
      headers.accept !== "application/vnd.github+json" ||
      headers["x-github-api-version"] !== "2022-11-28" ||
      !/^github-app\/\S+$/.test(headers["user-agent"] ?? "")
    )
      return answer(400, { message: "Bad request" });
    const match = /^\/repos\/([^/]+\/[^/]+)\/issues(?:\/(\d+)\/comments)?$/.exec(path);
    const repository = match?.[1];
    const known = this.#options.repositories.map((r) => r.toLowerCase());
    if (match === null || repository === undefined || !known.includes(repository.toLowerCase()))
      return answer(404, { message: "Not Found" });
    if (match[2] === undefined) {
      if (typeof body.title !== "string" || body.title === "")
        return answer(422, { message: "Validation Failed" });
      const number = this.#nextNumber++;
      const id = this.#nextId++;
      const htmlUrl = `https://github.com/${repository}/issues/${String(number)}`;
      answer(201, { id, number, html_url: htmlUrl, title: body.title, body: body.body });
      this.#echo("issues", "opened", repository, number, undefined, body);
      return;
    }
    const number = Number(match[2]);
    const id = this.#nextId++;
    answer(201, {
      id,
      html_url: `https://github.com/${repository}/issues/${String(number)}#issuecomment-${String(id)}`,
      body: body.body,
    });
    this.#echo("issue_comment", "created", repository, number, id, body);
  }

  #echo(
    event: string,
    action: string,
    repository: string,
    number: number,
    commentId: number | undefined,
    created: Record<string, unknown>,
  ): void {
    const target = this.autoEcho;
    if (target === undefined) return;
    const payload = fixture(event, action, { repository, number, commentId });
    const issue = payload.issue as Record<string, unknown>;
    if (event === "issues") {
      issue.title = created.title;
      issue.body = created.body;
    } else {
      (payload.comment as Record<string, unknown>).body = created.body;
    }
    this.#echoes.push(this.deliver(target.url, event, payload));
  }

  /**
   * Sends a delivery to the app the way GitHub does: `X-GitHub-Event`, `X-GitHub-Delivery`,
   * `X-Hub-Signature-256` (HMAC-SHA256 of the body with the secret), `Content-Type` and a
   * `User-Agent` of the form `GitHub-Hookshot/<id>`.
   *
   * @param url - The app's `POST /github/webhook` URL.
   * @param event - The `X-GitHub-Event`.
   * @param payload - The JSON payload.
   * @param options - Faults of the delivery.
   * @returns The status of the app's answer.
   */
  async deliver(
    url: string,
    event: string,
    payload: unknown,
    options: DeliverOptions = {},
  ): Promise<number> {
    const body = options.rawBody ?? new TextEncoder().encode(JSON.stringify(payload));
    const digest = createHmac("sha256", options.secret ?? this.#options.secret)
      .update(body)
      .digest("hex");
    let signature: string | undefined = `sha256=${digest}`;
    if (options.signature === "omit") signature = undefined;
    else if (options.signature === "alter")
      signature = `sha256=${digest.slice(0, -1)}${digest.endsWith("0") ? "1" : "0"}`;
    else if (options.signature === "truncate") signature = `sha256=${digest.slice(0, 8)}`;
    else if (options.signature !== undefined) signature = options.signature;
    const headers: Record<string, string> = {
      "Content-Type": options.contentType ?? "application/json",
      "User-Agent": `GitHub-Hookshot/${randomUUID().slice(0, 7)}`,
      "X-GitHub-Delivery": options.deliveryId ?? randomUUID(),
      "X-GitHub-Hook-ID": "1",
    };
    if (options.omitEvent !== true) headers["X-GitHub-Event"] = event;
    if (signature !== undefined) headers["X-Hub-Signature-256"] = signature;
    const response = await fetch(url, { method: "POST", headers, body });
    await response.body?.cancel();
    this.deliveries.push({ event, status: response.status });
    return response.status;
  }
}
