// SPDX-License-Identifier: Apache-2.0

package transcription

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"mime/multipart"
	"net/http"
	"net/netip"
	"net/textproto"
	"net/url"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/OWNER/bothy/services/server/internal/files/app"
	"github.com/OWNER/bothy/services/server/internal/platform/egress"
	"github.com/OWNER/bothy/services/server/internal/platform/otel/attr"
	"github.com/OWNER/bothy/services/server/internal/platform/secrets"
)

// The file names of the part file (design note 11 b): fixed by type, so the original name of the
// upload never leaves the server.
const (
	fileNameMP4 = "audio.m4a"
	fileNameMP3 = "audio.mp3"
	mimeMP3     = "audio/mpeg"
)

// The log message and the classes of a failed call (design note 11 d); constant text, so no value
// can slip into the message.
const (
	msgFailed      = "transcription failed"
	classTransient = "transient"
	classPermanent = "permanent"
)

// keyAPIKeyRef names the key reference in error texts, never its value.
const keyAPIKeyRef = "voice.transcription_api_key_ref"

// Doer sends one HTTP request; *egress.Client implements it. The adapter never builds an HTTP
// client of its own (19 SEC-NET-1).
type Doer interface {
	// Do sends req and returns the response or an error; it follows no redirect.
	Do(req *http.Request) (*http.Response, error)
}

// SecretResolver resolves voice.transcription_api_key_ref; *secrets.Provider implements it.
type SecretResolver interface {
	// Resolve returns the value and version of the reference raw.
	Resolve(ctx context.Context, raw string) (secrets.Value, secrets.Version, error)
}

// Config are the settings of the adapter from the configuration.
type Config struct {
	// URL is voice.transcription_url: the absolute http or https endpoint (ValidateURL).
	URL string
	// APIKeyRef is voice.transcription_api_key_ref; "" sends no Authorization header.
	APIKeyRef string
	// Model is voice.transcription_model; "" sends no model field.
	Model string
	// MaxTextBytes is message.max_content_bytes, the longest accepted transcript in bytes.
	MaxTextBytes int
}

// Deps are the collaborators of the adapter.
type Deps struct {
	// Client is the egress client built for URL with HostPolicy, no redirects and the time
	// limit voice.transcription_timeout; required.
	Client Doer
	// Secrets resolves Config.APIKeyRef on every call; required when APIKeyRef is set.
	Secrets SecretResolver
	// Logger receives one Warn line per failed call; nil discards them.
	Logger *slog.Logger
}

// OpenAICompatible is the adapter openai_compatible of app.Transcriber (see the package
// comment for the request form and the error classes). It is safe for concurrent use.
type OpenAICompatible struct {
	cfg     Config
	client  Doer
	secrets SecretResolver
	log     *slog.Logger
}

var _ app.Transcriber = (*OpenAICompatible)(nil)

// NewOpenAICompatible returns the adapter for cfg. It sends no request and reads no secret.
//
// Errors: a URL that ValidateURL refuses, MaxTextBytes not positive, a missing Client, or a key
// reference without Secrets. They name keys or fields, never values.
//
// Example:
//
//	hosts, _ := transcription.HostPolicy(cfg.URL)
//	client, _ := egress.New(egressCfg, egress.Options{Purpose: "transcription", Hosts: hosts,
//		Redirects: egress.RedirectNever, Timeout: timeout}, egressDeps)
//	t, err := transcription.NewOpenAICompatible(cfg, transcription.Deps{Client: client, Secrets: provider, Logger: logger})
func NewOpenAICompatible(cfg Config, deps Deps) (*OpenAICompatible, error) {
	if err := ValidateURL(cfg.URL); err != nil {
		return nil, err
	}
	switch {
	case cfg.MaxTextBytes <= 0:
		return nil, errors.New("transcription: Config.MaxTextBytes must be positive")
	case deps.Client == nil:
		return nil, errors.New("transcription: Deps.Client is required")
	case cfg.APIKeyRef != "" && deps.Secrets == nil:
		return nil, errors.New("transcription: " + keyAPIKeyRef + " is set but no secret provider resolves it")
	}
	logger := deps.Logger
	if logger == nil {
		logger = slog.New(slog.DiscardHandler)
	}
	return &OpenAICompatible{cfg: cfg, client: deps.Client, secrets: deps.Secrets, log: logger}, nil
}

