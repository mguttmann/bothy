// SPDX-License-Identifier: Apache-2.0

// Package discord is the read-only Discord client of the import tool `bothy import discord`
// (docs/spec/22-import-migration.md 4, ADR 0043 rulings 1 and 2, R-IM-01, R-IM-03).
//
// The client knows only the calls of 22 4.1, and every one of them is a GET: Me, Guild,
// GuildChannels, ActiveThreads, ArchivedPublicThreads and ArchivedPrivateThreads (page loops
// newest first with before = archive_timestamp of the last thread, deduplicated by ID, ending on
// has_more false or a page without a new ID), Channel, and MessagesAfter, MessagesBefore and
// MessagesAround (always in ascending ID order). No method takes a path, a verb or a URL; the
// paths are constants, the IDs are checked as snowflakes before they enter a path, and the
// static test TestClientHasOnlyGet holds that the one request construction uses http.MethodGet.
//
// The bot token (Token) prints as [redacted] in every form. It goes only into the header
// "Authorization: Bot <token>" of requests to the host and port of import.discord.api_base over
// https, set by one unexported function that checks the target first; redirects are never
// followed. Error texts name the method, the path template and the status, never an ID, a
// query, a header or a body.
//
// The rate-limit guard (22 4.3 items 1 to 4) waits X-RateLimit-Reset-After before the next call
// of a bucket whose X-RateLimit-Remaining was 0, waits retry_after (body) or Retry-After
// (header) after a 429 and retries at most import.discord.max_429_retries times, ends a call with
// ErrRateWaitTooLong instead of waiting longer than import.discord.max_rate_wait, never retries
// 401 or 403, and remembers a 403 per channel for the life of the Client (one run). A missing or
// unparsable wait counts as import.discord.max_rate_wait. 5xx and transport errors are returned,
// not retried.
//
// Every connection goes through platform/egress (19 8, rule bothy-go-egress-only) with a host
// policy of exactly the API host, no environment proxy and https only. The package imports no
// server module; the tool talks to the server only through the public API.
//
// Fetcher loads attachments from the CDN (22 6.5): GET only, https only, hosts of
// import.discord.cdn_hosts only (exact match, port 443 unless the entry names one), no token
// or cookie, no redirects, a size limit of import.attachment_max_bytes on the declared size, the
// Content-Length and the bytes read, and on 403 or 404 one re-read of the message for a fresh
// link and one more try. Its errors and logs name scheme, host and path only (SafeURL), and the
// body streams to the caller, never to disk.
//
// Example:
//
//	tok, err := discord.NewToken(tokenText)
//	if err != nil {
//		return err
//	}
//	c, err := discord.New(cfg, tok, discord.Deps{Logger: logger})
//	if err != nil {
//		return err
//	}
//	err = c.ArchivedPublicThreads(ctx, channelID, func(page []discord.Channel) error {
//		return mapThreads(page)
//	})
//	if errors.Is(err, discord.ErrRateWaitTooLong) {
//		// end the run with exit 1; the state stays and a new run resumes (22 10.3)
//	}
//
// Tests use the fake Discord of the subpackage discordtest.
package discord
