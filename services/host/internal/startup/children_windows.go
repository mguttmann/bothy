// SPDX-License-Identifier: Apache-2.0

package startup

import (
	"context"
	"os/exec"

	"github.com/OWNER/bothy/services/host/internal/hostos"
)

// ChildHelpers does not exist on Windows: the configuration refuses host_service.owner_account
// there (ADR 0073, D-OA-3). The type keeps the commands buildable.
type ChildHelpers struct {
	GOOS    string
	Dir     string
	Command func(ctx context.Context, user string) *exec.Cmd
}

// Endpoint returns the pipe name of the regular helper; nothing is started.
func (k *ChildHelpers) Endpoint(_, user string) string {
	return `\\.\pipe\bothy-host-helper-` + user
}

// MainPID implements hostos.ServiceManager: never running.
func (k *ChildHelpers) MainPID(context.Context, string) (int, error) { return 0, hostos.ErrNotRunning }

// Close does nothing.
func (k *ChildHelpers) Close() {}
