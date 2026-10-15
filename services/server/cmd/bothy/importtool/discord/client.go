// SPDX-License-Identifier: Apache-2.0

package discord

import (
	"context"
	"crypto/x509"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/OWNER/bothy/services/server/internal/kernel/clock"
	"github.com/OWNER/bothy/services/server/internal/platform/egress"
)

// userAgentURL is the project URL of the User-Agent "DiscordBot (<url>, <version>)" that Discord
// requires for bots ([R6]); the same placeholder as repo_url of mkdocs.yml.
const userAgentURL = "https://github.com/OWNER/bothy"

// userAgent returns the User-Agent of the tool for version.
func userAgent(version string) string { return "DiscordBot (" + userAgentURL + ", " + version + ")" }

// validUserAgentVersion reports whether version fits into the User-Agent.
func validUserAgentVersion(version string) bool {
	return version != "" && !strings.ContainsAny(version, "(),\r\n")
}

// maxPageSize is the largest page Discord serves ([R4]); import.discord.page_size must not
// exceed it.
const maxPageSize = 100

// Log attribute keys of the client. Records name the call by its path template and never carry
// an ID, a query, a header or a body.
const (
	attrOp      = "bothy.import.discord.op"
	attrStatus  = "http.response.status_code"
	attrAttempt = "bothy.import.discord.attempt"
	attrWait    = "bothy.import.discord.wait"
)

// egressPurpose labels the client in the logs of platform/egress.
const egressPurpose = "discord_import"

// Config are the values of the client (22 19).
type Config struct {
	// APIBase is import.discord.api_base: an https URL without user info, query or fragment.
	// The token goes to its host and port only.
	APIBase string
	// UserAgentVersion is the version of the tool in the User-Agent; without "(", ")" and ",".
	UserAgentVersion string
	// PageSize is import.discord.page_size: the limit of an archived thread page and the
	// largest limit of a message page, 1 up to Discord's maximum.
	PageSize int
	// MaxRetries is import.discord.max_429_retries: retries after 429 per call, 0 or more.
	MaxRetries int
	// MaxRateWait is import.discord.max_rate_wait: the longest single wait; a longer one ends
	// the call with ErrRateWaitTooLong. Positive.
	MaxRateWait time.Duration
	// HTTPTimeout is import.http_timeout: the time limit of one request. Positive.
	HTTPTimeout time.Duration
	// Egress are the egress.* values of the tool (allowed_hosts, timeout, max_response_bytes);
	// the host policy of the client adds exactly the host of APIBase, and only an
	// allowed_hosts rule with allow_private can open a private address (tests).
	Egress egress.Config
}

// Deps are the dependencies of the client.
type Deps struct {
	// Resolver resolves host names; nil means the system resolver.
	Resolver egress.Resolver
	// RootCAs are the trusted TLS roots; nil means the system roots.
	RootCAs *x509.CertPool
	// Logger receives debug records per call and info records per wait; nil discards them.
	// Records carry the path template, the status and the wait, never IDs, queries or headers.
	Logger *slog.Logger
	// Clock is the time source of the rate-limit guard; nil means clock.System().
	Clock clock.Clock
	// Sleep waits; nil means a timer that ends with the context.
	Sleep Sleeper
}

// Client reads one Discord server through the calls of 22 4.1, all GET, through
// platform/egress. It is safe for concurrent use. Its rate-limit state and the set of channels
// a 403 ended live as long as the value: one value per run (22 4.3).
type Client struct {
	http      *egress.Client
	base      *url.URL
	api       apiTarget
	token     Token
	userAgent string
	pageSize  int
	retries   int
	maxWait   time.Duration
	logger    *slog.Logger
	sleep     Sleeper
	guard     *guard
}

