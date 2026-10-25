// SPDX-License-Identifier: Apache-2.0

package app

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"

	"github.com/OWNER/bothy/services/server/internal/files"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
)

// probeKeyPrefix is the first segment of the probe objects: outside the file keys f/... of
// domain, so no file and no purge ever names one.
const probeKeyPrefix = "status-probe/"

// probeBody is the content of a probe object: fixed text, no data of anyone.
var probeBody = []byte("status probe\n")

// StorageCheck implements files.StorageProbe over the object store of the process. Each process
// has its own key, so two workers never delete each other's probe object; a probe whose delete
// fails leaves at most this one object, which the next round overwrites and deletes.
type StorageCheck struct {
	store ObjectStore
	key   string
}

// Compile-time check: StorageCheck is the storage probe of the port.
var _ files.StorageProbe = (*StorageCheck)(nil)

// NewStorageCheck returns the probe over store with the key of process (a fresh id per process).
// Errors: a nil store.
func NewStorageCheck(store ObjectStore, process ids.ID) (*StorageCheck, error) {
	if store == nil {
		return nil, errors.New("files: the storage probe needs the object store")
	}
	return &StorageCheck{store: store, key: probeKeyPrefix + process.String()}, nil
}

// ProbeStorage implements files.StorageProbe: Put, Get with a comparison of the bytes, Delete.
func (c *StorageCheck) ProbeStorage(ctx context.Context) error {
	if err := c.store.Put(ctx, c.key, bytes.NewReader(probeBody), int64(len(probeBody))); err != nil {
		return fmt.Errorf("files: probe put: %w", err)
	}
	rc, err := c.store.Get(ctx, c.key, nil)
	if err != nil {
		return fmt.Errorf("files: probe get: %w", err)
	}
	got, err := io.ReadAll(io.LimitReader(rc, int64(len(probeBody))+1))
	_ = rc.Close()
	if err != nil {
		return fmt.Errorf("files: probe read: %w", err)
	}
	if !bytes.Equal(got, probeBody) {
		return errors.New("files: the probe object came back different")
	}
	if err := c.store.Delete(ctx, c.key); err != nil {
		return fmt.Errorf("files: probe delete: %w", err)
	}
	return nil
}
