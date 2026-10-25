// SPDX-License-Identifier: Apache-2.0

package files

import "context"

// StorageProbe checks the object storage for the status page (health_source files.storage of
// docs/spec/16-operations.md 12, M7-09): it writes, reads and deletes one probe object. It is safe
// for concurrent use.
type StorageProbe interface {
	// ProbeStorage writes the probe object of this process, reads it back, compares the bytes and
	// deletes it. Errors: a storage failure or bytes that differ; the error never contains object
	// bytes.
	//
	// Example:
	//
	//	if err := probe.ProbeStorage(ctx); err != nil { /* no answer */ }
	ProbeStorage(ctx context.Context) error
}
