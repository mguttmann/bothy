// SPDX-License-Identifier: Apache-2.0

package profiles

// This file holds the login shell of Linux and macOS. It carries no build constraint, so its
// rules run in the tests of every build host; the GOOS field of Env selects it.

import (
	"path/filepath"
	"slices"
	"strings"

	"github.com/OWNER/bothy/services/host/internal/hostos"
)

// loginShell returns program and argv[0] of the login shell of e.Self (Design note 9): on Linux
// and macOS an absolute shell that resolves as the session user and is listed in e.Shells, started
// with argv[0] "-" plus its base name, the login(1) convention every POSIX shell, fish and tcsh
// read as "login shell"; on Windows PowerShell of %SystemRoot%. Anything else gives a
// *StartError with DetailPTYFailed for op.
func loginShell(e Env, op string) (Spec, error) {
	if e.GOOS == hostos.Windows {
		return powershell(e, op)
	}
	sh := e.Self.Shell
	if !isAbs(e.GOOS, sh) || !listed(e.Shells, sh) {
		return Spec{}, refuse(op, DetailPTYFailed)
	}
	if _, err := e.Resolver.Resolve(sh); err != nil {
		return Spec{}, refuse(op, DetailPTYFailed)
	}
	return Spec{Path: sh, Argv0: "-" + filepath.Base(sh)}, nil
}

// listed reports whether sh is a line of /etc/shells; blanks around a line and comment lines are
// ignored.
func listed(shells []string, sh string) bool {
	return slices.ContainsFunc(shells, func(line string) bool {
		line = strings.TrimSpace(line)
		return !strings.HasPrefix(line, "#") && line == sh
	})
}

// PersonalCommand returns the arguments after argv[0] with which the login shell of kind (the base
// name of the shell, "powershell" on Windows) starts the holder's own claude for the profile
// claude with personal: it sets CLAUDE_CONFIG_DIR to configDir when set is true, removes it
// otherwise, and then runs claude from the shell's own PATH. configDir is quoted for the shell, so
// it reaches claude unchanged whatever it holds. sh, bash, zsh, dash, ksh and an unknown kind get
// the POSIX form, fish and tcsh or csh their own.
func PersonalCommand(kind, configDir string, set bool) []string {
	switch kind {
	case "powershell", "powershell.exe", "pwsh", "pwsh.exe":
		return personalPowerShell(configDir, set)
	case "fish":
		if !set {
			return []string{"-c", "set -e CLAUDE_CONFIG_DIR; exec claude"}
		}
		return []string{"-c", "set -gx CLAUDE_CONFIG_DIR " + quoteFish(configDir) + "; exec claude"}
	case "tcsh", "csh":
		if !set {
			return []string{"-c", "unsetenv CLAUDE_CONFIG_DIR; exec claude"}
		}
		return []string{"-c", "setenv CLAUDE_CONFIG_DIR " + quoteSh(configDir) + "; exec claude"}
	}
	if !set {
		return []string{"-c", "unset CLAUDE_CONFIG_DIR; exec claude"}
	}
	return []string{"-c", "export CLAUDE_CONFIG_DIR=" + quoteSh(configDir) + "; exec claude"}
}

// quoteSh quotes s for sh and csh: inside single quotes nothing is special, and a single quote
// itself is closed, escaped and reopened.
func quoteSh(s string) string { return "'" + strings.ReplaceAll(s, "'", `'\''`) + "'" }

// quoteFish quotes s for fish: inside single quotes only the backslash and the single quote are
// special and are escaped with a backslash.
func quoteFish(s string) string {
	return "'" + strings.NewReplacer(`\`, `\\`, `'`, `\'`).Replace(s) + "'"
}
