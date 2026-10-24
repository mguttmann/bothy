// SPDX-License-Identifier: Apache-2.0
/**
 * The two GitHub REST calls of the app, written by hand (brief M2-49 Design note 14): create an
 * issue (`POST /repos/{owner}/{repo}/issues`) and create an issue comment
 * (`POST /repos/{owner}/{repo}/issues/{number}/comments`). Headers `Accept:
 * application/vnd.github+json`, `Authorization: Bearer <token>`, `X-GitHub-Api-Version:
 * 2022-11-28`, `User-Agent: github-app/<VERSION>`, `Content-Type: application/json`; time limit
 * `sdk.http_timeout`; redirects are not followed (a `3xx` is a failure); the response body is
 * read only on `201`; an error carries the status only. The token is read at each call, so a
 * rotation needs no restart; a token file that fails its check sends nothing (`not_sent`).
 *
 * References: https://docs.github.com/en/rest/issues/issues#create-an-issue,
 * https://docs.github.com/en/rest/issues/comments#create-an-issue-comment and
 * https://docs.github.com/en/rest/about-the-rest-api/api-versions (retrieved 2026-10-25).
 *
 * @example
 * ```ts
 * const github = new GitHubClient({ apiUrl, token: () => readSecretFile(file, key) });
 * const result = await github.createIssue("example-org/widgets", "Title", "Body");
 * if (result.kind === "created") record(result.number);
 * ```
 *
 * @module
 */

/** `X-GitHub-Api-Version` of every call. */
export const GITHUB_API_VERSION = "2022-11-28";

/** The version of the app (`VERSION`; `test/package.test.ts` holds both equal). */
export const APP_VERSION = "0.0.0";

/** What a create call ended with. */
export type GitHubResult =
  | {
      /** GitHub answered `201`. */
      readonly kind: "created";
      /** The issue number (for a comment: the number it was posted on). */
      readonly number: number;
      /** The ID of the created object. */
      readonly id: number;
      /** The `html_url` of the created object. */
      readonly htmlUrl: string;
    }
  | {
      /** GitHub answered `3xx` or `4xx`: nothing was created. */
      readonly kind: "refused";
      /** The status. */
      readonly status: number;
    }
  | {
      /** The token could not be read (its file failed the check): nothing was sent. */
      readonly kind: "not_sent";
      /** Always `secret_file`. */
      readonly reason: "secret_file";
    }
  | {
      /**
       * `5xx`, a time-out, a transport error or an unreadable `201`: GitHub may or may not have
       * created the object.
       */
      readonly kind: "unknown";
      /** The status when an answer arrived. */
      readonly status?: number | undefined;
      /** `timeout`, `transport` or `invalid_answer` when no usable answer arrived. */
      readonly reason?: string | undefined;
    };

/** Options of {@link GitHubClient}. */
export interface GitHubClientOptions {
  /** `github_app.github_api_url`. */
  readonly apiUrl: string;
  /** Reads the token at each call (`github_app.github_token_file`). */
  readonly token: () => Promise<string>;
  /** `sdk.http_timeout` in milliseconds. */
  readonly timeoutMs: number;
  /** `fetch` (tests pass the guarded one). */
  readonly fetch?: typeof globalThis.fetch | undefined;
}

/** The GitHub REST client of the app. */
export class GitHubClient {
  readonly #options: GitHubClientOptions;
  readonly #fetch: typeof globalThis.fetch;

  /**
   * Creates the client.
   *
   * @param options - Base URL, token source, time limit and `fetch`.
   */
  constructor(options: GitHubClientOptions) {
    this.#options = options;
    this.#fetch = options.fetch ?? globalThis.fetch;
  }

  /**
   * Creates an issue.
   *
   * @param repository - `<owner>/<name>`.
   * @param title - The title.
   * @param body - The body.
   * @returns The outcome.
   */
  createIssue(
    repository: string,
    title: string,
    body: string,
  ): Promise<GitHubResult> {
    return this.#post(`/repos/${repository}/issues`, { title, body }, undefined);
  }

  /**
   * Creates an issue comment (also on a pull request).
   *
   * @param repository - `<owner>/<name>`.
   * @param number - The issue or pull request number.
   * @param body - The comment.
   * @returns The outcome.
   */
  createComment(
    repository: string,
    number: number,
    body: string,
  ): Promise<GitHubResult> {
    return this.#post(
      `/repos/${repository}/issues/${String(number)}/comments`,
      { body },
      number,
    );
  }

  async #post(
    path: string,
    payload: Readonly<Record<string, string>>,
    number: number | undefined,
  ): Promise<GitHubResult> {
    let token: string;
    try {
      token = await this.#options.token();
    } catch {
      return { kind: "not_sent", reason: "secret_file" };
    }
    const controller = new AbortController();
    const timer = setTimeout(() => {
      controller.abort();
    }, this.#options.timeoutMs);
    timer.unref();
    let response: Response;
    try {
      response = await this.#fetch(`${this.#options.apiUrl}${path}`, {
        method: "POST",
        redirect: "manual",
        signal: controller.signal,
        headers: {
          Accept: "application/vnd.github+json",
          Authorization: `Bearer ${token}`,
          "X-GitHub-Api-Version": GITHUB_API_VERSION,
          "User-Agent": `github-app/${APP_VERSION}`,
          "Content-Type": "application/json",
        },
        body: JSON.stringify(payload),
      });
      if (response.status !== 201) {
        await response.body?.cancel().catch(() => undefined);
        return response.status >= 300 && response.status < 500
          ? { kind: "refused", status: response.status }
          : { kind: "unknown", status: response.status };
      }
      const created = (await response.json()) as Record<string, unknown>;
      const id = created.id;
      const url = created.html_url;
      const issueNumber = number ?? created.number;
      if (
        typeof id !== "number" ||
        typeof url !== "string" ||
        typeof issueNumber !== "number"
      )
        return { kind: "unknown", status: 201, reason: "invalid_answer" };
      return { kind: "created", number: issueNumber, id, htmlUrl: url };
    } catch {
      return {
        kind: "unknown",
        reason: controller.signal.aborted ? "timeout" : "transport",
      };
    } finally {
      clearTimeout(timer);
    }
  }
}
