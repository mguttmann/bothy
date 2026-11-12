// SPDX-License-Identifier: Apache-2.0
/**
 * Upload sessions, instance limits and attaching for the FakeServer (private; port of
 * `bothy_sdk/testing/_fake_uploads.py`).
 *
 * `getInstance` answers with configurable `InstanceLimits`. `createUpload`, `uploadPart`,
 * `completeUpload` and `abortUpload` follow docs/spec/12-files.md 3 in mode `direct`: a small
 * `part_size_bytes` per test, length and `Content-Digest` of each part checked, the same part
 * with the same digest answers again without new work and with another digest `409 conflict`
 * with `reason` `part_conflict`, a completed session cannot be aborted
 * (`409 invalid_state_transition`). The received bytes are kept per file, so a test compares
 * them with what it sent. `createMessage` attaches only completed own uploads of the message's
 * channel that are not attached yet (`409 conflict` with `reason` `file_not_attachable`
 * otherwise). An upload is never scanned (`scan_status` `pending`), so a voice message with one
 * answers `409 file_not_ready` (12 8.1).
 *
 * @module
 */

import { createHash } from "node:crypto";

import type { components } from "../../generated/openapi.js";
import { uuidv7 } from "../../ids.js";
import {
  byCodeUnit,
  canonicalUuid,
  FakeProblem,
  fieldError,
  iso,
  isPlainObject,
} from "./base.js";

type S = components["schemas"];

/** `InstanceLimits` of the FakeServer (the examples of docs/api/openapi.yaml). */
export const DEFAULT_LIMITS: Readonly<S["InstanceLimits"]> = Object.freeze({
  message_max_content_bytes: 262_144,
  message_max_attachments: 10,
  file_max_upload_bytes: 104_857_600,
  thread_title_max_chars: 200,
  voice_max_duration_ms: 900_000,
});

const DEFAULT_PART_SIZE = 5 * 1024 * 1024;
const SESSION_TTL_MS = 24 * 60 * 60 * 1000;
const SHA256_HEX = /^[0-9a-f]{64}$/;
const UPLOAD_MEMBERS = ["name", "size_bytes", "sha256", "declared_mime_type"];

/** One confirmed part: its digest header, its bytes and its etag. */
interface StoredPart {
  readonly digest: string;
  readonly data: Uint8Array;
  readonly etag: string;
}

/** One upload session and its file. */
export class Upload {
  /** Confirmed parts by number. */
  readonly parts = new Map<number, StoredPart>();
  /** `open`, `complete` or `aborted`. */
  state: "open" | "complete" | "aborted" = "open";
  /** The message the file is attached to, if any. */
  attached: string | undefined;

  /**
   * Creates the session.
   *
   * @param fileId - The file.
   * @param channelId - `scope_channel_id`.
   * @param uploader - `uploaded_by`.
   * @param name - `original_name`.
   * @param size - `size_bytes`.
   * @param sha256 - The declared lowercase hex SHA-256.
   * @param partSize - `part_size_bytes`.
   * @param createdAt - The creation time.
   * @param expiresAt - When the session expires.
   */
  constructor(
    readonly fileId: string,
    readonly channelId: string,
    readonly uploader: string,
    readonly name: string,
    readonly size: number,
    readonly sha256: string,
    readonly partSize: number,
    readonly createdAt: string,
    readonly expiresAt: string,
  ) {}

  /** How many parts the session has. */
  get partCount(): number {
    return Math.max(1, Math.ceil(this.size / this.partSize));
  }

  /**
   * Returns the received bytes in part order.
   *
   * @returns The bytes.
   */
  content(): Uint8Array {
    const numbers = [...this.parts.keys()].sort((a, b) => a - b);
    return Buffer.concat(
      numbers.map((n) => this.parts.get(n)?.data ?? new Uint8Array()),
    );
  }

  /**
   * Returns the `File` of the upload.
   *
   * @returns The document.
   */
  fileDoc(): S["File"] {
    const doc: S["File"] = {
      id: this.fileId,
      size_bytes: this.size,
      sha256: this.sha256,
      mime_type: "application/octet-stream",
      original_name: this.name,
      uploaded_by: this.uploader,
      purpose: "attachment",
      upload_state: this.state === "complete" ? "complete" : "pending",
      scan_status: "pending",
      transcript_status: "none",
      thumbnails: [],
      created_at: this.createdAt,
    };
    if (this.attached === undefined) doc.scope_channel_id = this.channelId;
    return doc;
  }
}