// New builds a client from the configuration, the bot token and the dependencies. It returns
// an error, naming the field and never its value, when APIBase is not an https URL with a host
// and without user info, query or fragment, UserAgentVersion is empty or contains "(", ")" or
// ",", PageSize is outside 1 to Discord's maximum, MaxRetries is negative, MaxRateWait or
// HTTPTimeout is not positive, the token is the zero Token, or egress.New rejects Egress.
//
// Example:
//
//	tok, err := discord.NewToken(tokenFromFile)
//	c, err := discord.New(discord.Config{APIBase: "https://discord.com/api/v10", UserAgentVersion: version,
//		PageSize: 100, MaxRetries: 5, MaxRateWait: time.Minute, HTTPTimeout: time.Minute, Egress: egressCfg},
//		tok, discord.Deps{Logger: logger})
//	me, err := c.Me(ctx)
func New(cfg Config, token Token, deps Deps) (*Client, error) {
	base, err := validateConfig(cfg)
	if err != nil {
		return nil, err
	}
	if token.empty() {
		return nil, errors.New("discord: the bot token is empty")
	}
	api := targetOf(base)
	port, err := strconv.ParseUint(api.port, 10, 16)
	if err != nil {
		return nil, errors.New("discord: Config.APIBase has an invalid port")
	}
	// The policy names exactly the host and port of the API base over https (19 SEC-NET-2);
	// redirects are never followed, so the token cannot travel to another host (22 4.1).
	policy := func(_ context.Context, scheme, host string, p uint16) bool {
		return scheme == schemeHTTPS && host == api.host && uint64(p) == port
	}
	hc, err := egress.New(cfg.Egress, egress.Options{Purpose: egressPurpose, Hosts: policy,
		Redirects: egress.RedirectNever, Timeout: cfg.HTTPTimeout},
		egress.Deps{Resolver: deps.Resolver, RootCAs: deps.RootCAs, Logger: deps.Logger})
	if err != nil {
		return nil, fmt.Errorf("discord: Config.Egress: %w", err)
	}
	c := &Client{http: hc, base: base, api: api, token: token,
		userAgent: userAgent(cfg.UserAgentVersion),
		pageSize:  cfg.PageSize, retries: cfg.MaxRetries, maxWait: cfg.MaxRateWait,
		logger: deps.Logger, sleep: deps.Sleep}
	if c.logger == nil {
		c.logger = slog.New(slog.DiscardHandler)
	}
	if c.sleep == nil {
		c.sleep = sleepContext
	}
	clk := deps.Clock
	if clk == nil {
		clk = clock.System()
	}
	c.guard = newGuard(clk, cfg.MaxRateWait)
	return c, nil
}

// validateConfig checks cfg and returns the parsed API base.
func validateConfig(cfg Config) (*url.URL, error) {
	base, err := url.Parse(cfg.APIBase)
	switch {
	case err != nil:
		return nil, errors.New("discord: Config.APIBase is not a URL")
	case base.Scheme != schemeHTTPS || base.Host == "" || base.Hostname() == "" || base.User != nil:
		return nil, errors.New("discord: Config.APIBase must be an https URL with a host and without user info")
	case base.RawQuery != "" || base.Fragment != "" || base.ForceQuery || base.Opaque != "":
		return nil, errors.New("discord: Config.APIBase must not have a query or fragment")
	case !validUserAgentVersion(cfg.UserAgentVersion):
		return nil, errors.New("discord: Config.UserAgentVersion must be a version without parentheses or commas")
	case cfg.PageSize < 1 || cfg.PageSize > maxPageSize:
		return nil, errors.New("discord: Config.PageSize (import.discord.page_size) is outside the range Discord serves")
	case cfg.MaxRetries < 0:
		return nil, errors.New("discord: Config.MaxRetries (import.discord.max_429_retries) must not be negative")
	case cfg.MaxRateWait <= 0:
		return nil, errors.New("discord: Config.MaxRateWait (import.discord.max_rate_wait) must be positive")
	case cfg.HTTPTimeout <= 0:
		return nil, errors.New("discord: Config.HTTPTimeout (import.http_timeout) must be positive")
	}
	base.Path = strings.TrimSuffix(base.Path, "/")
	base.RawPath = ""
	return base, nil
}

// route is one call of 22 4.1: its path template and how the major ID enters the path.
type route struct {
	// template is the path template, also the op of errors and logs.
	template string
	// placeholder is the part of template the major ID replaces, empty for none.
	placeholder string
	// channel marks routes on a channel, which a 403 ends for the run (22 4.3 item 4).
	channel bool
	// keepOn403 marks the private archived list: its 403 means only that MANAGE_THREADS is
	// missing, which is optional (22 4.2), so the channel stays readable.
	keepOn403 bool
	// hint is added to a 403 of the route.
	hint string
}

// The placeholders of the major IDs in the path templates.
const (
	guildPlaceholder   = "{guild.id}"
	channelPlaceholder = "{channel.id}"
)

// The calls of 22 4.1; there are no others.
var (
	routeMe              = route{template: "/users/@me"}
	routeGuild           = route{template: "/guilds/{guild.id}", placeholder: guildPlaceholder}
	routeGuildChannels   = route{template: "/guilds/{guild.id}/channels", placeholder: guildPlaceholder}
	routeActiveThreads   = route{template: "/guilds/{guild.id}/threads/active", placeholder: guildPlaceholder}
	routeArchivedPublic  = route{template: "/channels/{channel.id}/threads/archived/public", placeholder: channelPlaceholder, channel: true}
	routeArchivedPrivate = route{template: "/channels/{channel.id}/threads/archived/private", placeholder: channelPlaceholder, channel: true,
		keepOn403: true, hint: "the bot lacks MANAGE_THREADS; skip the private archived threads of this channel"}
	routeChannel  = route{template: "/channels/{channel.id}", placeholder: channelPlaceholder, channel: true}
	routeMessages = route{template: "/channels/{channel.id}/messages", placeholder: channelPlaceholder, channel: true}
)

