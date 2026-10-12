// SPDX-License-Identifier: Apache-2.0

package wire

import (
	"errors"
	"path/filepath"

	filesfs "github.com/OWNER/bothy/services/server/internal/files/adapters/fs"
	filesapp "github.com/OWNER/bothy/services/server/internal/files/app"
	"github.com/OWNER/bothy/services/server/internal/platform/config"
)

// filesStorageValues are the keys of the object storage as config.Bind fills them.
type filesStorageValues struct {
	Driver string `config:"file.storage.driver"`
	FSRoot string `config:"file.fs.root"`
}

// errS3NotSupported is the answer for file.storage.driver = s3 until the S3 driver of M3-02
// replaces this branch (docs/DECISIONS.md M3-01/13).
var errS3NotSupported = errors.New("config: file.storage.driver: driver s3 is not supported by this server version")

// FilesObjectStore opens the object storage of the files module from the loaded values: the
// driver fs gives the file system driver on file.fs.root (fs.Open checks the root). No role calls
// it yet: M3-03 opens the store in the roles together with the directory in the server image,
// because the image has no writable file.fs.root so far and opening it at start would stop the
// stack.
//
// The error reports the driver s3, which this version does not build, a schema without the keys
// (config.Bind), or a root that fs.Open refuses; it names keys, never values. A store of the
// driver fs holds a directory handle and implements io.Closer.
func FilesObjectStore(v *config.Values) (filesapp.ObjectStore, error) {
	var raw filesStorageValues
	if err := config.Bind(v, &raw); err != nil {
		return nil, err
	}
	switch raw.Driver {
	case "fs":
		store, err := filesfs.Open(filesfs.Config{Root: raw.FSRoot})
		if err != nil {
			return nil, err // never a typed nil in the interface
		}
		return store, nil
	default:
		return nil, errS3NotSupported
	}
}

// fileFSRootAbsolute is the cross-check file_fs_root_absolute (docs/DECISIONS.md M3-01/12): with
// the driver fs, file.fs.root is absolute and clean, because a relative root would depend on the
// working directory of the process. With s3 the root is unused and not checked.
func fileFSRootAbsolute(v *config.Values) error {
	if v.String("file.storage.driver") != "fs" {
		return nil
	}
	root := v.String("file.fs.root")
	if !filepath.IsAbs(root) || filepath.Clean(root) != root {
		return errors.New("file.fs.root must be an absolute path without . or .. elements and without a trailing slash when file.storage.driver is fs")
	}
	return nil
}