/** The upload part of the FakeServer world. */
export class FakeUploads {
  /** The `InstanceLimits` of `getInstance`; a test may change them. */
  limits: S["InstanceLimits"] = { ...DEFAULT_LIMITS };
  /** `part_size_bytes` of new sessions; a test sets a small one. */
  partSize = DEFAULT_PART_SIZE;
  /** The sessions by file ID. */
  readonly sessions = new Map<string, Upload>();

  /**
   * Creates an empty store with the default limits.
   *
   * @param now - The FakeServer clock, Unix milliseconds.
   * @param uploader - The agent whose token the FakeServer serves (`uploaded_by`).
   */
  constructor(
    private readonly now: () => number,
    private readonly uploader: string,
  ) {}

  /**
   * `getInstance`: an `InstanceInfo` with the configured limits.
   *
   * @param instanceId - The instance.
   * @returns The document.
   */
  instance(instanceId: string): S["InstanceInfo"] {
    return {
      instance_id: instanceId,
      branding: { product_name: "Team Chat", version: 1 },
      locale_default: "en",
      spec_version: "1.0.0",
      features: [],
      limits: { ...this.limits },
      app_lock: { required: false, after_ms: 300_000 },
    };
  }

  private get(fileId: string): Upload {
    const upload = this.sessions.get(canonicalUuid(fileId) ?? "");
    if (upload === undefined || upload.uploader !== this.uploader)
      throw new FakeProblem("not_found");
    return upload;
  }

  /**
   * `createUpload` (mode `direct`).
   *
   * @param channelId - The channel of the upload.
   * @param body - The request body.
   * @returns The upload session.
   * @throws {@link FakeProblem} for a body outside `UploadCreate` or a size above the limit.
   */
  create(channelId: string, body: unknown): S["UploadSession"] {
    if (!isPlainObject(body)) throw fieldError("", "required");
    const unknown = Object.keys(body)
      .filter((n) => !UPLOAD_MEMBERS.includes(n))
      .sort(byCodeUnit);
    if (unknown.length) throw fieldError(`/${unknown[0]}`, "unknown_member");
    for (const name of ["name", "size_bytes", "sha256"])
      if (!(name in body)) throw fieldError(`/${name}`, "required");
    const size = body.size_bytes;
    const sha = body.sha256;
    if (typeof size !== "number" || !Number.isInteger(size) || size < 1)
      throw fieldError("/size_bytes", "too_small");
    if (typeof sha !== "string" || !SHA256_HEX.test(sha))
      throw fieldError("/sha256", "pattern_mismatch");
    if (size > this.limits.file_max_upload_bytes)
      throw new FakeProblem("payload_too_large");
    const now = this.now();
    const upload = new Upload(
      uuidv7(),
      canonicalUuid(channelId) ?? channelId,
      this.uploader,
      typeof body.name === "string" ? body.name : JSON.stringify(body.name),
      size,
      sha,
      this.partSize,
      iso(now),
      iso(now + SESSION_TTL_MS),
    );
    this.sessions.set(upload.fileId, upload);
    const parts: S["UploadPart"][] = [];
    for (let n = 1; n <= upload.partCount; n++)
      parts.push({
        part_number: n,
        url: `/api/v1/uploads/${upload.fileId}/parts/${n}`,
        headers: [{ name: "Content-Type", value: "application/octet-stream" }],
      });
    return {
      file_id: upload.fileId,
      mode: "direct",
      part_size_bytes: upload.partSize,
      parts,
      expires_at: upload.expiresAt,
    };
  }

  /**
   * `uploadPart`: length and digest checked, idempotent per digest.
   *
   * @param fileId - The file.
   * @param partNumber - The part number of the path.
   * @param data - The raw body.
   * @param digest - The `Content-Digest` header.
   * @returns The confirmed part.
   * @throws {@link FakeProblem} `404`, `410`, `409` or `400` as the server answers.
   */
  putPart(
    fileId: string,
    partNumber: string,
    data: Uint8Array,
    digest: string | undefined,
  ): S["UploadPartResult"] {
    const upload = this.get(fileId);
    if (upload.state === "aborted") throw new FakeProblem("gone");
    if (upload.state === "complete")
      throw new FakeProblem("invalid_state_transition");
    if (!/^\d+$/.test(partNumber))
      throw fieldError("#path/part_number", "invalid_type");
    const number = Number(partNumber);
    if (number < 1 || number > upload.partCount)
      throw fieldError("#path/part_number", "out_of_range");
    const start = (number - 1) * upload.partSize;
    const expected = Math.min(upload.partSize, upload.size - start);
    const wanted = `sha-256=:${createHash("sha256").update(data).digest("base64")}:`;
    if (data.length !== expected || digest !== wanted)
      throw fieldError("", "part_digest_mismatch");
    const stored = upload.parts.get(number);
    if (stored !== undefined) {
      if (stored.digest !== digest)
        throw new FakeProblem("conflict", { reason: "part_conflict" });
      return { part_number: number, etag: stored.etag };
    }
    // An opaque value per content, in the form of the contract's example.
    const etag = `"${createHash("sha256").update(data).digest("hex").slice(0, 32)}"`;
    upload.parts.set(number, { digest, data: new Uint8Array(data), etag });
    return { part_number: number, etag };
  }

