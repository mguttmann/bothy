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

// LogPeerRefused is the log code of a refused helper peer (CU-T23).
const LogPeerRefused = "computer_peer_refused"

// PeerRule decides which process may act as the computer helper (docs/spec/27-computer-use.md 9.4,
// D-CU-01-24): the user of the active console session (the rule of the confirmation agent, which
// already refuses the service user and every configured or former session user), the executable at
// the installed helper path, a file and folder chain of a trusted owner (the rule of HD10), and the
// code signing requirement of the build when it carries one. On Linux the helper does not exist:
// only the user named by host_service.computer_test_peer passes there, and the path and signature
// parts do not apply. Every missing function and every lookup error refuses.
type PeerRule struct {
	// GOOS is the platform the rule runs on.
	GOOS string
	// Console is the console-user rule of internal/confirm.
	Console confirm.Rule
	// TestPeerUID is the uid of host_service.computer_test_peer (Linux only); empty admits nobody.
	TestPeerUID string
	// HelperPath is the installed path of the helper executable.
	HelperPath string
	// ExePath returns the executable path of a process (macOS kern.procargs2, Windows
	// QueryFullProcessImageNameW).
	ExePath func(pid int) (string, error)
	// Trusted applies the rule of a place for all (HD10) to a path.
	Trusted func(path string) bool
	// Requirement is the code signing requirement of this build (buildinfo); empty means a
	// development build without a signature check.
	Requirement string
	// VerifySignature checks that the executable at path satisfies requirement.
	VerifySignature func(ctx context.Context, path, requirement string) error
}

// Check returns nil when p may act as the helper, else ErrPeerRefused. The error names no user,
// path or process.
func (r PeerRule) Check(ctx context.Context, p confirm.Peer) error {
	if r.GOOS == hostos.Linux {
		if r.TestPeerUID == "" || p.UID != r.TestPeerUID || p.UID == r.Console.ServiceUID {
			return ErrPeerRefused
		}
		return nil
	}
	if r.ExePath == nil || r.Trusted == nil || r.HelperPath == "" || p.PID <= 0 || !r.Console.Allow(ctx, p) {
		return ErrPeerRefused
	}
	exe, err := r.ExePath(p.PID)
	if err != nil || !samePath(r.GOOS, exe, r.HelperPath) || !r.Trusted(exe) {
		return ErrPeerRefused
	}
	if r.Requirement != "" && (r.VerifySignature == nil || r.VerifySignature(ctx, exe, r.Requirement) != nil) {
		return ErrPeerRefused
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
