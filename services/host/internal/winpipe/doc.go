// SPDX-License-Identifier: Apache-2.0

// Package winpipe is the named pipe of Windows that the local channel (internal/localchan) and the
// confirmation endpoint (internal/confirm) both serve: instances created with overlapped handles,
// an Accept that Close can end, and a dial that waits while every instance is busy. Both ends are
// overlapped, so the runtime poller serves them: one end reads and writes at once, and Close ends
// a pending read. On a synchronous handle Windows runs the I/O of one file object one after the
// other, so a write waited behind a read of the same handle (docs/decisions.d/V1-13.md, V1-13/9).
// The security descriptor of a pipe is the caller's. This file is untagged; the code is Windows
// only.
package winpipe
