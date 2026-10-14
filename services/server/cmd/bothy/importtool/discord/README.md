# cmd/bothy/importtool/discord

Read-only Discord client of the import tool `bothy import discord`
([22 Import and migration](../../../../../../docs/spec/22-import-migration.md), section 4;
ADR 0043). It reads one Discord server and never writes to it: every call is a `GET`.

## Calls

Only the calls of 22 4.1 exist; the paths are constants and no method takes a path, a verb or
a URL.

| Method | Call |
|---|---|
| `Me` | `GET /users/@me` |
| `Guild` | `GET /guilds/{guild.id}` |
| `GuildChannels` | `GET /guilds/{guild.id}/channels` |
| `ActiveThreads` | `GET /guilds/{guild.id}/threads/active` |
| `ArchivedPublicThreads` | `GET /channels/{channel.id}/threads/archived/public`, page loop |
| `ArchivedPrivateThreads` | `GET /channels/{channel.id}/threads/archived/private`, page loop; a `403` means `MANAGE_THREADS` is missing |
| `Channel` | `GET /channels/{channel.id}` |
| `MessagesAfter`, `MessagesBefore`, `MessagesAround` | `GET /channels/{channel.id}/messages?after\|before\|around=...&limit=...` |

The archived lists come newest first. The next page asks with `before` equal to the
`archive_timestamp` of the last thread of the page and `limit` equal to
`import.discord.page_size`; threads are deduplicated by ID, and the loop ends on `has_more`
false or on a page without a new ID. Message results are always in ascending ID order. Every
Discord ID is checked as a snowflake (digits only); an invalid one is `ErrInvalidID` naming the
field, never the value.

## Rate-limit guard

Per call (22 4.3 items 1 to 4):

1. If the last answer of the same bucket said `X-RateLimit-Remaining: 0`, wait
   `X-RateLimit-Reset-After` first. The bucket is the `X-RateLimit-Bucket` Discord named for the
   route, else the path template, together with the guild or channel ID.
2. On `429`, wait `retry_after` from the body, else `Retry-After` from the header, and retry, at
   most `import.discord.max_429_retries` times (then `ErrRateLimited`). A global `429` follows the
   same rules.
3. A missing or unparsable wait counts as `import.discord.max_rate_wait`; a wait longer than that
   ends the call at once with `ErrRateWaitTooLong`.
4. `401` (`ErrUnauthorized`) and `403` (`ErrForbidden`) are never retried. A `403` on a channel
   ends every later call on that channel for the life of the `Client` (one run), except a `403`
   on the private archived list.

`5xx` (`ErrServer`) and transport errors are returned, not retried. Waits go through
`Deps.Sleep` and the clock `Deps.Clock`, so tests never wait. Each request is bounded by
`import.http_timeout`.

## Token

`Token` holds the bot token unexported; every print, log and marshal form is `[redacted]`. The
value goes only into `Authorization: Bot <token>` of requests to the host and port of
`import.discord.api_base` over `https`; the setter checks the target before it sets the header
(`ErrTokenHostMismatch`), and redirects are never followed. Errors name method, path template and
status only. `SignedURL` (attachment links) prints scheme, host and path only; `Raw` returns the
full link for the download.

## Network

Every connection goes through `internal/platform/egress`: no environment proxy, `https` only, a
host policy of exactly the API host, the egress address checks. The package imports no server
module (`TestDiscordClientImportsNoServerModule`).

## Fake Discord for tests

`discordtest` starts a TLS fake on `127.0.0.1` with a free port under the host name
`discord.test` and returns its base URL, the test-only egress rule
`discord.test:<port>;allow_private`, a resolver and the root CAs:

```go
fake := discordtest.New(t, discordtest.BasicFixture(t))
client, err := discord.New(discord.Config{
	APIBase: fake.BaseURL, UserAgentVersion: "1.2.3", PageSize: 100, MaxRetries: 5,
	MaxRateWait: time.Minute, HTTPTimeout: time.Minute, Egress: fake.EgressConfig(),
}, token, discord.Deps{Resolver: fake.Resolver, RootCAs: fake.RootCAs, Clock: fakeClock, Sleep: recorder})
fake.Inject(discordtest.APIPath+"/users/@me", discordtest.RateLimitBody("1.5"))
fake.Always(discordtest.AnyPath, discordtest.Echo(http.StatusBadRequest))
```

`BasicFixture` loads `testdata/guild_basic.json` (a guild with categories, text, announcement,
forum, voice and uncategorized channels, active threads, three archive pages, a private archived
list that answers `403`, and messages of every type of 22 6.1). Faults per path: any status,
`429` with `retry_after` in the body, the header or both, a global `429`, `X-RateLimit-Remaining:
0` with `Reset-After`, redirects and an echo of the token and a signed query into the body and a
header. `Requests` returns the log (method, host, path, query, whether `Authorization` was
present, `User-Agent`); every method other than `GET` answers `405`. Only `_test.go` files import
`discordtest` (`TestNoTestPackageInBinary`).

## Not proven

The client was never run against the real Discord; header names and wait semantics follow the
cited documentation ([R4], [R6], [R7] of chapter 22).
