// SPDX-License-Identifier: Apache-2.0

package helper

import (
	"context"
	"errors"
	"fmt"
	"os"
	"time"

	"golang.org/x/sys/windows"
	"golang.org/x/sys/windows/registry"

	"github.com/OWNER/bothy/services/host/internal/hostos"
)

// LoginEnv returns the probe of HD6 on Windows: whether the environment of the user in its
// registry hive sets CLAUDE_CODE_OAUTH_TOKEN (the value is never read) and the folder in
// CLAUDE_CONFIG_DIR.
func LoginEnv(_ hostos.Runner, _ hostos.Account) func(context.Context, time.Duration) (bool, string, error) {
	return func(context.Context, time.Duration) (bool, string, error) {
		k, err := registry.OpenKey(registry.CURRENT_USER, "Environment", registry.QUERY_VALUE)
		if errors.Is(err, registry.ErrNotExist) {
			return false, "", nil
		}
		if err != nil {
			return false, "", err
		}
		defer func() { _ = k.Close() }()
		names, err := k.ReadValueNames(-1)
		if err != nil {
			return false, "", err
		}
		token := false
		for _, n := range names {
			if n == "CLAUDE_CODE_OAUTH_TOKEN" {
				token = true
			}
		}
		dir, _, err := k.GetStringValue("CLAUDE_CONFIG_DIR")
		if err != nil && !errors.Is(err, registry.ErrNotExist) {
			return false, "", err
		}
		return token, dir, nil
	}
}

// MkdirPrivate creates path and gives it a protected ACL for the calling user, SYSTEM and
// Administrators only.
func MkdirPrivate(path string) error {
	if err := os.MkdirAll(path, 0o700); err != nil {
		return err
	}
	tok := windows.GetCurrentProcessToken()
	u, err := tok.GetTokenUser()
	if err != nil {
		return err
	}
	sd, err := windows.SecurityDescriptorFromString(fmt.Sprintf("D:P(A;OICI;FA;;;%s)(A;OICI;FA;;;SY)(A;OICI;FA;;;BA)", u.User.Sid.String()))
	if err != nil {
		return err
	}
	dacl, _, err := sd.DACL()
	if err != nil {
		return err
	}
	return windows.SetNamedSecurityInfo(path, windows.SE_FILE_OBJECT, windows.DACL_SECURITY_INFORMATION|windows.PROTECTED_DACL_SECURITY_INFORMATION, nil, nil, dacl, nil)
}
