// SPDX-License-Identifier: Apache-2.0

//go:build !unix

package executor

import (
	"context"

	"github.com/OWNER/bothy/services/server/internal/status/app"
)

// VolumeUsage implements app.Executor; without statfs it always fails with volume_usage_failed
// (bothy-backupd runs on Linux only).
func (e *Executor) VolumeUsage(context.Context, string) (app.VolumeUsage, error) {
	return app.VolumeUsage{}, execError(codeVolumeUsageFailed)
}
