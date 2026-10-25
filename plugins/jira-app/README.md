# jira-app

Status: built (M2-50). Reference app for ticket systems of
[docs/spec/11-extensions.md](../../docs/spec/11-extensions.md) section 10.2, version in
[`VERSION`](VERSION), changes in [`CHANGELOG.md`](CHANGELOG.md). It is a pattern for your own apps,
not a release component: operators build it from the repository (11 section 10).

## Purpose

- Jira issue and comment events go into a forum: one forum post per issue, one message per
  event (`jira:issue_created`, `jira:issue_updated` with its field changes, `comment_created`).
- `/issue` with a title opens a forum post with a form; the submitted form creates the issue
  through the Jira REST API, and the app answers in that post.
- Optionally, replies of people in a linked post go to Jira as comments (mirroring).

The app is its own Python process and uses only the public API through the Python SDK
(`bothy-sdk`, import package `bothy_sdk`, [sdk/python](../../sdk/python/README.md)); it is never
linked into the server.

## Layout

| Path | Content |
|---|---|
| `jira_app/__main__.py` | command line entry: `python -m jira_app --config <file>` |
| `jira_app/app.py` | `start_app(config, deps)`: checks, store, SDK client, worker, listener |
| `jira_app/config.py` | the configuration file and the checks of secret files and the state directory |
| `jira_app/log.py` | JSON log lines with an allow list of attributes |
| `jira_app/web.py` | the aiohttp application with the three routes and their checks |
| `jira_app/jira/` | signature check, event parser, ADF, the two Jira REST calls |
| `jira_app/chat/` | the port to the server, its SDK adapter, the endpoint handlers |
| `jira_app/domain/` | rendering of Jira text in the chat, mirroring rules |
| `jira_app/store/` | the `Store` port, the SQLite store and its in-memory form |
| `jira_app/worker/` | the queue worker and the jobs |
| `manifest.json` | the app manifest (11 section 3.1) |
| `config.example.json` | every configuration key |
| `tests/` | unit tests against the Fake-Jira API and an in-memory chat port; `tests/live/` against the real server |

## Configuration

`python -m jira_app --config <file>` reads one JSON object `jira_app` (11 section 16, place
"Jira app"); [`config.example.json`](config.example.json) shows every key. Unknown or duplicate
members, wrong types, bad durations and missing required keys abort the start; the message names
key and file, never a value.

| Key | Default | Meaning |
|---|---|---|
| `listen` | required | `host:port` of the HTTP listener |
| `server_url` | required | base address of the instance; `https`, `http` only for a loopback host |
| `token_file` | required | the app's API token (`createToken`), read once at start |
| `dpop_key_file` | empty | key of a DPoP-bound token (`auth.dpop_required_for`) |
| `webhook_secret_file` | required | the secret of the Jira webhook, read at each delivery |
| `jira_base_url` | required | base address of the Jira site (`https://<site>.atlassian.net` or a path prefix); `https`, `http` only for a loopback host |
| `jira_credentials_file` | required | one line `<account e-mail>:<API token>`, read at each call |
| `projects` | required | `[{"project": "<KEY>", "channel_id": "<forum>", "issue_type": "<name>"}]`; each key once; several projects may share a forum |
| `state_dir` | required | state directory, created with mode `0700` |
| `max_body_bytes` | `1048576` | largest request body on each endpoint |
| `dedupe_window` | `72h` | how long body digests, `webhook-id` values, created objects and drafts are kept |
| `max_queued_jobs` | `10000` | open jobs above which the endpoints answer `503` |

Keep `dedupe_window` at least as long as the server's `webhook.retry_max_age`, so a redelivery of
the server is still recognized (docs/api/webhooks.md section 3 point 4). The SDK values
`webhook.receiver_tolerance`, `sdk.http_timeout` (also the time limit of each Jira call),
`sdk.retry_max_attempts`, `sdk.retry_backoff_base` and `sdk.retry_backoff_cap` (also the pause
of the worker after a transient failure) keep the SDK's defaults.

Secret files (`token_file`, `dpop_key_file`, `webhook_secret_file`, `jira_credentials_file`)
must be regular files of the process user with mode `0400` or `0600`; otherwise the start aborts,
and a webhook secret or credentials file that fails the check later answers `503` or ends the
job. Rotate the Jira secret and credentials by replacing the files; the app token needs a
restart. The process sets its umask to `0o077`.

## Endpoints and the TLS proxy

| Route | Caller | Answer |
|---|---|---|
| `POST /jira/webhook` | Jira | `202` queued, `204` nothing to do or a duplicate, `401` signature, `400` JSON or payload, `413`, `415`, `503` |
| `POST /chat/interactions` | the server (`delivery.interactions_url`) | `202` for `/issue` and form submissions, `200` with `{"choices": []}` for autocomplete |
| `POST /chat/events` | the server (`delivery.events_url`, mirroring only) | `202` or `204` |

The app speaks plain HTTP on `listen` and runs behind the TLS-terminating proxy of the instance.
`delivery.interactions_url` and `delivery.events_url` of the manifest are `https` addresses of
that proxy (11 section 3.2 rule 3), for example with Caddy:

```text
jira-app.example.com {
    reverse_proxy 127.0.0.1:8091
}
```