// ValidateURL checks voice.transcription_url for mode service (cross-check voice_service_url,
// design note 16): an absolute URL with scheme https or http, a host, no user info and no
// fragment. The error states the rule and never quotes the URL, because a misplaced secret may
// stand where the URL belongs.
func ValidateURL(raw string) error {
	u, err := url.Parse(raw)
	switch {
	case err != nil || raw == "":
		return errors.New("voice.transcription_url must be an absolute http or https URL")
	case u.Scheme != "https" && u.Scheme != "http":
		return errors.New("voice.transcription_url must use the scheme https or http")
	case u.Opaque != "" || u.Hostname() == "":
		return errors.New("voice.transcription_url must name a host")
	case u.User != nil:
		return errors.New("voice.transcription_url must not carry user info")
	case u.Fragment != "" || strings.Contains(raw, "#"):
		return errors.New("voice.transcription_url must not carry a fragment")
	}
	return nil
}

// HostPolicy returns the host policy of the egress client for raw: it allows exactly the scheme,
// host and port of the URL (the default port of the scheme when the URL names none), compared in
// the canonical form the egress client passes (lower case without trailing dot, IP literals as
// net/netip prints them). The policy never opens http or a private address by itself; that
// needs an entry with allow_private in egress.allowed_hosts.
//
// Errors: those of ValidateURL, or a port that is not a number from 1 to 65535.
func HostPolicy(raw string) (egress.HostPolicy, error) {
	if err := ValidateURL(raw); err != nil {
		return nil, err
	}
	u, _ := url.Parse(raw) // checked by ValidateURL
	host := strings.TrimSuffix(strings.ToLower(u.Hostname()), ".")
	if a, err := netip.ParseAddr(host); err == nil {
		host = a.Unmap().String()
	}
	port := uint16(443)
	if u.Scheme == "http" {
		port = 80
	}
	if p := u.Port(); p != "" {
		n, err := strconv.ParseUint(p, 10, 16)
		if err != nil || n == 0 {
			return nil, errors.New("voice.transcription_url must have a port from 1 to 65535")
		}
		port = uint16(n)
	}
	scheme := u.Scheme
	return func(_ context.Context, s, h string, p uint16) bool {
		return s == scheme && h == host && p == port
	}, nil
}

// Transcribe sends req.Audio to the configured service and returns its transcript (see the
// package comment). Errors wrapping app.ErrTranscriptionPermanent are permanent, all others
// transient; every failure writes one Warn line. It reads req.Audio up to its end or until the
// request fails, and has stopped reading when it returns.
func (a *OpenAICompatible) Transcribe(ctx context.Context, req app.TranscribeRequest) (app.TranscribeResult, error) {
	res, status, err := a.transcribe(ctx, req)
	if err != nil {
		class := classTransient
		if errors.Is(err, app.ErrTranscriptionPermanent) {
			class = classPermanent
		}
		attrs := []any{slog.String(attr.FileID, req.FileID.String()), slog.String(attr.Reason, class)}
		if status != 0 {
			attrs = append(attrs, slog.Int(attr.HTTPResponseStatusCode, status))
		}
		a.log.WarnContext(ctx, msgFailed, attrs...)
	}
	return res, err
}

// transcribe is Transcribe without the log line; it also returns the status code of the
// response, 0 when none arrived.
func (a *OpenAICompatible) transcribe(ctx context.Context, req app.TranscribeRequest) (app.TranscribeResult, int, error) {
	auth, err := a.authorization(ctx)
	if err != nil {
		return app.TranscribeResult{}, 0, err
	}
	pr, pw := io.Pipe()
	form := multipart.NewWriter(pw)
	done := make(chan struct{})
	go func() {
		defer close(done)
		pw.CloseWithError(a.writeForm(form, req)) // nil closes normally
	}()
	// Closing the reader stops a writer that the transport left blocked (an early answer), and
	// waiting for it keeps req.Audio untouched after return, when the caller closes it.
	defer func() {
		_ = pr.Close()
		<-done
	}()
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, a.cfg.URL, pr)
	if err != nil {
		return app.TranscribeResult{}, 0, fmt.Errorf("transcription: build the request: %w", app.ErrTranscriptionPermanent)
	}
	httpReq.Header.Set("Content-Type", form.FormDataContentType())
	httpReq.Header.Set("Accept", "application/json")
	if auth != "" {
		httpReq.Header.Set("Authorization", auth)
	}
	resp, err := a.client.Do(httpReq)
	if err != nil {
		return app.TranscribeResult{}, 0, sendError(err)
	}
	defer func() { _ = resp.Body.Close() }()
	switch code := resp.StatusCode; {
	case code == http.StatusOK:
		res, err := a.decode(resp.Body)
		return res, code, err
	case code == http.StatusRequestTimeout, code == http.StatusTooManyRequests, code >= 500 && code <= 599:
		return app.TranscribeResult{}, code, fmt.Errorf("transcription: service answered status %d", code)
	default:
		// Every 3xx (never followed), every other 4xx and any unexpected status.
		return app.TranscribeResult{}, code, fmt.Errorf("transcription: service answered status %d: %w", code, app.ErrTranscriptionPermanent)
	}
}