// op is the method and template of a route for errors and logs.
func (r route) op() string { return http.MethodGet + " " + r.template }

// Me returns the bot user of the token: GET /users/@me ([R1]).
func (c *Client) Me(ctx context.Context) (User, error) {
	var u User
	err := c.getObject(ctx, routeMe, "", nil, &u, u.check)
	return u, err
}

// Guild returns the server: GET /guilds/{guild.id} ([R2]).
func (c *Client) Guild(ctx context.Context, guildID string) (Guild, error) {
	var g Guild
	err := c.getObject(ctx, routeGuild, guildID, nil, &g, g.check)
	return g, err
}

// GuildChannels returns the categories and channels without threads:
// GET /guilds/{guild.id}/channels ([R2]).
func (c *Client) GuildChannels(ctx context.Context, guildID string) ([]Channel, error) {
	var chans []Channel
	err := c.getObject(ctx, routeGuildChannels, guildID, nil, &chans, func(ic *idCheck, _ string) {
		checkChannels(ic, "", chans)
	})
	return chans, err
}

// ActiveThreads returns the active threads of the server:
// GET /guilds/{guild.id}/threads/active ([R2]).
func (c *Client) ActiveThreads(ctx context.Context, guildID string) ([]Channel, error) {
	var body struct {
		Threads []Channel `json:"threads"`
	}
	err := c.getObject(ctx, routeActiveThreads, guildID, nil, &body, func(ic *idCheck, _ string) {
		checkChannels(ic, "threads", body.Threads)
	})
	return body.Threads, err
}

// ArchivedPublicThreads hands the archived public threads of a channel to fn page by page,
// newest first: GET /channels/{channel.id}/threads/archived/public ([R3]). The next page asks
// with before equal to the archive_timestamp of the last thread of the page and limit equal to
// import.discord.page_size; threads seen before are dropped, and the loop ends when Discord says
// has_more false or a page brings no new ID (22 4.1). An error of fn ends the loop with that
// error.
func (c *Client) ArchivedPublicThreads(ctx context.Context, channelID string, fn func([]Channel) error) error {
	return c.archivedThreads(ctx, routeArchivedPublic, channelID, fn)
}

// ArchivedPrivateThreads is ArchivedPublicThreads for the private list
// (GET /channels/{channel.id}/threads/archived/private), which needs MANAGE_THREADS. Its 403
// returns ErrForbidden with that hint and, unlike every other call, does not end the channel:
// the caller notes it and skips the private list (22 4.1, 4.2).
func (c *Client) ArchivedPrivateThreads(ctx context.Context, channelID string, fn func([]Channel) error) error {
	return c.archivedThreads(ctx, routeArchivedPrivate, channelID, fn)
}

// archivedThreads is the page loop of both archived lists.
func (c *Client) archivedThreads(ctx context.Context, rt route, channelID string, fn func([]Channel) error) error {
	seen := map[string]bool{}
	before := ""
	for {
		page, err := c.archivedPage(ctx, rt, channelID, before)
		if err != nil {
			return err
		}
		fresh := freshThreads(seen, page.Threads)
		if len(fresh) == 0 {
			return nil
		}
		if err := fn(fresh); err != nil {
			return err
		}
		if !page.HasMore {
			return nil
		}
		last := page.Threads[len(page.Threads)-1].ThreadMetadata
		if last == nil || last.ArchiveTimestamp == "" {
			return &CallError{Op: rt.op(), Status: http.StatusOK,
				Kind: fmt.Errorf("%w: the last thread has no thread_metadata.archive_timestamp", ErrDecode)}
		}
		before = last.ArchiveTimestamp
	}
}

// archivedPage is one page of an archived thread list.
type archivedPage struct {
	Threads []Channel `json:"threads"`
	HasMore bool      `json:"has_more"`
}

// archivedPage reads the page before the archive timestamp before ("" for the newest).
func (c *Client) archivedPage(ctx context.Context, rt route, channelID, before string) (archivedPage, error) {
	q := url.Values{"limit": {strconv.Itoa(c.pageSize)}}
	if before != "" {
		q.Set("before", before)
	}
	var page archivedPage
	err := c.getObject(ctx, rt, channelID, q, &page, func(ic *idCheck, _ string) {
		checkChannels(ic, "threads", page.Threads)
	})
	return page, err
}

