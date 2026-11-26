// SPDX-License-Identifier: Apache-2.0

package computer

import (
	"context"
	"errors"
	"path/filepath"
	"slices"
	"strings"

	"github.com/OWNER/bothy/services/host/internal/confirm"
	"github.com/OWNER/bothy/services/host/internal/hostos"
)

// ErrPeerRefused is a helper connection that the peer rule of docs/spec/27-computer-use.md 9.4
// refuses; the connection is closed and logged with computer_peer_refused.
var ErrPeerRefused = errors.New("computer: helper peer refused")

// Log codes of the helper channel: a refused peer (CU-T23, with the key reason), a helper that
// passed the rule and sent hello, and the end of that connection.
const (
	LogPeerRefused     = "computer_peer_refused"
	LogHelperConnected = "computer_helper_connected"
	LogHelperGone      = "computer_helper_gone"
	attrRefusal        = "reason"
	attrEnabled        = "enabled"
	attrHelperVersion  = "helper_version"
)

// The reasons of a refused helper peer, logged with LogPeerRefused under the key reason, so a
// refusal on a machine can be told apart without naming a user, path or process.
const (
	// ReasonNoPeer: the user or process ID of the connection could not be read.
	ReasonNoPeer = "no_peer"
	// ReasonNoBinding: the peer process could not be bound to the connection (Windows: the
	// service account may not open the client process; the helper grants it, D-CU-03-12).
	ReasonNoBinding = "no_binding"
	// ReasonConsoleUser: the peer is not the user of the active console session (or is the service
	// user or a session user).
	ReasonConsoleUser = "console_user"
	// ReasonProcessChanged: the process behind the connection is no longer the captured one.
	ReasonProcessChanged = "process_changed"
	// ReasonExeUnavailable: the executable of the peer process could not be read.
	ReasonExeUnavailable = "exe_unavailable"
	// ReasonPathMismatch: the peer runs another executable than the installed helper.
	ReasonPathMismatch = "path_mismatch"
	// ReasonPathUntrusted: the helper's file or folder chain fails the rule of HD10.
	ReasonPathUntrusted = "path_untrusted"
	// ReasonSignature: the helper does not satisfy the code signing requirement of the build.
	ReasonSignature = "signature"
	// ReasonTestPeer: on Linux the peer is not host_service.computer_test_peer.
	ReasonTestPeer = "test_peer"
	// ReasonNoHello: the peer passed the rule but sent no valid hello.
	ReasonNoHello = "no_hello"
)

// RefusalError is the error of a refused peer: errors.Is(err, ErrPeerRefused) holds, and Reason
// is one of the Reason constants.
type RefusalError struct{ Reason string }

// Error names the reason.
func (e *RefusalError) Error() string { return ErrPeerRefused.Error() + ": " + e.Reason }

// Is makes errors.Is(err, ErrPeerRefused) hold.
func (e *RefusalError) Is(target error) bool { return target == ErrPeerRefused }

func refuse(reason string) error { return &RefusalError{Reason: reason} }

// RefusalReason returns the reason of err, ReasonNoPeer for any other error.
func RefusalReason(err error) string {
	var r *RefusalError
	if errors.As(err, &r) {
		return r.Reason
	}
	return ReasonNoPeer
}

// Process is the peer process of one helper connection as the connection captured it at connect
// time (macOS the audit token with its PID version, Windows a handle of the pipe client opened at
// the accept). It never names another process: an exit, a reuse of the PID or an exec of another
// image after the connect makes Current fail (D-CU-03-12).
type Process interface {
	// Current returns nil while the process behind the connection is still the captured one.
	Current() error
	// ExePath returns the executable of the captured process.
	ExePath() (string, error)
	// Close releases what the binding holds.
	Close() error
}