// authorization returns the Authorization header value, "" without a key reference. The key
// stays a secrets.Value until the header is built, and its copy is cleared afterwards.
func (a *OpenAICompatible) authorization(ctx context.Context) (string, error) {
	if a.cfg.APIKeyRef == "" {
		return "", nil
	}
	v, _, err := a.secrets.Resolve(ctx, a.cfg.APIKeyRef)
	if err != nil {
		// Transient: a rotation in progress or a file the operator is about to fix heals; the
		// job's attempts (voice.transcription_max_attempts) bound the retries.
		return "", fmt.Errorf("transcription: %s: %s", keyAPIKeyRef, secrets.ErrorCode(err))
	}
	b := v.Bytes()
	defer clear(b)
	return "Bearer " + string(b), nil
}

// writeForm writes the multipart body: the text fields first, then the part file streamed from
// req.Audio.
func (a *OpenAICompatible) writeForm(form *multipart.Writer, req app.TranscribeRequest) error {
	if a.cfg.Model != "" {
		if err := form.WriteField("model", a.cfg.Model); err != nil {
			return err
		}
	}
	if req.LanguageHint != "" {
		if err := form.WriteField("language", req.LanguageHint); err != nil {
			return err
		}
	}
	if err := form.WriteField("response_format", "json"); err != nil {
		return err
	}
	name := fileNameMP4
	if req.MIMEType == mimeMP3 {
		name = fileNameMP3
	}
	h := textproto.MIMEHeader{}
	h.Set("Content-Disposition", `form-data; name="file"; filename="`+name+`"`)
	h.Set("Content-Type", req.MIMEType)
	part, err := form.CreatePart(h)
	if err != nil {
		return err
	}
	if _, err := io.Copy(part, req.Audio); err != nil {
		return err
	}
	return form.Close()
}

// sendError maps an error of Client.Do. The *url.Error of net/http quotes the whole URL, path
// and query included, so only its cause is kept; the egress errors name scheme and host only.
func sendError(err error) error {
	var ue *url.Error
	if errors.As(err, &ue) {
		err = ue.Err
	}
	if errors.Is(err, egress.ErrBlocked) {
		return fmt.Errorf("transcription: %w: %w", app.ErrTranscriptionPermanent, err)
	}
	return fmt.Errorf("transcription: request failed: %w", err)
}

// response is the accepted part of the JSON answer; other members are ignored.
type response struct {
	Text     *string `json:"text"`
	Language *string `json:"language"`
}

// decode reads and checks the body of a 200 answer (design note 11 c).
func (a *OpenAICompatible) decode(body io.Reader) (app.TranscribeResult, error) {
	raw, err := io.ReadAll(body)
	if errors.Is(err, egress.ErrResponseTooLarge) {
		return app.TranscribeResult{}, fmt.Errorf("transcription: response over egress.max_response_bytes: %w", app.ErrTranscriptionPermanent)
	}
	if err != nil {
		return app.TranscribeResult{}, errors.New("transcription: reading the response failed")
	}
	var r response
	// The decoder's error may quote the body, so it is not wrapped.
	if json.Unmarshal(raw, &r) != nil || r.Text == nil {
		return app.TranscribeResult{}, fmt.Errorf("transcription: response is not a JSON object with a text string: %w", app.ErrTranscriptionPermanent)
	}
	text := *r.Text
	switch {
	case !utf8.ValidString(text) || strings.ContainsRune(text, 0):
		return app.TranscribeResult{}, fmt.Errorf("transcription: text is not valid UTF-8 without U+0000: %w", app.ErrTranscriptionPermanent)
	case len(text) > a.cfg.MaxTextBytes:
		return app.TranscribeResult{}, fmt.Errorf("transcription: text over message.max_content_bytes: %w", app.ErrTranscriptionPermanent)
	}
	res := app.TranscribeResult{Text: text}
	if r.Language != nil {
		res.Language = *r.Language
	}
	return res, nil
}