  /**
   * `completeUpload`: every part confirmed with its etag (the scan checks the hash later).
   *
   * @param fileId - The file.
   * @param body - The request body.
   * @returns The file.
   * @throws {@link FakeProblem} `404`, `410`, `409` or `400` as the server answers.
   */
  complete(fileId: string, body: unknown): S["File"] {
    const upload = this.get(fileId);
    if (upload.state === "aborted") throw new FakeProblem("gone");
    // A completed session answers the file again (12 3.4).
    if (upload.state === "complete") return upload.fileDoc();
    if (!isPlainObject(body) || !Array.isArray(body.parts))
      throw fieldError("/parts", "required");
    const given = new Map<number, unknown>();
    for (const item of body.parts as unknown[]) {
      if (!isPlainObject(item) || !Number.isInteger(item.part_number))
        throw fieldError("/parts", "invalid_type");
      const number = item.part_number as number;
      if (given.has(number) || number < 1 || number > upload.partCount)
        throw fieldError("/parts", "invalid_value");
      given.set(number, item.etag);
    }
    if (
      given.size !== upload.partCount ||
      [...given.keys()].some((n) => !upload.parts.has(n))
    )
      throw new FakeProblem("conflict", { reason: "upload_incomplete" });
    for (const [number, etag] of given)
      if (upload.parts.get(number)?.etag !== etag)
        throw new FakeProblem("conflict", { reason: "part_conflict" });
    upload.state = "complete";
    return upload.fileDoc();
  }

  /**
   * `abortUpload`: an open session ends; a completed one cannot be aborted.
   *
   * @param fileId - The file.
   * @throws {@link FakeProblem} `404` or `409 invalid_state_transition`.
   */
  abort(fileId: string): void {
    const upload = this.get(fileId);
    if (upload.state === "complete")
      throw new FakeProblem("invalid_state_transition");
    upload.state = "aborted";
    upload.parts.clear();
  }

  /**
   * Returns the uploads of `attachment_file_ids` of a new message, or throws the problem.
   *
   * @param ids - The member `attachment_file_ids` of the request.
   * @param channelId - The channel of the message.
   * @returns The uploads in the order of the list.
   * @throws {@link FakeProblem} `400` for a malformed or too long list, `409 conflict` with
   *   `reason` `file_not_attachable` for a file that is not attachable.
   */
  checkAttachable(ids: unknown, channelId: string): Upload[] {
    if (!Array.isArray(ids))
      throw fieldError("/attachment_file_ids", "invalid_type");
    if (ids.length > this.limits.message_max_attachments)
      throw fieldError("/attachment_file_ids", "limit_reached");
    const out: Upload[] = [];
    for (const [index, raw] of (ids as unknown[]).entries()) {
      const upload =
        typeof raw === "string"
          ? this.sessions.get(canonicalUuid(raw) ?? "")
          : undefined;
      if (
        upload === undefined ||
        upload.uploader !== this.uploader ||
        upload.channelId !== channelId ||
        upload.state !== "complete" ||
        upload.attached !== undefined ||
        out.includes(upload)
      )
        throw new FakeProblem("conflict", {
          reason: "file_not_attachable",
          detail: `/attachment_file_ids/${index}`,
        });
      out.push(upload);
    }
    return out;
  }

  /**
   * Attaches checked uploads to a stored message in their order.
   *
   * @param uploads - The uploads of {@link FakeUploads.checkAttachable}.
   * @param message - The stored message.
   */
  attach(uploads: readonly Upload[], message: S["Message"]): void {
    for (const [position, upload] of uploads.entries()) {
      upload.attached = message.id;
      message.attachments.push({ position, file: upload.fileDoc() });
    }
  }
}
