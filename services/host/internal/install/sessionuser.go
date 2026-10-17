// SPDX-License-Identifier: Apache-2.0

package install

import (
	"errors"
	"fmt"
	"path/filepath"
	"regexp"
	"slices"
	"strings"

	"github.com/OWNER/bothy/services/host/internal/hostos"
)

// BoundError refuses a name the state directory already binds to a person (D-W-67, T12).
type BoundError struct{ Name string }

func (e *BoundError) Error() string {
	return "bothy-host session-user add: " + e.Name + " is bound to a person on this host; create a new name"
}

// Errors of session-user add and remove.
var (
	ErrConfigured    = errors.New("session user already configured")
	ErrNotConfigured = errors.New("session user not configured")
	ErrInvalidName   = errors.New("session user name does not match ^[A-Za-z0-9_][A-Za-z0-9_.-]*$")
)

var namePattern = regexp.MustCompile(`^[A-Za-z0-9_][A-Za-z0-9_.-]*$`)

// SessionUserAdd returns the plan of "session-user add <name>": refuse a name bound in
// bindings.json or already configured; create user, home only for it and the helper service, add
// the name to host_service.session_users and restart the host service.
func SessionUserAdd(t Target, name string, configured []string, bindings map[string]string) ([]Step, error) {
	if !namePattern.MatchString(name) {
		return nil, ErrInvalidName
	}
	if _, bound := bindings[name]; bound {
		return nil, &BoundError{Name: name}
	}
	if slices.ContainsFunc(configured, func(c string) bool { return strings.EqualFold(c, name) }) {
		return nil, fmt.Errorf("bothy-host session-user add: %s: %w", name, ErrConfigured)
	}
	var steps []Step
	switch t.GOOS {
	case hostos.Linux:
		steps = append(linuxHelper(t, name), run("/usr/bin/systemctl", "daemon-reload"), run("/usr/bin/systemctl", "enable", "--now", "bothy-host-helper-"+name+".socket"))
	case hostos.Darwin:
		svc, ids, err := darwinIDs(t, []string{name})
		if err != nil {
			return nil, err
		}
		label := hostos.HelperServiceName(hostos.Darwin, name)
		steps = append(darwinHelper(t, name, ids[name], svc), load(label, filepath.Join(LaunchDaemonsDir, label+".plist"))...)
	case hostos.Windows:
		steps = windowsHelper(t, name)
	default:
		return nil, ErrUnsupported
	}
	steps = append(steps, Step{Kind: KindConfigAdd, Path: t.ConfigPath, Name: name})
	restart, _ := Restart(t)
	return append(steps, restart...), nil
}

// closedHomeSDDL is the ACL of a closed home on Windows: the user and the trusted owners.
const closedHomeSDDL = "D:P(A;OICI;FA;;;SY)(A;OICI;FA;;;BA)(A;OICI;FA;;;<user-sid>)"

// SessionUserRemove returns the plan of "session-user remove <name>": remove the name from
// host_service.session_users and host_service.claude_workdirs, remove the helper service, close
// the home (mode 0700, on Windows an ACL only for the user and the trusted owners) and restart.
// The binding stays, and no process of the user is ended.
func SessionUserRemove(t Target, name string, configured []string) ([]Step, error) {
	if !slices.Contains(configured, name) {
		return nil, fmt.Errorf("bothy-host session-user remove: %s: %w", name, ErrNotConfigured)
	}
	steps := []Step{{Kind: KindConfigRemove, Path: t.ConfigPath, Name: name}}
	switch t.GOOS {
	case hostos.Linux:
		socket := "bothy-host-helper-" + name + ".socket"
		service := hostos.HelperServiceName(hostos.Linux, name)
		steps = append(steps,
			run("/usr/bin/systemctl", "disable", "--now", socket, service),
			Step{Kind: KindRemove, Path: filepath.Join(SystemdDir, socket)},
			Step{Kind: KindRemove, Path: filepath.Join(SystemdDir, service)},
			run("/usr/bin/systemctl", "daemon-reload"),
			Step{Kind: KindChmod, Path: "~" + name, Mode: 0o700, Owner: name})
	case hostos.Darwin:
		label := hostos.HelperServiceName(hostos.Darwin, name)
		steps = append(steps,
			run("/bin/launchctl", "bootout", "system/"+label),
			Step{Kind: KindRemove, Path: filepath.Join(LaunchDaemonsDir, label+".plist")},
			Step{Kind: KindChmod, Path: "~" + name, Mode: 0o700, Owner: name})
	case hostos.Windows:
		steps = append(steps,
			Step{Kind: KindWinServiceDel, Name: hostos.HelperServiceName(hostos.Windows, name)},
			Step{Kind: KindWinACL, Path: "~" + name, Name: name, Content: closedHomeSDDL})
	default:
		return nil, ErrUnsupported
	}
	restart, _ := Restart(t)
	return append(steps, restart...), nil
}

// OperatorStepsAfterRemove is the text session-user remove prints (16 4.4 point 1): delete the
// user with its long runners now, because one of them can reopen the home and stop the host
// through HD9.
func OperatorStepsAfterRemove(name string) string {
	return "session-user remove: " + name + " is no longer a session user; its binding stays in the state directory.\n" +
		"Next steps for the operator (docs/spec/16-operations.md 4.4 point 1):\n" +
		"  1. remove the long runners of " + name + " (crontab entries, scheduled tasks, user services) and end its processes;\n" +
		"  2. delete the account " + name + " with its home;\n" +
		"  3. never give the name " + name + " to another person; create a new name with bothy-host session-user add.\n"
}
