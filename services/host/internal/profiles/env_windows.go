// SPDX-License-Identifier: Apache-2.0

package profiles

import "golang.org/x/sys/windows"

// TokenEnvironment returns the environment block of the calling process's token without
// inheritance (CreateEnvironmentBlock with inheritExisting false, then DestroyEnvironmentBlock,
// through windows.Token.Environ): the user and system variables of the session user, the
// equivalent of a login, without anything of the helper's own environment. The helper sets
// Env.EnvBlock from it; an error means the profile cannot start.
func TokenEnvironment() ([]string, error) {
	return windows.GetCurrentProcessToken().Environ(false)
}
