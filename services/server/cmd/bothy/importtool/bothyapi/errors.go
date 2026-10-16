// SPDX-License-Identifier: Apache-2.0

package bothyapi

import (
	"errors"
	"strconv"
)

// The sentinel errors of the package; callers test them with errors.Is. Their texts never carry
// a token, a key or the content of the session file.
var (
	// ErrNotLoggedIn means there is no session file: run bothy import login (exit 2).
	ErrNotLoggedIn = errors.New("bothyapi: not logged in, run bothy import login")
	// ErrAlreadyLoggedIn means a valid session for the same server exists: run bothy import
	// logout first (exit 2).
	ErrAlreadyLoggedIn = errors.New("bothyapi: already logged in to this server, run bothy import logout first")
	// ErrSessionRevoked means the server ended the session (a refresh was refused): run bothy
	// import login again (exit 2).
	ErrSessionRevoked = errors.New("bothyapi: session revoked, run bothy import login")
	// ErrInsecureSessionFile means the session file or its directory can be read or written by
	// group or others, or is not a regular file or directory.
	ErrInsecureSessionFile = errors.New("bothyapi: the session file or its directory is accessible to group or others")
	// ErrInvalidSessionFile means the session file is not a session of this tool.
	ErrInvalidSessionFile = errors.New("bothyapi: the session file is not valid")
	// ErrInvalidServerURL means the server URL is not an https origin, or an http origin of a
	// loopback host.
	ErrInvalidServerURL = errors.New("bothyapi: the server URL must be an https origin without path, query or user info (http only for a loopback host)")
	// ErrPairingExpired means the pairing code expired before the owner approved it.
	ErrPairingExpired = errors.New("bothyapi: the pairing code expired")
	// ErrPairingGone means the server ended the pairing (410: expired or rejected).
	ErrPairingGone = errors.New("bothyapi: the pairing was rejected or has expired")
	// ErrInterrupted means the login was interrupted (Ctrl-C); nothing was written.
	ErrInterrupted = errors.New("bothyapi: interrupted, nothing written")
	// ErrRateLimited means the server answered 429 more often than import.discord.max_429_retries
	// allows.
	ErrRateLimited = errors.New("bothyapi: rate limited by the server")
	// ErrRateWaitTooLong means a Retry-After of the server exceeds import.discord.max_rate_wait.
	ErrRateWaitTooLong = errors.New("bothyapi: the server asks for a wait above import.discord.max_rate_wait")
	// ErrRedirect means the server answered with a redirect, which the tool never follows.
	ErrRedirect = errors.New("bothyapi: the server answered with a redirect, which is not followed")
	// ErrUnexpectedStatus means a call ended with a status its caller does not handle.
	ErrUnexpectedStatus = errors.New("bothyapi: unexpected status")
)

// StatusError is a call that ended with an HTTP status the caller does not handle. It names the
// operation, the status and the problem code, never a body or a header.
type StatusError struct {
	// Op is the operationId of the call.
	Op string
	// Status is the HTTP status.
	Status int
	// Code is the problem code of the answer, "" without one.
	Code string
}

// Error returns "bothyapi: <op>: status <n> [<code>]".
func (e *StatusError) Error() string {
	s := "bothyapi: " + e.Op + ": status " + strconv.Itoa(e.Status)
	if e.Code != "" {
		s += " " + e.Code
	}
	return s
}

// Is reports ErrUnexpectedStatus.
func (e *StatusError) Is(target error) bool { return target == ErrUnexpectedStatus }
