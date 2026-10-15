// SPDX-License-Identifier: Apache-2.0

package discord

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"math"
	"net/http"
	"time"

	"github.com/OWNER/bothy/services/server/internal/platform/egress"
)

// cdnPurpose labels the fetcher in the logs of platform/egress.
const cdnPurpose = "import_cdn"

// cdnAccept is the Accept header of CDN requests: any media type, the attachment decides.
const cdnAccept = "*/*"

// CDNConfig are the values of the attachment fetcher (22 19).
type CDNConfig struct {
	// Hosts is import.discord.cdn_hosts: host names, each optionally with ":port"; a URL
	// matches an entry when its lower-cased host without trailing dot equals the entry and its
	// port equals the entry's port, 443 when the entry names none. No wildcards.
	Hosts []string
	// MaxBytes is import.attachment_max_bytes: the largest attachment. Positive.
	MaxBytes int64
	// Timeout is import.http_timeout: the time limit of one CDN request. Positive.
	Timeout time.Duration
	// MaxRetries is import.discord.max_429_retries: retries after a 429 of the CDN, 0 or more.
	MaxRetries int
	// MaxRateWait is import.discord.max_rate_wait: the longest single wait after a 429; a
	// longer one ends the fetch with ErrRateWaitTooLong. Positive.
	MaxRateWait time.Duration
	// UserAgentVersion is the version of the tool in the User-Agent, as in Config.
	UserAgentVersion string
	// Egress are the egress.* values of the tool, as in Config; only an allowed_hosts rule with
	// allow_private can open a private address (tests).
	Egress egress.Config
}

// MessageReader reads messages around one message; *Client satisfies it. The fetcher uses it
// to read a message again for a fresh attachment link.
type MessageReader interface {
	// MessagesAround is Client.MessagesAround.
	MessagesAround(ctx context.Context, channelID, aroundID string, limit int) ([]Message, error)
}

// AttachmentRef names one attachment of a message and its link as the message listed it.
type AttachmentRef struct {
	// ChannelID is the channel or thread of the message.
	ChannelID string
	// MessageID is the message.
	MessageID string
	// AttachmentID is the attachment ID, which stays when the link is renewed.
	AttachmentID string
	// URL is the signed CDN link; it prints without its query.
	URL SignedURL
	// Size is the declared size in bytes (Attachment.Size).
	Size int64
}

// Download is a fetched attachment. The caller reads Body as a stream and closes it; Read
// returns ErrTooLarge once more than import.attachment_max_bytes bytes arrived.
type Download struct {
	// Body is the response body.
	Body io.ReadCloser
	// Size is the Content-Length of the answer, -1 when the CDN sent none.
	Size int64
	// ContentType is the Content-Type of the answer, empty when none.
	ContentType string
}

// Fetcher loads attachments from the Discord CDN (22 6.5, REUSE 5.2): GET only, https only,
// hosts of import.discord.cdn_hosts only, no credential, no redirects, no environment proxy,
// bounded in size and time, streamed and never written to disk. It is safe for concurrent use.
type Fetcher struct {
	http      *egress.Client
	hosts     []cdnHost
	maxBytes  int64
	retries   int
	maxWait   time.Duration
	userAgent string
	reader    MessageReader
	logger    *slog.Logger
	sleep     Sleeper
}

