// SPDX-License-Identifier: Apache-2.0

package discord

import (
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"strings"
	"unicode"
)

// schemeHTTPS is the only scheme the token travels over.
const schemeHTTPS = "https"

// redacted is the only text any print or marshal form of a Token shows.
const redacted = "[redacted]"

// Token is the bot token of the import account (22 4.2). The value is unexported: String,
// GoString, Format, LogValue and the JSON and text marshalling all print [redacted], so a token
// that slips into a log line, an error or a report shows nothing (22 14, REUSE 4.3). The only
// use of the value is the header setter of the client, which sends it only to the host of
// import.discord.api_base.
type Token struct {
	value string
}

// NewToken wraps a bot token. It rejects an empty value and any white space or control
// character, which would break or inject into the Authorization header; the caller trims a
// trailing newline of a token file. The error never quotes the value.
func NewToken(s string) (Token, error) {
	if s == "" {
		return Token{}, errors.New("discord: the bot token is empty")
	}
	if strings.ContainsFunc(s, func(r rune) bool { return unicode.IsSpace(r) || unicode.IsControl(r) }) {
		return Token{}, errors.New("discord: the bot token contains white space or a control character")
	}
	return Token{value: s}, nil
}

// String returns [redacted].
func (Token) String() string { return redacted }

// GoString returns [redacted], also for %#v.
func (Token) GoString() string { return redacted }

// Format prints [redacted] for every verb and flag.
func (Token) Format(f fmt.State, _ rune) { _, _ = io.WriteString(f, redacted) }

// LogValue makes slog print [redacted].
func (Token) LogValue() slog.Value { return slog.StringValue(redacted) }

// MarshalJSON writes "[redacted]".
func (Token) MarshalJSON() ([]byte, error) { return []byte(`"` + redacted + `"`), nil }

// MarshalText writes [redacted].
func (Token) MarshalText() ([]byte, error) { return []byte(redacted), nil }

// empty reports whether the token holds no value (the zero Token).
func (t Token) empty() bool { return t.value == "" }

// apiTarget is the scheme, canonical host and port of import.discord.api_base.
type apiTarget struct {
	scheme string
	host   string
	port   string
}

// targetOf returns the scheme, canonical host (lower case, without a trailing dot) and port
// (the default port of the scheme when the URL names none) of u.
func targetOf(u *url.URL) apiTarget {
	scheme := strings.ToLower(u.Scheme)
	port := u.Port()
	if port == "" && scheme == schemeHTTPS {
		port = "443"
	}
	return apiTarget{scheme: scheme, host: strings.TrimSuffix(strings.ToLower(u.Hostname()), "."), port: port}
}

// setAuthorization sets "Authorization: Bot <token>" on req after checking that the request
// goes to api, the target of import.discord.api_base, over https; otherwise it sets nothing and
// returns ErrTokenHostMismatch (22 4.1: the token goes only to the API base). It is the only
// place that reads the value.
func (t Token) setAuthorization(req *http.Request, api apiTarget) error {
	if req.URL == nil || req.URL.User != nil || targetOf(req.URL) != api || api.scheme != schemeHTTPS {
		return ErrTokenHostMismatch
	}
	req.Header.Set("Authorization", "Bot "+t.value)
	return nil
}
