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

## CDN fetch

`Fetcher` loads one attachment (22 6.5, REUSE 5.2): `NewFetcher(CDNConfig, reader, Deps)` and
`Fetch(ctx, AttachmentRef)`, which returns a `Download` whose `Body` the caller streams and
closes (M8-04 uploads it). Rules:

- `GET` only, `https` only (`ErrCDNScheme`), no user info, and a host and port that match an
  entry of `import.discord.cdn_hosts` exactly after lowercasing and dropping a trailing dot; an
  entry without a port allows only `443` (`ErrCDNHostNotAllowed`). The check runs before any
  connection, also on the fresh link after a re-read; `platform/egress` then checks the
  resolved addresses with a host policy of exactly the same entries.
- No credential: the request carries only `User-Agent` and `Accept` (and the
  `Accept-Encoding` of the transport); the fetcher has no token and no cookie jar.
- No redirect is followed: a `3xx` is `ErrCDNRedirect`, the `Location` is never read.
- Size: an attachment whose declared `size` is above `import.attachment_max_bytes` is not
  requested; a larger `Content-Length` is closed unread; the body returns `ErrTooLarge` once the
  byte after the limit arrives. Each request is bounded by `import.http_timeout`.
- Expired link: on `403` or `404` the message is read once more (`MessagesAround` with limit 1),
  the attachment with the same ID gives the fresh link, and the fetch is tried exactly once
  more; a missing message or attachment or a second `403` or `404` is `ErrCDNGone`.
- `429` waits `retry_after` or `Retry-After` within `import.discord.max_429_retries` and
  `import.discord.max_rate_wait`, like the API client; `5xx` is a `*CallError` without retry.
- Errors and log lines name only scheme, host and path (`SafeURL`); the error body of the CDN is
  never read, except the body of a `429` for its wait, which is never shown.
- Nothing is written to disk: no temp file, no cache.

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

`discordtest.NewCDN` is a fake CDN over TLS on `127.0.0.1` with a free port under
`cdn.discord.test`, `media.discord.test` and the foreign `evil.test`. It serves files of a given
size with or without `Content-Length`, accepts only a given `hm` signature per file (any other
answers `403`, like an expired link), and injects a redirect, any status, `429` with
`retry_after`, an oversize body behind a small `Content-Length` and an error body that echoes the
request URL. Its log records method, host, path, query, the header names and whether
`Authorization` or `Cookie` was present. `Hosts` and `EgressConfig` give the allow list and the
test-only egress rules (all three names with `allow_private`, so only the fetcher's own list
keeps requests away from `evil.test`).

## Not proven

The client was never run against the real Discord, and the fetcher never against its CDN; header
names, wait semantics and the expiry of signed links follow the cited documentation ([R4],
[R5], [R6], [R7] of chapter 22).
