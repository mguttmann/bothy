// SPDX-License-Identifier: Apache-2.0

package discord

import (
	"errors"
	"net/http"
	"strconv"
)

// The kinds of errors the client returns; test them with errors.Is. A *CallError carries the
// kind of a failed call.
var (
	// ErrRateWaitTooLong: a requested wait (Reset-After, retry_after or Retry-After) is longer
	// than import.discord.max_rate_wait; the call ends without waiting (22 4.3 item 3).
	ErrRateWaitTooLong = errors.New("discord: rate limit wait longer than import.discord.max_rate_wait")
	// ErrRateLimited: Discord still answered 429 after import.discord.max_429_retries retries.
	ErrRateLimited = errors.New("discord: still rate limited after import.discord.max_429_retries retries")
	// ErrUnauthorized: Discord answered 401; the token is wrong or revoked. Never retried.
	ErrUnauthorized = errors.New("discord: unauthorized")
	// ErrForbidden: Discord answered 403, or an earlier 403 ended the channel for this client
	// (22 4.3 item 4). Never retried.
	ErrForbidden = errors.New("discord: forbidden")
	// ErrNotFound: Discord answered 404.
	ErrNotFound = errors.New("discord: not found")
	// ErrServer: Discord answered 5xx. The client does not retry it; an import run ends, the
	// read mirror backs off (M8-05).
	ErrServer = errors.New("discord: server error")
	// ErrUnexpectedStatus: any other status that is not a success, a redirect included (the
	// client never follows one).
	ErrUnexpectedStatus = errors.New("discord: unexpected status")
	// ErrDecode: the body of a success is not the expected JSON.
	ErrDecode = errors.New("discord: response is not the expected JSON")
	// ErrInvalidID: an ID in a response or an argument is not a snowflake digit string (22 7.1).
	// The error names the field, never the value.
	ErrInvalidID = errors.New("discord: invalid id")
	// ErrTokenHostMismatch: the token was about to go to another host, scheme or port than
	// import.discord.api_base; the request is not sent.
	ErrTokenHostMismatch = errors.New("discord: request target is not the API base, token withheld")
)

// CallError is the error of one call. Its text is "discord: <op>: <status text>", where op is
// the method and the path template (for example "GET /channels/{channel.id}/messages"); it
// never carries a header, a body, an ID or a query. Unwrap returns Kind, one of the sentinels
// above or the transport error of platform/egress (which names scheme and host only).
type CallError struct {
	// Op is the method and the path template of the call.
	Op string
	// Status is the HTTP status of the last answer, 0 when none arrived.
	Status int
	// Kind is the sentinel or the transport error.
	Kind error
	// Hint is an optional advice for the operator, for example the missing permission.
	Hint string
}

// Error implements error.
func (e *CallError) Error() string {
	s := "discord: " + e.Op + ": "
	if e.Status != 0 {
		s += strconv.Itoa(e.Status) + " " + http.StatusText(e.Status)
		if e.Kind != nil && !isStatusKind(e.Kind) {
			s += ": " + e.Kind.Error()
		}
	} else if e.Kind != nil {
		s += e.Kind.Error()
	}
	if e.Hint != "" {
		s += " (" + e.Hint + ")"
	}
	return s
}

// Unwrap returns the kind.
func (e *CallError) Unwrap() error { return e.Kind }

// isStatusKind reports whether kind only repeats what the status text says.
func isStatusKind(kind error) bool {
	for _, k := range []error{ErrUnauthorized, ErrForbidden, ErrNotFound, ErrServer, ErrUnexpectedStatus} {
		if errors.Is(kind, k) {
			return true
		}
	}
	return false
}

// statusKind maps a status that is neither a success nor a 429 to its kind.
func statusKind(status int) error {
	switch {
	case status == http.StatusUnauthorized:
		return ErrUnauthorized
	case status == http.StatusForbidden:
		return ErrForbidden
	case status == http.StatusNotFound:
		return ErrNotFound
	case status >= http.StatusInternalServerError:
		return ErrServer
	}
	return ErrUnexpectedStatus
}
