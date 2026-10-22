// SPDX-License-Identifier: Apache-2.0

package profiles

import (
	"errors"
	"testing"

	"github.com/OWNER/bothy/services/host/internal/hostos"
)

// configDirResolver resolves every path to itself except those in fail, which are unresolvable.
type configDirResolver struct{ fail map[string]bool }

func (r configDirResolver) Resolve(p string) (hostos.Resolved, error) {
	if r.fail[p] {
		return hostos.Resolved{}, hostos.ErrUnresolvable
	}
	return hostos.Resolved{Final: p}, nil
}

func (configDirResolver) Identity(string) (string, error) { return "", errors.New("no identity") }

// sharedLocation reports whether err is the refusal of the profile claude with shared_location.
func sharedLocation(err error) bool {
	var se *StartError
	return errors.As(err, &se) && se.Op == "claude" && se.Detail == DetailSharedLocation
}

// TestConfigDirUnresolvableHome (TM-T29 (k)): when the home of the session user does not resolve,
// no CLAUDE_CONFIG_DIR can be shown to lie inside it, so both kinds refuse with shared_location;
// with the home resolvable the same folders pass.
func TestConfigDirUnresolvableHome(t *testing.T) {
	const home = "/home/bothy-s-alex"
	env := func(fail ...string) Env {
		r := configDirResolver{fail: map[string]bool{}}
		for _, p := range fail {
			r.fail[p] = true
		}
		return Env{GOOS: hostos.Linux, Self: hostos.Account{Name: "bothy-s-alex", Home: home}, Resolver: r}
	}
	if dir, err := orgConfigDir(env()); err != nil || dir == "" {
		t.Errorf("organization with a resolvable home: %q %v", dir, err)
	}
	if err := personalConfigDir(env(), home+"/cfg"); err != nil {
		t.Errorf("personal with a resolvable home: %v", err)
	}
	if dir, err := orgConfigDir(env(home)); !sharedLocation(err) || dir != "" {
		t.Errorf("organization with an unresolvable home: %q %v", dir, err)
	}
	if err := personalConfigDir(env(home), home+"/cfg"); !sharedLocation(err) {
		t.Errorf("personal with an unresolvable home: %v", err)
	}
}