// NewFetcher builds a fetcher from the configuration, the reader for re-reading a message and
// the dependencies (Resolver, RootCAs, Logger and Sleep of Deps; the fetcher keeps no clock
// state). It returns an error, naming the field and never its value, when an entry of Hosts is
// not a host name with an optional port or Hosts is empty, MaxBytes, Timeout or MaxRateWait is
// not positive, MaxRetries is negative, UserAgentVersion does not fit into the User-Agent,
// reader is nil, or egress.New rejects Egress.
//
// Example:
//
//	f, err := discord.NewFetcher(discord.CDNConfig{Hosts: []string{"cdn.discordapp.com", "media.discordapp.net"},
//		MaxBytes: maxBytes, Timeout: time.Minute, MaxRetries: 5, MaxRateWait: time.Minute,
//		UserAgentVersion: version, Egress: egressCfg}, client, discord.Deps{Logger: logger})
//	d, err := f.Fetch(ctx, discord.AttachmentRef{ChannelID: m.ChannelID, MessageID: m.ID,
//		AttachmentID: a.ID, URL: a.URL, Size: a.Size})
func NewFetcher(cfg CDNConfig, reader MessageReader, deps Deps) (*Fetcher, error) {
	hosts, err := parseCDNHosts(cfg.Hosts)
	if err != nil {
		return nil, err
	}
	switch {
	case cfg.MaxBytes <= 0 || cfg.MaxBytes == math.MaxInt64:
		return nil, errors.New("discord: CDNConfig.MaxBytes (import.attachment_max_bytes) must be positive")
	case cfg.Timeout <= 0:
		return nil, errors.New("discord: CDNConfig.Timeout (import.http_timeout) must be positive")
	case cfg.MaxRetries < 0:
		return nil, errors.New("discord: CDNConfig.MaxRetries (import.discord.max_429_retries) must not be negative")
	case cfg.MaxRateWait <= 0:
		return nil, errors.New("discord: CDNConfig.MaxRateWait (import.discord.max_rate_wait) must be positive")
	case !validUserAgentVersion(cfg.UserAgentVersion):
		return nil, errors.New("discord: CDNConfig.UserAgentVersion must be a version without parentheses or commas")
	case reader == nil:
		return nil, errors.New("discord: the message reader of the fetcher is nil")
	}
	// The policy names exactly the entries of the allow list over https; the fetcher checks
	// every URL against the same list before the request, so the egress rules of the tool add
	// no CDN host (Design note 1, ruling D-M8-02-1). The body limit is the maximum plus one
	// byte, so that the fetcher sees the byte that makes a body too large.
	policy := func(_ context.Context, scheme, host string, port uint16) bool {
		for _, h := range hosts {
			if scheme == schemeHTTPS && h.host == host && h.port == port {
				return true
			}
		}
		return false
	}
	hc, err := egress.New(cfg.Egress, egress.Options{Purpose: cdnPurpose, Hosts: policy,
		Redirects: egress.RedirectNever, Timeout: cfg.Timeout, MaxResponseBytes: cfg.MaxBytes + 1},
		egress.Deps{Resolver: deps.Resolver, RootCAs: deps.RootCAs, Logger: deps.Logger})
	if err != nil {
		return nil, fmt.Errorf("discord: CDNConfig.Egress: %w", err)
	}
	f := &Fetcher{http: hc, hosts: hosts, maxBytes: cfg.MaxBytes, retries: cfg.MaxRetries,
		maxWait: cfg.MaxRateWait, userAgent: userAgent(cfg.UserAgentVersion), reader: reader,
		logger: deps.Logger, sleep: deps.Sleep}
	if f.logger == nil {
		f.logger = slog.New(slog.DiscardHandler)
	}
	if f.sleep == nil {
		f.sleep = sleepContext
	}
	return f, nil
}

// Fetch loads one attachment and returns its body as a stream. An attachment whose declared
// size is above import.attachment_max_bytes is not requested (ErrTooLarge). On 403 or 404 the
// link counts as expired: the fetcher reads the message once more through MessagesAround with
// limit 1, takes the fresh link of the same attachment ID and tries exactly once more; a
// missing message or attachment, or a second 403 or 404, returns ErrCDNGone. A 429 waits like
// the API client and retries at most import.discord.max_429_retries times. Other failures
// return a *CallError whose op is "GET <scheme://host/path>"; no error carries the query, a
// header or the body of the CDN, and nothing is retried. Errors wrap ErrCDNScheme,
// ErrCDNHostNotAllowed, ErrCDNRedirect, ErrTooLarge, ErrCDNGone, ErrRateLimited,
// ErrRateWaitTooLong, a status kind or a transport error of platform/egress.
func (f *Fetcher) Fetch(ctx context.Context, ref AttachmentRef) (*Download, error) {
	if ref.Size > f.maxBytes {
		return nil, &CallError{Op: safeOp(ref.URL.Raw()), Kind: ErrTooLarge}
	}
	d, err := f.get(ctx, ref.URL.Raw())
	if !expired(err) {
		return d, err
	}
	fresh, err := f.reread(ctx, ref)
	if err != nil {
		return nil, err
	}
	d, err = f.get(ctx, fresh.URL.Raw())
	if expired(err) {
		var ce *CallError
		errors.As(err, &ce)
		return nil, &CallError{Op: ce.Op, Status: ce.Status, Kind: ErrCDNGone}
	}
	return d, err
}

// expired reports whether err is a 403 or 404 of the CDN, the answer to an expired link.
func expired(err error) bool {
	var ce *CallError
	return errors.As(err, &ce) && (ce.Status == http.StatusForbidden || ce.Status == http.StatusNotFound) &&
		(errors.Is(err, ErrForbidden) || errors.Is(err, ErrNotFound))
}

// reread reads the message of ref again and returns its attachment with the same ID, also one
// of a forwarded message snapshot, or ErrCDNGone when the message or the attachment is gone.
func (f *Fetcher) reread(ctx context.Context, ref AttachmentRef) (Attachment, error) {
	msgs, err := f.reader.MessagesAround(ctx, ref.ChannelID, ref.MessageID, 1)
	if err != nil {
		return Attachment{}, err
	}
	for _, m := range msgs {
		if m.ID != ref.MessageID {
			continue
		}
		a, ok := findAttachment(m, ref.AttachmentID)
		if !ok {
			break
		}
		if a.Size > f.maxBytes {
			return Attachment{}, &CallError{Op: safeOp(a.URL.Raw()), Kind: ErrTooLarge}
		}
		return a, nil
	}
	return Attachment{}, &CallError{Op: safeOp(ref.URL.Raw()), Kind: ErrCDNGone}
}

// findAttachment returns the attachment with ID id of m or of one of its forwarded snapshots.
func findAttachment(m Message, id string) (Attachment, bool) {
	lists := [][]Attachment{m.Attachments}
	for _, s := range m.MessageSnapshots {
		lists = append(lists, s.Message.Attachments)
	}
	for _, list := range lists {
		for _, a := range list {
			if a.ID == id {
				return a, true
			}
		}
	}
	return Attachment{}, false
}

