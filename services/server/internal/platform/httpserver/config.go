// SPDX-License-Identifier: Apache-2.0

package httpserver

import (
	"errors"
	"fmt"
	"net/netip"
	"net/url"
	"time"
)

// ChainConfig holds the keys the chain reads (docs/operations/configuration.md); internal/wire
// binds it with config.Bind. Defaults come from the chapter tables.
type ChainConfig struct {
	// MaxJSONBodyBytes is http.max_json_body_bytes: the largest JSON request body.
	MaxJSONBodyBytes int64 `config:"http.max_json_body_bytes"`
	// MaxJSONDepth is http.max_json_depth: the deepest nesting of a JSON request body.
	MaxJSONDepth int `config:"http.max_json_depth"`
	// RequestTimeout is http.request_timeout: the context deadline of a normal request.
	RequestTimeout time.Duration `config:"http.request_timeout"`
	// TransferTimeout is http.transfer_timeout: the deadline of upload and download operations.
	TransferTimeout time.Duration `config:"http.transfer_timeout"`
	// UnavailableRetryAfter is http.unavailable_retry_after: Retry-After of 503 unavailable.
	UnavailableRetryAfter time.Duration `config:"http.unavailable_retry_after"`
	// MaxUploadBytes is file.max_upload_bytes: the largest multipart or octet-stream body.
	MaxUploadBytes int64 `config:"file.max_upload_bytes"`
	// CORSAllowedOrigins is api.cors_allowed_origins: origins that get CORS headers (SEC-CORS).
	CORSAllowedOrigins []string `config:"api.cors_allowed_origins"`
	// TrustedProxies is server.trusted_proxies: peers whose forwarding headers count.
	TrustedProxies []netip.Prefix `config:"server.trusted_proxies"`
	// PublicURL is server.public_url: the origin of the server, the base of the DPoP htu.
	PublicURL string `config:"server.public_url"`
}

// DefaultChainConfig returns the defaults of the chapter tables with server.public_url set to
// publicURL (the key has no default). Tests and the harness start from it.
func DefaultChainConfig(publicURL string) ChainConfig {
	return ChainConfig{
		MaxJSONBodyBytes:      1048576,
		MaxJSONDepth:          32,
		RequestTimeout:        15 * time.Second,
		TransferTimeout:       10 * time.Minute,
		UnavailableRetryAfter: 5 * time.Second,
		MaxUploadBytes:        104857600,
		TrustedProxies:        []netip.Prefix{netip.MustParsePrefix("127.0.0.1/32"), netip.MustParsePrefix("::1/128")},
		PublicURL:             publicURL,
	}
}

// validate reports values the configuration schema cannot check alone: limits that must be
// positive, the origin form of server.public_url and of every CORS origin.
func (c ChainConfig) validate() error {
	var problems []error
	if c.MaxJSONBodyBytes < 1 || c.MaxUploadBytes < 1 || c.MaxJSONDepth < 1 {
		problems = append(problems, errors.New("httpserver: body limits and JSON depth must be positive"))
	}
	if c.RequestTimeout <= 0 || c.TransferTimeout <= 0 || c.UnavailableRetryAfter < time.Second {
		problems = append(problems, errors.New("httpserver: timeouts must be positive and Retry-After at least 1s"))
	}
	if err := CheckOrigin(c.PublicURL); err != nil {
		problems = append(problems, fmt.Errorf("httpserver: server.public_url: %w", err))
	}
	for _, o := range c.CORSAllowedOrigins {
		if err := CheckOrigin(o); err != nil {
			problems = append(problems, fmt.Errorf("httpserver: api.cors_allowed_origins entry %q: %w", o, err))
		}
	}
	return errors.Join(problems...)
}

// CheckOrigin reports whether s is an origin: scheme http or https, a host, an optional port and
// nothing else (no user, path, query or fragment). server.public_url and every entry of
// api.cors_allowed_origins must be one (16 "Konfigurationswerte", 06 "Grundregeln" row CORS).
// The error says what is wrong; it never repeats a user or password part.
func CheckOrigin(s string) error {
	u, err := url.Parse(s)
	switch {
	case err != nil || u.Opaque != "":
		return errors.New("not a URL")
	case u.Scheme != "http" && u.Scheme != "https":
		return errors.New("scheme must be http or https")
	case u.Host == "" || u.Hostname() == "":
		return errors.New("host missing")
	case u.User != nil:
		return errors.New("must not contain user information")
	case u.Path != "" || u.RawQuery != "" || u.Fragment != "" || u.ForceQuery:
		return errors.New("must not contain a path, query or fragment")
	case s != u.Scheme+"://"+u.Host:
		return errors.New("not in origin form scheme://host[:port]")
	}
	return nil
}