// freshThreads returns the threads not in seen and adds them to it.
func freshThreads(seen map[string]bool, threads []Channel) []Channel {
	var fresh []Channel
	for _, t := range threads {
		if !seen[t.ID] {
			seen[t.ID] = true
			fresh = append(fresh, t)
		}
	}
	return fresh
}

// Channel returns one channel or thread: GET /channels/{channel.id} ([R3]).
func (c *Client) Channel(ctx context.Context, channelID string) (Channel, error) {
	var ch Channel
	err := c.getObject(ctx, routeChannel, channelID, nil, &ch, ch.check)
	return ch, err
}

// MessagesAfter returns up to limit messages after afterID (exclusive):
// GET /channels/{channel.id}/messages?after=...&limit=... ([R4]). limit is 1 up to
// import.discord.page_size. The result is in ascending ID order whatever order Discord used.
func (c *Client) MessagesAfter(ctx context.Context, channelID, afterID string, limit int) ([]Message, error) {
	return c.messages(ctx, channelID, "after", afterID, limit)
}

// MessagesBefore returns up to limit messages before beforeID (exclusive), the newest of them,
// in ascending ID order: GET /channels/{channel.id}/messages?before=...&limit=... ([R4]).
func (c *Client) MessagesBefore(ctx context.Context, channelID, beforeID string, limit int) ([]Message, error) {
	return c.messages(ctx, channelID, "before", beforeID, limit)
}

// MessagesAround returns up to limit messages around aroundID, in ascending ID order:
// GET /channels/{channel.id}/messages?around=...&limit=... ([R4]); used to read one message
// again for a fresh attachment link.
func (c *Client) MessagesAround(ctx context.Context, channelID, aroundID string, limit int) ([]Message, error) {
	return c.messages(ctx, channelID, "around", aroundID, limit)
}

// messages is the message call with one anchor parameter.
func (c *Client) messages(ctx context.Context, channelID, param, anchor string, limit int) ([]Message, error) {
	if !validSnowflake(anchor) {
		return nil, &CallError{Op: routeMessages.op(), Kind: fmt.Errorf("%w: argument %s", ErrInvalidID, param)}
	}
	if limit < 1 || limit > c.pageSize {
		return nil, &CallError{Op: routeMessages.op(),
			Kind: errors.New("discord: limit must be between 1 and import.discord.page_size")}
	}
	q := url.Values{param: {anchor}, "limit": {strconv.Itoa(limit)}}
	var msgs []Message
	err := c.getObject(ctx, routeMessages, channelID, q, &msgs, func(ic *idCheck, _ string) {
		for i := range msgs {
			msgs[i].check(ic, indexed("", "messages", i))
		}
	})
	if err != nil {
		return nil, err
	}
	slices.SortStableFunc(msgs, func(a, b Message) int { return compareSnowflakes(a.ID, b.ID) })
	return msgs, nil
}

// checkChannels checks the IDs of a list of channels.
func checkChannels(ic *idCheck, field string, chans []Channel) {
	for i := range chans {
		chans[i].check(ic, indexed("", field, i))
	}
}

// getObject runs the call and decodes the body of the success into out, then checks its IDs
// with check.
func (c *Client) getObject(ctx context.Context, rt route, major string, q url.Values, out any, check func(*idCheck, string)) error {
	body, err := c.call(ctx, rt, major, q)
	if err != nil {
		return err
	}
	if err := json.Unmarshal(body, out); err != nil {
		return &CallError{Op: rt.op(), Status: http.StatusOK, Kind: ErrDecode}
	}
	ic := &idCheck{}
	check(ic, "")
	if ic.err != nil {
		return &CallError{Op: rt.op(), Status: http.StatusOK, Kind: ic.err}
	}
	return nil
}

// call sends the GET of a route with its major ID and query under the rate-limit guard
// (22 4.3) and returns the body of a 2xx answer.
func (c *Client) call(ctx context.Context, rt route, major string, q url.Values) ([]byte, error) {
	if rt.placeholder != "" && !validSnowflake(major) {
		return nil, &CallError{Op: rt.op(), Kind: fmt.Errorf("%w: argument %s", ErrInvalidID, strings.Trim(rt.placeholder, "{}"))}
	}
	if rt.channel && c.guard.isForbidden(major) {
		return nil, &CallError{Op: rt.op(), Status: http.StatusForbidden, Kind: ErrForbidden,
			Hint: "an earlier 403 ended this channel for the run"}
	}
	u := *c.base
	u.Path += strings.Replace(rt.template, rt.placeholder, major, 1)
	u.RawQuery = q.Encode()
	for attempt := 0; ; attempt++ {
		body, retry, err := c.attempt(ctx, rt, major, &u, attempt)
		if !retry {
			return body, err
		}
	}
}

