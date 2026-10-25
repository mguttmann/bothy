# github-app

Status: built (M2-49). Reference app for code hosting of
[docs/spec/11-extensions.md](../../docs/spec/11-extensions.md) section 10.1, version in
[`VERSION`](VERSION), changes in [`CHANGELOG.md`](CHANGELOG.md). It is a pattern for your own apps,
not a release component: operators build it from the repository (11 section 10).

## Purpose

- GitHub issue and pull request events go into a forum: one forum post per issue or pull
  request, one message per event.
- `/issue` with a title opens a forum post with a form; the submitted form creates the issue
  through the GitHub REST API, and the app answers in that post.
- Optionally, replies of people in a linked post go to GitHub as comments (mirroring).

The app is its own Node.js process and uses only the public API through the TypeScript SDK
(`bothy-sdk`, [sdk/typescript](../../sdk/typescript/README.md)); it is never linked into the
server.

## Layout

| Path | Content |
|---|---|
| `src/main.ts` | command line entry: `node dist/main.js --config <file>` |
| `src/app.ts` | `startApp(config, deps)`: checks, store, SDK client, worker, listener |
| `src/config.ts` | the configuration file and the checks of secret files and the state directory |
| `src/log.ts` | JSON log lines with an allow list of attributes |
| `src/http/server.ts` | the three endpoints and their checks |
| `src/github/` | signature check, event parser, the two GitHub REST calls |
| `src/chat/` | the port to the server, its SDK adapter, the endpoint handlers |
| `src/domain/` | keys, rendering of foreign text, mirroring and loop rules |
| `src/store/` | the `Store` port, `FileStore` and `MemoryStore` |
| `src/worker/` | the queue worker and the four jobs |
| `manifest.json` | the app manifest (11 section 3.1) |
| `config.example.json` | every configuration key |
| `test/` | unit tests with the Fake-GitHub API and an in-memory chat port; `test/live/` against the real server |

## Configuration

`node dist/main.js --config <file>` reads one JSON object `github_app` (11 section 16, place
"GitHub app"); [`config.example.json`](config.example.json) shows every key. Unknown members,
wrong types, bad durations and missing required keys abort the start; the message names key and
file, never a value.

| Key | Default | Meaning |
|---|---|---|
| `listen` | required | `host:port` of the HTTP listener (an IP address or empty for every interface) |
| `server_url` | required | base address of the instance; `https`, `http` only for a loopback host |
| `token_file` | required | the app's API token (`createToken`), read once at start |
| `dpop_key_file` | empty | key of a DPoP-bound token (`auth.dpop_required_for`) |
| `webhook_secret_file` | required | the secret of the GitHub webhook, read at each delivery |
| `github_token_file` | required | the GitHub token, read at each call |
| `github_api_url` | `https://api.github.com` | base address of the GitHub REST API |
| `repositories` | required | `[{"repository": "<owner>/<name>", "channel_id": "<forum>"}]`; each repository once; several may share a forum |
| `state_dir` | required | state directory, created with mode `0700` |
| `max_body_bytes` | `1048576` | largest request body on each endpoint |
| `dedupe_window` | `72h` | how long delivery identifiers, created objects and drafts are kept |
| `max_queued_jobs` | `10000` | open jobs above which the endpoints answer `503` |

Keep `dedupe_window` at least as long as the server's `webhook.retry_max_age`, so a redelivery of
the server is still recognised (docs/api/webhooks.md section 3 point 4). The SDK values
`webhook.receiver_tolerance`, `sdk.http_timeout` (also the time limit of each GitHub call),
`sdk.retry_max_attempts`, `sdk.retry_backoff_base` and `sdk.retry_backoff_cap` (also the pause
of the worker after a transient failure) keep the SDK's defaults.

Secret files (`token_file`, `dpop_key_file`, `webhook_secret_file`, `github_token_file`) must be
regular files of the process user with mode `0400` or `0600`; otherwise the start aborts, and a
GitHub secret or token that fails the check later answers `503` or ends the job. Rotate the GitHub
secret and token by replacing the files; the app token needs a restart.

## Endpoints and the TLS proxy

| Route | Caller | Answer |
|---|---|---|
| `POST /github/webhook` | GitHub | `202` queued, `204` nothing to do, `401` signature, `400` header or JSON, `413`, `415`, `503` |
| `POST /chat/interactions` | the server (`delivery.interactions_url`) | `202` for `/issue` and form submissions, `200` with `{"choices": []}` for autocomplete |
| `POST /chat/events` | the server (`delivery.events_url`, mirroring only) | `202` or `204` |

The app speaks plain HTTP on `listen` and runs behind the TLS-terminating proxy of the instance.
`delivery.interactions_url` and `delivery.events_url` of the manifest are `https` addresses of
that proxy (11 section 3.2 rule 3), for example with Caddy:

```text
github-app.example.com {
    reverse_proxy 127.0.0.1:8090
}
```

Requests of the server are verified with the SDK (`verifyWebhook`, `v1a` keys of
`getWebhookKeys`, `webhook.receiver_tolerance`); GitHub deliveries with `X-Hub-Signature-256`
(HMAC-SHA256 over the raw body, constant-time comparison) before anything is parsed.

## Installation

1. The operator adds the host and port of the two addresses to `webhook.allowed_hosts` of the
   server (changes take effect at the next start).
2. Edit `manifest.json`: replace `github-app.example.com` with the proxy's address.
3. An owner installs the app with `installApp` and the required scopes `messages:write`,
   `commands:write` and `components:write` (the optional ones only for mirroring, below).
4. The owner grants `write` on each configured forum with `createGrant` (subject kind `app`).
   Grant `write` only on configured forums: `/issue` is listed where the app holds `write`, and
   in another forum the app ignores it (`channel_not_mapped`).
5. The owner creates the app's token with `createToken` and writes it to `token_file`.
6. Run `npm ci --ignore-scripts` in `sdk/typescript` and here, `npm run build`, then
   `node dist/main.js --config <file>`.

`/issue` has `required_level` `trigger`: the form is submitted with `interaction.create`, which
needs `trigger`, so people with `trigger` on the forum use it.

## GitHub set-up

- Webhook of each configured repository: payload URL `https://<proxy>/github/webhook`, content
  type `application/json`, the secret of `webhook_secret_file`, and the events Issues, Issue
  comments, Pull requests and Pull request reviews. Handled are `issues` (`opened`, `closed`,
  `reopened`), `issue_comment` (`created`), `pull_request` (`opened`, `closed`, `reopened`),
  `pull_request_review` (`submitted`) and `ping`; everything else is answered `204`.
- Token: the token of `github_token_file` needs only the right to create issues and issue
  comments in the configured repositories (a fine-grained token with "Issues: read and write").
- REST pages: https://docs.github.com/en/rest/issues/issues#create-an-issue,
  https://docs.github.com/en/rest/issues/comments#create-an-issue-comment and
  https://docs.github.com/en/rest/about-the-rest-api/api-versions (header
  `X-GitHub-Api-Version: 2022-11-28`).

## Mirroring, loops and foreign text

Mirroring needs `messages:read` and `events:subscribe` (`setAppScopes` with
`event_subscriptions` `[message.created]`), a subscription to `delivery.events_url` for
`message.created` (`createWebhookSubscription`, without `enable_hmac`) and a new token. Then a
message of a person (`origin` `human`, `kind` `text`) in a post linked to an issue or pull request
becomes a GitHub comment: "`<display name>` wrote in chat:" and the text in a code block. Never
mirrored: the app's own messages, messages of agents, apps, imports and the system, cards, deleted
messages. The `issues` `opened` event of an issue the app created and the `issue_comment`
`created` event of a comment it created are not posted back. GitHub text never becomes a mention
or a link in the chat: logins in code spans, bodies in fenced code blocks, links only to the
event's `https` address.

The processor register of the operator lists GitHub when mirroring or `/issue` is used
([docs/compliance/processors-template.md](../../docs/compliance/processors-template.md), row
"App `<name>`").

## Logs, state and lock

Logs are JSON lines on stdout with `time`, `level`, a fixed `msg` code and only the attributes
`event`, `action`, `delivery_id`, `webhook_id`, `job_id`, `job_kind`, `step`, `operation`,
`status`, `outcome`, `reason`, `code`, `number`, `thread_id` and `channel_id`; never titles,
bodies, names, logins, repository names, URLs, header values, tokens or secrets.

`state_dir` holds `state.json` (thread map, drafts, created objects, seen identifiers), one file
per queued job under `jobs/`, and `lock` with the PID of the running process; a second process on
the same directory does not start, a lock of a process that is gone is taken over. Jobs run one
at a time in creation order. A write to GitHub is attempted at most once: after a crash during it
the outcome counts as unknown, and the person is told to check the repository.

## Tests

```sh
npm --prefix ../../sdk/typescript run build
npm test                                   # unit tests and the live test, with coverage
npx vitest run --project unit              # only the unit tests
npx vitest run --project live              # only the live test
```

The unit tests run against the Fake-GitHub API (`test/fakes/github-api.ts`) with payloads of the
GitHub documentation ([test/fixtures/README.md](test/fixtures/README.md)) and an in-memory chat
port. The live test starts the server of this tree through `services/server/test/sdkserver` with
a TLS front for the app's addresses and needs Go and a Docker-compatible runtime; without them it
fails, it never skips. No test reaches a host other than the loopback host
(`test/setup/no-network.ts`).
