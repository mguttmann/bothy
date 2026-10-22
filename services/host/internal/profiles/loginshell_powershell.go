// SPDX-License-Identifier: Apache-2.0

package profiles

// This file holds the login shell of Windows. It is not named *_windows.go on purpose: that
// suffix would build it only on Windows, and its rules run in the tests of every build host.

import (
	"strings"

	"github.com/OWNER/bothy/services/host/internal/hostos"
)

// powershellNoLogo suppresses the banner of PowerShell.
const powershellNoLogo = "-NoLogo"

// powershell returns %SystemRoot%\System32\WindowsPowerShell\v1.0\powershell.exe, present on
// every Windows 10 and 11, as the login shell; an empty or relative SystemRoot gives a *StartError
// with DetailPTYFailed for op.
func powershell(e Env, op string) (Spec, error) {
	if !isAbs(hostos.Windows, e.SystemRoot) {
		return Spec{}, refuse(op, DetailPTYFailed)
	}
	return Spec{Path: joinPath(hostos.Windows, e.SystemRoot, "System32", "WindowsPowerShell", "v1.0", "powershell.exe"), Argv0: "powershell.exe"}, nil
}

// personalPowerShell is PersonalCommand for PowerShell.
func personalPowerShell(configDir string, set bool) []string {
	cmd := `Remove-Item Env:\CLAUDE_CONFIG_DIR -ErrorAction SilentlyContinue; & claude`
	if set {
		cmd = "$env:CLAUDE_CONFIG_DIR = " + quotePowerShell(configDir) + "; & claude"
	}
	return []string{powershellNoLogo, "-Command", cmd}
}

// quotePowerShell quotes s as a verbatim string of PowerShell: only single quotes are special,
// and PowerShell counts the typographic ones (U+2018 to U+201B) as single quotes too; each is
// doubled.
func quotePowerShell(s string) string {
	return "'" + strings.NewReplacer("'", "''", "‘", "‘‘", "’", "’’", "‚", "‚‚", "‛", "‛‛").Replace(s) + "'"
}