// attempt sends the request once, after the wait for an exhausted bucket. It returns the body
// of a success, or retry true after a 429 whose wait it has slept, or the error.
func (c *Client) attempt(ctx context.Context, rt route, major string, u *url.URL, attempt int) ([]byte, bool, error) {
	if err := c.wait(ctx, rt, c.guard.pending(rt.template, major), 0); err != nil {
		return nil, false, err
	}
	status, header, body, err := c.send(ctx, rt, u)
	if err != nil {
		return nil, false, err
	}
	c.guard.observe(rt.template, major, header)
	c.logger.LogAttrs(ctx, slog.LevelDebug, "discord call",
		slog.String(attrOp, rt.op()), slog.Int(attrStatus, status), slog.Int(attrAttempt, attempt))
	switch {
	case status >= 200 && status < 300:
		return body, false, nil
	case status != http.StatusTooManyRequests:
		return nil, false, c.statusError(rt, major, status)
	case attempt >= c.retries:
		return nil, false, &CallError{Op: rt.op(), Status: status, Kind: ErrRateLimited}
	}
	if err := c.wait(ctx, rt, retryWait(body, header, c.maxWait), status); err != nil {
		return nil, false, err
	}
	c.guard.clear(rt.template, major)
	return nil, true, nil
}

// statusError builds the error of a failed status; a 403 on a channel route other than the
// private archived list ends the channel for the run (22 4.3 item 4).
func (c *Client) statusError(rt route, major string, status int) error {
	e := &CallError{Op: rt.op(), Status: status, Kind: statusKind(status)}
	if status == http.StatusForbidden {
		e.Hint = rt.hint
		if rt.channel && !rt.keepOn403 {
			c.guard.forbid(major)
		}
	}
	return e
}

// wait sleeps d unless it is 0; a wait above import.discord.max_rate_wait ends the call at once
// with ErrRateWaitTooLong (22 4.3 item 3). status is the status that asked for the wait, 0 for
// an exhausted bucket.
func (c *Client) wait(ctx context.Context, rt route, d time.Duration, status int) error {
	if d <= 0 {
		return nil
	}
	if d > c.maxWait {
		return &CallError{Op: rt.op(), Status: status, Kind: ErrRateWaitTooLong}
	}
	c.logger.LogAttrs(ctx, slog.LevelInfo, "discord rate limit wait",
		slog.String(attrOp, rt.op()), slog.Duration(attrWait, d))
	if err := c.sleep(ctx, d); err != nil {
		return &CallError{Op: rt.op(), Status: status, Kind: err}
	}
	return nil
}

// newGet builds a GET of rawURL with the User-Agent userAgent and the Accept accept. It is the
// only place of the package that builds a request (TestClientHasOnlyGet); the API client and
// the CDN fetcher both use it.
func newGet(ctx context.Context, rawURL, userAgent, accept string) (*http.Request, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", userAgent)
	req.Header.Set("Accept", accept)
	return req, nil
}

// send builds and sends one GET and returns status, header and body; the token enters only
// through authorize.
func (c *Client) send(ctx context.Context, rt route, u *url.URL) (int, http.Header, []byte, error) {
	req, err := newGet(ctx, u.String(), c.userAgent, "application/json")
	if err != nil {
		return 0, nil, nil, &CallError{Op: rt.op(), Kind: errors.New("discord: the request could not be built")}
	}
	if err := c.authorize(req); err != nil {
		return 0, nil, nil, &CallError{Op: rt.op(), Kind: err}
	}
	resp, err := c.http.Do(req)
	if err != nil {
		return 0, nil, nil, &CallError{Op: rt.op(), Kind: err}
	}
	defer func() { _ = resp.Body.Close() }()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return 0, nil, nil, &CallError{Op: rt.op(), Status: resp.StatusCode, Kind: err}
	}
	return resp.StatusCode, resp.Header, body, nil
}

// authorize sets the token on req if it goes to the API base (Token.setAuthorization).
func (c *Client) authorize(req *http.Request) error {
	return c.token.setAuthorization(req, c.api)
}

// String describes the client without the token.
func (c *Client) String() string { return "discord.Client(" + c.api.host + ")" }

// GoString is String, so that %#v does not print the fields; fmt does not call the methods of
// unexported fields such as the token.
func (c *Client) GoString() string { return c.String() }