// PeerRule decides which process may act as the computer helper (docs/spec/27-computer-use.md 9.4,
// D-CU-01-24): the user of the active console session (the rule of the confirmation agent, which
// already refuses the service user and every configured or former session user), the executable of
// the process bound to the connection at the installed helper path, a file and folder chain of a
// trusted owner (the rule of HD10), and the code signing requirement of the build when it carries
// one. On Linux the helper does not exist: only the user named by host_service.computer_test_peer
// passes there (the uid SO_PEERCRED took at connect), and the path and signature parts do not
// apply. Every missing function and every lookup error refuses.
type PeerRule struct {
	// GOOS is the platform the rule runs on.
	GOOS string
	// Console is the console-user rule of internal/confirm.
	Console confirm.Rule
	// TestPeerUID is the uid of host_service.computer_test_peer (Linux only); empty admits nobody.
	TestPeerUID string
	// HelperPath is the installed path of the helper executable.
	HelperPath string
	// Trusted applies the rule of a place for all (HD10) to a path.
	Trusted func(path string) bool
	// Requirement is the code signing requirement of this build (buildinfo); empty means a
	// development build without a signature check.
	Requirement string
	// VerifySignature checks that the executable at path satisfies requirement.
	VerifySignature func(ctx context.Context, path, requirement string) error
}

// Check returns nil when the peer p, whose process proc the connection captured, may act as the
// helper, else ErrPeerRefused. The process must be the captured one before the executable is read
// and again after the signature check, so neither a reused PID nor an exec of another image after
// the connect passes. It runs at the connection, after hello and before every session. The error
// names no user, path or process.
func (r PeerRule) Check(ctx context.Context, p confirm.Peer, proc Process) error {
	if r.GOOS == hostos.Linux {
		if r.TestPeerUID == "" || p.UID != r.TestPeerUID || p.UID == r.Console.ServiceUID {
			return refuse(ReasonTestPeer)
		}
		return nil
	}
	switch {
	case proc == nil || r.Trusted == nil || r.HelperPath == "" || p.PID <= 0:
		return refuse(ReasonNoPeer)
	case !r.Console.Allow(ctx, p):
		return refuse(ReasonConsoleUser)
	case proc.Current() != nil:
		return refuse(ReasonProcessChanged)
	}
	exe, err := proc.ExePath()
	switch {
	case err != nil:
		return refuse(ReasonExeUnavailable)
	case !samePath(r.GOOS, exe, r.HelperPath):
		return refuse(ReasonPathMismatch)
	case !r.Trusted(exe):
		return refuse(ReasonPathUntrusted)
	}
	if r.Requirement != "" && (r.VerifySignature == nil || r.VerifySignature(ctx, exe, r.Requirement) != nil) {
		return refuse(ReasonSignature)
	}
	if proc.Current() != nil {
		return refuse(ReasonProcessChanged)
	}
	return nil
}

// samePath compares two absolute paths after cleaning; Windows compares without case.
func samePath(goos, a, b string) bool {
	if a == "" || b == "" {
		return false
	}
	if goos == hostos.Windows {
		return strings.EqualFold(filepath.Clean(a), filepath.Clean(b))
	}
	return filepath.Clean(a) == filepath.Clean(b)
}

// TrustedPath returns the rule of a place for all (HD10, docs/spec/24-terminal.md 2.2 item 6) for
// the helper executable: after resolution the file, every folder on the way, every component as
// written and every link with its folder belong to a trusted owner of goos and are writable by
// none of writers (the session users and everyone). An unresolvable path is not trusted.
func TrustedPath(goos string, res hostos.Resolver, sec hostos.Security, writers func() []hostos.Subject) func(string) bool {
	return func(p string) bool {
		if res == nil || sec == nil {
			return false
		}
		r, err := res.Resolve(p)
		if err != nil {
			return false
		}
		objects := slices.Concat(r.Chain, r.Components, []string{r.Final})
		for _, l := range r.Links {
			objects = append(objects, l.Path, l.Dir)
		}
		var subjects []hostos.Subject
		if writers != nil {
			subjects = writers()
		}
		return !slices.ContainsFunc(objects, func(o string) bool { return !trustedObject(goos, sec, o, subjects) })
	}
}

// trustedObject reports whether o belongs to a trusted owner and is writable by none of subjects.
func trustedObject(goos string, sec hostos.Security, o string, subjects []hostos.Subject) bool {
	fs, err := sec.Lstat(o)
	if err != nil || !hostos.TrustedOwner(goos, fs) {
		return false
	}
	return !slices.ContainsFunc(subjects, func(s hostos.Subject) bool { return hostos.Writable(fs, s) })
}
