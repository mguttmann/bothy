// SPDX-License-Identifier: Apache-2.0

package confirm

import (
	"context"
	"errors"
	"regexp"
	"strconv"
	"strings"

	"github.com/OWNER/bothy/services/host/internal/hostos"
)

// errNoConsole is returned when nobody owns the active console session.
var errNoConsole = errors.New("confirm: no console owner")

// sessionIDPattern is the form of a logind session ID; anything else is not passed on as an
// argument.
var sessionIDPattern = regexp.MustCompile(`^[A-Za-z0-9_]+$`)

// loginctlOwner returns the uid of the user of the active session on seat0: loginctl at the
// absolute path bin, run through hostos.Runner with an empty environment.
func loginctlOwner(ctx context.Context, run hostos.Runner, bin string) (string, error) {
	session, err := loginctlProperty(ctx, run, bin, []string{"show-seat", "seat0", "-p", "ActiveSession"}, "ActiveSession")
	if err != nil {
		return "", err
	}
	if !sessionIDPattern.MatchString(session) {
		return "", errNoConsole
	}
	user, err := loginctlProperty(ctx, run, bin, []string{"show-session", session, "-p", "User"}, "User")
	if err != nil {
		return "", err
	}
	if _, err := strconv.ParseUint(user, 10, 32); err != nil {
		return "", errNoConsole
	}
	return user, nil
}

// loginctlProperty runs loginctl with args and returns the value of the line "key=value".
func loginctlProperty(ctx context.Context, run hostos.Runner, bin string, args []string, key string) (string, error) {
	out, err := run.Run(ctx, hostos.Cmd{Path: bin, Args: args})
	if err != nil {
		return "", err
	}
	if out.TimedOut || out.ExitCode != 0 {
		return "", errNoConsole
	}
	for line := range strings.SplitSeq(string(out.Stdout), "\n") {
		if v, ok := strings.CutPrefix(strings.TrimSpace(line), key+"="); ok && v != "" {
			return v, nil
		}
	}
	return "", errNoConsole
}