Requests of the server are verified with the SDK (`bothy_sdk.webhooks.verify_webhook`, `v1a`
keys of `getWebhookKeys`, `webhook.receiver_tolerance`). Jira deliveries carry
`X-Hub-Signature: <method>=<hex signature>`: the app accepts only the method `sha256` and refuses
every other one before it computes anything, because Jira announces that the method may change
(RK-83); the HMAC-SHA256 over the raw body is compared in constant time before anything is
parsed.

## Installation

1. The operator adds the host and port of the two addresses to `webhook.allowed_hosts` of the
   server (changes take effect at the next start).
2. Edit `manifest.json`: replace `jira-app.example.com` with the proxy's address.
3. An owner installs the app with `installApp` and the required scopes `messages:write`,
   `commands:write` and `components:write` (the optional ones only for mirroring, below).
4. The owner grants `write` on each configured forum with `createGrant` (subject kind `app`).
   Grant `write` only on configured forums: `/issue` is listed where the app holds `write`, and
   in another forum the app ignores it (`channel_not_mapped`).
5. The owner creates the app's token with `createToken` and writes it to `token_file`. A scope
   added later needs a new token.
6. Run `uv run --directory plugins/jira-app --locked python -m jira_app --config <file>` from the
   repository root.

`/issue` has `required_level` `trigger`: the form is submitted with `interaction.create`, which
needs `trigger`, so people with `trigger` on the forum use it.

## Jira set-up

- Webhook (Jira administration, System, WebHooks): URL `https://<proxy>/jira/webhook`, a secret
  (the content of `webhook_secret_file`), the events "Issue created", "Issue updated" and
  "Comment created", and, as you choose, a JQL filter on the configured projects. Every other
  event and every issue of another project is answered `204`.
- Credentials: an Atlassian account and an API token for basic authentication, written as
  `<account e-mail>:<API token>` into `jira_credentials_file`. The account needs only the
  permissions to browse projects, create issues and add comments in the configured projects.
  Each project's `issue_type` must exist in that project.
- REST pages (Jira Cloud, REST version 3):
  https://developer.atlassian.com/cloud/jira/platform/rest/v3/api-group-issues/#api-rest-api-3-issue-post,
  https://developer.atlassian.com/cloud/jira/platform/rest/v3/api-group-issue-comments/#api-rest-api-3-issue-issueidorkey-comment-post,
  https://developer.atlassian.com/cloud/jira/platform/apis/document/structure/ (Atlassian
  Document Format) and
  https://developer.atlassian.com/cloud/jira/platform/basic-auth-for-rest-apis/ (basic
  authentication with an API token). Jira Data Center and Server are not supported.

## Mirroring, loops and foreign text

Mirroring needs `messages:read` and `events:subscribe` (`setAppScopes` with
`event_subscriptions` `[message.created]`), a subscription to `delivery.events_url` for
`message.created` (`createWebhookSubscription`, without `enable_hmac`) and a new token. Then a
message of a person (`origin` `human`, `kind` `text`) in a post linked to an issue becomes a Jira
comment: the display name as code, " wrote in chat:", and the text in a code block, always as
ADF text that Jira never reads as markup. Never mirrored: the app's own messages, messages of
agents, apps, imports and the system, cards, deleted messages. The `jira:issue_created` event of
an issue the app created and the `comment_created` event of a comment it created are not posted
back. Jira text never becomes a mention or a link in the chat: names and field values in code
spans, descriptions and comments in fenced code blocks, links only to
`<jira_base_url>/browse/<KEY>`.

The processor register of the operator lists Jira when the app is used
([docs/compliance/processors-template.md](../../docs/compliance/processors-template.md), row
"App `<name>`").

## Logs, state and lock

Logs are JSON lines on stdout with `time`, `level`, a fixed `msg` code and only the attributes
`event`, `delivery_id` (the first 16 hex digits of the SHA-256 of a Jira body), `webhook_id`,
`job_id`, `job_kind`, `step`, `operation`, `status`, `outcome`, `reason`, `code`, `issue_id`,
`thread_id` and `channel_id`; never summaries, descriptions, comments, form values, names,
e-mail addresses, account IDs, issue or project keys, URLs, header values, tokens or secrets.

`state_dir` holds `state.sqlite3` (thread map, drafts, created objects, seen identifiers, queued
jobs) and `lock`, held with `flock` while the process runs; a second process on the same
directory does not start, and the kernel releases the lock when the process ends. Jobs run one at
a time in creation order. A write to Jira is attempted at most once: after a crash during it the
outcome counts as unknown, and the person is told to check the project. SIGTERM and SIGINT stop
accepting (`503`), let the running step finish and exit with code 0.

## Tests

```sh
uv run --locked pytest                         # unit tests and the live test, with coverage
uv run --locked pytest --no-cov tests/live     # only the live test
uv run --locked ruff check . && uv run --locked mypy
```

The unit tests run against the Fake-Jira API (`tests/fakes/jira_api.py`) with fixtures after the
Jira documentation ([tests/fixtures/README.md](tests/fixtures/README.md)) and an in-memory chat
port. The live test starts the server of this tree through `services/server/test/sdkserver` with
a TLS front for the app's addresses and needs Go and a Docker-compatible runtime; without them it
fails, it never skips. No test reaches a host other than the loopback host (`tests/guard.py`).