// get fetches rawURL after the URL check, retrying after a 429 within the limits.
func (f *Fetcher) get(ctx context.Context, rawURL string) (*Download, error) {
	op := safeOp(rawURL)
	if _, err := checkCDNURL(rawURL, f.hosts); err != nil {
		return nil, &CallError{Op: op, Kind: err}
	}
	for attempt := 0; ; attempt++ {
		d, retry, wait, err := f.attempt(ctx, op, rawURL, attempt)
		if !retry {
			return d, err
		}
		if wait > f.maxWait {
			// The same rule as the API client: never wait longer than the limit (22 4.3 item 3).
			return nil, &CallError{Op: op, Status: http.StatusTooManyRequests, Kind: ErrRateWaitTooLong}
		}
		f.logger.LogAttrs(ctx, slog.LevelInfo, "discord cdn rate limit wait",
			slog.String(attrOp, op), slog.Duration(attrWait, wait))
		if err := f.sleep(ctx, wait); err != nil {
			return nil, &CallError{Op: op, Status: http.StatusTooManyRequests, Kind: err}
		}
	}
}

// attempt sends one GET. It returns the download of a 200, or retry true with the wait a 429
// asks for when a retry is left, or the error.
func (f *Fetcher) attempt(ctx context.Context, op, rawURL string, attempt int) (*Download, bool, time.Duration, error) {
	req, err := newGet(ctx, rawURL, f.userAgent, cdnAccept)
	if err != nil {
		return nil, false, 0, &CallError{Op: op, Kind: errors.New("discord: the request could not be built")}
	}
	resp, err := f.http.Do(req)
	if err != nil {
		return nil, false, 0, &CallError{Op: op, Kind: err}
	}
	status := resp.StatusCode
	f.logger.LogAttrs(ctx, slog.LevelDebug, "discord cdn fetch",
		slog.String(attrOp, op), slog.Int(attrStatus, status), slog.Int(attrAttempt, attempt))
	switch {
	case status == http.StatusOK:
		d, err := f.download(op, resp)
		return d, false, 0, err
	case status == http.StatusTooManyRequests && attempt < f.retries:
		// The body of a 429 is read only for retry_after and never shown; egress bounds it.
		body, _ := io.ReadAll(resp.Body)
		_ = resp.Body.Close()
		return nil, true, retryWait(body, resp.Header, f.maxWait), nil
	}
	// Every other answer is closed unread: an error body can quote the signed link.
	_ = resp.Body.Close()
	switch {
	case status == http.StatusTooManyRequests:
		return nil, false, 0, &CallError{Op: op, Status: status, Kind: ErrRateLimited}
	case status >= 300 && status < 400:
		return nil, false, 0, &CallError{Op: op, Status: status, Kind: ErrCDNRedirect}
	}
	return nil, false, 0, &CallError{Op: op, Status: status, Kind: statusKind(status)}
}

// download checks the Content-Length of a 200 and wraps its body in the size limit.
func (f *Fetcher) download(op string, resp *http.Response) (*Download, error) {
	if resp.ContentLength > f.maxBytes {
		_ = resp.Body.Close()
		return nil, &CallError{Op: op, Status: resp.StatusCode, Kind: ErrTooLarge}
	}
	return &Download{Body: &limitBody{rc: resp.Body, left: f.maxBytes, op: op},
		Size: resp.ContentLength, ContentType: resp.Header.Get("Content-Type")}, nil
}

// limitBody returns the bytes of a body up to the limit and ErrTooLarge once one more byte
// arrives; it never buffers the body. Read errors are CallErrors with the safe op.
type limitBody struct {
	rc   io.ReadCloser
	left int64
	op   string
	err  error
}

// Read implements io.Reader.
func (b *limitBody) Read(p []byte) (int, error) {
	if b.err != nil {
		return 0, b.err
	}
	// Ask for one byte beyond the limit, so that a body of exactly the limit ends with EOF and
	// a longer one shows its extra byte.
	if int64(len(p)) > b.left+1 {
		p = p[:b.left+1]
	}
	n, err := b.rc.Read(p)
	if int64(n) > b.left {
		n = int(b.left)
		b.left = 0
		b.err = &CallError{Op: b.op, Status: http.StatusOK, Kind: ErrTooLarge}
		return n, b.err
	}
	b.left -= int64(n)
	switch {
	case err == nil || errors.Is(err, io.EOF):
		return n, err
	case errors.Is(err, egress.ErrResponseTooLarge):
		b.err = &CallError{Op: b.op, Status: http.StatusOK, Kind: ErrTooLarge}
	default:
		b.err = &CallError{Op: b.op, Status: http.StatusOK, Kind: err}
	}
	return n, b.err
}

// Close implements io.Closer.
func (b *limitBody) Close() error { return b.rc.Close() }

// String describes the fetcher without its configuration.
func (f *Fetcher) String() string { return "discord.Fetcher" }

// GoString is String.
func (f *Fetcher) GoString() string { return f.String() }
