// SPDX-License-Identifier: Apache-2.0
/**
 * The HTTP surface of the app (brief M2-49 Design note 4).
 *
 * Routes `POST /github/webhook`, `POST /chat/interactions` and `POST /chat/events`; any other
 * path answers `404`, another method `405`. Per request, in this order: `Content-Type` must be
 * `application/json` (parameters allowed), else `415`; the body is read as bytes up to
 * `github_app.max_body_bytes` (a larger `Content-Length` answers `413` before reading, a stream
 * that grows beyond it is cut and answers `413`); then the route's handler decides. While the
 * app is stopping or not ready every route answers `503`. The app speaks plain HTTP behind the
 * TLS-terminating proxy of the instance; Node.js keeps its default header and request limits.
 *
 * @example
 * ```ts
 * const http = await startHttpServer({ host: "127.0.0.1", port: 0, maxBodyBytes, routes, ready, logger });
 * http.port; // the bound port
 * await http.close();
 * ```
 *
 * @module
 */

import {
  createServer,
  type IncomingHttpHeaders,
  type IncomingMessage,
  type ServerResponse,
} from "node:http";
import type { AddressInfo } from "node:net";

import type { Logger } from "bothy-sdk";

/** A request after the checks of the surface: headers and the raw body. */
export interface RawRequest {
  /** The request headers (lower-case names). */
  readonly headers: IncomingHttpHeaders;
  /** The body exactly as received. */
  readonly body: Uint8Array;
}

/** The answer of a route. */
export interface Reply {
  /** The status. */
  readonly status: number;
  /** A JSON body; absent for an empty body. */
  readonly json?: unknown;
}

/** A route: decides the answer of a checked request. */
export type RouteHandler = (request: RawRequest) => Promise<Reply>;

/** Options of {@link startHttpServer}. */
export interface HttpOptions {
  /** The host of the listener (empty for every interface). */
  readonly host: string;
  /** The port (0 lets the system choose). */
  readonly port: number;
  /** `github_app.max_body_bytes`. */
  readonly maxBodyBytes: number;
  /** The handler per path; every route takes `POST` only. */
  readonly routes: Readonly<Record<string, RouteHandler>>;
  /** `false` answers `503` (stopping, or not authenticated at the server). */
  readonly ready: () => boolean;
  /** The app's logger. */
  readonly logger: Logger;
}

/** A running listener. */
export interface HttpServer {
  /** The bound host. */
  readonly host: string;
  /** The bound port. */
  readonly port: number;
  /** Stops accepting and closes idle connections; resolves when the server closed. */
  close(): Promise<void>;
}

/** The body grew beyond the bound. */
class TooLarge extends Error {}

function isJson(contentType: string | undefined): boolean {
  if (contentType === undefined) return false;
  return contentType.split(";")[0]?.trim().toLowerCase() === "application/json";
}

function readBody(request: IncomingMessage, max: number): Promise<Uint8Array> {
  return new Promise((resolve, reject) => {
    const chunks: Buffer[] = [];
    let size = 0;
    const onData = (chunk: Buffer): void => {
      size += chunk.length;
      if (size > max) {
        request.off("data", onData);
        request.pause();
        reject(new TooLarge());
        return;
      }
      chunks.push(chunk);
    };
    request.on("data", onData);
    request.once("end", () => {
      resolve(new Uint8Array(Buffer.concat(chunks)));
    });
    request.once("error", reject);
  });
}

function send(response: ServerResponse, reply: Reply, close = false): void {
  const headers: Record<string, string> = {};
  if (close) headers.Connection = "close";
  if (reply.json === undefined) {
    response.writeHead(reply.status, headers).end();
    return;
  }
  const body = JSON.stringify(reply.json);
  headers["Content-Type"] = "application/json";
  response.writeHead(reply.status, headers).end(body);
}

/**
 * Starts the listener.
 *
 * @param options - Address, bound, routes, readiness and logger.
 * @returns The running server with its bound port.
 * @throws `Error` when the address cannot be bound.
 */
export async function startHttpServer(
  options: HttpOptions,
): Promise<HttpServer> {
  const { logger } = options;
  const server = createServer((request, response) => {
    void (async () => {
      const path = (request.url ?? "").split("?")[0] ?? "";
      const route = Object.hasOwn(options.routes, path)
        ? options.routes[path]
        : undefined;
      if (route === undefined) {
        send(response, { status: 404 });
        return;
      }
      if (request.method !== "POST") {
        response.setHeader("Allow", "POST");
        send(response, { status: 405 });
        return;
      }
      if (!options.ready()) {
        send(response, { status: 503 }, true);
        return;
      }
      if (!isJson(request.headers["content-type"])) {
        send(response, { status: 415 }, true);
        return;
      }
      const declared = Number(request.headers["content-length"] ?? "0");
      if (Number.isFinite(declared) && declared > options.maxBodyBytes) {
        logger.warn("http.body_too_large", {
          status: 413,
          reason: "content_length",
        });
        send(response, { status: 413 }, true);
        return;
      }
      let body: Uint8Array;
      try {
        body = await readBody(request, options.maxBodyBytes);
      } catch (err) {
        if (err instanceof TooLarge) {
          logger.warn("http.body_too_large", { status: 413, reason: "stream" });
          send(response, { status: 413 }, true);
          response.once("finish", () => request.destroy());
        } else {
          request.destroy();
        }
        return;
      }
      let reply: Reply;
      try {
        reply = await route({ headers: request.headers, body });
      } catch {
        logger.error("http.failed", { status: 503, reason: "internal" });
        reply = { status: 503 };
      }
      send(response, reply);
    })();
  });
  await new Promise<void>((resolve, reject) => {
    server.once("error", reject);
    server.listen(
      options.port,
      options.host === "" ? undefined : options.host,
      () => {
        server.off("error", reject);
        resolve();
      },
    );
  });
  const address = server.address() as AddressInfo;
  return {
    host: address.address,
    port: address.port,
    close: () =>
      new Promise<void>((resolve) => {
        server.close(() => {
          resolve();
        });
        server.closeIdleConnections();
      }),
  };
}
