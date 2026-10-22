// SPDX-License-Identifier: Apache-2.0

//go:build unix

package executor

import (
	"context"
	"syscall"

	"github.com/OWNER/bothy/services/server/internal/status/app"
)

// VolumeUsage implements app.Executor with statfs: free is what an unprivileged user may still
// write (f_bavail), size the whole file system (f_blocks), both times the block size. The error
// is volume_usage_failed, without the path.
func (e *Executor) VolumeUsage(ctx context.Context, path string) (app.VolumeUsage, error) {
	_ = ctx
	var st syscall.Statfs_t
	if err := syscall.Statfs(path, &st); err != nil {
		return app.VolumeUsage{}, execError(codeVolumeUsageFailed)
	}
	bsize := unsigned(st.Bsize)
	return app.VolumeUsage{FreeBytes: unsigned(st.Bavail) * bsize, SizeBytes: unsigned(st.Blocks) * bsize}, nil
}

// unsigned converts a field of syscall.Statfs_t, whose integer types differ per platform (the
// block size is int64 on Linux and uint32 on macOS), to uint64; a negative value is 0.
func unsigned[T ~int32 | ~int64 | ~uint32 | ~uint64](v T) uint64 {
	if v < 0 {
		return 0
	}
	return uint64(v)
}
